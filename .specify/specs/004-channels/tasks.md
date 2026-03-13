# Tasks: Channels

**Input**: Design documents from `/specs/004-channels/`
**Prerequisites**: spec.md (required), constitution.md (required)

**Tests**: Included per Go project conventions (table-driven tests, context propagation, slog logging).

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Database migration and package scaffolding for channels feature

- [ ] T001 Create database migration `schema/002_channels.sql` adding `channel_invites` table (`channel_id`, `agent_name`, `invited_by`, `created_at`, `status` with CHECK IN ('pending', 'accepted', 'declined')). Add composite unique constraint on (`channel_id`, `agent_name`). Add index on `channel_invites(agent_name)`. Update `channels.type` CHECK constraint to include `'public'` and `'private'` values alongside existing `'standard'`, `'blackboard'`, `'auction'` — or add an `is_private` boolean if the existing schema already handles it (note: `is_private INTEGER` already exists in 001_initial.sql, so this migration adds only the `channel_invites` table).
- [ ] T002 [P] Create channel domain types in `internal/channels/types.go`: `Channel` struct (ID, Name, Description, Topic, Type, IsPrivate, CreatedBy, CreatedAt, UpdatedAt), `Membership` struct (ID, ChannelID, AgentName, Role, JoinedAt), `ChannelInvite` struct (ID, ChannelID, AgentName, InvitedBy, CreatedAt, Status), `ChannelType` and `MemberRole` string constants, `CreateChannelRequest`, `JoinChannelRequest`, `InviteRequest`, `UpdateChannelRequest` input structs.
- [ ] T003 [P] Create channel name validation in `internal/channels/validate.go`: alphanumeric plus hyphens and underscores, max 64 characters, case-insensitive normalization (lowercase). Export `ValidateChannelName(name string) error`.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Repository layer and service skeleton that ALL user stories depend on

**CRITICAL**: No user story work can begin until this phase is complete

- [ ] T004 Implement channel repository in `internal/channels/repository.go`: define `Repository` interface with methods `CreateChannel(ctx, Channel) (Channel, error)`, `GetChannelByID(ctx, int64) (Channel, error)`, `GetChannelByName(ctx, string) (Channel, error)`, `ListChannels(ctx, agentName string) ([]Channel, error)`, `UpdateChannel(ctx, Channel) error`, `DeleteChannel(ctx, int64) error`.
- [ ] T005 [P] Implement membership repository in `internal/channels/membership_repo.go`: define `MembershipRepository` interface with methods `AddMember(ctx, Membership) error`, `RemoveMember(ctx, channelID int64, agentName string) error`, `GetMember(ctx, channelID int64, agentName string) (Membership, error)`, `ListMembers(ctx, channelID int64) ([]Membership, error)`, `IsMember(ctx, channelID int64, agentName string) (bool, error)`, `CountMembers(ctx, channelID int64) (int, error)`.
- [ ] T006 [P] Implement invite repository in `internal/channels/invite_repo.go`: define `InviteRepository` interface with methods `CreateInvite(ctx, ChannelInvite) error`, `GetInvite(ctx, channelID int64, agentName string) (ChannelInvite, error)`, `HasPendingInvite(ctx, channelID int64, agentName string) (bool, error)`, `AcceptInvite(ctx, channelID int64, agentName string) error`.
- [ ] T007 Implement SQLite channel repository in `internal/channels/sqlite_repository.go`: implement the `Repository` interface backed by `*sql.DB`. `ListChannels` must return all public channels plus private channels where the agent is a member or has a pending invite. Use `COLLATE NOCASE` for channel name uniqueness checks. Include slog logging for all operations.
- [ ] T008 [P] Implement SQLite membership repository in `internal/channels/sqlite_membership_repo.go`: implement the `MembershipRepository` interface backed by `*sql.DB`. Include slog logging.
- [ ] T009 [P] Implement SQLite invite repository in `internal/channels/sqlite_invite_repo.go`: implement the `InviteRepository` interface backed by `*sql.DB`. Include slog logging.
- [ ] T010 Create channel service skeleton in `internal/channels/service.go`: define `Service` struct taking `Repository`, `MembershipRepository`, `InviteRepository`, and a trace recorder dependency. Constructor `NewService(...)`. This is the entry point for all channel business logic. Use `context.Context` for all public methods and `slog` for structured logging.

