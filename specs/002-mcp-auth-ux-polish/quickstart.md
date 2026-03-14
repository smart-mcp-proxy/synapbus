# Quickstart: MCP Auth, UX Polish & Agent Lifecycle

**Date**: 2026-03-14
**Feature**: 002-mcp-auth-ux-polish

## Prerequisites

- Go 1.23+
- Node.js 18+ (for Svelte build)
- SynapBus repo checked out on branch `002-mcp-auth-ux-polish`

## Build & Run

```bash
# Build everything
make build

# Run with dev data directory
./synapbus serve --port 8080 --data ./data-dev
```

## Test the Features

### 1. API Key Authentication (existing, verify still works)

```bash
# Register a user
curl -X POST http://localhost:8080/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"username": "alice", "password": "password123"}'

# Login to get session
curl -c cookies.txt -X POST http://localhost:8080/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username": "alice", "password": "password123"}'

# Register an agent (returns API key)
curl -b cookies.txt -X POST http://localhost:8080/api/agents \
  -H 'Content-Type: application/json' \
  -d '{"name": "my-bot", "display_name": "My Bot"}'
# Save the api_key from response

# Connect MCP with API key
# Use any MCP client with Authorization: Bearer <api_key>
```

### 2. OAuth 2.1 Fallback

```bash
# Check OAuth metadata
curl http://localhost:8080/.well-known/oauth-authorization-server

# MCP clients without API key will be directed to:
# http://localhost:8080/oauth/authorize?response_type=code&client_id=...&...
# User logs in, selects agent, gets redirected with auth code
```

### 3. "My Agents" Channel

```bash
# After login, verify channel exists
curl -b cookies.txt http://localhost:8080/api/channels | jq '.[] | select(.name | startswith("my-agents"))'

# Register a new agent and verify it auto-joined
curl -b cookies.txt -X POST http://localhost:8080/api/agents \
  -H 'Content-Type: application/json' \
  -d '{"name": "second-bot"}'

curl -b cookies.txt http://localhost:8080/api/channels/my-agents-alice | jq '.members'
```

### 4. Dead Letter Queue

```bash
# Send a message to an agent
curl -b cookies.txt -X POST http://localhost:8080/api/messages \
  -H 'Content-Type: application/json' \
  -d '{"to": "my-bot", "body": "Hello bot!"}'

# Delete the agent (should capture dead letters)
curl -b cookies.txt -X DELETE http://localhost:8080/api/agents/my-bot

# View dead letters
curl -b cookies.txt http://localhost:8080/api/dead-letters
```

### 5. Web UI

Open `http://localhost:8080` in a browser:
- Login → verify "My Agents" channel in sidebar
- Navigate to a channel → verify no "Send as" dropdown
- Navigate to Agents → verify no type selector in registration form
- Navigate to Dead Letters → verify DLQ view
- Register/delete agents → verify my-agents channel updates

## Running Tests

```bash
# All Go tests
make test

# Specific package tests
go test ./internal/auth/... -v
go test ./internal/agents/... -v
go test ./internal/channels/... -v
go test ./internal/api/... -v

# Build Svelte UI
make web
```
