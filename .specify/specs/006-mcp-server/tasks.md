# Tasks: MCP Server

**Input**: Design documents from `/specs/006-mcp-server/`
**Prerequisites**: spec.md (required), constitution.md (required)

**Tests**: Not explicitly requested in the feature specification. Test tasks are omitted.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Add `mark3labs/mcp-go` dependency and create the base package structure for the MCP server.

- [ ] T001 Add `mark3labs/mcp-go` and `go-chi/chi` dependencies to `go.mod` via `go get`
- [ ] T002 [P] Create package scaffold `internal/mcp/doc.go` with package-level doc comment describing the MCP server subsystem
- [ ] T003 [P] Create `internal/mcp/config.go` with `Config` struct: server name, version, SSE path (default `/mcp/sse`), Streamable HTTP path (default `/mcp`), health path (default `/health`), max connections (default 100), shutdown timeout (default 30s)

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core types, interfaces, and the engine bridge that ALL user stories depend on. No MCP tool or transport can work without these.

**CRITICAL**: No user story work can begin until this phase is complete.

- [ ] T004 Define `ToolCallContext` struct in `internal/mcp/context.go` containing: agent ID (resolved from API key), request ID, tool name, raw arguments map, and a `context.Context` field for propagation
- [ ] T005 [P] Define `MCPConnection` struct in `internal/mcp/connection.go` with: connection ID (UUID), agent ID, transport type enum (`sse` | `streamable-http`), connected-at timestamp, last-activity timestamp, remote address
- [ ] T006 [P] Define `HealthStatus` struct in `internal/mcp/health.go` with: overall status (`ok` | `degraded` | `unhealthy`), server version, uptime, component statuses map (database, mcp_server), active connection count
- [ ] T007 Define `Engine` interface in `internal/mcp/engine.go` that abstracts the core operations the MCP tools will call: `SendMessage`, `ReadInbox`, `ClaimMessages`, `MarkDone`, `SearchMessages`, `CreateChannel`, `JoinChannel`, `ListChannels`, `RegisterAgent`, `DiscoverAgents`, `ValidateAPIKey(ctx, key) (agentID, error)`, `HealthCheck(ctx) HealthStatus`
- [ ] T008 Implement `ConnectionManager` in `internal/mcp/connmgr.go`: thread-safe map of active `MCPConnection` entries with `Add`, `Remove`, `Get`, `List`, `UpdateActivity`, and `Count` methods using `sync.RWMutex`
- [ ] T009 [P] Add structured `slog` logging helpers in `internal/mcp/logging.go`: `LogToolCall(logger, agentID, toolName, duration, err)` and `LogConnection(logger, agentID, transport, event)` that emit structured key-value log entries per FR-012

**Checkpoint**: Foundation ready - all types, interfaces, and connection tracking in place. User story implementation can now begin.

---

## Phase 3: User Story 1 - Agent Connects and Calls Messaging Tools via SSE (Priority: P1) MVP

**Goal**: An MCP client can connect over SSE, list all tools with JSON schemas, and execute `send_message` / `read_inbox` round-trips.

**Independent Test**: Start `synapbus serve`, connect an MCP client (e.g., `@modelcontextprotocol/inspector`) over SSE at `/mcp/sse`, list tools, send a message, read inbox.

### Implementation for User Story 1