### Tests for Phase 2

- [ ] T011 [P] Write table-driven tests for channel repository in `internal/channels/sqlite_repository_test.go`: test CreateChannel (success, duplicate name conflict, name validation), GetChannelByID (found, not found), ListChannels (returns public channels, hides uninvited private channels). Use an in-memory SQLite database for test isolation.
- [ ] T012 [P] Write table-driven tests for membership repository in `internal/channels/sqlite_membership_repo_test.go`: test AddMember (success, duplicate), RemoveMember (success, not found), IsMember, CountMembers.
- [ ] T013 [P] Write table-driven tests for invite repository in `internal/channels/sqlite_invite_repo_test.go`: test CreateInvite (success, duplicate idempotent), HasPendingInvite, AcceptInvite.

**Checkpoint**: Foundation ready — user story implementation can now begin in parallel

---

## Phase 3: User Story 1 — Agent Creates and Broadcasts to a Public Channel (Priority: P1) MVP

**Goal**: An agent can create a public channel, another agent joins, and broadcast messages reach all members.

**Independent Test**: Register two agents, agent A calls `create_channel` (public), agent B calls `join_channel`, agent A calls `send_message` with `channel_id`. Agent B reads inbox and sees the broadcast message.

### Tests for User Story 1

> **NOTE: Write these tests FIRST, ensure they FAIL before implementation**

- [ ] T014 [P] [US1] Write table-driven tests for `Service.CreateChannel` in `internal/channels/service_test.go`: test creating public channel sets creator as owner, duplicate name returns conflict error, invalid name returns validation error, trace is recorded.
- [ ] T015 [P] [US1] Write table-driven tests for `Service.JoinChannel` in `internal/channels/service_test.go`: test joining public channel succeeds, joining non-existent channel returns not-found error, joining channel agent is already a member of is idempotent (no error), trace is recorded.
- [ ] T016 [P] [US1] Write integration test for channel broadcast in `internal/channels/broadcast_test.go`: test that when a message is sent to a channel, all members except the sender receive it in their inbox. Test with 0 other members (no error), 1 member, and multiple members.

### Implementation for User Story 1

- [ ] T017 [US1] Implement `Service.CreateChannel(ctx, CreateChannelRequest) (Channel, error)` in `internal/channels/service.go`: validate name via `ValidateChannelName`, check uniqueness, insert channel, auto-add creator as `owner` member, record trace entry, return channel with ID.
- [ ] T018 [US1] Implement `Service.JoinChannel(ctx, agentName string, channelID int64) error` in `internal/channels/service.go`: verify channel exists, verify channel is public (for US1; private join gated by invite will be added in US3), check if already a member (idempotent return), add member with role `member`, record trace entry.
- [ ] T019 [US1] Implement channel broadcast logic in `internal/channels/broadcast.go`: export `BroadcastToChannel(ctx, channelID int64, senderAgent string, messageBody string, metadata map[string]any) error`. Query all channel members, exclude the sender, create a message for each recipient with the `channel_id` field set. This function calls into the messaging repository (depends on `internal/messaging` package — accept an interface `MessageCreator` to avoid tight coupling).
- [ ] T020 [US1] Register channel MCP tools (`create_channel`, `join_channel`) in `internal/mcp/channel_tools.go`: define JSON Schema `inputSchema` for each tool, implement tool handler functions that call `channels.Service` methods, wire into MCP tool registry. `create_channel` accepts `name` (required), `type` ("public"|"private", default "public"), `description` (optional), `topic` (optional). `join_channel` accepts `channel_id` (required integer).
- [ ] T021 [US1] Extend `send_message` MCP tool to support channel broadcast in `internal/mcp/message_tools.go`: when `channel_id` is provided (and `to_agent` is omitted), verify sender is a channel member (authorization), then delegate to `BroadcastToChannel`. Return error if agent is not a member (FR-015). Record trace.

**Checkpoint**: At this point, agents can create public channels, join them, and broadcast messages. User Story 1 is fully functional and testable independently.

---

## Phase 4: User Story 2 — Agent Discovers and Lists Available Channels (Priority: P2)

**Goal**: An agent can call `list_channels` and see all public channels plus private channels it has been invited to, with full metadata.

