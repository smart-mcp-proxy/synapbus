# Tasks: Swarm Patterns

**Input**: Design documents from `/specs/010-swarm-patterns/`
**Prerequisites**: spec.md (required), constitution.md (reviewed)

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story. Channel type enforcement (US4) is co-located with Phase 2 since it is foundational infrastructure required by US1 and US2.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3, US4)
- Include exact file paths in descriptions

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Schema migration, domain types, and shared constants for swarm patterns

- [ ] T001 Create migration `schema/002_swarm_patterns.sql` — add `tags` column (JSON text, default '[]') to `messages` table, add `expired` to `tasks.status` CHECK constraint, add `result` column (JSON text, nullable) to `tasks` table, add `confidence` column (REAL, nullable) to `task_bids` table, add `time_estimate_seconds` column (INTEGER, nullable) to `task_bids` table, add capability card columns to `agents` table (`skills` JSON text default '[]', `capability_description` TEXT default '', `availability` TEXT default 'available' with CHECK), create FTS index `agents_fts` over `name, capability_description, skills` for fallback keyword search, add index `idx_messages_tags` for tag-based filtering
- [ ] T002 [P] Define channel type constants and validation in `internal/channels/types.go` — `ChannelTypeStandard`, `ChannelTypeBlackboard`, `ChannelTypeAuction` string constants; `ValidChannelType(t string) bool` function; `ChannelTypeError` struct with descriptive messages per FR-011
- [ ] T003 [P] Define task domain types in `internal/channels/task.go` — `Task` struct (id, channel_id, posted_by, title, description, requirements JSON, deadline, status, assigned_to, result JSON, created_at, updated_at), `Bid` struct (id, task_id, agent_name, time_estimate_seconds int, confidence float64, capabilities JSON, status, created_at), `TaskStatus` constants (`open`, `assigned`, `completed`, `expired`, `cancelled`), `BidStatus` constants (`pending`, `accepted`, `rejected`), lifecycle validation functions per FR-010
- [ ] T004 [P] Define capability card types in `internal/agents/capability.go` — `CapabilityCard` struct (skills []string, description string, availability string), `Availability` constants (`available`, `busy`, `offline`), JSON marshal/unmarshal helpers per FR-009
- [ ] T005 [P] Define message tag types in `internal/messaging/tags.go` — `WellKnownTags` list (`#finding`, `#task`, `#decision`, `#trace`), `ParseTags(input []string) []string` normalization function, `TagsToJSON`/`TagsFromJSON` helpers per FR-002

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Storage layer, channel type enforcement, and shared services that ALL user stories depend on

**CRITICAL**: No user story work can begin until this phase is complete

- [ ] T006 Extend channel store in `internal/channels/store.go` — update `CreateChannel` to accept and persist `type` field; update `GetChannel`/`ListChannels` to return type; add `ValidateChannelType` check that rejects unknown types with descriptive error per FR-001 (listing valid types: standard, blackboard, auction); ensure type is immutable after creation (reject any update attempt to change type)
- [ ] T007 [P] Extend message store in `internal/messaging/store.go` — update `CreateMessage` to accept and persist `tags` JSON array; update `GetMessages`/`ReadInbox` to support optional `tag` filter parameter; implement tag-based filtering query using JSON functions or LIKE matching against the tags column; return empty result set (not error) when no messages match a tag filter per edge case requirement
- [ ] T008 [P] Extend agent store in `internal/agents/store.go` — update `RegisterAgent`/`UpdateAgent` to accept and persist capability card fields (`skills`, `capability_description`, `availability`); add `SearchAgents(ctx, query string, limit int) ([]AgentWithScore, error)` using FTS5 `agents_fts` table; return empty result set when no agents match per edge case requirement
- [ ] T009 [P] [US4] Implement channel type enforcement middleware in `internal/channels/enforcement.go` — `RequireChannelType(channelID int64, requiredType string) error` function that loads the channel and returns a typed error if the channel type does not match (e.g., "post_task requires a channel of type 'auction'"); used by all auction and blackboard operations per FR-011 and US4 acceptance scenarios
- [ ] T010 Create task store in `internal/channels/task_store.go` — `CreateTask`, `GetTask`, `ListTasksByChannel`, `UpdateTaskStatus`, `AssignTask`, `CompleteTask` with SQL operations against `tasks` table; `CreateBid`, `GetBid`, `ListBidsByTask`, `UpdateBidStatus` against `task_bids` table; enforce lifecycle transitions per FR-010 (open->assigned->completed, open->expired, open->cancelled); reject bids on non-open tasks per edge case; reject completion by non-assigned agent per edge case
- [ ] T011 [P] Add swarm trace helpers in `internal/trace/swarm.go` — helper functions `TraceTaskPosted`, `TraceBidSubmitted`, `TraceBidAccepted`, `TraceTaskCompleted`, `TraceTaskExpired`, `TraceDiscoverAgents`, `TraceTagFilteredRead` that wrap the existing trace store with structured JSON details per FR-013 and Principle VIII

**Checkpoint**: Foundation ready — channel types enforced, storage layer supports tags/tasks/capability cards, user story implementation can begin

