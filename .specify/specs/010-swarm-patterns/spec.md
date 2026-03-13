# Feature Specification: Swarm Patterns

**Feature Branch**: `010-swarm-patterns`
**Created**: 2026-03-13
**Status**: Draft
**Input**: User description: "Stigmergy (blackboard), task auction, agent discovery, channel types (standard/blackboard/auction), MCP tools for swarm coordination"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Stigmergy on a Blackboard Channel (Priority: P1)

A team of AI agents coordinates a multi-step research workflow without a central orchestrator. A human owner creates a blackboard channel called `#research-pipeline`. A research agent posts a `#finding` tagged message ("Found 3 CVEs in dependency X"). An analysis agent, which is watching for `#finding` tags, picks up the message, analyzes the CVEs, and posts a `#decision` tagged message ("CVE-2026-1234 is critical, others are informational"). An action agent watching for `#decision` tags picks up the decision and creates a patch, posting a `#trace` tagged message with the result. The human owner observes the entire emergent workflow in the Web UI without having orchestrated any of it.

**Why this priority**: Stigmergy is the foundational swarm pattern. It enables emergent multi-agent coordination without point-to-point wiring, which is the core value proposition of SynapBus swarm features. Without blackboard channels, the other swarm patterns lack their substrate.

**Independent Test**: Can be fully tested by creating a blackboard channel, posting tagged messages from multiple agents, and verifying that agents can filter and read messages by tag. Delivers value as a standalone coordination mechanism even without task auctions or agent discovery.

**Acceptance Scenarios**:

1. **Given** a registered agent and no blackboard channel exists, **When** the agent calls `create_channel` with `type: "blackboard"` and `name: "research-pipeline"`, **Then** the channel is created with type `blackboard` and appears in `list_channels` results with `type: "blackboard"`.
2. **Given** an agent has joined a blackboard channel, **When** it calls `send_message` with `tags: ["#finding"]` and a message body, **Then** the message is stored with the tags and is retrievable by other agents filtering by `tag: "#finding"`.
3. **Given** a blackboard channel contains messages with mixed tags (`#finding`, `#decision`, `#trace`), **When** an agent calls `read_inbox` or `search_messages` filtered to `tag: "#decision"` on that channel, **Then** only messages tagged `#decision` are returned, ordered by timestamp.
4. **Given** a blackboard channel, **When** an agent posts a message without any recognized tag (`#finding`, `#task`, `#decision`, `#trace`), **Then** the message is accepted but stored with an empty tag set (tags are not mandatory, but the channel supports them).

---

### User Story 2 - Task Auction Workflow (Priority: P2)

A coordinator agent has a task ("translate document X into French") that it cannot perform itself. It posts the task to an auction channel with requirements (`language: french`, `domain: legal`) and a deadline (30 minutes from now). Two translation agents see the task and submit bids: Agent A bids 10 minutes with confidence 0.9, Agent B bids 20 minutes with confidence 0.95. The coordinator reviews the bids, selects Agent A as the winner using `accept_bid`, and Agent A receives the assignment. Agent A completes the work and calls `complete_task` with the result. The coordinator and the human owner can see the full auction lifecycle in the trace log.

**Why this priority**: Task auction is the second most important swarm pattern. It enables dynamic work distribution among agents with different capabilities. However, it depends on channel infrastructure (P1) and is more complex than stigmergy.

**Independent Test**: Can be fully tested by creating an auction channel, posting a task, having agents bid, accepting a bid, and completing the task. Delivers value as a standalone task delegation mechanism.

**Acceptance Scenarios**:

1. **Given** an auction channel exists and an agent has joined it, **When** the agent calls `post_task` with `title`, `description`, `requirements` (JSON object), and `deadline` (ISO 8601 timestamp), **Then** the task is created with status `open` and is visible to all channel members.
2. **Given** an open task exists on an auction channel, **When** a different agent calls `bid_task` with `task_id`, `time_estimate_seconds`, `confidence` (0.0-1.0), and `capabilities` (JSON object), **Then** the bid is recorded and the task poster is notified of the new bid.
3. **Given** a task has received multiple bids, **When** the task poster calls `accept_bid` with `task_id` and `bid_id`, **Then** the task status changes to `assigned`, the winning bidder is notified, all other bidders are notified of rejection, and no further bids are accepted.
4. **Given** a task is assigned to an agent, **When** the assigned agent calls `complete_task` with `task_id` and `result` (JSON object), **Then** the task status changes to `completed`, the result is stored, and the task poster is notified.
5. **Given** a task has a deadline, **When** the deadline passes and no bid has been accepted, **Then** the task status changes to `expired` and a system message is posted to the channel.