**Independent Test**: Create several public and private channels, call `list_channels` as a new agent. Response includes all public channels with metadata, excludes uninvited private channels.

### Tests for User Story 2

> **NOTE: Write these tests FIRST, ensure they FAIL before implementation**

- [ ] T022 [P] [US2] Write table-driven tests for `Service.ListChannels` in `internal/channels/service_test.go`: test returns all public channels, excludes uninvited private channels, includes private channels where agent is a member, includes private channels where agent has a pending invite, returns empty list when no channels exist. Verify response includes `name`, `description`, `topic`, `created_by`, `member_count`, `type` fields.

### Implementation for User Story 2

- [ ] T023 [US2] Implement `Service.ListChannels(ctx, agentName string) ([]ChannelWithCount, error)` in `internal/channels/service.go`: define `ChannelWithCount` struct (embeds `Channel` + `MemberCount int`). Delegate to repository `ListChannels` which already filters by visibility. Enrich results with member count via `MembershipRepository.CountMembers`. Record trace entry.
- [ ] T024 [US2] Register `list_channels` MCP tool in `internal/mcp/channel_tools.go`: no required input parameters (agent identity comes from MCP connection context). Return JSON array of channel objects with fields: `id`, `name`, `description`, `topic`, `type`, `created_by`, `member_count`. Add JSON Schema for the tool.

**Checkpoint**: At this point, User Stories 1 AND 2 should both work independently. Agents can create, join, discover, and broadcast to channels.

---

## Phase 5: User Story 3 — Channel Owner Manages Private Channel Membership (Priority: P2)

**Goal**: A channel owner creates a private channel, invites specific agents, and can kick members. Uninvited agents cannot join.

**Independent Test**: Create a private channel, invite agent B, confirm agent B can join. Verify uninvited agent C gets an authorization error on `join_channel`. Owner kicks agent B, verify agent B no longer receives channel messages.

### Tests for User Story 3

> **NOTE: Write these tests FIRST, ensure they FAIL before implementation**

- [ ] T025 [P] [US3] Write table-driven tests for `Service.InviteToChannel` in `internal/channels/service_test.go`: test owner can invite, non-owner gets authorization error, inviting an already-member agent is idempotent, inviting to a non-existent channel returns not-found.
- [ ] T026 [P] [US3] Write table-driven tests for `Service.KickFromChannel` in `internal/channels/service_test.go`: test owner can kick a member, non-owner gets authorization error, kicking a non-member returns not-found, owner cannot kick themselves.
- [ ] T027 [P] [US3] Write table-driven tests for private channel join gating in `internal/channels/service_test.go`: test uninvited agent gets authorization error on `join_channel` for private channel, invited agent can join, invite status changes to `accepted` after join.

### Implementation for User Story 3

- [ ] T028 [US3] Extend `Service.JoinChannel` in `internal/channels/service.go` to enforce private channel invite gating: if channel `is_private`, check `InviteRepository.HasPendingInvite`. If no pending invite, return authorization error. On successful join, call `InviteRepository.AcceptInvite`.
- [ ] T029 [US3] Implement `Service.InviteToChannel(ctx, ownerAgent string, channelID int64, inviteeAgent string) error` in `internal/channels/service.go`: verify channel exists, verify caller is the channel owner (role check via `MembershipRepository.GetMember`), check if invitee is already a member (idempotent return), create invite via `InviteRepository.CreateInvite`, record trace entry.
- [ ] T030 [US3] Implement `Service.KickFromChannel(ctx, ownerAgent string, channelID int64, targetAgent string) error` in `internal/channels/service.go`: verify channel exists, verify caller is the channel owner, verify target is a member (not-found if not), prevent owner from kicking themselves, remove member via `MembershipRepository.RemoveMember`, record trace entry.
- [ ] T031 [US3] Register `invite_to_channel` MCP tool in `internal/mcp/channel_tools.go`: accepts `channel_id` (required integer), `agent_id` (required string — the invitee agent name). Only callable by the channel owner. Add JSON Schema.
- [ ] T032 [US3] Register `kick_from_channel` MCP tool in `internal/mcp/channel_tools.go`: accepts `channel_id` (required integer), `agent_id` (required string — the target agent name). Only callable by the channel owner. Add JSON Schema.

**Checkpoint**: At this point, User Stories 1, 2, and 3 are all functional. Public and private channels work with full membership management.

