# Feature Specification: Human Auth (OAuth 2.1)

**Feature Branch**: `003-human-auth`
**Created**: 2026-03-13
**Status**: Draft
**Input**: User description: "OAuth 2.1 authorization server embedded in SynapBus using fosite, with local accounts, token endpoints, PKCE, session management, refresh token rotation, and user CRUD."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Human Owner Logs Into Web UI (Priority: P1)

A human user opens the SynapBus Web UI in their browser, creates a local account with a username and password, and logs in. After authentication, the browser holds an httponly session cookie that grants access to all Web UI endpoints. The user can view their dashboard, see their agents, and browse messages without re-authenticating until the session expires or they log out.

**Why this priority**: Without human authentication, no other feature in SynapBus can enforce ownership or multi-tenancy. This is the foundation for Principle IV (Multi-Tenant with Ownership). A working login flow is the single most critical auth capability.

**Independent Test**: Can be fully tested by starting `synapbus serve`, opening the Web UI, creating an account via the registration form, logging in, and verifying the session cookie is set and subsequent API calls succeed. Delivers value as a standalone access-control gate for the Web UI.

**Acceptance Scenarios**:

1. **Given** a running SynapBus instance with no accounts, **When** a user navigates to `/register` and submits a valid username (3-64 chars, alphanumeric + underscore) and password (minimum 8 characters), **Then** the account is created, the password is stored as a bcrypt hash (cost >= 12), and the user is redirected to the login page.
2. **Given** a registered user, **When** they submit valid credentials to the OAuth authorization code flow with PKCE (`/oauth/authorize` -> `/oauth/token`), **Then** the server issues an access token and refresh token, sets an httponly secure cookie containing the session identifier, and redirects to the Web UI dashboard.
3. **Given** a logged-in user with a valid session cookie, **When** they make requests to protected Web UI API endpoints, **Then** the server validates the session and returns the requested data with the user's identity in context.
4. **Given** a logged-in user, **When** they click "Log out", **Then** the session is invalidated server-side, the session cookie is cleared, and subsequent requests redirect to the login page.

---

### User Story 2 - Programmatic Client Obtains Tokens via Client Credentials (Priority: P2)

An external automation script or CI pipeline needs to interact with the SynapBus REST API (e.g., to provision agents or query system status). The operator registers an OAuth client with a `client_id` and `client_secret`, then uses the `client_credentials` grant type to obtain an access token from `/oauth/token`. The token is used in `Authorization: Bearer <token>` headers for subsequent API calls.

**Why this priority**: Client credentials flow enables programmatic access without a browser, which is essential for DevOps automation, agent provisioning scripts, and integration testing. It is the second most common auth flow after interactive login.

**Independent Test**: Can be tested by registering an OAuth client via `synapbus` CLI or admin API, then using `curl` to POST to `/oauth/token` with `grant_type=client_credentials`, verifying a valid access token is returned, and using that token to call a protected endpoint.

**Acceptance Scenarios**:

1. **Given** a registered OAuth client with `client_id` and `client_secret`, **When** the client sends a POST to `/oauth/token` with `grant_type=client_credentials` and valid credentials, **Then** the server returns a JSON response containing `access_token`, `token_type: "bearer"`, `expires_in` (default 3600 seconds), and `scope`.
2. **Given** a valid access token obtained via client credentials, **When** the token is sent in the `Authorization: Bearer` header to `/oauth/introspect`, **Then** the server responds with `active: true`, the client identity, granted scopes, and expiration time.
3. **Given** an expired access token, **When** it is presented to any protected endpoint, **Then** the server responds with HTTP 401 and a `WWW-Authenticate` header indicating the token is expired.

---

### User Story 3 - Refresh Token Rotation (Priority: P2)

A logged-in Web UI user's access token expires after its TTL. The browser (or a programmatic client using authorization_code grant) silently refreshes the session by exchanging the refresh token for a new access token and a new refresh token. The old refresh token is invalidated immediately upon use, preventing replay attacks.

**Why this priority**: Refresh token rotation is critical for security in long-lived sessions. Without it, a stolen refresh token grants indefinite access. This is a P2 because it builds on top of the P1 login flow and is required for production-grade security.

**Independent Test**: Can be tested by obtaining a token pair, waiting for the access token to expire (or using a short TTL in test config), exchanging the refresh token at `/oauth/token` with `grant_type=refresh_token`, verifying a new token pair is returned, and confirming the old refresh token is rejected on a second attempt.

**Acceptance Scenarios**:

