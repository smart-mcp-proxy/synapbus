# Feature Specification: Core Messaging

**Feature Branch**: `001-core-messaging`
**Created**: 2026-03-13
**Status**: Draft
**Input**: User description: "Core messaging: send (DM or channel), read inbox, claim for processing, mark done/failed. Conversations, priority, status tracking, metadata, read/unread, SQLite storage, MCP tools."

## Constitution Compliance

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Local-First, Single Binary | Compliant | SQLite embedded storage, no external dependencies |
| II. MCP-Native | Compliant | All agent operations exposed as MCP tools only |
| III. Pure Go, Zero CGO | Compliant | Uses `modernc.org/sqlite`, no CGO |
| IV. Multi-Tenant with Ownership | Compliant | Messages scoped to agent identity; agents only read own inbox |
| VI. Semantic-Ready Storage | Compliant | FTS5 full-text search now, vector search deferred to later spec |
| VIII. Observable by Default | Compliant | All tool calls produce trace entries |
| IX. Progressive Complexity | Compliant | This spec is tier 1 (basic messaging), no advanced features required |

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Agent Sends a Direct Message and Recipient Reads It (Priority: P1)

An AI agent (e.g., a research agent) needs to send a direct message to another specific agent (e.g., an analysis agent). The recipient agent reads its inbox and sees the new message with full context: sender, subject, body, priority, and metadata. If no conversation exists between them on this subject, one is auto-created.

**Why this priority**: This is the fundamental operation of the entire system. Without send + read, no other feature has value. This is the minimal viable slice of SynapBus.

**Independent Test**: Can be fully tested by registering two agents, sending a message from one to the other via the `send_message` MCP tool, then calling `read_inbox` from the recipient. Delivers immediate value: two agents can communicate.

**Acceptance Scenarios**:

1. **Given** agent "researcher" and agent "analyzer" are registered, **When** "researcher" calls `send_message` with `to_agent: "analyzer"`, `body: "Found 3 CVEs in dependency tree"`, `subject: "Security Audit Results"`, `priority: 8`, **Then** a new conversation is created with subject "Security Audit Results", the message is stored with status "pending" and priority 8, and a success response is returned containing the message ID and conversation ID.

2. **Given** agent "analyzer" has one pending message from "researcher", **When** "analyzer" calls `read_inbox` with no filters, **Then** the response contains exactly one message with `from_agent: "researcher"`, the full body, priority 8, status "pending", and the conversation subject. The inbox_state for "analyzer" is updated to reflect the message has been read.

3. **Given** agent "researcher" has already sent a message to "analyzer" with subject "Security Audit Results", **When** "researcher" sends another message with the same `subject` and same `to_agent`, **Then** the message is appended to the existing conversation (same conversation_id) rather than creating a new one.

4. **Given** agent "analyzer" calls `read_inbox`, **When** "analyzer" calls `read_inbox` again immediately, **Then** the previously read messages are no longer returned (unless `include_read: true` is passed), because inbox_state.last_read_message_id was advanced.

---

### User Story 2 - Agent Claims and Processes Messages (Priority: P1)

An agent reads its inbox, sees pending work, and claims one or more messages for processing. This prevents other agents from processing the same message (atomic claim). After finishing work, the agent marks the message as done or failed.

**Why this priority**: Claim-and-process is the core workflow pattern for task-oriented agents. Without it, two agents could process the same message simultaneously, causing duplicate work. This is co-equal with Story 1 for MVP.

**Independent Test**: Can be tested by sending a message, claiming it with `claim_messages`, verifying the status changes to "processing", then calling `mark_done`. Delivers value: reliable task processing without race conditions.

**Acceptance Scenarios**:

1. **Given** agent "worker" has 3 pending messages in its inbox, **When** "worker" calls `claim_messages` with `limit: 2`, **Then** up to 2 messages are atomically updated to status "processing" with `claimed_by: "worker"` and `claimed_at` set to the current timestamp, and the claimed messages are returned in the response.

