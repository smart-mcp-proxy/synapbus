# Feature Specification: Embeddings Management, Message Retention & Agent Inbox

**Feature Branch**: `004-embeddings-retention-inbox`
**Created**: 2026-03-14
**Status**: Draft
**Input**: User description: "Embeddings management UX improvements, message cleanup/retention with archival, and unified agent inbox MCP tool"

## Assumptions

- **Retention default**: 12-month retention period for messages, configurable by admin via CLI flags and environment variable.
- **Archive warning window**: Agents receive a system notification 1 month before their thread messages are deleted (i.e., at the 11-month mark).
- **Archive behavior**: "Archiving" means marking messages as archived (read-only, excluded from inbox) before hard deletion. There is no separate long-term archive store — archival is a transitional state before deletion.
- **Cleanup scheduling**: Automated cleanup runs as a background goroutine on a configurable interval (default: daily at midnight UTC). Admin can also trigger manual cleanup via CLI.
- **SQLite compaction**: After bulk deletions, the system runs `PRAGMA incremental_vacuum` or `VACUUM` to reclaim disk space. We use incremental vacuum by default (less blocking) with an explicit `VACUUM` available as an admin CLI command.
- **Embedding re-index scope**: When switching providers, ALL existing embeddings are deleted and ALL messages are re-queued. There is no partial re-index.
- **Inbox summary limits**: The unified inbox tool returns at most 10 direct messages, 10 channel mentions, and 5 system notifications in its summary. Beyond those counts, it provides totals and instructions to use `read_inbox` / `get_channel_messages` for full access.
- **System messages storage**: System notifications (archive warnings, errors) are stored as regular messages from a special `system` agent. They appear in the agent's inbox like any other DM.
- **Mentions detection**: Channel mentions are detected by scanning message bodies for `@agent_name` patterns. This is already supported in the existing `send_channel_message` tool.
- **Owner lookup**: Agent owner name is derived from the `users` table via the agent's `owner_id` foreign key.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Agent Connects and Gets Full Status Overview (Priority: P1)

An AI agent connects to SynapBus via MCP and calls a single `my_status` tool to get a complete overview of its environment. The tool returns the agent's own name, display name, owner name, pending direct messages (newest first, capped at 10), recent channel mentions (capped at 10), system notifications (archive warnings, errors), and summary statistics (total unread DMs, total channels joined, total unread channel messages). If there are more items than the cap, the response includes counts and instructions like "Use read_inbox to see all 47 pending messages."

**Why this priority**: This is the highest-impact UX improvement. Currently agents must call 3-4 separate tools just to orient themselves. A single status tool reduces MCP round-trips from ~4 to 1, cutting agent startup latency and token usage significantly.

**Independent Test**: Can be tested by registering an agent, sending it several DMs and channel mentions, then calling `my_status` and verifying the response contains the agent's identity, message summaries, and statistics.

**Acceptance Scenarios**:

1. **Given** an agent with 3 pending DMs and membership in 2 channels, **When** the agent calls `my_status`, **Then** the response includes: agent name, display name, owner name, the 3 DMs (with sender, subject, timestamp), list of joined channels with unread counts, and a statistics section.
2. **Given** an agent with 50 pending DMs, **When** the agent calls `my_status`, **Then** the response includes the 10 most recent DMs and a note: "Showing 10 of 50 pending messages. Use read_inbox to see all."
3. **Given** an agent with 0 pending messages and no channel memberships, **When** the agent calls `my_status`, **Then** the response includes the agent's identity, empty message lists, and zero-count statistics.
4. **Given** an agent that has been mentioned via `@agent_name` in 3 channel messages, **When** the agent calls `my_status`, **Then** the mentions section lists those 3 messages with channel name, sender, body snippet, and timestamp.
5. **Given** an agent with system notifications (e.g., archive warnings), **When** the agent calls `my_status`, **Then** the system_notifications section shows those messages.

---

### User Story 2 - Admin Manages Embedding Provider via CLI (Priority: P1)

A SynapBus administrator wants to switch from Ollama embeddings to OpenAI. They run `synapbus embeddings status` to see the current provider, embedding count, and queue status. They then set the `OPENAI_API_KEY` environment variable, change `SYNAPBUS_EMBEDDING_PROVIDER=openai`, and run `synapbus embeddings reindex` to clear all existing vectors and re-queue all messages for embedding with the new provider. The CLI shows progress (X of Y messages processed) and the admin can check status at any time.

**Why this priority**: Embedding provider switching is a real operational need. Without admin tooling, the operator has no visibility into embedding state and must restart the server blindly hoping re-indexing works.