---

## Phase 6: User Story 4 — Agent Leaves a Channel (Priority: P3)

**Goal**: An agent can voluntarily leave a channel and stop receiving messages. Owner cannot leave without transferring ownership or deleting the channel.

**Independent Test**: Agent joins a public channel, calls `leave_channel`, verify subsequent channel messages are not delivered. Agent can rejoin. Owner leaving returns an error.

### Tests for User Story 4

> **NOTE: Write these tests FIRST, ensure they FAIL before implementation**

- [ ] T033 [P] [US4] Write table-driven tests for `Service.LeaveChannel` in `internal/channels/service_test.go`: test member can leave, non-member returns not-found, owner cannot leave (returns error with guidance to transfer ownership or delete), after leaving agent does not receive broadcast messages, agent can rejoin a public channel.

### Implementation for User Story 4

- [ ] T034 [US4] Implement `Service.LeaveChannel(ctx, agentName string, channelID int64) error` in `internal/channels/service.go`: verify channel exists, verify agent is a member, if agent role is `owner` return error with message "channel owner cannot leave; transfer ownership or delete the channel first", remove member via `MembershipRepository.RemoveMember`, record trace entry.
- [ ] T035 [US4] Register `leave_channel` MCP tool in `internal/mcp/channel_tools.go`: accepts `channel_id` (required integer). Agent identity from MCP context. Add JSON Schema.

**Checkpoint**: All five MCP tools from FR-016 are now registered: `create_channel`, `join_channel`, `leave_channel`, `list_channels`, `invite_to_channel`.

---

## Phase 7: User Story 5 — Channel Metadata and Topic Management (Priority: P3)

**Goal**: Channel owner can update topic and description. Non-owners get an authorization error.

**Independent Test**: Create a channel with a topic, update it via `update_channel`, verify `list_channels` reflects the change. Non-owner update returns error.

### Tests for User Story 5

> **NOTE: Write these tests FIRST, ensure they FAIL before implementation**

- [ ] T036 [P] [US5] Write table-driven tests for `Service.UpdateChannel` in `internal/channels/service_test.go`: test owner can update topic, owner can update description, non-owner gets authorization error, non-existent channel returns not-found, updated_at timestamp is refreshed.

### Implementation for User Story 5

- [ ] T037 [US5] Implement `Service.UpdateChannel(ctx, agentName string, channelID int64, req UpdateChannelRequest) (Channel, error)` in `internal/channels/service.go`: verify channel exists, verify caller is owner, apply updates (topic, description — only non-nil fields), update `updated_at`, persist via `Repository.UpdateChannel`, record trace entry, return updated channel.
- [ ] T038 [US5] Register `update_channel` MCP tool in `internal/mcp/channel_tools.go`: accepts `channel_id` (required integer), `topic` (optional string), `description` (optional string). Only callable by the channel owner. Add JSON Schema.

**Checkpoint**: All user stories are now independently functional.

---

## Phase 8: Polish & Cross-Cutting Concerns

**Purpose**: Edge cases, performance, and integration hardening

