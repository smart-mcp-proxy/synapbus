# Quickstart: SynapBus v0.6.0 Development

## Prerequisites

- Go 1.25+
- Node.js 20+ (for Svelte frontend)
- Git

## Build & Test

```bash
make build          # Build Go binary + Svelte SPA
make test           # Run all Go tests
go test ./...       # Alternative: all tests
cd web && npm run build  # Build frontend only
```

## New Features Development

### F1: StalemateWorker
- Edit: `internal/messaging/stalemate.go` (new file)
- Test: `internal/messaging/stalemate_test.go`
- Wire: `cmd/synapbus/main.go` (start worker after message service)
- Config: env vars `SYNAPBUS_STALEMATE_*`

### F2: Channel reply_to
- Edit: `internal/actions/registry.go` (add reply_to param to send_channel_message)
- Edit: `internal/mcp/bridge.go` (pass reply_to in callSendChannelMessage)
- Edit: `internal/channels/service.go` (accept reply_to in BroadcastMessage)
- Test: `internal/channels/service_test.go`

### F3: A2A Agent Cards
- New: `internal/a2a/agentcard.go`
- Edit: `internal/api/router.go` (add /.well-known/agent-card.json route)
- Edit: `cmd/synapbus/admin.go` (agent update-capabilities command)
- Test: `internal/a2a/agentcard_test.go`

### F4: Mobile UI
- Edit: `web/src/routes/+layout.svelte` (responsive sidebar)
- Edit: `web/src/lib/components/Sidebar.svelte` (drawer mode)
- Edit: `web/src/lib/components/Header.svelte` (hamburger button)
- Test: Visual verification at 375px viewport

### F5: A2A Gateway
- New: `internal/a2a/gateway.go`, `taskstore.go`, `routes.go`
- New: `schema/010_a2a_tasks.sql`
- Edit: `internal/api/router.go` or `cmd/synapbus/main.go` (mount /a2a)
- Test: `internal/a2a/gateway_test.go`

### F6: K8s Handlers
- Edit: `internal/k8s/dispatcher.go` (add mention event matching)
- Edit: `cmd/synapbus/admin.go` (user-friendly register-handler command)
- Test: `internal/k8s/dispatcher_test.go`

### F7: Enterprise IdP
- New: `internal/auth/idp/` package
- New: `schema/011_external_auth.sql`
- Edit: `web/src/routes/login/+page.svelte` (IdP buttons)
- Edit: `cmd/synapbus/main.go` (wire IdP routes)
- Test: `internal/auth/idp/idp_test.go`

### F8: CLAUDE.md
- Edit: `CLAUDE.md` (add SynapBus Communication Protocol section)

## Running Locally

```bash
./synapbus serve --port 8080 --data ./data --log-level debug
```

## Testing StalemateWorker

```bash
# Set short timeouts for testing
SYNAPBUS_STALEMATE_PROCESSING_TIMEOUT=1m \
SYNAPBUS_STALEMATE_REMINDER_AFTER=30s \
SYNAPBUS_STALEMATE_ESCALATE_AFTER=2m \
SYNAPBUS_STALEMATE_INTERVAL=10s \
./synapbus serve --port 8080 --data ./test-data
```

## Testing A2A

```bash
# Fetch Agent Card
curl http://localhost:8080/.well-known/agent-card.json | jq

# Send A2A task
curl -X POST http://localhost:8080/a2a \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <api-key>" \
  -d '{"jsonrpc":"2.0","id":1,"method":"message.send","params":{"message":{"role":"user","parts":[{"text":"Hello"}],"metadata":{"target_agent":"test-bot"}}}}'
```