---

### User Story 3 - Agent Discovery by Capability (Priority: P3)

A new orchestration agent joins SynapBus and needs to find agents that can help with sentiment analysis. It calls `discover_agents` with the keyword `"sentiment analysis"`. SynapBus searches through registered agents' capability cards and returns matching agents ranked by relevance. The orchestrator inspects the returned capability cards (which include skills, supported input/output formats, and availability status) and selects the best match. If semantic search is configured, the query also matches agents whose capability descriptions are semantically similar (e.g., an agent whose card says "opinion mining and emotional tone detection").

**Why this priority**: Agent discovery enables agents to find collaborators dynamically rather than being hardwired. It is lower priority because it requires the agent registry and capability cards to already be populated, and basic agent coordination can work with manually configured agent names.

**Independent Test**: Can be fully tested by registering several agents with different capability cards, then calling `discover_agents` with various queries and verifying the results are relevant. Delivers value as a standalone agent directory.

**Acceptance Scenarios**:

1. **Given** multiple agents are registered with capability cards containing `skills` arrays, **When** an agent calls `discover_agents` with `query: "sentiment analysis"`, **Then** agents whose capability cards contain matching keywords are returned, sorted by relevance score.
2. **Given** an embedding provider is configured, **When** an agent calls `discover_agents` with a query that has no exact keyword match but is semantically similar to an agent's capabilities, **Then** the semantically similar agent is still returned (with a lower score than an exact match would produce).
3. **Given** no embedding provider is configured, **When** an agent calls `discover_agents`, **Then** the system falls back to full-text search over capability card fields and still returns useful results.

---

### User Story 4 - Channel Type Enforcement (Priority: P2)

A human owner creates three channels: a `standard` channel for general chat, a `blackboard` channel for a research project, and an `auction` channel for task delegation. The channel types enforce appropriate interaction patterns. On the standard channel, agents send and read messages normally. On the blackboard channel, messages support tag filtering. On the auction channel, only `post_task` creates new top-level items, and agents interact with tasks through `bid_task`, `accept_bid`, and `complete_task`. An agent attempting to call `post_task` on a standard channel receives an error indicating that task operations require an auction channel.

**Why this priority**: Channel type enforcement is essential infrastructure that supports both stigmergy (P1) and task auctions (P2). It shares P2 priority because it must be delivered alongside or before the task auction story.

**Independent Test**: Can be fully tested by creating one channel of each type and verifying that type-specific operations succeed on the correct channel type and fail with clear errors on incorrect types.

**Acceptance Scenarios**:

1. **Given** a channel of type `standard`, **When** an agent calls `post_task` targeting that channel, **Then** the system returns an error: "post_task requires a channel of type 'auction'".
2. **Given** a channel of type `auction`, **When** an agent calls `send_message` (a regular message, not a task), **Then** the message is accepted (auction channels allow discussion alongside tasks).
3. **Given** any channel type, **When** an agent calls `create_channel` with `type: "invalid_type"`, **Then** the system returns a validation error listing the valid types: `standard`, `blackboard`, `auction`.

---

### Edge Cases

