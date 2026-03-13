# Tasks: Agent Registry & Auth

**Input**: Design documents from `/specs/002-agent-registry/`
**Prerequisites**: spec.md (required)

**Tests**: Included per spec requirements (SC-004, SC-005 mandate integration tests with 100% pass rate and negative-path access-violation tests).

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3, US4)
- Include exact file paths in descriptions

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Project initialization, schema migration, and base types

- [ ] T001 Create SQLite migration `schema/002_agent_registry.sql`: add `deregistered_at TIMESTAMP` column to `agents` table, add case-insensitive unique index on `LOWER(agents.name)`, add index on `agents.capabilities` for FTS on the skills JSON field. The `agents` table already exists in `001_initial.sql` so this is an ALTER migration.
- [ ] T002 [P] Define Agent domain model in `internal/agents/agent.go`: `Agent` struct (ID, Name, DisplayName, Type, Capabilities, OwnerID, APIKeyHash, Status, CreatedAt, UpdatedAt, DeregisteredAt), `CapabilityCard` struct (Skills []string, Description string, InputFormats []string, OutputFormats []string, Version string), validation constants (name regex `^[a-z0-9][a-z0-9._-]{0,62}[a-z0-9]$`, max capabilities size 64KB), and agent status constants.
- [ ] T003 [P] Define request/response types in `internal/agents/dto.go`: `RegisterRequest`, `RegisterResponse` (agent_id, api_key, name, created_at), `UpdateRequest` (display_name, capabilities, rotate_key), `UpdateResponse`, `DeregisterRequest`, `DiscoverRequest` (capability, type, owner_id, limit, cursor), `DiscoverResponse` (agents, total_count, next_cursor).
- [ ] T004 [P] Define agent-related error types in `internal/agents/errors.go`: sentinel errors for `ErrAgentNameTaken`, `ErrAgentNotFound`, `ErrInvalidAgentName`, `ErrCapabilitiesTooLarge`, `ErrInvalidCapabilities`, `ErrForbidden`, `ErrUnauthorized`, `ErrAgentDeregistered`. Each error should carry a machine-readable code string (e.g., `agent_name_taken`) for MCP error responses.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core infrastructure that MUST be complete before ANY user story can be implemented

**CRITICAL**: No user story work can begin until this phase is complete

- [ ] T005 Implement API key generation in `internal/auth/apikey.go`: `GenerateAPIKey() (raw string, hash string, error)` using `crypto/rand` for 256 bits of entropy, prefix `sk-synapbus-`, base64url encoding. `HashAPIKey(raw string) (string, error)` using bcrypt. `VerifyAPIKey(raw, hash string) bool` using bcrypt.CompareHashAndPassword. Include table-driven tests in `internal/auth/apikey_test.go` covering generation uniqueness, hash verification, invalid key rejection.
- [ ] T006 [P] Implement agent SQLite repository in `internal/agents/repository.go`: `Repository` interface with methods `Create(ctx, Agent) error`, `GetByName(ctx, name) (Agent, error)`, `GetByID(ctx, id) (Agent, error)`, `Update(ctx, Agent) error`, `SoftDelete(ctx, name) error`, `FindByCapability(ctx, keyword, limit, cursor) ([]Agent, int, string, error)`, `FindAll(ctx, limit, cursor) ([]Agent, int, string, error)`, `FindByOwner(ctx, ownerID) ([]Agent, error)`, `SoftDeleteByOwner(ctx, ownerID) error`. Implement `SQLiteRepository` struct using `modernc.org/sqlite`. Cursor-based pagination with default limit 50, max 200. All queries filter `status = 'active'` unless explicitly requesting inactive.
- [ ] T007 [P] Implement agent repository tests in `internal/agents/repository_test.go`: table-driven tests for all repository methods. Test cases: create agent, duplicate name rejection (case-insensitive), get by name, get by ID, update capabilities, soft-delete, find by capability keyword, pagination (limit/cursor), find by owner, cascade soft-delete by owner, get deregistered agent returns error.
- [ ] T008 Implement authentication middleware in `internal/auth/middleware.go`: `AgentAuthMiddleware` that extracts `Authorization: Bearer <key>` from MCP request headers, looks up active agents by verifying the key against all stored bcrypt hashes (optimize with in-memory cache of active agent key hashes), injects authenticated agent identity into `context.Context`. Return `ErrUnauthorized` for missing/invalid keys. Return `ErrAgentDeregistered` for inactive agents (same generic unauthorized message to prevent enumeration per FR-003/edge case). Use `slog` to log auth attempts (without logging the key itself). Include tests in `internal/auth/middleware_test.go`.
- [ ] T009 [P] Implement trace recording for registry operations in `internal/trace/registry.go`: `RecordRegistration(ctx, agentName, details)`, `RecordUpdate(ctx, agentName, beforeAfter)`, `RecordDeregistration(ctx, agentName, performedBy)`, `RecordKeyRotation(ctx, agentName)`. Each writes to the existing `traces` table with action types: `register`, `update`, `deregister`, `key_rotate`. Details stored as JSON. Use `slog` for structured logging per Constitution Principle VIII.