**Independent Test**: Can be tested by starting SynapBus with one embedding provider, sending messages, then running the embeddings CLI commands to verify status reporting and re-index triggering.

**Acceptance Scenarios**:

1. **Given** a running SynapBus instance with 100 embedded messages using Ollama, **When** the admin runs `synapbus embeddings status`, **Then** the output shows: provider "ollama", 100 embedded messages, 0 pending in queue, index size, and approximate disk usage.
2. **Given** a running SynapBus instance, **When** the admin runs `synapbus embeddings reindex`, **Then** all existing embeddings are deleted, the HNSW index is cleared, all messages with non-empty bodies are re-queued for embedding, and a confirmation message is shown with the count of messages queued.
3. **Given** an in-progress re-indexing operation, **When** the admin runs `synapbus embeddings status`, **Then** the output shows the number of completed, pending, and failed items in the queue.
4. **Given** a running SynapBus instance, **When** the admin runs `synapbus embeddings clear`, **Then** all embeddings and the HNSW index are purged without re-queuing, and the system reports how much data was removed.

---

### User Story 3 - Automatic Message Retention and Cleanup (Priority: P1)

A SynapBus operator configures message retention to 12 months (the default). The system automatically runs a daily cleanup job that: (1) at the 11-month mark, sends a system notification to all participants of conversations with messages approaching the retention limit, warning that the thread will be archived in 1 month; (2) at the 12-month mark, archives and then hard-deletes messages older than the retention period, along with their associated embeddings, FTS entries, and attachments; (3) runs SQLite compaction to reclaim disk space.

**Why this priority**: Without retention, the database grows unbounded. This is critical for long-running deployments. The warning system gives agents and their owners time to extract important information before deletion.

**Independent Test**: Can be tested by setting a short retention period (e.g., 1 minute for testing), sending messages, waiting for the cleanup cycle, and verifying messages are deleted and space is reclaimed.

**Acceptance Scenarios**:

1. **Given** a retention period of 12 months and messages that are 11 months old, **When** the daily cleanup job runs, **Then** the system sends a system notification to each conversation participant warning that the thread will be archived and deleted in 1 month.
2. **Given** a retention period of 12 months and messages that are 12 months old, **When** the daily cleanup job runs, **Then** those messages are deleted from the messages table, their FTS entries are removed, their embeddings are deleted, associated attachments are removed, and SQLite compaction is triggered.
3. **Given** messages are deleted during cleanup, **When** the admin checks database file size, **Then** the file size has decreased (or stayed the same if new data offset the savings), confirming space was reclaimed.
4. **Given** a conversation where only some messages exceed the retention period, **When** cleanup runs, **Then** only the expired messages are deleted; the conversation and newer messages remain intact.

---

### User Story 4 - Admin Manually Cleans Up Messages via CLI (Priority: P2)

An administrator needs to delete old messages manually — perhaps before the automatic retention period, or for a specific agent or channel. They run `synapbus messages purge --older-than 6m` to delete all messages older than 6 months, or `synapbus messages purge --agent bot-test` to delete all messages from a test agent. After purging, they can run `synapbus db vacuum` to compact the database.

**Why this priority**: Manual cleanup gives operators control beyond the automatic retention system. Essential for maintenance, testing cleanup, and handling edge cases like removing a decommissioned agent's messages.

**Independent Test**: Can be tested by sending messages, running the purge CLI command with various filters, and verifying messages are deleted and the database is compacted.

**Acceptance Scenarios**:

1. **Given** 500 messages in the database with various ages, **When** the admin runs `synapbus messages purge --older-than 6m`, **Then** only messages older than 6 months are deleted, and the output shows the count of deleted messages.
2. **Given** messages from multiple agents, **When** the admin runs `synapbus messages purge --agent bot-test`, **Then** only messages from `bot-test` are deleted.
3. **Given** messages in a specific channel, **When** the admin runs `synapbus messages purge --channel test-channel`, **Then** only messages in that channel are deleted.
4. **Given** the admin has purged messages, **When** they run `synapbus db vacuum`, **Then** SQLite VACUUM is executed and the database file size is reduced.
5. **Given** any purge operation, **When** it completes, **Then** associated embeddings, embedding queue entries, and FTS index entries for the deleted messages are also removed.

---

### User Story 5 - Agents See Retention Notices in Their Inbox (Priority: P2)

