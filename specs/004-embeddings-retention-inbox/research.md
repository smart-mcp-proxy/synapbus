# Research: Embeddings Management, Message Retention & Agent Inbox

## R1: SQLite Compaction Strategy

**Decision**: Use `PRAGMA auto_vacuum = INCREMENTAL` for automated cleanup and `VACUUM` for manual CLI compaction.

**Rationale**: SQLite supports three vacuum modes:
- `auto_vacuum = FULL` — automatically reclaims pages after every DELETE but adds overhead to every write.
- `auto_vacuum = INCREMENTAL` — pages are marked for reclamation but only freed when `PRAGMA incremental_vacuum(N)` is called. This allows batching the space reclamation.
- `VACUUM` — rewrites the entire database file. Slow for large DBs but guarantees maximum compaction.

For SynapBus: the DB is already created with default auto_vacuum mode. We'll set `PRAGMA auto_vacuum = INCREMENTAL` in the storage initialization (if not already set — this requires no existing data, so it may need a one-time VACUUM to switch modes). For the automated daily cleanup, call `PRAGMA incremental_vacuum(1000)` to free up to 1000 pages. For the manual `db vacuum` CLI command, run full `VACUUM`.

**Alternatives considered**:
- Full auto_vacuum: Too much per-write overhead for a messaging system with high insert rates.
- No compaction: Database file would grow monotonically. Rejected per requirements.

## R2: Cascade Deletion of Embeddings and FTS on Message Delete

**Decision**: Use explicit DELETE queries in the retention service, not SQLite CASCADE triggers.

**Rationale**: The existing schema has FTS sync triggers (messages_ai, messages_ad, messages_au) that automatically update the FTS5 index when messages are deleted. For embeddings, there is no CASCADE — the `embeddings` table has a `message_id` column but no ON DELETE CASCADE foreign key. Similarly, `embedding_queue` has no CASCADE.

The retention service will:
1. Collect message IDs to delete
2. DELETE from `embedding_queue` WHERE message_id IN (...)
3. DELETE from `embeddings` WHERE message_id IN (...)
4. DELETE from `attachments` WHERE message_id IN (...) — track hashes for CAS cleanup
5. DELETE from `messages` WHERE id IN (...) — FTS trigger handles FTS cleanup automatically
6. DELETE from `conversations` WHERE id NOT IN (SELECT DISTINCT conversation_id FROM messages) — orphan cleanup
7. Clean up attachment files from CAS for unreferenced hashes

**Alternatives considered**:
- Adding ON DELETE CASCADE to schema: Would require a migration and schema change. The explicit approach is clearer and allows batch operations.
- Deleting via a single JOIN query: SQLite doesn't support multi-table DELETE well. Explicit per-table deletes are clearer.

## R3: System Agent Implementation

**Decision**: Create a `system` agent at startup, owned by the first admin user (user ID 1). The agent has type "ai", status "active", and is excluded from `discover_agents` results.

**Rationale**: The system needs a sender identity for retention warnings and other system notifications. Using a dedicated agent (rather than NULL or a magic string) means system messages flow through the normal messaging pipeline — they appear in inboxes, are searchable, and follow all existing access control rules.

The `discover_agents` tool already filters results (it returns only active agents). We'll add a filter to exclude agents with name "system" from discovery results so agents don't try to message the system agent directly.

**Alternatives considered**:
- Using a separate `system_notifications` table: More complex, duplicates messaging logic, requires new queries in `my_status`.
- Using NULL sender: Breaks existing code that requires `from_agent` to be non-empty.

## R4: Mention Detection for my_status

**Decision**: Scan for `@agent_name` in message bodies using a simple SQL LIKE query. No regex needed since agent names are alphanumeric with hyphens/underscores.

**Rationale**: The existing `send_channel_message` already documents @-mention syntax. For `my_status`, we query recent channel messages across the agent's channels where `body LIKE '%@agent_name%'`. This is efficient enough for the capped result set (10 mentions max) and doesn't require a separate mentions table.

**Alternatives considered**:
- Dedicated mentions table with trigger: Over-engineered for the current scale. Would add write overhead to every channel message.
- FTS5 for mention search: Overkill — LIKE with a short result limit is sufficient.

## R5: Admin Socket Protocol for New Commands

**Decision**: Follow the existing admin socket JSON-RPC pattern. Add new command prefixes: `embeddings.*`, `retention.*`, `messages.purge`, `db.vacuum`.

**Rationale**: The admin socket already uses a `{"command": "...", "args": {...}}` → `{"ok": true, "data": {...}}` protocol. All existing CLI commands use this pattern via `adminRequest()`. New commands follow the same pattern exactly.

Commands:
- `embeddings.status` → returns provider, counts, index size
- `embeddings.reindex` → clears and re-queues all
- `embeddings.clear` → clears without re-queuing
- `retention.status` → returns retention config and stats
- `messages.purge` → deletes matching messages, returns count
- `db.vacuum` → runs VACUUM, returns before/after sizes

**Alternatives considered**: None — the pattern is well-established and consistent.

## R6: Retention Worker Architecture

**Decision**: Implement as a background goroutine (like the existing `RetentionCleaner` for traces) that runs on a configurable interval.

**Rationale**: The codebase already has the pattern: `trace.RetentionCleaner` runs periodically to clean old traces. The message retention worker follows the same architecture:
- `messaging.RetentionWorker` struct with `Start()` / `Stop()` methods
- Configurable interval (default 24h)
- Each tick: (1) send warnings for messages approaching retention, (2) delete expired messages, (3) run incremental vacuum

The worker needs access to: the DB, the messaging service (for sending system messages), the embedding store (for cascade cleanup), and the attachment service (for CAS cleanup).

**Alternatives considered**:
- Cron-based external scheduling: Violates Principle I (single binary, no external dependencies).
- On-demand only (CLI): Wouldn't provide automatic cleanup.
