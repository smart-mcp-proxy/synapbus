# Tasks: Human Auth (OAuth 2.1)

**Input**: Design documents from `/specs/003-human-auth/`
**Prerequisites**: spec.md (required)

**Tests**: Integration and unit tests are included per SC-005 which requires all 13 functional requirements to have corresponding tests covering both success and error paths.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Add auth dependencies and create package skeleton

- [ ] T001 Add `ory/fosite` and `golang.org/x/crypto` (bcrypt) dependencies to `go.mod` via `go get`
- [ ] T002 [P] Create package skeleton: `internal/auth/` directory with placeholder files `internal/auth/doc.go`, `internal/auth/oauth.go`, `internal/auth/user.go`, `internal/auth/session.go`, `internal/auth/client.go`, `internal/auth/middleware.go`
- [ ] T003 [P] Create auth configuration struct in `internal/auth/config.go` with fields: bcrypt cost (default 12), access token TTL (default 1h), refresh token lifetime (default 30d), session lifetime (default 24h), issuer URL, dev mode flag

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Schema migrations, core domain types, fosite storage adapter, and auth middleware that ALL user stories depend on

**CRITICAL**: No user story work can begin until this phase is complete

- [ ] T004 Create SQLite migration `schema/002_auth.sql`: add `role` column (`TEXT NOT NULL DEFAULT 'user' CHECK (role IN ('user', 'admin'))`) to `users` table; add `scopes` and `owner_id` columns to `oauth_clients` table; create `oauth_sessions` table (`session_id TEXT PRIMARY KEY, user_id INTEGER NOT NULL REFERENCES users(id), created_at, expires_at, last_active_at`); create `oauth_authorization_codes` table (`code TEXT PRIMARY KEY, client_id, user_id, redirect_uri, scopes, code_challenge, code_challenge_method, expires_at, created_at, used INTEGER DEFAULT 0`); extend `oauth_tokens` table with `token_type TEXT` (access/refresh), `session_data TEXT` (JSON), `consumed INTEGER DEFAULT 0`, `parent_signature TEXT` columns; add indexes on new columns
- [ ] T005 Register migration 002 in storage migration runner (extend existing migration logic in `internal/storage/` to apply `002_auth.sql`)
- [ ] T006 [P] Define User domain model in `internal/auth/user.go`: `User` struct with `ID`, `Username`, `PasswordHash`, `DisplayName`, `Role`, `CreatedAt`, `UpdatedAt`; `UserStore` interface with `CreateUser`, `GetUserByID`, `GetUserByUsername`, `UpdatePassword`, `ListUsers`, `CountUsers`
- [ ] T007 [P] Define OAuthClient domain model in `internal/auth/client.go`: `OAuthClient` struct implementing `fosite.Client` interface with `ID`, `SecretHash`, `Name`, `RedirectURIs`, `GrantTypes`, `Scopes`, `OwnerID`, `CreatedAt`; `ClientStore` interface with `CreateClient`, `GetClient`, `ListClientsByOwner`
- [ ] T008 [P] Define Session domain model in `internal/auth/session.go`: `Session` struct with `SessionID`, `UserID`, `CreatedAt`, `ExpiresAt`, `LastActiveAt`; `SessionStore` interface with `CreateSession`, `GetSession`, `DeleteSession`, `DeleteSessionsByUser`, `DeleteSessionsByUserExcept`
- [ ] T009 Implement `UserStore` backed by SQLite in `internal/auth/user_store.go`: all CRUD methods, bcrypt hashing/verification helpers, constant-time username existence check
- [ ] T010 Implement `ClientStore` backed by SQLite in `internal/auth/client_store.go`: implements both `ClientStore` interface and `fosite.ClientManager` (`GetClient` returns `fosite.Client`)
- [ ] T011 Implement `SessionStore` backed by SQLite in `internal/auth/session_store.go`: all session CRUD, expiration checking, cleanup of expired sessions
- [ ] T012 Implement fosite storage adapter in `internal/auth/fosite_store.go`: struct implementing `fosite.Storage`, `oauth2.CoreStorage`, `oauth2.TokenRevocationStorage`, `pkce.PKCERequestStorage` interfaces; backed by SQLite tables for authorization codes, access tokens, refresh tokens; handles refresh token rotation and family revocation on reuse
- [ ] T013 Configure fosite OAuth2 provider in `internal/auth/provider.go`: compose fosite with `oauth2.AuthorizeExplicitFactory`, `oauth2.ClientCredentialsGrantFactory`, `oauth2.RefreshTokenGrantFactory`; enforce PKCE S256-only; set token lifetimes from config; create `NewOAuthProvider(config, store) fosite.OAuth2Provider`
- [ ] T014 Implement auth middleware in `internal/auth/middleware.go`: `RequireSession` middleware (checks session cookie, injects user into `context.Context`); `RequireBearer` middleware (validates access token via fosite introspection, injects client/user identity into context); `RequireAdmin` middleware (checks user role); context helper functions `UserFromContext(ctx)`, `ClientFromContext(ctx)`
- [ ] T015 Implement structured auth event logging in `internal/auth/logging.go`: `LogAuthEvent(ctx, slog.Logger, event)` function; event types: `login_success`, `login_failure`, `token_issued`, `token_refreshed`, `token_revoked`, `session_created`, `session_destroyed`, `user_created`, `password_changed`; includes user/client identity and remote IP (FR-013)