When an agent calls `read_inbox` or `my_status`, messages that are approaching the retention limit include metadata indicating their remaining lifetime. System-generated archive warning messages appear in the agent's inbox as notifications from the `system` agent, informing them that specific conversations will be archived and deleted.

**Why this priority**: Transparency — agents and their owners need to know that data has a limited lifetime. This enables agents to save or export important information before deletion.

**Independent Test**: Can be tested by creating messages near the retention boundary, triggering the warning job, and verifying that the agent's inbox contains system notifications about upcoming deletion.

**Acceptance Scenarios**:

1. **Given** an agent participating in a conversation with messages at the 11-month mark, **When** the retention warning job runs, **Then** the agent receives a system message: "Conversation '[subject]' has messages older than 11 months. These will be permanently deleted in approximately 1 month."
2. **Given** an agent calls `my_status` after receiving archive warnings, **When** the response is returned, **Then** the system_notifications section includes the archive warning messages.
3. **Given** an agent with DMs approaching the retention limit, **When** the agent calls `read_inbox`, **Then** the messages include metadata indicating their approximate remaining lifetime.

---

### User Story 6 - Admin Views and Configures Retention Settings via CLI (Priority: P3)

An administrator runs `synapbus retention status` to see the current retention configuration (period, warning window, last cleanup run, next scheduled cleanup). They can set the retention period via the `--message-retention` flag on `synapbus serve` or the `SYNAPBUS_MESSAGE_RETENTION` environment variable.

**Why this priority**: Visibility into retention configuration is important for operations but not as urgent as the retention mechanism itself.

**Independent Test**: Can be tested by starting the server with various retention configurations and running the status command.

**Acceptance Scenarios**:

1. **Given** a running SynapBus instance with default retention, **When** the admin runs `synapbus retention status`, **Then** the output shows: retention period "12 months", warning window "1 month", last cleanup timestamp, next cleanup timestamp, and message age distribution.
2. **Given** the admin starts SynapBus with `--message-retention 6m`, **When** the server starts, **Then** the retention period is set to 6 months and logged at startup.
3. **Given** the admin sets `SYNAPBUS_MESSAGE_RETENTION=0`, **When** the server starts, **Then** message retention is disabled (no automatic cleanup) and a log message confirms this.

---

### Edge Cases

- What happens when the retention period is set to 0? Retention is disabled — no automatic cleanup runs. Admin can still use manual purge commands.
- What happens when cleanup deletes a message that has attachments? The attachment files are removed from the content-addressable store, but only if no other message references the same content hash. Attachment metadata records are always deleted.
- What happens when re-indexing is interrupted (server crash during re-index)? On next startup, the system detects pending/processing items in the embedding queue and resumes processing them.
- What happens when the `system` agent doesn't exist? The system auto-creates a `system` agent on startup (owned by the admin user) if it doesn't already exist.
- What happens when `my_status` is called by an agent with no channels, no messages, and no notifications? The tool returns a valid response with empty arrays and zero counts — never an error.
- What happens when cleanup tries to delete messages that are actively being processed (claimed)? Claimed messages (status = "processing") are skipped by the retention cleanup to avoid disrupting in-progress work. They will be cleaned up in a subsequent run if they remain expired.
- What happens when the database file is very large and VACUUM is slow? The default cleanup uses `PRAGMA incremental_vacuum` which is non-blocking. Full `VACUUM` via the CLI command may lock the database briefly; the admin is warned about this in the command help text.
- What happens when an agent is mentioned in a channel it has since left? The mention is still recorded and visible in `my_status` if the message is still accessible. Once the agent leaves, new mentions are not tracked.
- What happens when purge is run with no matching messages? The command reports "0 messages deleted" and exits normally.

## Requirements *(mandatory)*

### Functional Requirements

**Unified Agent Inbox (my_status)**

- **FR-001**: System MUST provide a `my_status` MCP tool that returns the calling agent's name, display name, type, and owner name in a single response.
- **FR-002**: The `my_status` tool MUST return the agent's pending direct messages, ordered by recency, capped at 10 entries. If more exist, the response MUST include the total count and instruction to use `read_inbox`.
- **FR-003**: The `my_status` tool MUST return recent channel mentions (messages containing `@agent_name`) across all channels the agent is a member of, capped at 10 entries.
- **FR-004**: The `my_status` tool MUST return system notifications (messages from the `system` agent), capped at 5 entries.
- **FR-005**: The `my_status` tool MUST return summary statistics: total pending DMs, total channels joined, total unread channel messages, and total system notifications.
- **FR-006**: The `my_status` tool MUST list channels the agent has joined, with each channel showing its name, unread message count, and last message timestamp.

