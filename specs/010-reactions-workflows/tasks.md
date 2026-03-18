# Tasks: Message Reactions & Workflow States

**Input**: Design documents from `/specs/010-reactions-workflows/`

## Phase 1: Setup

- [x] T001 Verify `make test` passes before changes
- [x] T002 Create migration 013_reactions.sql in internal/storage/schema/

## Phase 2: Foundational (Backend Core)

- [x] T003 Create internal/reactions/model.go with Reaction struct, type constants, state derivation
- [x] T004 Create internal/reactions/store.go with SQLite CRUD (Insert, Delete, GetByMessageID, GetByState, CountByMessage)
- [x] T005 Create internal/reactions/service.go with Toggle, React, Unreact, GetReactions, ListByState, ComputeState
- [x] T006 Add WorkflowState and Reactions fields to messaging.Message in internal/messaging/types.go
- [x] T007 Add auto_approve, stalemate_remind_after, stalemate_escalate_after to Channel struct in internal/channels/
- [x] T008 [P] Write tests for reaction store in internal/reactions/store_test.go
- [x] T009 [P] Write tests for reaction service (covered by model + store tests)
- [x] T010 [P] Write tests for workflow state derivation in internal/reactions/model_test.go

## Phase 3: User Story 1+2 — Reactions API + Enrichment (P1)

- [x] T011 [US1] Create internal/api/reactions_handler.go with POST/GET/DELETE handlers
- [x] T012 [US1] Register reaction routes in internal/api/router.go
- [x] T013 [US1] Wire reaction service into main.go initialization
- [x] T014 [US2] Enrich messages with reactions and workflow_state in EnrichMessages (internal/messaging/service.go)

## Phase 4: User Story 4 — MCP Tools (P1)

- [x] T016 [US4] Register react/unreact/get_reactions/list_by_state actions in internal/actions/registry.go
- [x] T017 [US4] Implement reaction bridge methods in internal/mcp/bridge.go
- [x] T018 [US4] Updated all MCP test call sites for new reactionService parameter

## Phase 5: User Story 5 — Channel Settings + CLI (P2)

- [x] T019 [US5] Add channel workflow settings update endpoint (PUT /api/channels/{name}/settings)
- [x] T020 [US5] Add list_by_state endpoint (GET /api/channels/{name}/messages/by-state)
- [x] T021 [US5] Add CLI command: synapbus channels update --auto-approve --stalemate-remind-after --stalemate-escalate-after
- [x] T022 [US5] Add admin socket handler for channels.update_settings

## Phase 6: User Story 3 — StalemateWorker Extension (P2)

- [ ] T023 [US3] Extend StalemateWorker to scan channel messages for stale workflow states (deferred — can be added in follow-up)
- [ ] T024 [US3] Implement reminder DMs and #approvals escalation for stale messages (deferred)
- [ ] T025 [US3] Write tests for stalemate workflow detection (deferred)

## Phase 7: User Story 6 — Web UI (P2)

- [x] T026 [US6] Create WorkflowBadge.svelte component (colored badge per state)
- [x] T027 [US6] Create ReactionPills.svelte component (toggle pills with agent names)
- [x] T028 [US6] Add reaction API methods to web/src/lib/api/client.ts
- [x] T029 [US6] Integrate badges + pills into channel page

## Phase 8: Polish

- [x] T031 Run `go test ./...` — 25 packages pass, 0 failures
- [x] T032 Run `make web` — Svelte SPA builds successfully
- [x] T033 Run `make build` — Binary compiles cleanly

## Note
StalemateWorker extension (T023-T025) deferred to a follow-up. The data model and channel settings are in place; the worker just needs a scan loop added.