- [ ] T010 [US1] Implement tool definitions in `internal/mcp/tools.go`: register all 10 MCP tools (`send_message`, `read_inbox`, `claim_messages`, `mark_done`, `search_messages`, `create_channel`, `join_channel`, `list_channels`, `register_agent`, `discover_agents`) with `mcp.NewTool()` from `mark3labs/mcp-go`, each with name, description string, and JSON Schema `inputSchema` defining `type`, `properties`, `required`, and per-property `description` fields (FR-006, FR-007)
- [ ] T011 [US1] Implement tool handler dispatch in `internal/mcp/handlers.go`: a `HandleToolCall(ctx, toolCallContext) (*mcp.CallToolResult, error)` function that switches on tool name, extracts typed arguments from the raw map, calls the corresponding `Engine` interface method, and wraps the result as MCP tool content (text JSON). Return `isError: true` with descriptive messages on failures (FR-013)
- [ ] T012 [US1] Implement `MCPServer` in `internal/mcp/server.go`: constructor `NewMCPServer(cfg Config, engine Engine, logger *slog.Logger)` that creates a `mark3labs/mcp-go` server instance via `server.NewMCPServer()`, registers all tools from T010, wires the tool call handler from T011, and stores references to the `ConnectionManager` and `Engine`
- [ ] T013 [US1] Implement SSE transport setup in `internal/mcp/transport_sse.go`: function `NewSSEHandler(mcpServer, connMgr, logger) http.Handler` that creates a `mark3labs/mcp-go` SSE transport handler, wraps it to track connections in `ConnectionManager` on connect/disconnect, and updates last-activity on each message (FR-002, FR-010, FR-011)
- [ ] T014 [US1] Implement `Mount(router chi.Router)` method on `MCPServer` in `internal/mcp/server.go` that mounts the SSE handler at the configured SSE path (default `/mcp/sse`) and the health endpoint at `/health` on the provided chi router
- [ ] T015 [US1] Wire MCP server into `cmd/synapbus/main.go`: in `runServe`, create a chi router, instantiate `MCPServer` with config and a stub `Engine` implementation, call `Mount`, and start `http.Server` with graceful shutdown on SIGTERM/SIGINT (FR-014)
- [ ] T016 [US1] Implement graceful shutdown in `internal/mcp/server.go`: `Shutdown(ctx context.Context) error` method that stops accepting new connections, waits for in-flight requests up to the configured timeout, closes all tracked SSE connections, and logs shutdown progress (FR-014)
- [ ] T017 [US1] Add `slog` logging to all tool call paths in `internal/mcp/handlers.go`: wrap each tool call with timing, log agent ID, tool name, duration, and success/failure using the helpers from T009 (FR-012)

**Checkpoint**: At this point, an MCP client can connect via SSE, list all 10 tools with full JSON schemas, and call any tool (dispatched to the Engine interface). This is the MVP.

---

## Phase 4: User Story 2 - Agent Authenticates with API Key in MCP Headers (Priority: P1)

**Goal**: MCP connections are authenticated via `Authorization: Bearer <api-key>` header. Unauthenticated requests are rejected with HTTP 401. All tool calls are scoped to the authenticated agent's identity.

**Independent Test**: Attempt connections with valid, invalid, and missing API keys. Verify 401 rejection for bad keys. Verify tool calls are scoped to the authenticated agent.

### Implementation for User Story 2

- [ ] T018 [US2] Implement auth middleware in `internal/mcp/auth.go`: `AuthMiddleware(engine Engine, logger *slog.Logger) func(http.Handler) http.Handler` that extracts `Authorization: Bearer <key>` from the request header, calls `engine.ValidateAPIKey()`, and either injects the agent ID into the request context or responds with HTTP 401 Unauthorized (FR-004, FR-005)
- [ ] T019 [US2] Define context key and helpers in `internal/mcp/auth.go`: `AgentIDFromContext(ctx) (string, bool)` to retrieve the authenticated agent ID from context, used by tool handlers to scope operations
- [ ] T020 [US2] Update `HandleToolCall` in `internal/mcp/handlers.go` to extract agent ID from context via `AgentIDFromContext`, populate `ToolCallContext.AgentID`, and pass it to all `Engine` method calls. Reject calls where agent ID is missing with an MCP error result
- [ ] T021 [US2] Add authorization enforcement in `internal/mcp/handlers.go`: for `read_inbox`, verify the requested agent matches the authenticated agent; for `send_message`, enforce the sender is the authenticated agent. Return MCP error result with "access denied" for violations (FR-008)
- [ ] T022 [US2] Wire auth middleware into `Mount` in `internal/mcp/server.go`: apply `AuthMiddleware` to the SSE route group so that authentication happens before the MCP protocol handshake. Unauthenticated clients never reach the SSE handler

**Checkpoint**: At this point, User Stories 1 AND 2 are complete. SSE connections require valid API keys, tool calls are identity-scoped, and unauthorized access is rejected.

---

## Phase 5: User Story 3 - Agent Connects via Streamable HTTP Transport (Priority: P2)

**Goal**: Agents can use Streamable HTTP (POST to `/mcp`) as an alternative to SSE. Same tools, same auth, same results.

**Independent Test**: Send MCP JSON-RPC requests via HTTP POST to `/mcp` with a valid API key. Verify responses match SSE behavior for the same tool calls.

### Implementation for User Story 3