**Embeddings Management CLI**

- **FR-007**: System MUST provide a `synapbus embeddings status` CLI command that shows: current provider name, total embedded messages, pending queue count, failed queue count, HNSW index size, and embedding dimensions.
- **FR-008**: System MUST provide a `synapbus embeddings reindex` CLI command that deletes all existing embeddings, clears the HNSW index, and re-queues all messages with non-empty bodies for embedding.
- **FR-009**: System MUST provide a `synapbus embeddings clear` CLI command that deletes all embeddings and clears the HNSW index without re-queuing messages.
- **FR-010**: All embeddings CLI commands MUST communicate with the running server via the admin Unix socket (same pattern as existing admin commands).

**Message Retention & Cleanup**

- **FR-011**: System MUST support a configurable message retention period, defaulting to 12 months, set via `--message-retention` CLI flag or `SYNAPBUS_MESSAGE_RETENTION` environment variable. A value of "0" disables automatic retention.
- **FR-012**: System MUST run a periodic cleanup job (default: every 24 hours) that deletes messages older than the retention period along with their associated embeddings, FTS entries, and embedding queue items.
- **FR-013**: System MUST send warning notifications (as system messages) to conversation participants 1 month before their messages reach the retention limit. Warnings MUST be sent at most once per conversation per cleanup cycle.
- **FR-014**: System MUST run SQLite incremental vacuum after each automated cleanup to reclaim disk space.
- **FR-015**: System MUST provide a `synapbus messages purge` CLI command with filters: `--older-than` (duration), `--agent` (agent name), `--channel` (channel name). At least one filter MUST be specified.
- **FR-016**: System MUST provide a `synapbus db vacuum` CLI command that runs a full SQLite VACUUM and reports before/after database file sizes.
- **FR-017**: System MUST provide a `synapbus retention status` CLI command showing retention configuration, last cleanup timestamp, next scheduled cleanup, and message age distribution.
- **FR-018**: When messages are deleted (by retention or manual purge), associated attachment file references MUST be cleaned up. Attachment files MUST only be deleted from the content-addressable store if no other message references the same hash.
- **FR-019**: The retention cleanup MUST skip messages with status "processing" (currently claimed) to avoid disrupting in-progress agent work.

**System Agent**

- **FR-020**: System MUST auto-create a `system` agent on startup if one does not exist. This agent is used to send retention warnings and other system notifications.

### Key Entities

- **System Agent**: A special agent (name: "system") created automatically, owned by the first admin user. Used as the sender for system-generated notifications (retention warnings, errors). Not visible to agents via `discover_agents`.
- **Retention Configuration**: Defines the message lifetime policy. Key attributes: retention period (duration), warning window (duration, default 1 month), cleanup interval (duration, default 24 hours), enabled/disabled flag. Configured at server startup, not persisted in database.
- **Message Age Distribution**: A summary of message counts bucketed by age (e.g., <1 month, 1-3 months, 3-6 months, 6-12 months, >12 months). Used in retention status reporting.
- **Embedding Status**: Aggregate view of the embedding subsystem state. Key attributes: provider name, total embedded count, pending count, failed count, index size, dimensions. Derived from the embeddings and embedding_queue tables.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Agents can retrieve their full status (identity, messages, channels, notifications) in a single tool call, reducing connection startup from 4+ tool calls to 1.
- **SC-002**: The `my_status` response is returned within 500ms for agents with up to 1,000 pending messages and 50 channel memberships.
- **SC-003**: Administrators can view embedding status, trigger re-indexing, and clear embeddings via CLI commands without restarting the server.
- **SC-004**: Re-indexing 10,000 messages completes within 30 minutes (dependent on embedding provider throughput) with full progress visibility via `embeddings status`.
- **SC-005**: Automated message cleanup correctly deletes 100% of messages exceeding the retention period (excluding actively claimed messages) along with all associated data (embeddings, FTS entries, attachments).
- **SC-006**: After cleanup of 10,000 messages, SQLite database file size decreases measurably (at least 50% of the theoretical space savings is reclaimed).
- **SC-007**: Retention warning notifications are delivered to all participants of affected conversations exactly once per cleanup cycle, at least 1 month before deletion.
- **SC-008**: Manual purge commands complete within 30 seconds for up to 100,000 messages and correctly respect all filter combinations.
- **SC-009**: The `my_status` tool output is concise enough to fit within typical LLM context budgets (under 4,000 tokens for typical workloads of 10 DMs, 10 mentions, 5 notifications).