1. **Given** a user with a valid refresh token, **When** they POST to `/oauth/token` with `grant_type=refresh_token` and the current refresh token, **Then** the server returns a new access token and a new refresh token, and the old refresh token is marked as consumed in storage.
2. **Given** a refresh token that has already been used (consumed), **When** a client attempts to use it again, **Then** the server rejects the request with HTTP 401 and revokes all tokens in the grant chain (the new refresh token issued from the original is also revoked) as a security precaution against token theft.
3. **Given** a refresh token that has exceeded its absolute lifetime (e.g., 30 days), **When** a client attempts to use it, **Then** the server rejects the request with HTTP 401 and the user must re-authenticate.

---

### User Story 4 - User Account Management (Priority: P3)

A logged-in human user manages their account through the Web UI or REST API. They can change their password (requiring the current password for verification), view the list of agents they own, and see their active OAuth sessions. An admin user (the first account created, or one explicitly granted admin role) can list all users and deactivate accounts.

**Why this priority**: Account management is a supporting capability. Users can function with a fixed password and no self-service management in an MVP, but password changes and agent listing are necessary for production use.

**Independent Test**: Can be tested by logging in, calling `PUT /api/users/me/password` with old and new passwords, verifying the old password no longer works and the new one does. Agent listing can be tested by creating agents under the user and calling `GET /api/users/me/agents`.

**Acceptance Scenarios**:

1. **Given** a logged-in user, **When** they submit a password change request with their correct current password and a new password meeting the minimum length requirement, **Then** the password is updated (bcrypt re-hashed), all existing sessions except the current one are invalidated, and the server returns HTTP 200.
2. **Given** a logged-in user who owns 3 registered agents, **When** they call `GET /api/users/me/agents`, **Then** the server returns a JSON array of their 3 agents with `agent_id`, `name`, `display_name`, `type`, and `created_at` fields.
3. **Given** a user submitting a password change with an incorrect current password, **When** the request is processed, **Then** the server returns HTTP 403 and the password remains unchanged.

---

### User Story 5 - PKCE Enforcement on Authorization Code Flow (Priority: P1)

Any client using the authorization code grant MUST include a PKCE `code_challenge` in the authorization request and the corresponding `code_verifier` when exchanging the code for tokens. Requests without PKCE parameters are rejected. This prevents authorization code interception attacks, even for confidential clients.

**Why this priority**: PKCE is mandated by Constitution Principle V and OAuth 2.1 specification. It is not optional; it is a security requirement baked into the protocol. This is P1 because the authorization code flow (User Story 1) cannot ship without it.

**Independent Test**: Can be tested by sending an authorization request to `/oauth/authorize` without `code_challenge` and verifying it is rejected, then sending one with a valid S256 code challenge and verifying the flow completes when the correct `code_verifier` is provided at the token endpoint.

**Acceptance Scenarios**:

1. **Given** a client initiating an authorization code flow, **When** the request to `/oauth/authorize` omits the `code_challenge` parameter, **Then** the server responds with HTTP 400 and an error `invalid_request` indicating PKCE is required.
2. **Given** a client that included a valid `code_challenge` (method S256) in the authorization request and received an authorization code, **When** the client exchanges the code at `/oauth/token` with the correct `code_verifier`, **Then** the server validates `SHA256(code_verifier) == code_challenge` and issues tokens.
3. **Given** a client that provides an incorrect `code_verifier` at the token endpoint, **When** the exchange is attempted, **Then** the server responds with HTTP 400 and an error `invalid_grant`, and the authorization code is invalidated (single-use enforcement).

---

### Edge Cases