**Checkpoint**: Foundation ready - user story implementation can now begin in parallel

---

## Phase 3: User Story 1 - Agent Self-Registration (Priority: P1) MVP

**Goal**: An AI agent can call `register_agent` via MCP, receive an API key, and use it to authenticate all subsequent MCP tool calls.

**Independent Test**: Call `register_agent` via MCP client, verify returned API key works for a subsequent authenticated tool call.

### Tests for User Story 1

> **NOTE: Write these tests FIRST, ensure they FAIL before implementation**

- [ ] T010 [P] [US1] Integration test for agent registration in `internal/agents/service_test.go`: table-driven tests covering all acceptance scenarios: (1) successful registration returns agent_id, api_key, name, created_at; (2) duplicate name returns `agent_name_taken` error; (3) empty/invalid name returns validation error; (4) name regex enforcement; (5) capabilities exceeding 64KB rejected; (6) invalid capabilities JSON rejected; (7) API key hash stored (not raw key); (8) trace entry created on registration.
- [ ] T011 [P] [US1] Integration test for API key authentication in `internal/auth/middleware_test.go`: table-driven tests covering: (1) valid API key authenticates successfully and populates context with agent identity; (2) invalid API key returns unauthorized; (3) missing Authorization header returns unauthorized; (4) malformed Bearer token returns unauthorized; (5) deregistered agent's key returns unauthorized with generic message.

### Implementation for User Story 1

- [ ] T012 [US1] Implement agent service in `internal/agents/service.go`: `Service` struct with `Register(ctx, RegisterRequest) (RegisterResponse, error)` method. Validation: name regex `^[a-z0-9][a-z0-9._-]{0,62}[a-z0-9]$`, capabilities size <= 64KB, capabilities valid JSON conforming to CapabilityCard schema. Generate API key via `auth.GenerateAPIKey()`, store bcrypt hash, return raw key exactly once. Record trace entry. Accept `context.Context` as first param. Log with `slog`.
- [ ] T013 [US1] Register `register_agent` MCP tool in `internal/mcp/tools_agents.go`: define MCP tool with JSON Schema for input (name, display_name, type, owner_id, capabilities). Handler calls `agents.Service.Register()`. Tool description follows MCP conventions. Wire into the MCP server tool registry. Return structured JSON response with agent_id, api_key, name, created_at.
- [ ] T014 [US1] Wire authentication middleware into MCP server in `internal/mcp/server.go`: apply `AgentAuthMiddleware` to all MCP tool calls except `register_agent` (which is unauthenticated since the agent has no key yet). Ensure authenticated agent identity is available in context for all downstream handlers.