**Checkpoint**: Foundation ready -- user story implementation can now begin in parallel

---

## Phase 3: User Story 1 -- Human Owner Logs Into Web UI (Priority: P1) MVP

**Goal**: A human user can register a local account, log in via OAuth 2.1 authorization code flow with PKCE, and access the Web UI with a session cookie.

**Independent Test**: Start `synapbus serve`, open the Web UI, create an account via `/register`, log in, verify session cookie is set and subsequent API calls succeed.

### Tests for User Story 1

> **NOTE: Write these tests FIRST, ensure they FAIL before implementation**

- [ ] T016 [P] [US1] Integration test for user registration in `internal/auth/user_store_test.go`: create user with valid credentials, verify bcrypt hash cost >= 12; reject duplicate username with 409; reject short password; reject invalid username characters; verify constant-time behavior (no timing leak on existence check)
- [ ] T017 [P] [US1] Integration test for authorization code + PKCE flow in `internal/auth/oauth_test.go`: full flow test using fosite test helpers -- authorize request with S256 code_challenge -> obtain code -> exchange with code_verifier -> verify access + refresh tokens; verify session cookie is set as httponly/SameSite=Lax
- [ ] T018 [P] [US1] Integration test for session middleware in `internal/auth/middleware_test.go`: request with valid session cookie succeeds; request with expired session returns 401; request with no session redirects to login; request after logout is rejected

### Implementation for User Story 1

- [ ] T019 [US1] Implement registration handler in `internal/api/auth_handlers.go`: `POST /api/users` -- validate username (3-64 chars, `[a-zA-Z0-9_]`), validate password (8+ chars, max 72 bytes bcrypt limit), create user via `UserStore`, return 201 with user profile (no password hash); return 409 on duplicate username, 400 on validation errors (FR-002)
- [ ] T020 [US1] Implement OAuth authorize endpoint in `internal/api/auth_handlers.go`: `GET /oauth/authorize` -- validate client_id, redirect_uri, response_type=code; require code_challenge with method S256 (reject plain and missing); if user not logged in, render login form; on valid credentials, generate authorization code via fosite and redirect with code (FR-003)
- [ ] T021 [US1] Implement OAuth token endpoint in `internal/api/auth_handlers.go`: `POST /oauth/token` -- delegate to fosite for `grant_type=authorization_code` with PKCE verification; on success, create server-side session, set httponly secure SameSite=Lax cookie, return access_token + refresh_token JSON (FR-004)
- [ ] T022 [US1] Implement logout handler in `internal/api/auth_handlers.go`: `POST /api/auth/logout` -- delete session from store, clear session cookie, return 200
- [ ] T023 [US1] Implement `GET /api/users/me` handler in `internal/api/auth_handlers.go`: return current user profile (id, username, display_name, role, created_at) from session context (FR-008)
- [ ] T024 [US1] Register auth routes on chi router in `internal/api/router.go`: mount `/oauth/authorize`, `/oauth/token`, `/api/users` (POST), `/api/users/me` (GET), `/api/auth/logout` (POST); apply `RequireSession` middleware to protected routes; handle zero-users redirect to registration
- [ ] T025 [US1] Wire auth subsystem into `synapbus serve` command in `cmd/synapbus/main.go` (or `cmd/synapbus/serve.go`): initialize auth config from flags/env, create stores, create fosite provider, register handlers; add `--dev` flag to allow HTTP without TLS warning (FR-001)
- [ ] T026 [US1] Add auth event logging calls to all handlers: login success/failure, session created/destroyed, user created (FR-013)

**Checkpoint**: User Story 1 fully functional -- user can register, log in with PKCE, access protected endpoints, log out

