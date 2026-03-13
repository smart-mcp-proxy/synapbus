# Tasks: Core Messaging

**Input**: Design documents from `/specs/001-core-messaging/`
**Prerequisites**: spec.md (required), constitution.md (required)

**Tests**: Included per spec requirements (SC-008 mandates 80% unit test coverage; SC-002 requires concurrency tests).

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Go module dependencies and build configuration

- [ ] T001 Add `modernc.org/sqlite` dependency to `go.mod` (pure Go SQLite driver, zero CGO)
- [ ] T002 Add `mark3labs/mcp-go` dependency to `go.mod` (MCP server library)
- [ ] T003 [P] Verify `CGO_ENABLED=0 go build ./cmd/synapbus` compiles cleanly with new deps

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Storage layer, migration runner, domain types, and trace infrastructure that MUST be complete before ANY user story can be implemented

**CRITICAL**: No user story work can begin until this phase is complete

- [ ] T004 Implement SQLite connection manager in `internal/storage/sqlite.go`: open database, enable WAL mode, set `busy_timeout=5000`, set `foreign_keys=ON`, expose `*sql.DB`. Constructor takes `ctx context.Context` and `dataDir string`. Use `modernc.org/sqlite` driver.

- [ ] T005 Implement migration runner in `internal/storage/migrate.go`: read `schema_migrations` table, apply unapplied `.sql` files from `schema/` directory in version order, each migration runs in a transaction, create `schema_migrations` table if not exists. Function signature: `RunMigrations(ctx context.Context, db *sql.DB, schemaDir string) error`.

- [ ] T006 [P] Define core domain types in `internal/messaging/types.go`: `Message` struct (id, conversation_id, from_agent, to_agent, channel_id, body, priority, status, metadata as `json.RawMessage`, claimed_by, claimed_at, created_at, updated_at), `Conversation` struct (id, subject, created_by, channel_id, created_at, updated_at), `InboxState` struct (agent_name, conversation_id, last_read_message_id). Define `MessageStatus` string constants: `StatusPending`, `StatusProcessing`, `StatusDone`, `StatusFailed`.

- [ ] T007 [P] Define storage interface in `internal/storage/store.go`: `MessageStore` interface with methods matching all CRUD operations needed by user stories (InsertMessage, InsertConversation, FindConversation, GetInboxMessages, UpdateInboxState, ClaimMessages, UpdateMessageStatus, SearchMessages, GetMessageByID). Each method takes `ctx context.Context` as first parameter.

- [ ] T008 [P] Implement trace recorder in `internal/trace/trace.go`: `Recorder` struct backed by `*sql.DB`. Method `Record(ctx context.Context, agentName, action string, details json.RawMessage, traceErr error) error` inserts into `traces` table. Use `slog` to log each trace at Info level. (Satisfies FR-012)

- [ ] T009 [P] Write table-driven unit tests for migration runner in `internal/storage/migrate_test.go`: test fresh DB (no schema_migrations table), test idempotency (run twice, no errors), test sequential ordering. Uses in-memory SQLite. (Satisfies SC-006)

- [ ] T010 [P] Write unit tests for trace recorder in `internal/trace/trace_test.go`: test successful recording, test recording with error field, verify row content in traces table.

- [ ] T011 Wire storage initialization into `cmd/synapbus/main.go` `runServe`: open SQLite DB via `storage.New()`, run migrations, defer close. Replace TODO placeholder. Log startup with `slog`.

**Checkpoint**: Foundation ready -- SQLite opens in WAL mode, migrations run, domain types defined, trace recorder works. User story implementation can now begin.

---

## Phase 3: User Story 1 -- Agent Sends a DM and Recipient Reads It (Priority: P1)

**Goal**: Two agents can exchange direct messages. Sender calls `send_message`, recipient calls `read_inbox`. Conversations are auto-created. Read/unread tracking works.

**Independent Test**: Register two agents, send message from one to the other via `send_message` MCP tool, then call `read_inbox` from the recipient. Verify message content, priority, status, and read/unread behavior.

### Tests for User Story 1