---

## Phase 3: User Story 1 — Stigmergy on a Blackboard Channel (Priority: P1)

**Goal**: Agents coordinate via tagged messages on blackboard channels without a central orchestrator

**Independent Test**: Create a blackboard channel, post tagged messages from multiple agents, verify agents can filter and read messages by tag

### Implementation for User Story 1

- [ ] T012 [US1] Implement blackboard MCP tool extensions in `internal/mcp/blackboard.go` — extend `send_message` tool to accept optional `tags` parameter (array of strings) when targeting a blackboard channel; messages without tags are accepted with empty tag set per acceptance scenario 4; validate that tags parameter is stored correctly as JSON array per FR-002
- [ ] T013 [US1] Implement tag-filtered read in `internal/mcp/blackboard.go` — extend `read_inbox` and `search_messages` MCP tools to accept optional `tag` filter parameter; when tag filter is provided on a blackboard channel, return only matching messages ordered by timestamp per acceptance scenario 3; trace every tag-filtered read via `TraceTagFilteredRead` per FR-013
- [ ] T014 [US1] Register blackboard MCP tools in `internal/mcp/server.go` — wire up the extended `send_message` (with tags support) and tag-filtered `read_inbox`/`search_messages` into the MCP tool registry with JSON Schema descriptions per Principle II

**Checkpoint**: User Story 1 fully functional — agents can create blackboard channels, post tagged messages, and filter by tag

---

## Phase 4: User Story 4 — Channel Type Enforcement (Priority: P2)

**Goal**: Channel types enforce appropriate interaction patterns; type-incorrect operations fail with clear errors

**Independent Test**: Create one channel of each type, verify type-specific operations succeed on correct types and fail with descriptive errors on incorrect types

### Implementation for User Story 4

- [ ] T015 [US4] Wire enforcement into existing MCP tools in `internal/mcp/server.go` — ensure `create_channel` tool validates `type` parameter against the three valid types (standard, blackboard, auction) and returns validation error listing valid types for invalid input per acceptance scenario 3; ensure channel type is included in `list_channels` results
- [ ] T016 [US4] Add enforcement guards to auction tools in `internal/mcp/auction.go` — `post_task`, `bid_task`, `accept_bid`, and `complete_task` must call `RequireChannelType(channelID, "auction")` before proceeding per acceptance scenario 1 and FR-011; `send_message` must remain allowed on auction channels per acceptance scenario 2
- [ ] T017 [US4] Enforce channel deletion cascade in `internal/channels/store.go` — when a channel with open tasks is deleted, transition all open tasks to `cancelled` status before removing the channel per edge case requirement; emit trace entries for each cancelled task

**Checkpoint**: User Story 4 fully functional — channel types enforce correct interaction patterns with clear error messages

---

## Phase 5: User Story 2 — Task Auction Workflow (Priority: P2)

**Goal**: Agents post tasks, bid, and coordinate work assignment through an auction lifecycle

**Independent Test**: Create an auction channel, post a task, have agents bid, accept a bid, complete the task — verify full lifecycle

### Implementation for User Story 2

- [ ] T018 [US2] Implement `post_task` MCP tool in `internal/mcp/auction.go` — accepts `channel_id`, `title`, `description`, `requirements` (JSON), `deadline` (ISO 8601); validates channel is type `auction`; rejects deadline in the past per edge case; creates task with status `open`; traces via `TraceTaskPosted`; notifies channel members per FR-004
- [ ] T019 [US2] Implement `bid_task` MCP tool in `internal/mcp/auction.go` — accepts `task_id`, `time_estimate_seconds` (int), `confidence` (float 0.0-1.0), `capabilities` (JSON); validates task is `open`; rejects if bidder is the task poster per FR-015; rejects bid on assigned/completed/expired tasks per edge case; records bid and traces via `TraceBidSubmitted` per FR-005
- [ ] T020 [US2] Implement `accept_bid` MCP tool in `internal/mcp/auction.go` — accepts `task_id` and `bid_id`; validates caller is the task poster per FR-006; validates task is `open` (rejects if expired per edge case); transitions task to `assigned`, winning bid to `accepted`, all other bids to `rejected`; notifies winner and rejected bidders; traces via `TraceBidAccepted`
- [ ] T021 [US2] Implement `complete_task` MCP tool in `internal/mcp/auction.go` — accepts `task_id` and `result` (JSON); validates caller is the assigned agent per FR-007; rejects if called by non-assigned agent per edge case; handles idempotent completion (same agent, same result returns success) per edge case; transitions task to `completed`; stores result; traces via `TraceTaskCompleted`
- [ ] T022 [US2] Implement task expiration background goroutine in `internal/channels/expiry.go` — periodic check (every 30 seconds) for tasks past their deadline with status `open`; transitions expired tasks to `expired` status; posts system message to the auction channel per FR-014; traces via `TraceTaskExpired`; must detect and update within 60 seconds of deadline per SC-005
- [ ] T023 [US2] Register auction MCP tools in `internal/mcp/server.go` — wire up `post_task`, `bid_task`, `accept_bid`, `complete_task` into MCP tool registry with JSON Schema descriptions per Principle II; include parameter validation schemas (confidence range, ISO 8601 format, etc.)