---

## Phase 4: User Story 5 -- PKCE Enforcement (Priority: P1) MVP

**Goal**: Authorization code flow without PKCE is rejected; only S256 is accepted; incorrect code_verifier is rejected with code invalidation.

**Independent Test**: Send authorization request without `code_challenge` and verify rejection; send with S256 and verify full flow; send incorrect `code_verifier` and verify rejection.

> Note: PKCE enforcement is implemented as part of the fosite provider configuration (T013) and authorize/token endpoints (T020, T021). This phase validates and hardens it.

### Tests for User Story 5

- [ ] T027 [P] [US5] Integration test for PKCE enforcement in `internal/auth/pkce_test.go`: request without code_challenge returns 400 `invalid_request`; request with `code_challenge_method=plain` returns 400; correct S256 flow succeeds; incorrect code_verifier returns 400 `invalid_grant` and code is invalidated (single-use); reuse of authorization code is rejected

### Implementation for User Story 5

- [ ] T028 [US5] Harden PKCE validation in fosite provider config (`internal/auth/provider.go`): ensure `fosite.MinParameterEntropy` is set appropriately; confirm S256-only enforcement; add explicit rejection message for `plain` method; ensure authorization codes are single-use (FR-003)
- [ ] T029 [US5] Add PKCE-specific error responses in `internal/api/auth_handlers.go`: ensure `/oauth/authorize` returns descriptive error when PKCE is missing or uses `plain`; ensure `/oauth/token` returns descriptive error on verifier mismatch

**Checkpoint**: PKCE fully enforced -- authorization code flow cannot proceed without valid S256 challenge/verifier

---

## Phase 5: User Story 2 -- Programmatic Client Credentials (Priority: P2)

**Goal**: External automation can register an OAuth client and obtain tokens via `client_credentials` grant to access the REST API.

**Independent Test**: Register OAuth client via CLI, POST to `/oauth/token` with `grant_type=client_credentials`, verify valid access token, use token to call protected endpoint.

### Tests for User Story 2

- [ ] T030 [P] [US2] Integration test for client credentials flow in `internal/auth/client_credentials_test.go`: valid client_id + secret returns access_token with correct expires_in and scope; invalid secret returns 401; unknown client_id returns 401; token introspection returns `active: true` with client identity and scopes
- [ ] T031 [P] [US2] Integration test for token introspection in `internal/auth/introspect_test.go`: valid token returns active=true with client_id, scopes, exp; expired token returns active=false; revoked token returns active=false; malformed token returns active=false

### Implementation for User Story 2

- [ ] T032 [US2] Extend OAuth token endpoint to handle `grant_type=client_credentials` in `internal/api/auth_handlers.go`: fosite already supports this via factory (T013), ensure handler routes the grant type correctly; return `access_token`, `token_type: "bearer"`, `expires_in`, `scope` (FR-004)
- [ ] T033 [US2] Implement token introspection endpoint in `internal/api/auth_handlers.go`: `POST /oauth/introspect` per RFC 7662 -- validate bearer token, return `active`, `client_id`, `scope`, `exp`, `iat`, `token_type`; protect with client authentication (FR-005)
- [ ] T034 [US2] Implement `synapbus client create` CLI command in `cmd/synapbus/client.go`: `--name <name>` flag, generates random `client_id` and `client_secret`, stores bcrypt-hashed secret via `ClientStore`, prints credentials to stdout; optional `--grant-types` and `--scopes` flags (FR-011)
- [ ] T035 [US2] Register introspection route and bearer middleware on chi router in `internal/api/router.go`: mount `/oauth/introspect`; ensure `RequireBearer` middleware validates access tokens for API endpoints; return 401 with `WWW-Authenticate` header on expired tokens
- [ ] T036 [US2] Add auth event logging for client credentials: token issued, introspection events (FR-013)

**Checkpoint**: User Stories 1, 5, and 2 functional -- both interactive and programmatic auth work

---

## Phase 6: User Story 3 -- Refresh Token Rotation (Priority: P2)

**Goal**: Access tokens can be refreshed silently; old refresh tokens are invalidated on use; reuse of a consumed refresh token revokes the entire token family.

**Independent Test**: Obtain token pair, exchange refresh token for new pair at `/oauth/token`, verify old refresh token is rejected, verify family revocation on replay.

### Tests for User Story 3