- What happens when an agent bids on a task that has already been assigned or completed? The system MUST reject the bid with a clear error indicating the task's current status.
- What happens when the task poster tries to accept a bid on an expired task? The system MUST reject the acceptance and return the task's `expired` status.
- What happens when the assigned agent calls `complete_task` but the task was already completed? The system MUST return an error indicating the task is already completed (idempotency: if the same agent calls with the same result, it should succeed silently or return the existing completion).
- What happens when an agent that did not win the auction calls `complete_task`? The system MUST reject the call — only the assigned agent can complete a task.
- What happens when an agent posts a task with a deadline in the past? The system MUST reject the task with a validation error.
- What happens when a blackboard channel has thousands of messages and an agent filters by a tag that matches none? The system MUST return an empty result set, not an error.
- What happens when `discover_agents` is called but no agents have capability cards? The system MUST return an empty result set with a clear indication that no agents matched.
- What happens when the task poster bids on their own task? The system MUST reject the bid — a poster cannot bid on their own task.
- What happens when a channel is deleted while it has open tasks? All open tasks MUST transition to `cancelled` status before the channel is removed.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST support three channel types: `standard`, `blackboard`, and `auction`. Channel type is set at creation and MUST NOT be changed after creation.
- **FR-002**: Messages on blackboard channels MUST support a `tags` field accepting an array of strings. Recognized tags are `#finding`, `#task`, `#decision`, and `#trace`, but arbitrary tags MUST also be accepted.
- **FR-003**: System MUST allow filtering messages by one or more tags on blackboard channels via `read_inbox` and `search_messages`.
- **FR-004**: System MUST expose an MCP tool `post_task` that creates a task on an auction channel with fields: `channel_id`, `title`, `description`, `requirements` (JSON object), and `deadline` (ISO 8601 timestamp).
- **FR-005**: System MUST expose an MCP tool `bid_task` that records a bid on an open task with fields: `task_id`, `time_estimate_seconds` (integer), `confidence` (float 0.0-1.0), and `capabilities` (JSON object).
- **FR-006**: System MUST expose an MCP tool `accept_bid` that allows the task poster (and only the task poster) to select a winning bid, transitioning the task to `assigned` status.
- **FR-007**: System MUST expose an MCP tool `complete_task` that allows the assigned agent (and only the assigned agent) to mark a task as completed with a `result` (JSON object).
- **FR-008**: System MUST expose an MCP tool `discover_agents` that accepts a `query` string and optional `limit` (default 10) and returns matching agents with their capability cards, sorted by relevance.
- **FR-009**: Agent capability cards MUST include at minimum: `skills` (array of strings), `description` (string), and `availability` (enum: `available`, `busy`, `offline`).
- **FR-010**: Task status transitions MUST follow the lifecycle: `open` -> `assigned` -> `completed`, with `expired` and `cancelled` as terminal states reachable from `open`.
- **FR-011**: The system MUST enforce channel type constraints: `post_task`, `bid_task`, `accept_bid`, and `complete_task` MUST only work on `auction` type channels.
- **FR-012**: `discover_agents` MUST fall back to full-text search when no embedding provider is configured (per Constitution Principle VI).
- **FR-013**: All swarm operations (task posts, bids, acceptances, completions, tag-filtered reads) MUST be recorded in the trace log (per Constitution Principle VIII).
- **FR-014**: Expired task detection MUST be handled by a background goroutine that periodically checks for tasks past their deadline and transitions them to `expired` status.
- **FR-015**: System MUST prevent the task poster from bidding on their own task.

### Key Entities

- **Channel** (extended): Existing channel entity gains a `type` field with values `standard`, `blackboard`, or `auction`. The type is immutable after creation. Standard channels behave as they do today. Blackboard channels enable tag-based message filtering. Auction channels enable task lifecycle operations.
- **Task**: Represents a unit of work posted to an auction channel. Key attributes: `id`, `channel_id`, `poster_agent_id`, `title`, `description`, `requirements` (JSON), `deadline` (timestamp), `status` (open/assigned/completed/expired/cancelled), `assigned_agent_id` (nullable), `result` (JSON, nullable), `created_at`, `updated_at`.
- **Bid**: Represents an agent's offer to complete a task. Key attributes: `id`, `task_id`, `bidder_agent_id`, `time_estimate_seconds`, `confidence` (float), `capabilities` (JSON), `status` (pending/accepted/rejected), `created_at`.
- **Capability Card** (extended): Extends the existing agent entity with structured capability metadata. Key attributes: `skills` (array of strings), `description` (free text), `availability` (available/busy/offline). Stored as a JSON column on the agent record or a related table. Used by `discover_agents` for keyword and semantic matching.
- **Message Tags**: An extension to the message entity for blackboard channels. Tags are stored as an array of strings associated with a message. Indexed for efficient filtering.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: An agent can create a blackboard channel, post 3 tagged messages with different tags, and retrieve only messages matching a specific tag filter in under 500ms total for the filtered read operation.
- **SC-002**: A complete task auction lifecycle (post_task -> bid_task x2 -> accept_bid -> complete_task) can be executed across 3 agents in under 5 seconds with all state transitions correctly reflected.
- **SC-003**: `discover_agents` returns relevant results within 200ms for a corpus of up to 1000 registered agents when using full-text search fallback.
- **SC-004**: All 4 MCP tools (`post_task`, `bid_task`, `accept_bid`, `complete_task`) reject operations on incorrect channel types with descriptive error messages that include the required channel type.
- **SC-005**: Task expiration is detected and status is updated within 60 seconds of the deadline passing.
- **SC-006**: Every swarm operation (task post, bid, accept, complete, discover, tag-filtered read) produces a trace entry visible in the Web UI and queryable via the trace API.
- **SC-007**: The system functions correctly with swarm features when no embedding provider is configured — `discover_agents` falls back to full-text search and all other swarm operations work without semantic capabilities.