**Checkpoint**: User Story 2 fully functional — complete auction lifecycle works end-to-end across multiple agents

---

## Phase 6: User Story 3 — Agent Discovery by Capability (Priority: P3)

**Goal**: Agents find collaborators by searching capability cards using keyword or semantic matching

**Independent Test**: Register agents with different capability cards, call `discover_agents` with various queries, verify relevant results returned

### Implementation for User Story 3

- [ ] T024 [US3] Implement `discover_agents` MCP tool in `internal/mcp/discovery.go` — accepts `query` (string) and optional `limit` (int, default 10) per FR-008; calls agent store's `SearchAgents` for FTS5 keyword search; returns matching agents with capability cards sorted by relevance score; returns empty result set with clear indication when no agents match per edge case; traces via `TraceDiscoverAgents` per FR-013
- [ ] T025 [US3] Implement semantic search path in `internal/mcp/discovery.go` — when an embedding provider is configured (`SYNAPBUS_EMBEDDING_PROVIDER` env var), embed the query and search agent capability card embeddings via HNSW index; merge semantic results with FTS results, using semantic score as a tiebreaker; when no embedding provider is configured, fall back to FTS5-only search per FR-012 and Principle VI
- [ ] T026 [US3] Add capability card embedding pipeline in `internal/agents/embeddings.go` — when an agent registers or updates capability card fields AND an embedding provider is configured, asynchronously compute and store an embedding vector for the agent's combined skills + description text; store in HNSW index keyed by agent ID; skip silently when no provider is configured per Principle VI
- [ ] T027 [US3] Register discovery MCP tool in `internal/mcp/server.go` — wire up `discover_agents` into MCP tool registry with JSON Schema description per Principle II; document that semantic search is optional and requires embedding provider configuration

**Checkpoint**: User Story 3 fully functional — agents can discover collaborators by keyword or semantic search

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Integration testing, performance validation, and cross-story consistency

- [ ] T028 [P] Validate SC-001 — write benchmark test in `internal/channels/blackboard_bench_test.go`: create blackboard channel, post 100+ tagged messages, verify filtered read returns correct results in under 500ms
- [ ] T029 [P] Validate SC-002 — write integration test in `internal/channels/auction_integration_test.go`: execute full auction lifecycle (post_task -> bid x2 -> accept_bid -> complete_task) across 3 agents, verify all state transitions complete in under 5 seconds
- [ ] T030 [P] Validate SC-003 — write benchmark test in `internal/agents/discovery_bench_test.go`: register 1000 agents with varied capability cards, verify `discover_agents` FTS5 fallback returns results within 200ms
- [ ] T031 [P] Validate SC-006 — write integration test in `internal/trace/swarm_integration_test.go`: execute one of each swarm operation, verify each produces a trace entry queryable via the trace store
- [ ] T032 Validate SC-007 — write integration test in `internal/mcp/swarm_no_embeddings_test.go`: run all swarm operations with no embedding provider configured, verify `discover_agents` falls back to FTS and all other operations function correctly
- [ ] T033 Run `make lint` and fix any linting issues across all new files
- [ ] T034 Run `make test` and ensure all existing tests still pass (no regressions)

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — can start immediately
- **Foundational (Phase 2)**: Depends on Phase 1 completion — BLOCKS all user stories
- **US1 Stigmergy (Phase 3)**: Depends on Phase 2 — can start immediately after foundation
- **US4 Channel Type Enforcement (Phase 4)**: Depends on Phase 2 — can run in parallel with Phase 3
- **US2 Task Auction (Phase 5)**: Depends on Phase 2; benefits from Phase 4 (enforcement) being complete but T016 can integrate enforcement inline
- **US3 Agent Discovery (Phase 6)**: Depends on Phase 2 (agent store extensions); can run in parallel with Phases 3-5
- **Polish (Phase 7)**: Depends on all user stories being complete

### User Story Dependencies

- **US1 (P1)**: Can start after Phase 2 — no dependencies on other stories
- **US4 (P2)**: Can start after Phase 2 — no dependencies on other stories; provides enforcement used by US2
- **US2 (P2)**: Can start after Phase 2 — uses enforcement from US4 but can implement inline if US4 is not yet complete
- **US3 (P3)**: Can start after Phase 2 — fully independent of US1, US2, US4

### Within Each User Story

- Domain types (Phase 1) before store layer (Phase 2)
- Store layer before MCP tool implementation
- MCP tool implementation before MCP server registration
- Core logic before edge case handling
- All operations must emit traces before the story is considered complete

### Parallel Opportunities

- Phase 1: T002, T003, T004, T005 are all [P] — different files, no dependencies
- Phase 2: T007, T008, T009, T011 are all [P] — different packages
- Once Phase 2 completes: US1 (Phase 3), US4 (Phase 4), and US3 (Phase 6) can start in parallel
- US2 (Phase 5) can start in parallel but benefits from US4 being complete first
- Phase 7: T028, T029, T030, T031 are all [P] — independent test files