> **NOTE: Write these tests FIRST, ensure they FAIL before implementation**

- [ ] T012 [P] [US1] Write table-driven tests for `MessageService.SendDirectMessage` in `internal/messaging/service_test.go`: test successful send creates conversation + message, test send to non-existent agent returns error, test send with empty body returns error, test send with invalid priority returns error, test send with invalid metadata JSON returns error, test send with both to_agent and channel_id returns error, test second message with same subject+recipient reuses conversation. (Covers acceptance scenarios 1, 3 and edge cases)

- [ ] T013 [P] [US1] Write table-driven tests for `MessageService.ReadInbox` in `internal/messaging/service_test.go`: test returns unread messages ordered by priority desc then created_at asc, test advances last_read_message_id after read, test second read returns empty when include_read=false, test second read returns messages when include_read=true, test filters by status/from_agent/conversation_id/min_priority, test respects limit parameter. (Covers acceptance scenarios 2, 4)

- [ ] T014 [P] [US1] Write integration test in `internal/messaging/integration_test.go`: end-to-end test that creates a MessageStore with in-memory SQLite, sends a DM, reads inbox, verifies full round-trip. Verify trace entries exist for each operation. (Satisfies SC-001, SC-007)

### Implementation for User Story 1

- [ ] T015 [US1] Implement `MessageStore` SQLite methods for DM send in `internal/storage/sqlite_messages.go`: `InsertConversation(ctx, conv)`, `FindConversationBySubjectAndParties(ctx, subject, fromAgent, toAgent)`, `InsertMessage(ctx, msg)`, `AgentExists(ctx, agentName) (bool, error)`. All use prepared statements. (Satisfies FR-001, FR-002, FR-008)

- [ ] T016 [US1] Implement `MessageStore` SQLite methods for inbox read in `internal/storage/sqlite_messages.go`: `GetInboxMessages(ctx, agentName, filters)` queries messages where `to_agent=agentName` AND `id > last_read_message_id` (unless include_read), ordered by `priority DESC, created_at ASC`, with limit. `UpdateInboxState(ctx, agentName, conversationID, lastReadMsgID)` upserts into `inbox_state`. Define `InboxFilters` struct with optional fields: Status, FromAgent, ConversationID, MinPriority, Limit, IncludeRead. (Satisfies FR-003, FR-007)

- [ ] T017 [US1] Implement `MessageService` in `internal/messaging/service.go`: business logic layer wrapping `MessageStore`. Methods: `SendMessage(ctx, params) (*Message, error)` -- validates inputs (body not empty, priority 1-10, metadata valid JSON, exactly one of to_agent/channel_id, agent exists), finds or creates conversation, inserts message, records trace. `ReadInbox(ctx, agentName, filters) ([]Message, error)` -- fetches messages, advances inbox_state, records trace. Service holds `MessageStore` interface and `trace.Recorder`. (Satisfies FR-001, FR-002, FR-003, FR-007, FR-011, FR-012, FR-013)

- [ ] T018 [US1] Implement `send_message` MCP tool in `internal/mcp/tools.go`: register MCP tool with JSON Schema for parameters (to_agent, channel_id, body, subject, priority, metadata). Handler extracts calling agent identity from context, delegates to `MessageService.SendMessage`, returns structured JSON response with message_id and conversation_id. (Satisfies FR-001, SC-004)

- [ ] T019 [US1] Implement `read_inbox` MCP tool in `internal/mcp/tools.go`: register MCP tool with JSON Schema for parameters (status, from_agent, conversation_id, min_priority, limit, include_read). Handler extracts calling agent identity, delegates to `MessageService.ReadInbox`, returns structured JSON with messages array. (Satisfies FR-003, SC-004)

- [ ] T020 [US1] Implement MCP server bootstrap in `internal/mcp/server.go`: create `mcp-go` server instance, register tools, expose SSE transport endpoint. Constructor takes `MessageService` and returns configured server. Define `AgentFromContext(ctx) string` helper for extracting agent identity.