**Checkpoint**: At this point, agents can self-register and authenticate. User Story 1 is fully functional and testable independently.

---

## Phase 4: User Story 2 - Agent Discovery by Capability (Priority: P2)

**Goal**: A coordinating agent can find other agents by capability keyword, receiving a list of matching agents with their full capability cards.

**Independent Test**: Register 3-4 agents with different capabilities, call `discover_agents` with various queries, verify correct filtering and pagination.

### Tests for User Story 2

> **NOTE: Write these tests FIRST, ensure they FAIL before implementation**

- [ ] T015 [P] [US2] Integration test for agent discovery in `internal/agents/service_test.go`: table-driven tests covering: (1) discover by capability keyword returns matching agents with full capability cards; (2) discover with non-matching keyword returns empty list (no error); (3) discover with no filter returns all active agents; (4) pagination works with default limit 50; (5) pagination cursor returns correct next page; (6) deregistered agents excluded from results; (7) results include display_name, type, capabilities for each agent.

### Implementation for User Story 2

- [ ] T016 [US2] Implement discovery service method in `internal/agents/service.go`: `Discover(ctx, DiscoverRequest) (DiscoverResponse, error)` method. Query agents by capability keyword (match against `skills` array in capabilities JSON), agent type, and/or owner_id. Return paginated results with `total_count` and `next_cursor`. Filter out inactive (deregistered) agents. Accept `context.Context`, log with `slog`.
- [ ] T017 [US2] Register `discover_agents` MCP tool in `internal/mcp/tools_agents.go`: define MCP tool with JSON Schema for input (capability, type, owner_id, limit, cursor -- all optional). Handler calls `agents.Service.Discover()`. Return structured JSON response with agents array, total_count, next_cursor. This tool requires authentication (caller must be a registered agent).

**Checkpoint**: At this point, User Stories 1 AND 2 are both independently functional. Agents can register and discover each other.

---

## Phase 5: User Story 3 - Agent Lifecycle Management (Priority: P2)

**Goal**: Agents can update their own display_name and capabilities. Owners can deregister agents. API key rotation is supported.

**Independent Test**: Register an agent, update capabilities, verify change in discovery results, deregister, verify removal.

### Tests for User Story 3

> **NOTE: Write these tests FIRST, ensure they FAIL before implementation**

- [ ] T018 [P] [US3] Integration test for agent update in `internal/agents/service_test.go`: table-driven tests covering: (1) agent updates own capabilities successfully; (2) updated capabilities reflected in discover_agents; (3) agent cannot change own owner_id (forbidden); (4) agent cannot change own name (immutable); (5) API key rotation returns new key and invalidates old; (6) trace entry created on update; (7) trace entry created on key rotation.
- [ ] T019 [P] [US3] Integration test for agent deregistration in `internal/agents/service_test.go`: table-driven tests covering: (1) owner deregisters own agent successfully (soft-delete); (2) deregistered agent no longer appears in discover_agents; (3) deregistered agent's API key is invalidated; (4) non-owner cannot deregister another owner's agent (forbidden); (5) agent cannot deregister itself; (6) trace entry created on deregistration; (7) owner cascade: deleting owner soft-deletes all owned agents.

### Implementation for User Story 3

- [ ] T020 [US3] Implement update service method in `internal/agents/service.go`: `Update(ctx, agentName, UpdateRequest) (UpdateResponse, error)` method. Authenticated agent can update own `display_name` and `capabilities`. Reject changes to `owner_id` and `name` with `ErrForbidden`. If `rotate_key: true`, generate new API key, bcrypt hash it, invalidate old, return new key exactly once. Record trace entries for update and/or key rotation. Validate capabilities schema and size.
- [ ] T021 [US3] Implement deregister service method in `internal/agents/service.go`: `Deregister(ctx, DeregisterRequest) error` method. Only the agent's owner (authenticated as human user) can deregister. Set `status = 'inactive'`, set `deregistered_at = NOW()`. Invalidate API key hash (overwrite with empty/invalid hash). Record trace entry with `performed_by` as the owner. Implement `DeregisterByOwner(ctx, ownerID) error` for cascade owner deletion.
- [ ] T022 [US3] Register `update_agent` and `deregister_agent` MCP tools in `internal/mcp/tools_agents.go`: define MCP tools with JSON Schema for inputs. `update_agent` handler: extract authenticated agent from context, call `agents.Service.Update()`. `deregister_agent` handler: verify caller is agent's owner (not the agent itself), call `agents.Service.Deregister()`. Both require authentication.