- [ ] T023 [US3] Implement Streamable HTTP transport in `internal/mcp/transport_streamhttp.go`: function `NewStreamableHTTPHandler(mcpServer, connMgr, logger) http.Handler` that creates a `mark3labs/mcp-go` Streamable HTTP transport handler, tracks per-request logical connections in `ConnectionManager` (FR-003, FR-010)
- [ ] T024 [US3] Update `Mount` in `internal/mcp/server.go` to mount the Streamable HTTP handler at the configured path (default `/mcp`) with the same `AuthMiddleware` applied. Validate API key on each POST request (FR-004)
- [ ] T025 [US3] Update `ConnectionManager` in `internal/mcp/connmgr.go` to handle Streamable HTTP logical connections: create connection entry on request start, remove on request completion, set transport type to `streamable-http`

**Checkpoint**: Both SSE and Streamable HTTP transports are functional with identical tool behavior and authentication. SC-008 is achievable.

---

## Phase 6: User Story 4 - Operator Monitors Server Health and Connected Agents (Priority: P2)

**Goal**: Health endpoint at `/health` reports server status, version, and component health. Operators can query active connections.

**Independent Test**: Call `/health` and verify JSON response. Connect multiple agents, query connection list, disconnect one, verify it disappears.

### Implementation for User Story 4

- [ ] T026 [US4] Implement health check handler in `internal/mcp/health.go`: `NewHealthHandler(engine Engine, connMgr *ConnectionManager, version string, startTime time.Time, logger *slog.Logger) http.HandlerFunc` that calls `engine.HealthCheck()`, adds connection count from `ConnectionManager`, computes uptime, and returns JSON `HealthStatus` with HTTP 200 (ok) or HTTP 503 (degraded/unhealthy) (FR-009)
- [ ] T027 [US4] Handle startup and database-unavailable states in `internal/mcp/health.go`: if the engine returns a database error, set component status to unhealthy and overall status to `degraded` or `unhealthy`. During startup (before engine is ready), return HTTP 503 with `{"status": "starting"}`
- [ ] T028 [US4] Add `ListConnections` endpoint or MCP tool in `internal/mcp/connmgr.go`: return all active connections with agent ID, transport type, connected-at, and last-activity timestamps. Expose via the health handler as an optional `?connections=true` query parameter (FR-010)
- [ ] T029 [US4] Wire health endpoint in `Mount` in `internal/mcp/server.go`: mount the health handler at `/health` WITHOUT auth middleware (unauthenticated per FR-009)

**Checkpoint**: Operators can monitor server health, see connected agents, and integrate with load balancers / Kubernetes probes.

---

## Phase 7: User Story 5 - Agent Discovers Available Tools with Full JSON Schemas (Priority: P3)

**Goal**: Every MCP tool has comprehensive, human-readable JSON Schema definitions with descriptions on all parameters, correct types, required fields, and constraints.

**Independent Test**: Connect and call `tools/list`. Validate every tool's `inputSchema` is a valid JSON Schema with descriptions on all properties.

### Implementation for User Story 5

- [ ] T030 [US5] Enhance `send_message` tool schema in `internal/mcp/tools.go`: define `to` (string, required, description), `body` (string, required, description), `subject` (string, optional, description), `priority` (integer, optional, minimum 1, maximum 10, description), `channel_id` (string, optional, description), `metadata` (object, optional, description) per acceptance scenario 3
- [ ] T031 [US5] Enhance all remaining tool schemas in `internal/mcp/tools.go`: for each of the 10 tools, ensure every input property has a `description` string, correct `type`, and that `required` arrays are accurate. Add enum constraints where applicable (e.g., channel type in `create_channel`)
- [ ] T032 [US5] Add tool-level descriptions in `internal/mcp/tools.go`: ensure every tool's top-level `description` is a clear, plain-language sentence explaining what the tool does, suitable for LLM consumption

**Checkpoint**: All tools have production-quality JSON schemas. SC-002 is achievable.

---

## Phase 8: Edge Cases & Robustness

**Purpose**: Handle the edge cases enumerated in the spec: revoked keys, dropped connections, max connections, malformed requests, non-existent entities, duplicate connections.