- What happens when a user tries to register a username that already exists? The server MUST return HTTP 409 Conflict with a clear error message. It MUST NOT reveal whether the username exists via timing differences (use constant-time comparison or always hash before checking).
- What happens when the bcrypt cost factor makes login unacceptably slow on low-powered hardware? The system MUST use a configurable bcrypt cost (minimum 10, default 12) so operators can tune it for their hardware.
- What happens when the SQLite database holding OAuth tokens becomes corrupted or is deleted while sessions are active? All sessions become invalid, and users must re-authenticate. The server MUST handle missing/corrupt token storage gracefully by logging the error and rejecting all token validations rather than panicking.
- What happens when a client sends a `code_challenge_method` of `plain` instead of `S256`? The server MUST reject it. Only S256 is supported per OAuth 2.1 requirements.
- What happens when multiple concurrent requests attempt to use the same refresh token simultaneously? Only the first request succeeds. Subsequent requests MUST trigger revocation of the entire token family as a potential replay attack.
- What happens when the system has zero registered users and a request hits a protected endpoint? The server MUST redirect to the registration page (Web UI) or return HTTP 401 (API), never expose data.
- What happens when a user attempts to register with a password shorter than 8 characters? The server MUST return HTTP 400 with a validation error. Password requirements: minimum 8 characters, no maximum length cap (up to 72 bytes, the bcrypt limit), no character class requirements.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST embed an OAuth 2.1 authorization server using `ory/fosite`, started as part of `synapbus serve`, with no external service dependencies.
- **FR-002**: System MUST support local account creation with `username` (unique, 3-64 chars, `[a-zA-Z0-9_]`) and `password` (8+ characters, bcrypt hashed with configurable cost, default 12).
- **FR-003**: System MUST expose `/oauth/authorize` endpoint supporting the authorization code grant with PKCE (S256 only). The `plain` challenge method MUST be rejected.
- **FR-004**: System MUST expose `/oauth/token` endpoint supporting `grant_type=authorization_code` (with PKCE verification) and `grant_type=client_credentials`.
- **FR-005**: System MUST expose `/oauth/introspect` endpoint per RFC 7662, returning token validity, scopes, client identity, and expiration.
- **FR-006**: System MUST implement refresh token rotation: each refresh token use issues a new refresh token and invalidates the old one. Reuse of a consumed refresh token MUST revoke the entire token family.
- **FR-007**: System MUST manage Web UI sessions via httponly, secure (when TLS enabled), SameSite=Lax cookies. Session lifetime MUST be configurable (default 24 hours).
- **FR-008**: System MUST provide user CRUD operations: create account (`POST /api/users`), change password (`PUT /api/users/me/password`), list owned agents (`GET /api/users/me/agents`), get current user profile (`GET /api/users/me`).
- **FR-009**: System MUST enforce that every agent has an `owner_id` referencing a valid user. Users MUST only see agents they own (unless admin).
- **FR-010**: System MUST store all OAuth data (clients, tokens, authorization codes, sessions) in the embedded SQLite database within the `--data` directory.
- **FR-011**: System MUST support registering OAuth clients via CLI command (`synapbus client create --name <name>`) or admin API, generating a `client_id` and `client_secret` pair.
- **FR-012**: Access tokens MUST have a configurable TTL (default 1 hour). Refresh tokens MUST have a configurable absolute lifetime (default 30 days).
- **FR-013**: System MUST log all authentication events (login success, login failure, token issuance, token refresh, token revocation) via `slog` with structured fields including user/client identity and remote IP.

### Key Entities

- **User**: A human account. Attributes: `id` (UUID), `username` (unique), `password_hash` (bcrypt), `role` (user/admin), `created_at`, `updated_at`. Owns zero or more Agents. Owns zero or more OAuthClients.
- **OAuthClient**: A registered OAuth client (Web UI frontend, CLI tool, or external automation). Attributes: `client_id` (generated, unique), `client_secret_hash` (bcrypt), `name`, `redirect_uris` (JSON array), `grant_types` (JSON array), `scopes` (JSON array), `owner_id` (FK to User), `created_at`.
- **OAuthToken**: An issued access or refresh token. Managed internally by fosite's storage interface. Attributes include: `signature` (token lookup key), `client_id`, `user_id`, `scopes`, `expires_at`, `created_at`, `session_data` (JSON).
- **AuthorizationCode**: A short-lived code issued during the authorization code flow. Attributes: `code` (hashed), `client_id`, `user_id`, `redirect_uri`, `scopes`, `code_challenge`, `code_challenge_method`, `expires_at`, `created_at`, `used` (boolean).
- **Session**: A Web UI session linking a browser cookie to a user identity. Attributes: `session_id` (random, stored in cookie), `user_id`, `created_at`, `expires_at`, `last_active_at`.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A new user can create an account and log into the Web UI in under 30 seconds (wall-clock time from opening registration page to seeing the dashboard).
- **SC-002**: The full authorization code flow with PKCE (authorize -> code -> token exchange) completes in under 500ms server-side processing time (excluding network latency and user interaction).
- **SC-003**: Client credentials token issuance responds in under 100ms at the 99th percentile under 50 concurrent requests.
- **SC-004**: Refresh token rotation correctly invalidates old tokens in 100% of cases, verified by an automated test that attempts reuse of consumed refresh tokens and confirms rejection.
- **SC-005**: All 13 functional requirements have corresponding integration tests that pass in CI, covering both success paths and error paths (invalid credentials, expired tokens, missing PKCE, duplicate usernames).
- **SC-006**: Zero authentication endpoints are accessible without TLS in production mode (when `--tls` flag is set). In development mode (`--dev`), HTTP is permitted with a logged warning.
- **SC-007**: The auth subsystem adds zero external runtime dependencies -- verified by building with `CGO_ENABLED=0` and running all auth tests against the embedded SQLite store.
