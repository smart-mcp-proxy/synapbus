# Feature Specification: MCP Server

**Feature Branch**: `006-mcp-server`
**Created**: 2026-03-13
**Status**: Draft
**Input**: User description: "MCP server using mark3labs/mcp-go with SSE and Streamable HTTP transports, exposing all messaging operations as MCP tools with API key auth, JSON schema tool listing, health check, and connection management."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Agent Connects and Calls Messaging Tools via SSE (Priority: P1)

An AI agent operator configures their MCP client (e.g., Claude Desktop, a custom LLM agent) to connect to SynapBus's MCP server over SSE transport. The agent authenticates using an API key passed in the `Authorization` header. Once connected, the agent lists available tools, sees JSON schema descriptions for each, and calls `send_message` to send a direct message to another agent. The agent then calls `read_inbox` to check for incoming messages.

**Why this priority**: This is the foundational interaction path. Without SSE transport and tool execution working end-to-end, no agent can use SynapBus at all. SSE is the primary transport per the constitution (Principle II).

**Independent Test**: Can be fully tested by starting `synapbus serve`, connecting an MCP client over SSE with a valid API key, listing tools, and sending/reading a message. Delivers the core value of MCP-native agent messaging.

**Acceptance Scenarios**:

1. **Given** SynapBus is running and an agent has a valid API key, **When** the agent connects via SSE transport at `/mcp/sse`, **Then** the connection is established, the server sends an SSE endpoint event, and the agent can issue MCP `initialize` and `tools/list` requests.
2. **Given** an agent is connected via SSE, **When** the agent calls `tools/list`, **Then** the server returns all available MCP tools with their names, descriptions, and JSON Schema `inputSchema` definitions.
3. **Given** an agent is connected via SSE, **When** the agent calls `tools/call` with `send_message` and a valid payload (recipient, body), **Then** the message is persisted and the server returns a success result containing the message ID.
4. **Given** an agent is connected via SSE, **When** the agent calls `tools/call` with `read_inbox`, **Then** the server returns the agent's pending messages as structured JSON content.

---

### User Story 2 - Agent Authenticates with API Key in MCP Headers (Priority: P1)

An AI agent attempts to connect to the MCP server. The server validates the API key provided in the HTTP `Authorization` header (as `Bearer <api-key>`) during the initial SSE or Streamable HTTP connection handshake. If the key is valid, the server associates the connection with the corresponding agent identity and allows tool calls scoped to that agent. If the key is missing or invalid, the server rejects the connection immediately.

**Why this priority**: Authentication is a hard requirement before any tool execution. Without it, any client could impersonate any agent or access arbitrary messages, violating Principle IV (multi-tenant with ownership).

**Independent Test**: Can be tested by attempting connections with valid, invalid, and missing API keys and verifying the server accepts or rejects appropriately.

**Acceptance Scenarios**:

1. **Given** an agent has a valid API key, **When** the agent connects to `/mcp/sse` with `Authorization: Bearer <valid-key>`, **Then** the connection is accepted and subsequent tool calls are scoped to that agent's identity.
2. **Given** no API key is provided, **When** a client connects to `/mcp/sse` without an `Authorization` header, **Then** the server responds with HTTP 401 Unauthorized and closes the connection.
3. **Given** an invalid API key is provided, **When** a client connects with `Authorization: Bearer <invalid-key>`, **Then** the server responds with HTTP 401 Unauthorized and the connection is not established.
4. **Given** an agent is authenticated, **When** the agent calls `send_message` specifying itself as the sender, **Then** the server accepts the call. **When** the agent attempts to call `read_inbox` for a different agent, **Then** the server returns an MCP error result indicating access denied.

---

### User Story 3 - Agent Connects via Streamable HTTP Transport (Priority: P2)

An agent operator whose environment does not support long-lived SSE connections (e.g., serverless functions, firewalled networks) connects to SynapBus using the Streamable HTTP transport. The agent sends MCP requests as HTTP POST to `/mcp` and receives responses (including streaming results) over the same HTTP connection. All the same tools and authentication mechanisms work identically to the SSE transport.

**Why this priority**: Streamable HTTP is the secondary transport. Some deployment environments cannot maintain persistent SSE connections, so this transport broadens compatibility. However, SSE covers the majority of use cases, making this P2.

**Independent Test**: Can be tested by sending MCP JSON-RPC requests via HTTP POST to `/mcp` with a valid API key and verifying tool responses match SSE behavior.

**Acceptance Scenarios**:

1. **Given** SynapBus is running, **When** an agent sends an MCP `initialize` request as HTTP POST to `/mcp` with `Authorization: Bearer <valid-key>`, **Then** the server responds with a valid MCP initialize result containing server capabilities.
2. **Given** an agent is using Streamable HTTP, **When** the agent calls `tools/list` via POST, **Then** the response contains the same tool set with the same JSON schemas as the SSE transport.
3. **Given** an agent is using Streamable HTTP, **When** the agent calls `tools/call` with `send_message`, **Then** the message is persisted identically to an SSE-originated call and the response format matches.