**Checkpoint**: All CRUD lifecycle operations work. User Stories 1, 2, and 3 are independently functional.

---

## Phase 6: User Story 4 - Owner-Scoped Access Enforcement (Priority: P1)

**Goal**: Agents can only access their own messages, send as themselves, and see channels they have joined. Security isolation is enforced at the service layer.

**Independent Test**: Register two agents with different owners, have each send messages, verify neither can read the other's inbox or access unauthorized channels.

### Tests for User Story 4

> **NOTE: Write these tests FIRST, ensure they FAIL before implementation**

- [ ] T023 [P] [US4] Negative-path access violation tests in `internal/agents/access_test.go`: at least 5 access-violation test cases per SC-005: (1) agent B cannot read agent A's inbox; (2) agent A cannot send messages with `from` set to agent B (server overrides `from` with authenticated identity); (3) agent A cannot list private channels it has not joined; (4) agent A cannot read messages from a channel it has not joined; (5) agent A cannot deregister agent B (owned by different owner); (6) agent A cannot impersonate agent B in any tool call.
- [ ] T024 [P] [US4] Integration test for owner-scoped query filtering in `internal/agents/access_test.go`: table-driven tests verifying: (1) `read_inbox` returns only authenticated agent's messages; (2) `list_channels` excludes private channels agent has not joined; (3) public channels visible regardless of membership; (4) `send_message` always sets `from` to authenticated agent identity (server-side).

### Implementation for User Story 4

- [ ] T025 [US4] Implement access control layer in `internal/agents/access.go`: `AccessControl` struct with methods: `CanReadInbox(ctx, callerAgent, targetAgent) error`, `CanSendAs(ctx, callerAgent, fromAgent) error`, `CanAccessChannel(ctx, callerAgent, channelID) error`, `CanDeregister(ctx, callerIdentity, targetAgent) error`. Each returns nil on success, `ErrForbidden` with descriptive message on violation. Extract caller identity from `context.Context` (set by auth middleware).
- [ ] T026 [US4] Integrate access control into MCP tool handlers in `internal/mcp/server.go` and `internal/mcp/tools_agents.go`: add access control checks before all message operations (`send_message`, `read_inbox`, `list_channels`). Ensure `send_message` always overrides the `from` field with the authenticated agent's identity (server-side enforcement). Add access check to `deregister_agent` verifying caller is the agent's owner.
- [ ] T027 [US4] Add server-side `from` field enforcement in `internal/messaging/service.go` (or create if needed): any message send operation MUST set `from_agent` to the authenticated caller's agent name, ignoring any client-provided value. Log a warning via `slog` if a client attempts to set a different `from` value.

**Checkpoint**: Security isolation is enforced. All four user stories are independently functional and tested.

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Improvements that affect multiple user stories