2. **Given** agent "worker" has claimed message ID 42 (status: "processing"), **When** "worker" calls `mark_done` with `message_id: 42`, **Then** the message status is updated to "done" and `updated_at` is refreshed.

3. **Given** agent "worker" has claimed message ID 42 (status: "processing"), **When** "worker" calls `mark_done` with `message_id: 42, status: "failed", metadata: {"error": "timeout"}`, **Then** the message status is updated to "failed" and the metadata is merged with the failure reason.

4. **Given** message ID 42 has status "processing" and `claimed_by: "worker-a"`, **When** agent "worker-b" calls `claim_messages` and message 42 matches the filter, **Then** message 42 is NOT included in the claim result because it is already claimed.

5. **Given** agent "worker-b" tries to call `mark_done` on message ID 42 which is claimed by "worker-a", **Then** the operation fails with an error indicating the message is not claimed by "worker-b".

---

### User Story 3 - Agent Sends a Channel Message (Priority: P2)

An agent broadcasts a message to a channel. All agents who are members of that channel see the message in their inbox. This enables one-to-many communication patterns.

**Why this priority**: Channel messaging extends the system from point-to-point to broadcast. It is important but not strictly required for the simplest two-agent use case. Depends on channel infrastructure which will be wired in but the channel creation/join itself is a separate spec scope.

**Independent Test**: Can be tested by creating a channel, adding two member agents, sending a message to the channel, and verifying both members see it in their inboxes. Delivers value: group communication for agent swarms.

**Acceptance Scenarios**:

1. **Given** channel "security-alerts" exists with members "analyzer" and "responder", **When** agent "scanner" (also a member) calls `send_message` with `channel_id: 1`, `body: "Critical: RCE in log4j detected"`, `priority: 10`, **Then** a message is created in the channel's conversation, and both "analyzer" and "responder" see it when they call `read_inbox`.

2. **Given** agent "outsider" is NOT a member of channel "security-alerts", **When** "outsider" calls `send_message` with `channel_id: 1`, **Then** the operation fails with a permission error.

3. **Given** a channel message is sent, **When** agent "analyzer" reads it but "responder" does not, **Then** "analyzer"'s inbox_state is updated independently of "responder"'s. Each agent's read/unread state is tracked separately.

---

### User Story 4 - Agent Searches Message History (Priority: P2)

An agent needs to find past messages by keyword, sender, priority range, status, or time range. Full-text search via SQLite FTS5 enables keyword matching against message bodies. Metadata filters allow structured queries.

**Why this priority**: Search is essential for agents that need context from prior conversations, but the system is functional without it for basic send/read/process workflows.

**Independent Test**: Can be tested by inserting several messages with varied content and metadata, then calling `search_messages` with a query string and verifying relevant results are returned ranked by FTS5 relevance. Delivers value: agents can find historical context.

**Acceptance Scenarios**:

1. **Given** 100 messages exist in the database with various content, **When** agent "analyzer" calls `search_messages` with `query: "deployment failure"`, **Then** messages containing "deployment" and/or "failure" in their body are returned, ordered by FTS5 relevance score, limited to messages the agent has access to (own DMs + joined channels).

2. **Given** messages exist with various priorities, **When** agent calls `search_messages` with `query: "error"`, `min_priority: 7`, **Then** only messages with priority >= 7 that match "error" are returned.

3. **Given** agent "analyzer" has DMs and channel messages, **When** "analyzer" searches with `from_agent: "scanner"`, **Then** only messages sent by "scanner" that "analyzer" has access to are returned.

---

### User Story 5 - Conversation Threading and Metadata (Priority: P3)

Agents use conversation threads to keep related messages grouped. Conversations have subjects and are auto-created on first message if no matching conversation exists. Messages carry rich JSON metadata that agents use for filtering and context passing.

**Why this priority**: Threading and metadata enrich the messaging experience but are not blockers for basic message flow. The auto-create conversation logic is implicitly exercised by Story 1 but this story covers explicit conversation management and metadata usage.