- [ ] T033 [P] Implement connection limit enforcement in `internal/mcp/connmgr.go`: reject new connections with HTTP 503 when `Count()` reaches `Config.MaxConnections`, return descriptive error message
- [ ] T034 [P] Implement SSE disconnect detection in `internal/mcp/transport_sse.go`: detect closed client connections (context cancellation, write errors), clean up `ConnectionManager` entry within 5 seconds (SC-006)
- [ ] T035 [P] Implement revoked API key handling in `internal/mcp/auth.go`: on each tool call (not just connection), re-validate the API key via `Engine.ValidateAPIKey`. If revoked, return MCP error result and close the SSE connection
- [ ] T036 [P] Implement malformed request handling in `internal/mcp/transport_sse.go` and `internal/mcp/transport_streamhttp.go`: ensure `mark3labs/mcp-go` returns standard JSON-RPC errors (`-32700` parse error, `-32600` invalid request) for malformed input rather than crashing
- [ ] T037 Implement non-existent entity errors in `internal/mcp/handlers.go`: when `Engine` methods return "not found" errors (e.g., sending to a non-existent agent), wrap them as MCP tool error results (`isError: true`) with clear messages, not protocol-level errors
- [ ] T038 [P] Handle concurrent connections from same API key in `internal/mcp/connmgr.go`: support multiple simultaneous connections per agent, track each with a unique connection ID, list all sessions for the same agent

---

## Phase 9: Polish & Cross-Cutting Concerns

**Purpose**: Improvements that affect multiple user stories.

- [ ] T039 [P] Review and verify all `slog` structured log fields across `internal/mcp/` are consistent: agent_id, tool_name, transport, duration_ms, connection_id, error (SC-007)
- [ ] T040 [P] Verify graceful shutdown sequence in `internal/mcp/server.go`: SIGTERM stops new connections, drains in-flight requests within timeout, closes all SSE connections, logs each step (FR-014)
- [ ] T041 Add server version injection: pass build version (via `-ldflags`) from `cmd/synapbus/main.go` through to `MCPServer` config and health endpoint response
- [ ] T042 Verify both transports produce identical results: manually or via script, run the same tool call sequence against SSE and Streamable HTTP and confirm matching output (SC-008)
- [ ] T043 Run `make lint` and fix any linting issues in `internal/mcp/` package

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies - can start immediately
- **Foundational (Phase 2)**: Depends on Phase 1 completion - BLOCKS all user stories
- **User Story 1 (Phase 3)**: Depends on Phase 2 - delivers MVP
- **User Story 2 (Phase 4)**: Depends on Phase 2, integrates with Phase 3 (adds auth to existing SSE)
- **User Story 3 (Phase 5)**: Depends on Phase 2, integrates with Phase 3 and Phase 4 (adds second transport)
- **User Story 4 (Phase 6)**: Depends on Phase 2, uses ConnectionManager from Phase 3
- **User Story 5 (Phase 7)**: Depends on Phase 3 (tool definitions must exist to enhance)
- **Edge Cases (Phase 8)**: Depends on Phases 3-6 being complete
- **Polish (Phase 9)**: Depends on all prior phases

### User Story Dependencies

- **US1 (P1)**: Can start after Phase 2 - no dependencies on other stories
- **US2 (P1)**: Can start after Phase 2 - integrates with US1's SSE transport but is independently testable
- **US3 (P2)**: Can start after Phase 2 - reuses tools from US1, auth from US2, but adds an independent transport
- **US4 (P2)**: Can start after Phase 2 - uses ConnectionManager but is otherwise independent
- **US5 (P3)**: Depends on US1 tool definitions existing - enhances schema quality

### Recommended Execution Order (Single Developer)

1. Phase 1 (Setup) + Phase 2 (Foundational)
2. Phase 3 (US1 - SSE + Tools) - **MVP checkpoint**
3. Phase 4 (US2 - Auth) - secures the MVP
4. Phase 5 (US3 - Streamable HTTP) + Phase 6 (US4 - Health) in parallel
5. Phase 7 (US5 - Schema polish)
6. Phase 8 (Edge cases)
7. Phase 9 (Polish)

### Parallel Opportunities

- T002 and T003 (Phase 1) can run in parallel
- T005, T006, and T009 (Phase 2) can run in parallel
- T033, T034, T035, T036, and T038 (Phase 8) can run in parallel
- T039 and T040 (Phase 9) can run in parallel
- Phase 5 and Phase 6 can run in parallel after Phase 4

---

## Notes

- All file paths are under `internal/mcp/` per the project's directory structure in CLAUDE.md
- The `Engine` interface (T007) is the critical abstraction: it decouples MCP tools from the core messaging/channels/agents implementations in other `internal/` packages
- A stub `Engine` implementation is needed for US1 to be testable before core messaging is built (T015)
- `mark3labs/mcp-go` handles MCP protocol details (JSON-RPC, SSE framing); our code handles auth, connection tracking, tool dispatch, and engine bridging
- Zero CGO constraint is satisfied: `mark3labs/mcp-go` and `go-chi/chi` are pure Go (Principle III)