- [ ] T021 [US1] Wire MCP server into `cmd/synapbus/main.go`: create `MessageService` with `MessageStore` and `trace.Recorder`, create MCP server, mount SSE endpoint on chi router at `/mcp`, start HTTP server.

**Checkpoint**: At this point, two agents can send DMs and read their inboxes via MCP tools. Conversations auto-create. Read/unread tracking works. All acceptance scenarios for US1 verified.

---

## Phase 4: User Story 2 -- Agent Claims and Processes Messages (Priority: P1)

**Goal**: Agents can atomically claim pending messages and mark them done or failed. Prevents duplicate processing.

**Independent Test**: Send a message, claim it with `claim_messages`, verify status changes to "processing", call `mark_done`, verify status changes to "done". Test concurrent claims to verify atomicity.

### Tests for User Story 2

> **NOTE: Write these tests FIRST, ensure they FAIL before implementation**

- [ ] T022 [P] [US2] Write table-driven tests for `MessageService.ClaimMessages` in `internal/messaging/service_test.go`: test claim by explicit message_ids, test claim by limit, test claimed messages have status "processing" and claimed_by set, test already-claimed messages are skipped, test no pending messages returns empty list (not error), test claims are atomic (single UPDATE). (Covers acceptance scenarios 1, 4 and edge cases)

- [ ] T023 [P] [US2] Write table-driven tests for `MessageService.MarkDone` in `internal/messaging/service_test.go`: test mark as "done", test mark as "failed" with error metadata merge, test wrong agent cannot mark done, test already-done message returns error, test non-existent message returns error. (Covers acceptance scenarios 2, 3, 5 and edge cases)

- [ ] T024 [P] [US2] Write concurrency test in `internal/messaging/concurrency_test.go`: 10 goroutines simultaneously claim the same set of pending messages, verify each message is claimed by exactly one agent. Uses real SQLite (not mock) with WAL mode. (Satisfies SC-002)

### Implementation for User Story 2

- [ ] T025 [US2] Implement `MessageStore` SQLite methods for claim in `internal/storage/sqlite_messages.go`: `ClaimMessages(ctx, agentName, messageIDs []int64, limit int) ([]Message, error)` -- single `UPDATE messages SET status='processing', claimed_by=?, claimed_at=? WHERE status='pending' AND to_agent=?` with either `id IN (?)` or `LIMIT ?`. Return updated rows. (Satisfies FR-004)

- [ ] T026 [US2] Implement `MessageStore` SQLite methods for mark_done in `internal/storage/sqlite_messages.go`: `GetMessageByID(ctx, id) (*Message, error)`, `UpdateMessageStatus(ctx, id, status, claimedBy, metadata) error` -- verifies claimed_by matches, merges metadata JSON, updates status and updated_at. (Satisfies FR-005)

- [ ] T027 [US2] Implement `MessageService.ClaimMessages` and `MessageService.MarkDone` in `internal/messaging/service.go`: validation (status transitions, ownership checks), delegation to store, trace recording. (Satisfies FR-004, FR-005, FR-011, FR-012)

- [ ] T028 [US2] Implement `claim_messages` MCP tool in `internal/mcp/tools.go`: register tool with JSON Schema for parameters (message_ids, limit). Handler delegates to `MessageService.ClaimMessages`. (Satisfies FR-004, SC-004)

- [ ] T029 [US2] Implement `mark_done` MCP tool in `internal/mcp/tools.go`: register tool with JSON Schema for parameters (message_id, status, metadata). Handler delegates to `MessageService.MarkDone`. (Satisfies FR-005, SC-004)

**Checkpoint**: At this point, the full claim-and-process workflow works. Concurrent claims are safe. All acceptance scenarios for US2 verified.

---

## Phase 5: User Story 3 -- Agent Sends a Channel Message (Priority: P2)

**Goal**: An agent broadcasts a message to a channel. All channel members see it in their inboxes.

**Independent Test**: Create a channel with two members, send a message to the channel, verify both members see it in `read_inbox`. Verify non-members cannot send.

### Tests for User Story 3

> **NOTE: Write these tests FIRST, ensure they FAIL before implementation**

