# Quickstart: Reactive Agent Triggering

## Prerequisites

- Go 1.25+ installed
- Access to K8s cluster (MicroK8s on kubic) for integration tests
- SynapBus built and running locally or on kubic

## Development Setup

```bash
# Build SynapBus
make build

# Run with hot reload
make dev

# Run tests
make test
```

## Key Files to Modify

### New Files
- `internal/reactor/reactor.go` — Core reactor engine
- `internal/reactor/store.go` — SQLite persistence
- `internal/reactor/poller.go` — K8s Job status poller
- `internal/reactor/reactor_test.go` — Unit tests
- `internal/api/runs.go` — REST API for Web UI
- `schema/015_reactive_triggers.sql` — Migration
- `cmd/synapbus/runs.go` — CLI commands
- `cmd/synapbus/agent_triggers.go` — CLI trigger config commands
- `web/src/routes/runs/+page.svelte` — Web UI Agent Runs page

### Modified Files
- `internal/agents/model.go` — Add trigger fields
- `internal/agents/store.go` — Add trigger config CRUD
- `internal/k8s/runner.go` — Add trigger env vars to Job creation
- `internal/dispatcher/dispatcher.go` — Wire reactor into fan-out
- `internal/webhooks/delivery.go` — Add trigger block to payloads
- `internal/mcp/bridge.go` — Register admin MCP tools
- `cmd/synapbus/main.go` — Register CLI commands

## Testing Approach

### Unit Tests (no K8s required)
```bash
go test ./internal/reactor/... -v
```

Test the reactor decision logic with mock K8s runner:
- Cooldown enforcement
- Budget counting
- Depth checking
- Sequential execution / coalescing
- Self-mention filtering

### Integration Tests (requires K8s)
```bash
go test ./internal/reactor/... -tags=integration -v
```

Test actual K8s Job creation and polling on kubic.

## Configuring an Agent

```bash
# 1. Set trigger mode and rate limits
./synapbus agent set-triggers research-mcpproxy \
  --mode reactive --cooldown 600 --daily-budget 8

# 2. Set K8s image and env vars
./synapbus agent set-image research-mcpproxy \
  --image localhost:32000/universal-agent:latest \
  --env AGENT_GIT_REPO=Dumbris/agent-research-mcpproxy \
  --secret-env SYNAPBUS_API_KEY=synapbus-agent-keys:RESEARCH_MCPPROXY_API_KEY

# 3. Send a DM to test
# (via Web UI or MCP client)

# 4. Check run status
./synapbus runs list --agent research-mcpproxy
```