---

### User Story 4 - Operator Monitors Server Health and Connected Agents (Priority: P2)

A system operator or monitoring tool checks the MCP server's health endpoint to verify the service is running and responsive. The operator can also query the connection management subsystem to see how many agents are currently connected, their identities, transport type, and connection duration. This enables operational monitoring and capacity planning.

**Why this priority**: Health checks are essential for production deployments (load balancers, Kubernetes liveness probes, Docker health checks). Connection tracking supports observability (Principle VIII). However, these are operational concerns, not core messaging, making them P2.

**Independent Test**: Can be tested by calling the health endpoint and verifying the response, then connecting multiple agents and querying the connection list.

**Acceptance Scenarios**:

1. **Given** SynapBus is running, **When** an HTTP GET request is sent to `/health`, **Then** the server responds with HTTP 200 and a JSON body containing at minimum `{"status": "ok"}` and the server version.
2. **Given** SynapBus is running but the database is unreachable, **When** an HTTP GET request is sent to `/health`, **Then** the server responds with HTTP 503 and a JSON body indicating the unhealthy component.
3. **Given** three agents are connected via SSE and one via Streamable HTTP, **When** an authenticated operator queries connected agents (via an internal MCP tool or REST endpoint), **Then** the response lists all four connections with agent ID, transport type (`sse` or `streamable-http`), connected-at timestamp, and last-activity timestamp.
4. **Given** an agent disconnects (SSE connection drops), **When** the operator queries connected agents, **Then** the disconnected agent is no longer listed.

---

### User Story 5 - Agent Discovers Available Tools with Full JSON Schemas (Priority: P3)

A newly developed agent connects to SynapBus for the first time and needs to understand what operations are available. The agent calls `tools/list` and receives a comprehensive list of all MCP tools with human-readable descriptions and full JSON Schema definitions for each tool's input parameters. The schemas include property types, required fields, enums for constrained values, and description strings for each parameter.

**Why this priority**: Tool discovery is handled by the MCP protocol's built-in `tools/list` method. While essential for agent usability, the JSON schema quality is an incremental improvement over having the tools work at all (covered in P1). This story focuses on schema completeness and documentation quality.

**Independent Test**: Can be tested by connecting and calling `tools/list`, then validating each returned tool's `inputSchema` is a valid JSON Schema with descriptions on all parameters.

**Acceptance Scenarios**:

1. **Given** an agent is connected, **When** the agent calls `tools/list`, **Then** every tool in the response has a non-empty `description` string explaining what the tool does in plain language.
2. **Given** an agent is connected, **When** the agent calls `tools/list`, **Then** every tool's `inputSchema` is a valid JSON Schema object with `type`, `properties`, and `required` fields defined.
3. **Given** an agent examines the `send_message` tool schema, **Then** the schema defines `to` (string, required), `body` (string, required), `subject` (string, optional), `priority` (integer, optional, minimum 1, maximum 10), `channel_id` (string, optional), and `metadata` (object, optional) with descriptions for each property.

---

### Edge Cases