- [ ] T039 [P] Handle edge case: channel name conflict returns structured error with clear message in `internal/channels/service.go`. Ensure error type is distinguishable (e.g., `ErrChannelNameConflict`) for MCP tool handlers to return appropriate error codes.
- [ ] T040 [P] Handle edge case: deregistered channel owner — in `internal/channels/service.go`, ensure channel remains accessible when owner agent is deregistered. Document behavior: channel becomes ownerless but functional, messages can still be broadcast.
- [ ] T041 [P] Define sentinel errors in `internal/channels/errors.go`: `ErrChannelNotFound`, `ErrChannelNameConflict`, `ErrNotChannelMember`, `ErrNotChannelOwner`, `ErrOwnerCannotLeave`, `ErrNotInvited`, `ErrInvalidChannelName`. MCP tool handlers in `internal/mcp/channel_tools.go` should map these to appropriate MCP error responses.
- [ ] T042 [P] Add REST API endpoints for Web UI channel management in `internal/api/channels.go`: `GET /api/channels` (list), `GET /api/channels/:id` (detail with members), `POST /api/channels` (create), `PUT /api/channels/:id` (update). These are internal endpoints for the Web UI only (per Constitution Principle II, not for agents).
- [ ] T043 Write concurrency test in `internal/channels/concurrency_test.go`: verify that 50 agents can join a channel and receive broadcast messages concurrently without message loss or race conditions (SC-006). Use `sync.WaitGroup` and `t.Parallel()`.
- [ ] T044 [P] Verify all channel MCP tools have complete JSON Schema descriptions with field types, required markers, and descriptions. Ensure `tools/list` returns all five channel tools (SC-004). Write a test in `internal/mcp/channel_tools_test.go`.
- [ ] T045 Review and verify trace entries for all channel operations in `internal/channels/service.go`: create, join, leave, invite, kick, update, broadcast. Each trace must include `agent_name`, `action` (e.g., `channel.create`, `channel.join`), and `details` JSON with relevant IDs (SC-005, FR-012).
- [ ] T046 Run `make lint` and `make test` to verify all code passes linting and tests.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — can start immediately
- **Foundational (Phase 2)**: Depends on Phase 1 (migration and types must exist) — BLOCKS all user stories
- **User Story 1 (Phase 3)**: Depends on Phase 2 completion
- **User Story 2 (Phase 4)**: Depends on Phase 2 completion; can run in parallel with Phase 3
- **User Story 3 (Phase 5)**: Depends on Phase 2 completion; can run in parallel with Phases 3-4
- **User Story 4 (Phase 6)**: Depends on Phase 2 completion; can run in parallel with Phases 3-5
- **User Story 5 (Phase 7)**: Depends on Phase 2 completion; can run in parallel with Phases 3-6
- **Polish (Phase 8)**: Depends on all user story phases being complete

### Cross-Package Dependencies

- `internal/channels/` depends on `internal/storage/` (SQLite connection)
- `internal/channels/broadcast.go` depends on `internal/messaging/` (MessageCreator interface for delivering broadcast messages to agent inboxes)
- `internal/mcp/channel_tools.go` depends on `internal/channels/` (Service) and `internal/mcp/` (tool registry)
- `internal/api/channels.go` depends on `internal/channels/` (Service)
- `internal/channels/service.go` depends on `internal/trace/` (trace recorder for FR-012)

### Within Each User Story

- Tests MUST be written and FAIL before implementation
- Repository layer before service layer
- Service layer before MCP tool registration
- Core implementation before integration
- Story complete before moving to next priority

### Parallel Opportunities

- All Phase 1 tasks marked [P] can run in parallel (T002, T003)
- All Phase 2 repository interfaces marked [P] can run in parallel (T005, T006)
- All Phase 2 SQLite implementations marked [P] can run in parallel (T008, T009)
- All Phase 2 tests marked [P] can run in parallel (T011, T012, T013)
- Once Phase 2 completes, all user story phases (3-7) can start in parallel
- All test tasks within a story marked [P] can run in parallel
- Phase 8 polish tasks marked [P] can run in parallel

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup (migration, types, validation)
2. Complete Phase 2: Foundational (repositories, service skeleton)
3. Complete Phase 3: User Story 1 (create, join, broadcast)
4. **STOP and VALIDATE**: Test User Story 1 independently — two agents communicate via a public channel
5. Deploy/demo if ready

### Incremental Delivery

1. Setup + Foundational -> Foundation ready
2. Add User Story 1 -> Test independently -> Deploy/Demo (MVP!)
3. Add User Story 2 -> Test independently -> Deploy/Demo (discovery)
4. Add User Story 3 -> Test independently -> Deploy/Demo (private channels)
5. Add User Story 4 -> Test independently -> Deploy/Demo (leave)
6. Add User Story 5 -> Test independently -> Deploy/Demo (metadata updates)
7. Each story adds value without breaking previous stories

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Each user story is independently completable and testable
- Verify tests fail before implementing
- Commit after each task or logical group
- Stop at any checkpoint to validate story independently
- Channel names are stored lowercase; all comparisons are case-insensitive
- The existing schema already has `channels` and `channel_members` tables in `schema/001_initial.sql` — the migration in T001 only adds the `channel_invites` table
- `kick_from_channel` is not in FR-016's explicit MCP tool list but is required by US3 acceptance scenario 3; it is registered as an additional MCP tool
- The `update_channel` tool is not in FR-016's explicit list but is required by US5; it is registered as an additional MCP tool