**Independent Test**: Can be tested by sending messages with metadata, then filtering inbox by metadata fields. Delivers value: structured agent-to-agent context passing.

**Acceptance Scenarios**:

1. **Given** no conversation exists between "agent-a" and "agent-b" with subject "Data Pipeline", **When** "agent-a" sends a message with `subject: "Data Pipeline"` and `metadata: {"pipeline_id": "pipe-42", "stage": "extraction"}`, **Then** a new conversation is created, the message stores the metadata as JSON, and subsequent reads include the metadata.

2. **Given** a conversation with subject "Data Pipeline" already has 5 messages, **When** `read_inbox` is called with `conversation_id` filter, **Then** all messages in the thread are returned in chronological order with their individual metadata.

---

### Edge Cases

- What happens when an agent sends a message to a non-existent `to_agent`? The system MUST return a clear error ("agent not found") rather than silently creating a dead-letter message.
- What happens when `send_message` is called with both `to_agent` and `channel_id`? The system MUST reject this as invalid; a message is either a DM or a channel message, not both.
- What happens when an agent tries to claim messages but none are pending? The system MUST return an empty list, not an error.
- What happens when `mark_done` is called on a message with status "done"? The system MUST return an error indicating the message is already completed (idempotency consideration: alternatively, succeed silently -- decision: fail with clear error to surface logic bugs in agents).
- What happens when the message body is empty? The system MUST reject messages with empty or whitespace-only bodies.
- What happens when priority is outside 1-10 range? The SQLite CHECK constraint rejects it; the MCP tool MUST validate before insert and return a user-friendly error.
- What happens when metadata is not valid JSON? The MCP tool MUST validate metadata as valid JSON before insert and return a descriptive error.
- What happens when two agents simultaneously try to claim the same message? Only one MUST succeed due to atomic UPDATE with WHERE status='pending'; the other gets an empty result for that message.
- What happens when `search_messages` query is empty? The system MUST return recent messages (no FTS filter applied) with any other filters still active, behaving as a list operation.
- What happens when the SQLite database file is locked (e.g., during backup)? The storage layer MUST use WAL mode and busy_timeout to minimize lock contention and retry transparently.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST expose `send_message` MCP tool that accepts `to_agent` (string, optional), `channel_id` (integer, optional), `body` (string, required), `subject` (string, optional), `priority` (integer 1-10, default 5), and `metadata` (JSON object, default `{}`). Exactly one of `to_agent` or `channel_id` MUST be provided.

- **FR-002**: System MUST auto-create a conversation when a message is sent with a subject that does not match an existing conversation between the same parties (or in the same channel). If the subject is empty, a new conversation is created for each message unless a `conversation_id` is explicitly provided.

- **FR-003**: System MUST expose `read_inbox` MCP tool that returns unread messages for the calling agent, ordered by priority (descending) then created_at (ascending). Supports optional filters: `status` (string), `from_agent` (string), `conversation_id` (integer), `min_priority` (integer), `limit` (integer, default 50), `include_read` (boolean, default false).

- **FR-004**: System MUST expose `claim_messages` MCP tool that atomically updates up to N pending messages to status "processing" with `claimed_by` set to the calling agent. Accepts `message_ids` (explicit list) or `limit` (integer) for batch claim. Uses a single SQL UPDATE with WHERE status='pending' to guarantee atomicity.

- **FR-005**: System MUST expose `mark_done` MCP tool that transitions a message from "processing" to "done" or "failed". Only the agent that claimed the message (matching `claimed_by`) may mark it. Accepts `message_id` (integer, required), `status` (string: "done" or "failed", default "done"), and `metadata` (JSON object, optional, merged with existing metadata).

- **FR-006**: System MUST expose `search_messages` MCP tool that performs full-text search via SQLite FTS5 on message bodies. Accepts `query` (string), `from_agent` (string), `to_agent` (string), `channel_id` (integer), `min_priority` (integer), `status` (string), `limit` (integer, default 20). Results are scoped to messages the calling agent has access to.