- [ ] T037 [P] [US3] Integration test for refresh token rotation in `internal/auth/refresh_test.go`: exchange valid refresh token returns new access + refresh token; old refresh token is rejected on reuse; reuse of consumed token triggers family revocation (all descendant tokens revoked); refresh token beyond absolute lifetime (30d) is rejected with 401
- [ ] T038 [P] [US3] Unit test for token family revocation logic in `internal/auth/fosite_store_test.go`: create token chain (parent -> child -> grandchild); reuse parent triggers revocation of child and grandchild; verify all tokens in chain are inactive after revocation

### Implementation for User Story 3

- [ ] T039 [US3] Implement refresh token rotation in fosite store (`internal/auth/fosite_store.go`): on `CreateRefreshTokenSession`, mark parent token as consumed and store `parent_signature` on new token; on `RevokeRefreshTokenMaybeGracePeriod`, implement family revocation by walking `parent_signature` chain; ensure concurrent reuse of same token triggers revocation (FR-006)
- [ ] T040 [US3] Extend token endpoint handling for `grant_type=refresh_token` in `internal/api/auth_handlers.go`: fosite handles validation via factory, ensure handler returns new access_token + refresh_token pair; return 401 on consumed/expired refresh tokens (FR-006)
- [ ] T041 [US3] Add auth event logging for refresh operations: `token_refreshed`, `token_revoked` (family revocation) with parent/child token signatures (FR-013)

**Checkpoint**: Token lifecycle complete -- access tokens refresh silently, security against token theft via family revocation

---

## Phase 7: User Story 4 -- User Account Management (Priority: P3)

**Goal**: Users can change passwords, view owned agents, and view their profile. Admins can list and deactivate users.

**Independent Test**: Log in, call `PUT /api/users/me/password` with old + new password, verify old no longer works; call `GET /api/users/me/agents` and verify owned agents listed.

### Tests for User Story 4

- [ ] T042 [P] [US4] Integration test for password change in `internal/auth/user_store_test.go` (extend): correct current password + valid new password succeeds; sessions except current are invalidated; incorrect current password returns 403; new password below 8 chars returns 400
- [ ] T043 [P] [US4] Integration test for user agents listing in `internal/api/auth_handlers_test.go`: user with 3 agents sees all 3; user with 0 agents sees empty array; user does not see agents owned by other users
- [ ] T044 [P] [US4] Integration test for admin operations in `internal/api/auth_handlers_test.go`: admin can list all users; admin can deactivate account; non-admin gets 403 on admin endpoints

### Implementation for User Story 4

- [ ] T045 [US4] Implement password change handler in `internal/api/auth_handlers.go`: `PUT /api/users/me/password` -- require `current_password` and `new_password` in body; verify current password with bcrypt; re-hash new password; invalidate all sessions except current via `SessionStore.DeleteSessionsByUserExcept`; return 200 on success, 403 on wrong current password, 400 on invalid new password (FR-008)
- [ ] T046 [US4] Implement user agents listing handler in `internal/api/auth_handlers.go`: `GET /api/users/me/agents` -- query agents table by `owner_id` from session context; return JSON array with `id`, `name`, `display_name`, `type`, `created_at` (FR-008, FR-009)
- [ ] T047 [US4] Implement admin user management handlers in `internal/api/auth_handlers.go`: `GET /api/admin/users` -- list all users (admin only); `PUT /api/admin/users/{id}/deactivate` -- deactivate user account (admin only); first user created is automatically admin (FR-008)
- [ ] T048 [US4] Register user management routes in `internal/api/router.go`: mount `/api/users/me/password` (PUT), `/api/users/me/agents` (GET), `/api/admin/users` (GET), `/api/admin/users/{id}/deactivate` (PUT); apply `RequireSession` + `RequireAdmin` where appropriate
- [ ] T049 [US4] Add auth event logging for account management: `password_changed`, admin operations (FR-013)

**Checkpoint**: All user stories functional -- full auth lifecycle from registration to account management

---

## Phase 8: Polish & Cross-Cutting Concerns

**Purpose**: Edge cases, security hardening, and integration quality