- [ ] T030 [P] [US3] Write table-driven tests for channel message sending in `internal/messaging/service_test.go`: test send to channel creates message visible to all members, test non-member cannot send (permission error), test channel message has null to_agent, test each member's inbox_state is independent. (Covers acceptance scenarios 1, 2, 3)

- [ ] T031 [P] [US3] Write tests for channel helpers in `internal/storage/sqlite_channels_test.go`: test `GetChannelMembers`, test `IsChannelMember`.

### Implementation for User Story 3

- [ ] T032 [US3] Implement channel query methods in `internal/storage/sqlite_channels.go`: `GetChannelByID(ctx, id) (*Channel, error)`, `GetChannelMembers(ctx, channelID) ([]string, error)`, `IsChannelMember(ctx, channelID, agentName) (bool, error)`. Define `Channel` struct in `internal/messaging/types.go` if not already present.

- [ ] T033 [US3] Extend `MessageService.SendMessage` in `internal/messaging/service.go` to handle channel messages: when `channel_id` is set, verify sender is a member, create message with null `to_agent`, ensure all members see the message in `read_inbox` by querying channel membership during inbox read.

- [ ] T034 [US3] Extend `GetInboxMessages` in `internal/storage/sqlite_messages.go` to include channel messages: query messages where `to_agent=agentName` OR (`channel_id IN (SELECT channel_id FROM channel_members WHERE agent_name=agentName)` AND `from_agent != agentName`). Ensure read/unread state is per-agent. (Satisfies FR-003, FR-013)

- [ ] T035 [US3] Update `send_message` MCP tool schema in `internal/mcp/tools.go` to document `channel_id` parameter (already defined in FR-001 but implementation was DM-only in Phase 3).

**Checkpoint**: At this point, both DM and channel messaging work. All acceptance scenarios for US3 verified.

---

## Phase 6: User Story 4 -- Agent Searches Message History (Priority: P2)

**Goal**: Agents can search past messages by keyword (FTS5), sender, priority, status, and time range. Results scoped to accessible messages.

**Independent Test**: Insert varied messages, call `search_messages` with a query string, verify relevant results ranked by FTS5 relevance and scoped to the agent's accessible messages.

### Tests for User Story 4

> **NOTE: Write these tests FIRST, ensure they FAIL before implementation**

