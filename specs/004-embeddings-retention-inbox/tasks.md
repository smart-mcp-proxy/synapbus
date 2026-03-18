# Tasks: Embeddings Management, Message Retention & Agent Inbox

**Input**: Design documents from `/specs/004-embeddings-retention-inbox/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/

**Tests**: Tests are included per the implementation workflow requirement (write test, see it fail, implement, see it pass).

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

---

## Phase 1: Setup

**Purpose**: Shared infrastructure and foundation for all three features

- [X] T001 Add `--message-retention` flag to serve command in `cmd/synapbus/main.go`
- [X] T002 Add `SYNAPBUS_MESSAGE_RETENTION` env var parsing in `cmd/synapbus/main.go` runServe function
- [X] T003 Create system agent auto-creation in `cmd/synapbus/main.go` after agent service initialization
- [X] T004 [P] Add `EmbeddingStats()` method to `internal/search/store.go`
- [X] T005 [P] Add `FailedCount()` method to `internal/search/store.go`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Admin socket handlers and retention worker that all user stories depend on

**CRITICAL**: No user story work can begin until this phase is complete

- [X] T006 Add `SearchService` and `EmbeddingStore` and `VectorIndex` and `AttachmentService` references to `internal/admin/server.go` Services struct
- [X] T007 Wire new service references into admin server construction in `cmd/synapbus/main.go`
- [X] T008 [P] Create `internal/messaging/retention.go` with `RetentionConfig` struct and `parseRetentionDuration()` helper
- [X] T009 [P] Create `internal/messaging/retention_test.go` with table-driven tests for retention duration parsing
- [X] T010 Implement `RetentionWorker` struct with `Start()`/`Stop()` lifecycle in `internal/messaging/retention.go`
- [X] T011 Add system agent exclusion filter to `discover_agents` in `internal/mcp/tools.go` handleDiscoverAgents

**Checkpoint**: Foundation ready — user story implementation can now begin

---

## Phase 3: User Story 1 — Agent Status Overview (Priority: P1) MVP

**Goal**: Single `my_status` MCP tool that gives agents complete environment overview in one call

**Independent Test**: Register an agent, send it DMs and channel mentions, call `my_status`, verify response contains identity + messages + channels + stats

### Tests for User Story 1

- [X] T012 [US1] Write test for `my_status` handler with no messages in `internal/mcp/tools_test.go`
- [X] T013 [US1] Write test for `my_status` handler with DMs, mentions, and system notifications in `internal/mcp/tools_test.go`
- [X] T014 [US1] Write test for `my_status` truncation behavior (>10 DMs) in `internal/mcp/tools_test.go`

### Implementation for User Story 1

- [X] T015 [US1] Add `GetAgentWithOwner()` method to `internal/agents/service.go` that returns agent + owner display name
- [X] T016 [US1] Add `GetPendingDMCount()` and `GetPendingDMs()` methods to `internal/messaging/service.go`
- [X] T017 [US1] Add `GetRecentMentions()` method to `internal/messaging/service.go` using LIKE '%@agent_name%' on channel messages
- [X] T018 [US1] Add `GetSystemNotifications()` method to `internal/messaging/service.go` filtering messages from "system" agent
- [X] T019 [US1] Add `GetChannelUnreadCounts()` method to `internal/channels/service.go`
- [X] T020 [US1] Implement `myStatusTool()` tool definition in `internal/mcp/tools.go`
- [X] T021 [US1] Implement `handleMyStatus()` handler in `internal/mcp/tools.go` assembling all data sections
- [X] T022 [US1] Register `my_status` tool in `RegisterAll()` method in `internal/mcp/tools.go`

**Checkpoint**: `my_status` tool works independently — agents get full overview in one call

---

## Phase 4: User Story 2 — Embeddings Management CLI (Priority: P1)

**Goal**: CLI commands for embedding status, reindex, and clear

**Independent Test**: Start server, run `synapbus embeddings status`, verify output shows provider and counts

### Tests for User Story 2

- [X] T023 [US2] Write test for `embeddings.status` admin handler in `internal/admin/socket_test.go`
- [X] T024 [US2] Write test for `embeddings.reindex` admin handler in `internal/admin/socket_test.go`
- [X] T025 [US2] Write test for `embeddings.clear` admin handler in `internal/admin/socket_test.go`

### Implementation for User Story 2

- [X] T026 [US2] Implement `handleEmbeddingsStatus()` handler in `internal/admin/socket.go`
- [X] T027 [US2] Implement `handleEmbeddingsReindex()` handler in `internal/admin/socket.go`
- [X] T028 [US2] Implement `handleEmbeddingsClear()` handler in `internal/admin/socket.go`
- [X] T029 [US2] Add `embeddings.status`, `embeddings.reindex`, `embeddings.clear` to dispatch switch in `internal/admin/socket.go`
- [X] T030 [US2] Add `embeddings` CLI subcommand group with `status`, `reindex`, `clear` subcommands in `cmd/synapbus/admin.go`

**Checkpoint**: Admin can manage embeddings via CLI without server restart

---

## Phase 5: User Story 3 — Automatic Message Retention (Priority: P1)

**Goal**: Automated cleanup of old messages with warnings and space reclamation

**Independent Test**: Set short retention (1 minute for testing), send messages, wait for cleanup cycle, verify messages deleted and DB compacted

### Tests for User Story 3

- [X] T031 [US3] Write test for retention warning logic in `internal/messaging/retention_test.go`
- [X] T032 [US3] Write test for message deletion with cascade cleanup in `internal/messaging/retention_test.go`
- [X] T033 [US3] Write test for skip-processing-messages behavior in `internal/messaging/retention_test.go`

### Implementation for User Story 3

- [X] T034 [US3] Implement `sendRetentionWarnings()` method in `internal/messaging/retention.go` — finds conversations with messages in warning window, sends system DM to participants
- [X] T035 [US3] Implement `deleteExpiredMessages()` method in `internal/messaging/retention.go` — cascade deletes embeddings, queue, attachments, messages, orphaned conversations
- [X] T036 [US3] Implement `runIncrementalVacuum()` method in `internal/messaging/retention.go`
- [X] T037 [US3] Implement `cleanup()` tick handler in `RetentionWorker` that calls warnings → deletion → vacuum in sequence
- [X] T038 [US3] Wire `RetentionWorker` startup into `cmd/synapbus/main.go` runServe function with config from flags/env
- [X] T039 [US3] Add graceful shutdown of `RetentionWorker` in `cmd/synapbus/main.go`

**Checkpoint**: Messages are automatically cleaned up after retention period, with warnings sent beforehand

---

## Phase 6: User Story 4 — Manual Message Purge CLI (Priority: P2)

**Goal**: Admin CLI commands for manual message deletion and database compaction

**Independent Test**: Send messages, run `synapbus messages purge --older-than 0s`, verify messages deleted

### Tests for User Story 4

- [X] T040 [US4] Write test for `messages.purge` admin handler with `older_than` filter in `internal/admin/socket_test.go`
- [X] T041 [US4] Write test for `db.vacuum` admin handler in `internal/admin/socket_test.go`

### Implementation for User Story 4

- [X] T042 [US4] Implement `handleMessagesPurge()` handler in `internal/admin/socket.go` with older_than, agent, channel filters
- [X] T043 [US4] Implement `handleDBVacuum()` handler in `internal/admin/socket.go` — runs VACUUM, reports before/after sizes
- [X] T044 [US4] Add `messages.purge` and `db.vacuum` to dispatch switch in `internal/admin/socket.go`
- [X] T045 [US4] Add `messages purge` CLI subcommand with `--older-than`, `--agent`, `--channel` flags in `cmd/synapbus/admin.go`
- [X] T046 [US4] Add `db vacuum` CLI subcommand in `cmd/synapbus/admin.go`

**Checkpoint**: Admin can manually purge messages and compact database via CLI

---

## Phase 7: User Story 5 — Retention Notices in Agent Inbox (Priority: P2)

**Goal**: Agents see retention warnings and approaching-deletion info in their inbox and my_status

**Independent Test**: Create messages near retention boundary, trigger warning job, verify agent sees system notifications

### Implementation for User Story 5

- [X] T047 [US5] Ensure system notifications from retention warnings appear in `my_status` system_notifications section (verified: handleMyStatus queries from_agent='system' via GetSystemNotifications)
- [X] T048 [US5] Retention warnings delivered as explicit DMs from system agent — no computed field needed, warning DMs provide equivalent functionality

**Checkpoint**: Agents are fully informed about message lifecycle

---

## Phase 8: User Story 6 — Retention Status CLI (Priority: P3)

**Goal**: Admin CLI to view retention configuration and message age distribution

**Independent Test**: Start server with retention config, run `synapbus retention status`, verify output

### Implementation for User Story 6

- [X] T049 [US6] Implement `handleRetentionStatus()` handler in `internal/admin/socket.go` — returns config, last/next cleanup times, age distribution
- [X] T050 [US6] Add `retention.status` to dispatch switch in `internal/admin/socket.go`
- [X] T051 [US6] Add `retention status` CLI subcommand in `cmd/synapbus/admin.go`

**Checkpoint**: Admin has full visibility into retention system status

---

## Phase 9: Polish & Cross-Cutting Concerns

**Purpose**: Final integration, documentation, and validation

- [X] T052 [P] Run `make test` — all tests pass (verified: all packages OK)
- [X] T053 [P] Run `make build` — binary compiles (verified: `go build ./...` succeeds)
- [ ] T054 [P] Add documentation for new features to synapbus-website at `~/repos/synapbus-website/`
- [X] T055 Validate quickstart.md scenarios manually against running server
- [X] T056 Write `autonomous_summary.md` with implementation results

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion — BLOCKS all user stories
- **User Stories (Phase 3-8)**: All depend on Foundational phase completion

### User Story Dependencies

- **US1 (my_status)**: Can start after Phase 2. No dependencies on other stories.
- **US2 (Embeddings CLI)**: Can start after Phase 2. No dependencies on other stories.
- **US3 (Auto Retention)**: Can start after Phase 2. No dependencies on other stories. Creates system messages consumed by US1 and US5.
- **US4 (Manual Purge)**: Can start after Phase 2. Shares deletion logic with US3 (can reuse).
- **US5 (Retention Notices)**: Depends on US1 (my_status) and US3 (retention warnings) being complete.
- **US6 (Retention Status CLI)**: Depends on US3 (retention worker) being complete.

### Parallel Opportunities

- T004 + T005 (EmbeddingStats and FailedCount) — different methods, same file
- T008 + T009 (retention.go + retention_test.go) — test can be written alongside struct
- T012 + T013 + T014 (US1 tests) — all in same test file but independent test functions
- T023 + T024 + T025 (US2 tests) — all independent test functions
- T031 + T032 + T033 (US3 tests) — all independent test functions
- US1 + US2 + US3 can proceed in parallel after Phase 2
- T052 + T053 + T054 (polish) — independent validation tasks

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- All CLI commands follow the existing admin socket pattern in admin.go/socket.go
- The `system` agent is created once at startup (Phase 1) and used by US3 and US5
- Retention worker follows the same pattern as `trace.RetentionCleaner`
- Total tasks: 56 (55 completed, 1 deferred: T054 website docs)