- [ ] T050 [P] Handle zero-users edge case in middleware (`internal/auth/middleware.go`): when `UserStore.CountUsers() == 0`, redirect Web UI requests to registration; return 401 for API requests; never expose data
- [ ] T051 [P] Handle corrupt/missing token storage gracefully in `internal/auth/fosite_store.go`: catch SQLite errors, log via slog, reject all token validations without panicking; return appropriate OAuth error responses
- [ ] T052 [P] Implement TLS enforcement in `cmd/synapbus/main.go` (or serve command): when `--tls` is set, refuse to start OAuth endpoints on plain HTTP; in `--dev` mode, allow HTTP with logged `slog.Warn` (SC-006)
- [ ] T053 [P] Add configurable bcrypt cost in `internal/auth/config.go` and `internal/auth/user_store.go`: minimum 10, default 12, configurable via `--bcrypt-cost` flag or `SYNAPBUS_BCRYPT_COST` env var; validate range at startup
- [ ] T054 [P] Add configurable token TTLs via flags/env: `--access-token-ttl` (default 1h), `--refresh-token-lifetime` (default 30d), `--session-lifetime` (default 24h); wire into auth config (FR-012)
- [ ] T055 [P] Verify zero-CGO build compatibility: add build tag test or CI step that runs `CGO_ENABLED=0 go build ./...` and `CGO_ENABLED=0 go test ./internal/auth/...` to confirm no C dependencies (SC-007)
- [ ] T056 End-to-end integration test in `internal/auth/e2e_test.go`: full lifecycle -- register user -> log in with PKCE -> access protected endpoint -> refresh token -> change password -> re-login -> register OAuth client -> client_credentials token -> introspect -> logout; covers SC-001 through SC-005
- [ ] T057 Code cleanup: ensure all exported types have godoc comments in `internal/auth/`; ensure error messages are consistent; ensure all SQL queries use parameterized statements (no SQL injection)

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies -- can start immediately
- **Foundational (Phase 2)**: Depends on Phase 1 completion -- BLOCKS all user stories
- **User Story 1 (Phase 3)**: Depends on Phase 2 -- can start immediately after
- **User Story 5 (Phase 4)**: Depends on Phase 3 (hardens PKCE already built in US1)
- **User Story 2 (Phase 5)**: Depends on Phase 2 -- can run in parallel with Phase 3/4
- **User Story 3 (Phase 6)**: Depends on Phase 2 -- can run in parallel with Phase 3/4/5 (fosite store built in Phase 2)
- **User Story 4 (Phase 7)**: Depends on Phase 2 -- can run in parallel with others
- **Polish (Phase 8)**: Depends on all user stories being complete

### User Story Dependencies

- **US1 (P1)**: Foundation only -- no cross-story dependencies
- **US5 (P1)**: Validates PKCE enforcement built in US1 -- depends on US1 endpoints existing
- **US2 (P2)**: Foundation only -- independent of US1 (different grant type)
- **US3 (P2)**: Foundation only (fosite store) -- benefits from US1 being done for authorization_code refresh testing, but client_credentials refresh can test independently
- **US4 (P3)**: Foundation + US1 (needs session infrastructure) -- depends on US1 for session context

### Within Each User Story

- Tests MUST be written and FAIL before implementation
- Domain models/stores before handlers
- Handlers before route registration
- Core implementation before integration wiring
- Story complete before moving to next priority

### Parallel Opportunities

- Phase 2: T006, T007, T008 (domain models) can run in parallel
- Phase 2: T009, T010, T011 (store implementations) can run in parallel after their models
- Phase 3+: All test tasks marked [P] within a story can run in parallel
- US2, US3, US4 can theoretically start in parallel after Phase 2, but sequential P1->P2->P3 is recommended for a single developer

---

## Implementation Strategy

### MVP First (User Stories 1 + 5)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational (CRITICAL -- blocks all stories)
3. Complete Phase 3: User Story 1 (registration + login + session)
4. Complete Phase 4: User Story 5 (PKCE hardening)
5. **STOP and VALIDATE**: Test full login flow independently
6. Deploy/demo if ready -- Web UI is access-controlled

### Incremental Delivery

1. Setup + Foundational -> Foundation ready
2. US1 + US5 -> Interactive login with PKCE -> Deploy (MVP!)
3. US2 -> Programmatic access via client credentials -> Deploy
4. US3 -> Refresh token rotation for long-lived sessions -> Deploy
5. US4 -> Account management for self-service -> Deploy
6. Polish -> Hardening and edge cases -> Production ready

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- All OAuth data stored in SQLite within `--data` directory per Constitution Principle I
- `ory/fosite` is the OAuth 2.1 framework; storage adapter in `internal/auth/fosite_store.go` is the primary integration point
- `modernc.org/sqlite` is already the project database (zero CGO, Principle III)
- Existing schema (`001_initial.sql`) has `users`, `oauth_tokens`, `oauth_clients` tables; migration 002 extends them
- Commit after each task or logical group
- Stop at any checkpoint to validate story independently
