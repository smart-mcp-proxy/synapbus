# Implementation Plan: Hybrid MCP Tool Architecture

## Phase 1 — Foundation (parallel, no cross-dependencies)

### Task A: JS/TS Runtime Engine (`internal/jsruntime/`)
Port goja-based runtime from `../mcpproxy-go/internal/jsruntime/`. Simplified for SynapBus:
- `runtime.go` — Execute(code, opts) with sandbox, timeout, call bridge
- `pool.go` — Reusable runtime pool for concurrent executions
- `typescript.go` — esbuild type-stripping transpilation
- `*_test.go` — Table-driven tests for all components
- ToolCaller interface: `Call(ctx, actionName, args) (any, error)`

### Task B: Action Registry + BM25 Search (`internal/actions/`)
- `types.go` — Action struct (name, category, description, params, examples)
- `registry.go` — Registry collecting all 23 action definitions with schemas
- `search.go` — BM25 in-memory search over action docs
- `*_test.go` — Tests for registry + search relevance
- Each action definition maps to an existing service method

### Task C: Pagination + Advanced Filtering
Modify existing service layer (no new packages):
- `messaging/options.go` — Add Offset, After, Before fields to ReadOptions + SearchOptions
- `messaging/store.go` — Update SQL queries for offset pagination + total count + date filtering
- `messaging/service.go` — Return PaginatedResult{Items, Total, Offset, Limit}
- `channels/store.go` — Pagination for GetChannelMessages, ListChannels
- `channels/service.go` — Propagate pagination
- `channels/task_store.go` — Pagination for ListTasks
- `agents/` — Pagination for DiscoverAgents/ListAgents
- Schema migration if indices needed
- Tests for all modified queries

### Task D: CLI Subcommands
Add to existing cobra CLI in `cmd/synapbus/admin.go`:
- `synapbus webhook register|list|delete` — via admin Unix socket
- `synapbus k8s register|list|delete` — via admin Unix socket
- `synapbus attachments gc` — via admin Unix socket
- Server-side: add admin socket handlers in `internal/admin/socket.go`
- Wire webhook/k8s/attachment services into admin.Services struct
- Tests

## Phase 2 — MCP Rewrite (depends on A + B + C)

### Task E: 4-Tool MCP Architecture (`internal/mcp/`)
- Rewrite `server.go` constructor to accept jsruntime + action registry
- New `tools_hybrid.go` — my_status, search, execute, send_message
- Wire execute → jsruntime.Execute() with ToolCaller bridging to action registry
- Wire search → actions.Registry.Search()
- Wire send_message → merged DM + channel messaging
- Remove old registrar files (tools.go handlers, channel_tools.go, swarm_tools.go, webhook_tools.go, tools_attachments.go)
- Keep: server.go (modified), auth.go, connection.go, health.go
- Update main.go constructor call
- Comprehensive tests

## Phase 3 — Documentation (depends on all)

### Task F: Website Docs (`../synapbus-website/`)
- Update MCP tools documentation (4 tools instead of 30)
- Document execute code examples
- Document CLI commands for admin operations
- Update API reference
