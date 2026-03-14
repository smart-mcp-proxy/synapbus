# Tasks: MCP Auth, UX Polish & Agent Lifecycle

**Input**: Design documents from `/specs/002-mcp-auth-ux-polish/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Database migration and shared schema changes needed by multiple stories

- [x] T001 Create migration file `schema/008_dead_letters.sql` with dead_letters table and channels.is_system column per data-model.md
- [x] T002 Register migration 008 in storage initialization at `internal/storage/sqlite.go`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Backend service changes that multiple user stories depend on

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [x] T003 Add `is_system` field to Channel struct in `internal/channels/types.go`
- [x] T004 Modify channel store to persist and load `is_system` flag in `internal/channels/store.go`
- [x] T005 [P] Add dead letter store with Create, List, Acknowledge methods in `internal/messaging/deadletter_store.go`
- [x] T006 [P] Add dead letter types (DeadLetter struct, ListOptions) in `internal/messaging/types.go`

**Checkpoint**: Foundation ready — schema migrated, dead letter store available, is_system flag available

---

## Phase 3: User Story 1 — MCP Agent Connection with API Key (Priority: P1) 🎯 MVP

**Goal**: Verify API key auth works correctly for MCP connections, agent identity is enforced

**Independent Test**: Connect MCP client with valid/invalid API key, verify auth and tool access

### Implementation for User Story 1

- [x] T007 [US1] Add test in `internal/mcp/server_test.go` verifying MCP tool calls with valid API key resolve correct agent identity
- [x] T008 [US1] Add test in `internal/mcp/server_test.go` verifying MCP tool calls without auth return 401
- [x] T009 [US1] Verify `send_message` MCP tool enforces `from_agent` from authenticated agent (not user-supplied) in `internal/mcp/server.go`

**Checkpoint**: API key MCP auth verified and enforced

---

## Phase 4: User Story 2 — MCP Connection with OAuth 2.1 Fallback (Priority: P1)

**Goal**: Unauthenticated MCP clients get redirected to OAuth flow with agent selection

**Independent Test**: Connect without API key, complete OAuth in browser with agent selection, use resulting token

### Implementation for User Story 2

- [x] T010 [US2] Add `/.well-known/oauth-authorization-server` metadata endpoint in `internal/auth/handlers.go`
- [x] T011 [US2] Create OAuth authorize HTML template with login form + agent selector dropdown in `internal/auth/handlers.go` (inlined template)
- [x] T012 [US2] Implement GET `/oauth/authorize` handler serving the HTML page with agent list in `internal/auth/handlers.go`
- [x] T013 [US2] Implement POST `/oauth/authorize` handler processing login + agent selection + code issuance in `internal/auth/handlers.go`
- [x] T014 [US2] Modify fosite session to store `agent_name` in session_data in `internal/auth/fosite_store.go`
- [x] T015 [US2] Add `RequireBearer` middleware to extract agent_name from OAuth token session and set agent context in `internal/auth/middleware.go`
- [x] T016 [US2] Register a default MCP OAuth client on server startup (public client, PKCE S256) in `cmd/synapbus/main.go`
- [x] T017 [US2] Wire OAuth metadata + authorize endpoints and bearer middleware on `/mcp` route in `cmd/synapbus/main.go`
- [x] T018 [US2] Add test for OAuth metadata endpoint in `internal/auth/handlers_test.go`
- [x] T019 [US2] Add test for authorize page rendering with agent list in `internal/auth/handlers_test.go`

**Checkpoint**: OAuth 2.1 fallback flow functional — MCP clients without API key can authenticate via browser

---

## Phase 5: User Story 3 — Human Users Always Send as Human Account (Priority: P2)

**Goal**: Remove "Send as" dropdown, force human agent identity for all Web UI messages

**Independent Test**: Log into Web UI, send message in channel/DM, verify it's from human account

### Implementation for User Story 3

- [x] T020 [US3] Modify POST `/api/messages` handler to override `from` with user's human agent when session-authenticated in `internal/api/messages_handler.go`
- [x] T021 [US3] Add helper `GetHumanAgentForUser` to agent service in `internal/agents/service.go`
- [x] T022 [P] [US3] Remove "Send as" dropdown from channel page in `web/src/routes/channels/[name]/+page.svelte`
- [x] T023 [P] [US3] Remove "Send as" dropdown from DM page in `web/src/routes/dm/[name]/+page.svelte`
- [x] T024 [US3] Update ThreadPanel reply to use human agent (remove agent selection logic) in `web/src/lib/components/ThreadPanel.svelte`
- [x] T025 [US3] Add test verifying session-auth messages always use human agent in `internal/api/messages_handler_test.go`

**Checkpoint**: Web UI enforces human-only sending — no agent impersonation possible

---

## Phase 6: User Story 4 — Simplified Agent Management (Priority: P2)

**Goal**: Remove type selector from agent registration, keep AI/Human badges in displays

**Independent Test**: Register agent via UI, verify no type field and agent is type "ai"

### Implementation for User Story 4

- [x] T026 [P] [US4] Remove agent type selector from registration form in `web/src/routes/agents/+page.svelte`
- [x] T027 [P] [US4] Hardcode type="ai" in agent registration API call in `web/src/routes/agents/+page.svelte`
- [x] T028 [US4] Verify AI/Human badges still display correctly in `web/src/lib/components/AgentCard.svelte` (no changes needed, just verify)
- [x] T029 [US4] Verify AI/Human badges display in `web/src/lib/components/Sidebar.svelte` DM list (no changes needed, just verify)
- [x] T030 [US4] Verify AI/Human badges display in `web/src/lib/components/MessageList.svelte` (add badge if missing)

**Checkpoint**: Agent registration simplified — badges preserved throughout UI

---

## Phase 7: User Story 5 — "My Agents" Channel (Priority: P2)

**Goal**: Auto-create private "my-agents" channel per user, auto-join on agent registration

**Independent Test**: Register user, verify channel exists, create agent, verify auto-joined

### Implementation for User Story 5

- [x] T031 [US5] Add `EnsureMyAgentsChannel` method to channel service in `internal/channels/service.go` — creates private `my-agents-{username}` channel with `is_system=1` if not exists
- [x] T032 [US5] Call `EnsureMyAgentsChannel` during login flow (after EnsureHumanAgent) in `cmd/synapbus/main.go`
- [x] T033 [US5] Auto-join newly registered agent to owner's my-agents channel in `internal/api/agents_handler.go`
- [x] T034 [US5] Prevent deletion/leave of system channels in `internal/channels/service.go` LeaveChannel and any delete logic
- [x] T035 [US5] Filter `my-agents-*` channels to only show owner's own in channel listing API (already handled by private channel filtering)
- [x] T036 [US5] Display "My Agents" as channel display name for `my-agents-*` channels in `web/src/lib/components/Sidebar.svelte`
- [x] T037 [US5] Add test for EnsureMyAgentsChannel creation and idempotency in `internal/channels/service_test.go`
- [x] T038 [US5] Add test for auto-join on agent registration in `internal/agents/service_test.go`

**Checkpoint**: My-agents channel auto-creates and auto-populates — broadcast to all agents works

---

## Phase 8: User Story 6 — Dead Letter Queue (Priority: P3)

**Goal**: Capture unread messages on agent deletion, display in owner's DLQ view

**Independent Test**: Send message to agent, delete agent, verify dead letters visible

### Implementation for User Story 6

- [x] T039 [US6] Add dead letter capture logic to agent deregistration in `internal/agents/service.go` Deregister method — query pending/processing messages, insert into dead_letters
- [x] T040 [US6] Create dead letters REST handler with List and Acknowledge endpoints in `internal/api/deadletters_handler.go`
- [x] T041 [US6] Register dead letter API routes in `internal/api/router.go`
- [x] T042 [US6] Add dead letter API client methods (list, acknowledge) in `web/src/lib/api/client.ts`
- [x] T043 [US6] Create dead letters page in `web/src/routes/dead-letters/+page.svelte` — list with acknowledge buttons
- [x] T044 [US6] Add "Dead Letters" navigation item in `web/src/lib/components/Sidebar.svelte` with unacknowledged count badge
- [x] T045 [US6] Add test for dead letter capture on agent deletion in `internal/agents/service_test.go`
- [x] T046 [US6] Add test for dead letter list/acknowledge API in `internal/api/deadletters_handler_test.go`

**Checkpoint**: Dead letter queue captures unread messages, UI shows them, owner can acknowledge

---

## Phase 9: Polish & Cross-Cutting Concerns

**Purpose**: Build verification and final integration

- [x] T047 [P] Build Svelte SPA with `make web`
- [x] T048 [P] Run `make test` to verify no regressions
- [x] T049 Run `make build` to verify single binary compiles
- [x] T050 Run quickstart.md validation — test all features end-to-end
- [x] T051 Write `autonomous_summary.md` with completion status

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — start immediately
- **Foundational (Phase 2)**: Depends on Phase 1 (migration must exist)
- **US1 (Phase 3)**: Depends on Phase 2 — independent of other stories
- **US2 (Phase 4)**: Depends on Phase 2 — independent of other stories
- **US3 (Phase 5)**: Depends on Phase 2 — independent of other stories
- **US4 (Phase 6)**: Depends on Phase 2 — independent, frontend-only
- **US5 (Phase 7)**: Depends on Phase 2 (is_system flag) — independent
- **US6 (Phase 8)**: Depends on Phase 2 (dead letter store) — independent
- **Polish (Phase 9)**: Depends on all user stories

### User Story Dependencies

- **US1 (P1)**: No cross-story dependencies
- **US2 (P1)**: No cross-story dependencies (uses existing OAuth infrastructure)
- **US3 (P2)**: No cross-story dependencies
- **US4 (P2)**: No cross-story dependencies (frontend-only)
- **US5 (P2)**: No cross-story dependencies
- **US6 (P3)**: No cross-story dependencies

### Within Each User Story

- Models/types before services
- Services before API handlers
- Backend before frontend
- Core implementation before tests (tests validate the implementation)

### Parallel Opportunities

- T005 + T006 (dead letter store + types)
- T022 + T023 (remove send-as from channel + DM pages)
- T026 + T027 (agent form changes)
- T047 + T048 (web build + go test)
- All user stories (Phases 3-8) can run in parallel after Phase 2

---

## Implementation Strategy

### MVP First (User Stories 1 + 2 Only)

1. Complete Phase 1: Setup (migration)
2. Complete Phase 2: Foundational (types, stores)
3. Complete Phase 3: US1 — API key auth verified
4. Complete Phase 4: US2 — OAuth fallback working
5. **STOP and VALIDATE**: Both MCP auth paths work

### Incremental Delivery

1. Setup + Foundational → Foundation ready
2. US1 + US2 → MCP auth complete (MVP!)
3. US3 + US4 → UI polish (send-as removed, agent form simplified)
4. US5 → My-agents channel operational
5. US6 → Dead letter queue complete
6. Polish → Full verification

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Each user story is independently completable and testable
- Total tasks: 51 — ALL COMPLETED ✅
- Tasks per story: US1=3, US2=10, US3=6, US4=5, US5=8, US6=8, Setup=2, Foundation=4, Polish=5