- What happens when an agent's API key is revoked while the agent has an active SSE connection? The server MUST terminate the connection within a reasonable timeframe (e.g., on the next tool call or within 60 seconds via a background sweep).
- What happens when an SSE connection drops unexpectedly (network failure, client crash)? The server MUST detect the closed connection, clean up the connection tracking entry, and release any resources associated with that connection.
- What happens when the maximum number of concurrent connections is reached? The server MUST reject new connections with HTTP 503 Service Unavailable and a descriptive error message, rather than silently dropping or hanging.
- What happens when an agent sends a malformed MCP JSON-RPC request? The server MUST respond with a standard JSON-RPC error (`-32700` parse error or `-32600` invalid request) rather than crashing or returning an HTTP error.
- What happens when a tool call references an entity that does not exist (e.g., sending a message to a non-existent agent)? The server MUST return an MCP tool error result with a clear error message, not a protocol-level error.
- What happens when two agents connect with the same API key simultaneously? The server MUST either reject the second connection or support multiple concurrent connections per agent, with connection tracking reflecting all active sessions.
- What happens when the health check endpoint is called during server startup before the database is initialized? The server MUST return HTTP 503 with a status indicating the service is starting up.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST implement an MCP server using `mark3labs/mcp-go` that handles the full MCP protocol lifecycle: `initialize`, `initialized`, `tools/list`, `tools/call`, and `ping`.
- **FR-002**: System MUST support SSE transport, accepting connections on a configurable path (default `/mcp/sse`) with standard MCP SSE semantics (server-sent events for server-to-client, HTTP POST for client-to-server).
- **FR-003**: System MUST support Streamable HTTP transport, accepting MCP JSON-RPC requests via HTTP POST on a configurable path (default `/mcp`).
- **FR-004**: System MUST authenticate MCP connections using API keys passed in the HTTP `Authorization` header as `Bearer <api-key>`. The API key MUST be validated on the initial connection (SSE) or on each request (Streamable HTTP).
- **FR-005**: System MUST reject unauthenticated or invalidly authenticated connections with HTTP 401 Unauthorized before any MCP protocol messages are exchanged.
- **FR-006**: System MUST expose all messaging operations as MCP tools: `send_message`, `read_inbox`, `claim_messages`, `mark_done`, `search_messages`, `create_channel`, `join_channel`, `list_channels`, `register_agent`, `discover_agents`.
- **FR-007**: Each MCP tool MUST have a complete JSON Schema `inputSchema` with `type`, `properties`, `required` array, and human-readable `description` strings on both the tool itself and each input property.
- **FR-008**: System MUST scope all tool calls to the authenticated agent's identity. An agent MUST NOT be able to read another agent's inbox, send messages impersonating another agent, or access channels it has not joined.
- **FR-009**: System MUST provide a health check endpoint at `/health` (HTTP GET, no authentication required) that returns the server's health status, version, and component states (database, MCP server).
- **FR-010**: System MUST track all active MCP connections, recording: agent ID, transport type (SSE or Streamable HTTP), connection timestamp, and last activity timestamp.
- **FR-011**: System MUST clean up connection tracking entries when SSE connections are closed (client disconnect, server shutdown, or error).
- **FR-012**: System MUST log all MCP tool calls via `slog` structured logging, including agent ID, tool name, request duration, and success/failure status (Principle VIII).
- **FR-013**: System MUST return standard MCP error responses for tool failures (not HTTP errors), using the `isError` field in tool results with descriptive error messages.
- **FR-014**: System MUST handle graceful shutdown: on SIGTERM/SIGINT, the server MUST stop accepting new connections, allow in-flight requests to complete (with a configurable timeout, default 30 seconds), and close all SSE connections.

### Key Entities

- **MCPServer**: The top-level server component that registers tools, manages transports, and dispatches tool calls to the core engine. Wraps `mark3labs/mcp-go` server instance. Configured with server name, version, and transport options.
- **MCPConnection**: Represents an active agent connection. Attributes: connection ID (UUID), agent ID (resolved from API key), transport type (SSE or Streamable HTTP), connected-at timestamp, last-activity timestamp, remote address.
- **MCPTool**: A registered MCP tool definition. Attributes: name, description, input JSON schema, handler function reference. Each tool maps to a core engine operation.
- **ToolCallContext**: Per-request context created for each `tools/call` invocation. Contains: authenticated agent identity, request ID, tool name, raw arguments. Passed to the core engine handler for authorization and execution.
- **HealthStatus**: Response object for the `/health` endpoint. Attributes: overall status (ok/degraded/unhealthy), server version, uptime, component statuses (database, mcp_server), active connection count.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: An MCP client (e.g., `npx @modelcontextprotocol/inspector`) can connect via SSE, list tools, and execute a `send_message` / `read_inbox` round-trip within 5 seconds on localhost.
- **SC-002**: All MCP tools return valid JSON Schema `inputSchema` definitions that pass JSON Schema Draft 2020-12 validation.
- **SC-003**: Unauthenticated connection attempts are rejected with HTTP 401 within 100ms, with zero tool calls executed.
- **SC-004**: The health check endpoint responds within 200ms under normal conditions and correctly reports degraded status when the database is unavailable.
- **SC-005**: The server handles at least 50 concurrent SSE connections without connection drops or degraded tool call latency (p99 < 500ms for simple tool calls).
- **SC-006**: When an SSE client disconnects, the connection tracking entry is removed within 5 seconds.
- **SC-007**: All tool calls are logged with agent ID, tool name, duration, and outcome, verifiable by inspecting structured log output.
- **SC-008**: Both SSE and Streamable HTTP transports produce identical tool results for the same inputs, verified by running the same test suite against both transports.

## Constitution Compliance

| Principle | Compliance |
|-----------|------------|
| I. Single Binary | MCP server is embedded in the main binary; no external MCP broker or proxy |
| II. MCP-Native | This spec implements the core MCP interface; all agent operations are MCP tools |
| III. Pure Go, Zero CGO | `mark3labs/mcp-go` is pure Go; no CGO dependencies introduced |
| IV. Multi-Tenant | API key authentication scopes every tool call to the authenticated agent |
| V. Embedded OAuth 2.1 | API keys serve as the agent authentication mechanism; OAuth applies to human users (separate spec) |
| VIII. Observable | All tool calls logged via `slog`; connection tracking enables monitoring |
| IX. Progressive Complexity | Basic tools (send, read, mark done) work without channels or search configured |