- **FR-007**: System MUST track read/unread state per agent per conversation in the `inbox_state` table. When `read_inbox` returns messages, the `last_read_message_id` is advanced to the highest message ID returned. Messages with ID <= `last_read_message_id` are considered read.

- **FR-008**: System MUST store all messages in SQLite using the schema defined in `schema/001_initial.sql` (tables: `messages`, `conversations`, `inbox_state`, `messages_fts`).

- **FR-009**: System MUST apply database migrations on startup. The `schema_migrations` table tracks applied versions. Migrations run sequentially and transactionally.

- **FR-010**: System MUST use WAL mode for SQLite and set `busy_timeout` to at least 5000ms to handle concurrent access from multiple MCP tool calls.

- **FR-011**: System MUST validate all MCP tool inputs (body not empty, priority in range, metadata is valid JSON, agent exists) and return structured error responses with descriptive messages.

- **FR-012**: System MUST record a trace entry (in the `traces` table) for every MCP tool call: `send_message`, `read_inbox`, `claim_messages`, `mark_done`, `search_messages`. The trace includes agent_name, action, details (JSON with parameters), and any error.

- **FR-013**: System MUST enforce access control: agents can only read messages addressed to them (DM) or posted in channels they are members of. An agent MUST NOT read another agent's DMs.

### Key Entities

- **Message**: The core unit of communication. Key attributes: `id`, `conversation_id`, `from_agent`, `to_agent` (null for channel messages), `channel_id` (null for DMs), `body`, `priority` (1-10), `status` (pending/processing/done/failed), `metadata` (JSON), `claimed_by`, `claimed_at`, `created_at`, `updated_at`. Lives in `internal/messaging/` as a Go struct and in the `messages` SQLite table.

- **Conversation**: Groups related messages into a thread. Key attributes: `id`, `subject`, `created_by` (agent name), `channel_id` (null for DM conversations), `created_at`, `updated_at`. A conversation is auto-created on first message if no matching conversation exists. Lives in `internal/messaging/`.

- **InboxState**: Tracks per-agent, per-conversation read position. Key attributes: `agent_name`, `conversation_id`, `last_read_message_id`. Used by `read_inbox` to determine which messages are unread. Lives in `internal/messaging/`.

- **MessageStore**: The storage interface in `internal/storage/` that provides CRUD operations for messages, conversations, and inbox state. Backed by SQLite via `modernc.org/sqlite`. Responsible for FTS5 synchronization (handled by triggers), migrations, WAL mode, and busy_timeout configuration.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: An agent can send a direct message and the recipient can read it within a single `send_message` + `read_inbox` round-trip. End-to-end latency (send to read) MUST be under 50ms for a local SQLite database.

- **SC-002**: Concurrent claim operations on the same set of pending messages MUST never result in a message being claimed by more than one agent. Verified by a concurrency test with 10 goroutines claiming simultaneously.

- **SC-003**: Full-text search via `search_messages` MUST return relevant results for a query against 10,000 stored messages in under 200ms.

- **SC-004**: All five MCP tools (`send_message`, `read_inbox`, `claim_messages`, `mark_done`, `search_messages`) MUST be callable via the MCP protocol (SSE transport) using a standard MCP client (e.g., `mcp-go` client).

- **SC-005**: Read/unread tracking MUST correctly distinguish read from unread messages: after calling `read_inbox`, a subsequent call with `include_read: false` MUST NOT return previously read messages.

- **SC-006**: Database migrations MUST apply cleanly on a fresh database (no pre-existing file) and MUST be idempotent (running migrations twice produces no errors or schema changes).

- **SC-007**: Every MCP tool call MUST produce a corresponding entry in the `traces` table, verified by querying traces after each operation in integration tests.

- **SC-008**: The messaging package (`internal/messaging/`) MUST have unit test coverage of at least 80% for the core service logic (send, read, claim, mark_done, search).