- [ ] T028 [P] API key security audit in `internal/auth/apikey_audit_test.go`: grep-based test (per SC-006) that runs integration tests and verifies API keys are never present in logs, traces, error messages, or database records in plaintext. Scan all `slog` output and `traces` table entries for key patterns matching `sk-synapbus-`.
- [ ] T029 [P] Performance benchmark tests in `internal/agents/bench_test.go`: (1) registration completes in < 500ms (SC-001); (2) API key auth adds < 5ms latency (SC-002); (3) `discover_agents` across 1,000 agents returns in < 200ms (SC-003). Use `testing.B` for benchmarks.
- [ ] T030 [P] Add comprehensive `slog` structured logging across all agent registry operations: ensure every `Register`, `Update`, `Deregister`, `Discover`, and `KeyRotate` operation logs agent identity, action type, and outcome at appropriate levels (Info for success, Warn for access violations, Error for failures). Never log raw API keys.
- [ ] T031 Validate end-to-end flow: register agent -> authenticate -> discover agents -> update capabilities -> rotate key -> re-authenticate with new key -> deregister. Manual or scripted validation using MCP client against running SynapBus instance. Verify all trace entries are recorded.
- [ ] T032 [P] Add bcrypt verification cache in `internal/auth/cache.go`: in-memory LRU cache mapping `sha256(raw_key) -> agent_identity` to avoid bcrypt.CompareHashAndPassword on every request (per SC-002). Cache entries invalidated on key rotation or agent deregistration. TTL-based expiry (e.g., 5 minutes). Include tests in `internal/auth/cache_test.go`.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies - can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion - BLOCKS all user stories
- **User Stories (Phase 3-6)**: All depend on Foundational phase completion
  - US1 (Phase 3) and US4 (Phase 6) are both P1 priority
  - US1 MUST complete before US4 (access control depends on working auth)
  - US2 (Phase 4) can start after Phase 2, independent of US1
  - US3 (Phase 5) can start after Phase 2, independent of US1
- **Polish (Phase 7)**: Depends on all user stories being complete

### User Story Dependencies

- **User Story 1 (P1)**: Can start after Foundational (Phase 2) - No dependencies on other stories. This is the MVP.
- **User Story 2 (P2)**: Can start after Foundational (Phase 2) - Uses repository layer but no dependency on US1 service methods.
- **User Story 3 (P2)**: Can start after Foundational (Phase 2) - Uses repository layer and API key generation from Phase 2.
- **User Story 4 (P1)**: Depends on US1 completion (needs working auth middleware). Integrates with messaging/channels services from other features.

### Within Each User Story

- Tests MUST be written and FAIL before implementation
- Domain model/DTOs before service layer
- Service layer before MCP tool handlers
- Access control checks before integration wiring
- Story complete before moving to next priority

### Parallel Opportunities

- All Setup tasks marked [P] can run in parallel (T002, T003, T004)
- All Foundational tasks marked [P] can run in parallel (T006, T007, T009 alongside T005)
- T008 depends on T005 (needs API key functions)
- Once Foundational phase completes, US2 and US3 can start in parallel with US1
- All tests for a user story marked [P] can run in parallel
- All Polish tasks marked [P] can run in parallel

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup (schema migration, domain types)
2. Complete Phase 2: Foundational (API key gen, repository, auth middleware, trace)
3. Complete Phase 3: User Story 1 (register_agent MCP tool, auth wiring)
4. **STOP and VALIDATE**: Test registration + authentication independently
5. Deploy/demo if ready

### Incremental Delivery

1. Complete Setup + Foundational -> Foundation ready
2. Add User Story 1 -> Test independently -> Deploy/Demo (MVP!)
3. Add User Story 2 -> Test independently -> Deploy/Demo (agents can discover each other)
4. Add User Story 3 -> Test independently -> Deploy/Demo (full CRUD lifecycle)
5. Add User Story 4 -> Test independently -> Deploy/Demo (security isolation enforced)
6. Polish phase -> Performance, audit, caching

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Each user story should be independently completable and testable
- Verify tests fail before implementing
- Commit after each task or logical group
- Stop at any checkpoint to validate story independently
- All `context.Context` must be propagated through function signatures
- All logging via `slog` with structured fields (never log raw API keys)
- Table-driven tests for all test files
- `modernc.org/sqlite` only (zero CGO constraint)
