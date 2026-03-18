# Data Model: Embeddings Management, Message Retention & Agent Inbox

## Existing Entities (Modified)

### messages (existing table)
No schema changes. Retention operates on the existing `created_at` column.
- Retention queries: `WHERE created_at < ? AND status != 'processing'`
- Warning queries: `WHERE created_at < ? AND created_at >= ?` (11-month to 12-month window)

### embeddings (existing table)
No schema changes. Existing methods `DeleteAllEmbeddings()`, `EmbeddingCount()`, `GetEmbeddingProvider()` are sufficient for the new CLI commands.

New method needed:
- `EmbeddingStats(ctx) → (provider, count, pending, failed, dimensions)` — aggregates data from `embeddings` and `embedding_queue` tables.

### embedding_queue (existing table)
No schema changes. Existing methods `ClearQueue()`, `EnqueueAllMessages()`, `PendingCount()` are sufficient.

New method needed:
- `FailedCount(ctx) → int64` — counts items with `status = 'failed'`

### agents (existing table)
No schema changes. The `system` agent is created as a regular row with `name = 'system'`, `type = 'ai'`, `owner_id = 1` (first admin user).

### conversations (existing table)
No schema changes. Orphaned conversations (no remaining messages) are cleaned up during retention.

### attachments (existing table)
No schema changes. Attachments for deleted messages are cleaned up during retention. CAS files are only removed if no other attachment record references the same hash.

## New Entities

### RetentionConfig (in-memory, not persisted)

Configuration for the message retention system. Set at server startup from CLI flags / env vars.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| RetentionPeriod | time.Duration | 12 months (8760h) | Messages older than this are deleted |
| WarningWindow | time.Duration | 1 month (720h) | How long before deletion to send warnings |
| CleanupInterval | time.Duration | 24h | How often the cleanup job runs |
| Enabled | bool | true | false if retention period is 0 |

### RetentionState (derived, not stored)

Runtime state queried by `retention.status` admin command.

| Field | Type | Description |
|-------|------|-------------|
| Config | RetentionConfig | Current configuration |
| LastCleanupAt | *time.Time | When cleanup last ran (tracked in memory) |
| NextCleanupAt | *time.Time | When next cleanup will run |
| MessageAgeDistribution | map[string]int64 | Counts bucketed by age |

### EmbeddingStatus (derived, not stored)

Aggregated from embeddings + embedding_queue tables by `embeddings.status` admin command.

| Field | Type | Description |
|-------|------|-------------|
| Provider | string | Current embedding provider name |
| TotalEmbedded | int64 | Count of embedded messages |
| PendingCount | int64 | Queue items with status pending/processing |
| FailedCount | int64 | Queue items with status failed |
| IndexSize | int | Number of vectors in HNSW index |
| Dimensions | int | Vector dimensions (from provider) |

### MyStatusResponse (MCP tool response, not stored)

Response structure for the `my_status` MCP tool.

| Field | Type | Description |
|-------|------|-------------|
| agent | object | {name, display_name, type, owner_name} |
| direct_messages | []object | Up to 10 pending DMs, newest first |
| direct_messages_total | int | Total pending DM count |
| mentions | []object | Up to 10 recent @-mentions in channels |
| mentions_total | int | Total mention count |
| system_notifications | []object | Up to 5 system messages |
| system_notifications_total | int | Total system notification count |
| channels | []object | Joined channels with unread counts |
| stats | object | {pending_dms, channels_joined, unread_channel_messages, system_notifications} |
| truncated | bool | true if any section was capped |
| instructions | string | Guidance on how to get full data if truncated |

## State Transitions

### Message Lifecycle (updated with retention)

```
created → pending → processing → done
                  → failed

After retention period:
  any status (except processing) → WARNING_SENT → DELETED
```

### Embedding Lifecycle (updated with admin commands)

```
message created → enqueued → processing → completed (embedded)
                           → failed → requeued (up to 3 retries)

Admin reindex: all embeddings DELETED → all messages re-enqueued
Admin clear: all embeddings DELETED, queue cleared
```

## Relationships

```
messages 1──* embeddings (message_id)
messages 1──* embedding_queue (message_id)
messages 1──* attachments (message_id)
messages *──1 conversations (conversation_id)
agents   1──* messages (from_agent, to_agent)
agents   *──1 users (owner_id)
channels 1──* messages (channel_id)
channels 1──* channel_members (channel_id)
```