- [ ] T036 [P] [US4] Write table-driven tests for `MessageService.SearchMessages` in `internal/messaging/service_test.go`: test FTS5 keyword match, test empty query returns recent messages, test min_priority filter, test from_agent filter, test access scoping (cannot search other agent's DMs), test limit parameter. (Covers acceptance scenarios 1, 2, 3 and edge cases)

- [ ] T037 [P] [US4] Write performance benchmark in `internal/messaging/bench_test.go`: insert 10,000 messages, search by keyword, assert latency under 200ms. (Satisfies SC-003)

### Implementation for User Story 4

- [ ] T038 [US4] Implement `SearchMessages` in `internal/storage/sqlite_messages.go`: build query joining `messages` with `messages_fts` when query is non-empty (using `messages_fts MATCH ?` with `rank` ordering), apply optional filters (from_agent, to_agent, channel_id, min_priority, status), scope to accessible messages (agent's DMs + joined channels), apply limit. When query is empty, return recent messages with filters applied. (Satisfies FR-006)

- [ ] T039 [US4] Implement `MessageService.SearchMessages` in `internal/messaging/service.go`: validate inputs, delegate to store, record trace. (Satisfies FR-006, FR-012)

- [ ] T040 [US4] Implement `search_messages` MCP tool in `internal/mcp/tools.go`: register tool with JSON Schema for parameters (query, from_agent, to_agent, channel_id, min_priority, status, limit). Handler delegates to `MessageService.SearchMessages`. (Satisfies FR-006, SC-004)

**Checkpoint**: At this point, full-text search works across DMs and channel messages with access scoping. All acceptance scenarios for US4 verified.

---

## Phase 7: User Story 5 -- Conversation Threading and Metadata (Priority: P3)

**Goal**: Messages are grouped into conversation threads. Metadata is carried as JSON and available for filtering. Conversations auto-create or reuse based on subject.

**Independent Test**: Send messages with metadata, filter inbox by conversation_id, verify threading and metadata roundtrip.

### Tests for User Story 5

> **NOTE: Write these tests FIRST, ensure they FAIL before implementation**

- [ ] T041 [P] [US5] Write table-driven tests for conversation threading in `internal/messaging/service_test.go`: test auto-create conversation on new subject, test reuse conversation on matching subject+parties, test explicit conversation_id parameter, test metadata JSON roundtrip (stored and returned correctly), test filter by conversation_id returns threaded messages in chronological order. (Covers acceptance scenarios 1, 2)

### Implementation for User Story 5

- [ ] T042 [US5] Extend `FindConversationBySubjectAndParties` in `internal/storage/sqlite_messages.go` to handle edge cases: empty subject creates new conversation per message (unless conversation_id provided), subject matching is exact. Verify metadata JSON column roundtrip (store as TEXT, parse as `json.RawMessage` on read).

- [ ] T043 [US5] Extend `ReadInbox` in `internal/messaging/service.go` to support `conversation_id` filter that returns all messages in the thread (chronological order), including metadata. Ensure metadata is parsed and returned in MCP tool response.

- [ ] T044 [US5] Update `read_inbox` MCP tool response schema in `internal/mcp/tools.go` to include `conversation_subject` and `metadata` fields in each returned message (verify these are already present from earlier phases; add if missing).

**Checkpoint**: All user stories (US1-US5) are independently functional. Conversation threading and metadata enrichment work end-to-end.

---

## Phase 8: Polish & Cross-Cutting Concerns

**Purpose**: Improvements that affect multiple user stories

- [ ] T045 [P] Input validation hardening in `internal/messaging/service.go`: audit all validation paths -- empty body (whitespace-only check), priority range, metadata JSON parse, agent existence check, message status transitions. Ensure all return structured error messages per FR-011.

- [ ] T046 [P] Add `slog` structured logging throughout: `internal/storage/sqlite.go` (DB open, WAL mode, migrations), `internal/messaging/service.go` (send, read, claim, mark_done, search with agent name and params), `internal/mcp/server.go` (tool registration, request handling). Use `slog.With("agent", agentName)` for per-agent context. (Satisfies Principle VIII)

- [ ] T047 [P] Write unit tests for MCP tool input validation in `internal/mcp/tools_test.go`: test each tool with missing required params, invalid types, boundary values. Verify structured error responses.

- [ ] T048 [P] Write integration test suite in `internal/messaging/integration_test.go`: full lifecycle test covering all 5 user stories sequentially -- register agents, send DM, read inbox, claim, mark done, send channel message, search, verify traces. Uses in-memory SQLite. (Satisfies SC-004, SC-007)

- [ ] T049 [P] Add `Makefile` target `test-coverage` that runs `go test ./... -coverprofile=coverage.out` and verifies `internal/messaging/` package has >= 80% coverage. (Satisfies SC-008)

- [ ] T050 Run `make lint` and fix any issues. Run `make test` and verify all tests pass. Run `make build` with `CGO_ENABLED=0` for `darwin/arm64` and `linux/amd64` to verify cross-compilation. (Satisfies Principle III)

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies -- can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion -- BLOCKS all user stories
- **User Story 1 (Phase 3)**: Depends on Foundational phase completion
- **User Story 2 (Phase 4)**: Depends on Foundational phase completion. Can run in parallel with US1 (different methods in same files, but shared service.go may cause merge conflicts -- recommend sequential after US1)
- **User Story 3 (Phase 5)**: Depends on US1 completion (extends SendMessage and GetInboxMessages)
- **User Story 4 (Phase 6)**: Depends on Foundational phase completion. Can run in parallel with US1/US2 (separate methods/files)
- **User Story 5 (Phase 7)**: Depends on US1 completion (extends conversation and metadata handling)
- **Polish (Phase 8)**: Depends on all user stories being complete

### User Story Dependencies

- **US1 (P1)**: Can start after Foundational (Phase 2) -- No dependencies on other stories
- **US2 (P1)**: Can start after Foundational (Phase 2) -- Independent of US1 (different methods), but recommended after US1 to avoid merge conflicts in `service.go`
- **US3 (P2)**: Depends on US1 -- extends `SendMessage` and `GetInboxMessages` with channel support
- **US4 (P2)**: Can start after Foundational (Phase 2) -- Independent search implementation, but benefits from US1 test data setup patterns
- **US5 (P3)**: Depends on US1 -- extends conversation auto-creation and metadata handling

### Within Each User Story

- Tests MUST be written and FAIL before implementation
- Storage layer (sqlite_messages.go) before service layer (service.go)
- Service layer before MCP tool layer (tools.go)
- Core implementation before wiring/integration

### Parallel Opportunities

- All Setup tasks (T001-T003) can run in parallel
- Foundational tasks T006, T007, T008, T009, T010 can all run in parallel (different files)
- Once Foundational completes: US1 and US4 can proceed in parallel (different files)
- Within each user story: test tasks marked [P] can run in parallel
- Phase 8 polish tasks marked [P] can all run in parallel

---

## Implementation Strategy

### MVP First (User Stories 1 + 2 Only)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational (CRITICAL -- blocks all stories)
3. Complete Phase 3: User Story 1 (send + read)
4. **STOP and VALIDATE**: Test US1 independently
5. Complete Phase 4: User Story 2 (claim + mark_done)
6. **STOP and VALIDATE**: Test US2 independently
7. Deploy/demo: two agents can send, read, claim, and process messages

### Incremental Delivery

1. Setup + Foundational -> Foundation ready
2. US1 -> Send + Read DMs (MVP!)
3. US2 -> Claim + Process workflow
4. US3 -> Channel broadcasting
5. US4 -> Full-text search
6. US5 -> Threading + metadata enrichment
7. Polish -> Hardening, coverage, cross-compilation

---

## Key File Map

| File | Purpose | Created In |
|------|---------|------------|
| `internal/storage/sqlite.go` | SQLite connection, WAL, busy_timeout | T004 |
| `internal/storage/migrate.go` | Migration runner | T005 |
| `internal/storage/store.go` | `MessageStore` interface | T007 |
| `internal/storage/sqlite_messages.go` | Message/conversation/inbox CRUD | T015, T016, T025, T026, T038 |
| `internal/storage/sqlite_channels.go` | Channel query methods | T032 |
| `internal/messaging/types.go` | Domain structs + constants | T006 |
| `internal/messaging/service.go` | `MessageService` business logic | T017, T027, T033, T039, T043 |
| `internal/messaging/service_test.go` | Unit tests for all service methods | T012, T013, T022, T023, T030, T036, T041 |
| `internal/messaging/concurrency_test.go` | Concurrent claim test | T024 |
| `internal/messaging/integration_test.go` | End-to-end integration tests | T014, T048 |
| `internal/messaging/bench_test.go` | FTS5 performance benchmark | T037 |
| `internal/trace/trace.go` | Trace recorder | T008 |
| `internal/trace/trace_test.go` | Trace recorder tests | T010 |
| `internal/mcp/server.go` | MCP server bootstrap | T020 |
| `internal/mcp/tools.go` | MCP tool registrations + handlers | T018, T019, T028, T029, T035, T040 |
| `internal/mcp/tools_test.go` | MCP tool validation tests | T047 |
| `cmd/synapbus/main.go` | CLI + server wiring | T011, T021 |
| `schema/001_initial.sql` | Database schema (already exists) | -- |

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Each user story should be independently completable and testable
- Verify tests fail before implementing
- Commit after each task or logical group
- Stop at any checkpoint to validate story independently
- All SQL operations use `context.Context` for cancellation support
- `modernc.org/sqlite` is the ONLY SQLite driver allowed (zero CGO, Principle III)
- `json.RawMessage` for metadata fields to avoid double-marshaling
