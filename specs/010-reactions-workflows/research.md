# Research: Message Reactions & Workflow States

**Branch**: `010-reactions-workflows` | **Date**: 2026-03-18

## Decision 1: Reaction Storage Model

**Decision**: New `message_reactions` table with (message_id, agent_name, reaction, metadata, created_at) and UNIQUE(message_id, agent_name, reaction).

**Rationale**: Reactions are a separate domain from messages. Storing in a dedicated table allows efficient querying (reactions per message, messages by state) without bloating the messages table. The UNIQUE constraint enforces one-per-type-per-agent at the DB level.

**Alternatives considered**:
- JSON array on messages table: Loses relational integrity, harder to query by state
- Separate workflow_state column: Denormalization would require sync logic; derived state is simpler

## Decision 2: Workflow State Derivation

**Decision**: Compute workflow state on read by selecting the highest-priority reaction. Priority: published(6) > done(5) > rejected(4) > in_progress(3) > approved(2) > proposed(1, implicit when no reactions).

**Rationale**: No denormalization needed. State is always consistent with reactions. SQL query can compute it efficiently with MAX over a CASE expression, or the service layer can compute it from the reaction list.

**Alternatives considered**:
- Stored state column: Requires triggers or app-level sync, risks inconsistency
- Event sourcing: Overkill for this use case

## Decision 3: Channel Workflow Columns

**Decision**: Add `auto_approve`, `stalemate_remind_after`, `stalemate_escalate_after` columns to the existing `channels` table via ALTER TABLE in migration 013.

**Rationale**: These are channel-level settings, not a separate entity. Adding columns is simpler than a join table. The channel struct in Go already exists — just add fields.

## Decision 4: StalemateWorker Extension

**Decision**: Extend the existing StalemateWorker's periodic loop to also scan channel messages for stale workflow states, using the new channel timeout settings.

**Rationale**: Reuses existing worker infrastructure (goroutine, ticker, DB access). Adding a second scan phase is cleaner than creating a separate worker.

## Decision 5: Toggle Semantics

**Decision**: Toggle is implemented as: check if (message_id, agent_name, reaction) exists → if yes, DELETE; if no, INSERT. This is done in a transaction.

**Rationale**: Simple, atomic, idempotent. No special "removed" flag needed — absence means not reacted.

## Decision 6: MCP Tool Exposure

**Decision**: Expose react/unreact/get_reactions/list_by_state as actions in the bridge (execute tool), not as top-level hybrid tools.

**Rationale**: Consistent with existing attachment tools. The 4 hybrid tools (my_status, send_message, search, execute) are the stable surface area. New actions go through the execute/bridge path.
