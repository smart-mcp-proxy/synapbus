# Implementation Plan: MCP Auth, UX Polish & Agent Lifecycle

**Branch**: `002-mcp-auth-ux-polish` | **Date**: 2026-03-14 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/002-mcp-auth-ux-polish/spec.md`

## Summary

Implement dual MCP authentication (API key + OAuth 2.1 fallback with agent selection), remove the "Send as" dropdown from the Web UI so humans always send as their human account, simplify agent management by removing the type selector, auto-create a "my-agents" private channel per user with auto-join on agent registration, and add a dead letter queue for unread messages when agents are deleted.

## Technical Context

**Language/Version**: Go 1.23+
**Primary Dependencies**: ory/fosite (OAuth 2.1), mark3labs/mcp-go (MCP server), go-chi/chi (HTTP), Svelte 5 + Tailwind (Web UI)
**Storage**: modernc.org/sqlite (pure Go), TFMV/hnsw (vectors)
**Testing**: `go test ./...` (Go), manual browser testing (Svelte)
**Target Platform**: linux/amd64, darwin/arm64
**Project Type**: Web service (single binary with embedded SPA)
**Performance Goals**: MCP auth < 1s, OAuth flow < 30s
**Constraints**: Zero CGO, single binary, all storage in `--data` directory
**Scale/Scope**: Single-instance deployment, multi-tenant with ownership

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Local-First, Single Binary | PASS | All features embedded, no external dependencies added |
| II. MCP-Native | PASS | OAuth flow serves MCP clients; REST API changes are Web UI only |
| III. Pure Go, Zero CGO | PASS | No new C dependencies; fosite already in use |
| IV. Multi-Tenant with Ownership | PASS | Dead letters scoped to owner; my-agents channel scoped to owner |
| V. Embedded OAuth 2.1 | PASS | Extending existing fosite integration with agent selection |
| VI. Semantic-Ready Storage | PASS | New tables follow SQLite-only pattern |
| VII. Swarm Intelligence Patterns | N/A | No swarm changes |
| VIII. Observable by Default | PASS | Agent deletion and dead letter actions will be traced |
| IX. Progressive Complexity | PASS | Dead letters and my-agents are additive, don't break basic messaging |
| X. Web UI as First-Class Citizen | PASS | UI improvements: remove send-as, add DLQ view, simplify agent form |

**Gate result**: PASS — all applicable principles satisfied.

## Project Structure

### Documentation (this feature)

```text
specs/002-mcp-auth-ux-polish/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output
└── tasks.md             # Phase 2 output (via /speckit.tasks)
```

### Source Code (repository root)

```text
# Backend (Go)
internal/
├── auth/
│   ├── handlers.go          # MODIFY: Add OAuth authorize page with agent selector
│   ├── fosite_store.go      # MODIFY: Store agent_name in session data
│   └── middleware.go        # MODIFY: Extract agent from OAuth token
├── agents/
│   ├── service.go           # MODIFY: Auto-join my-agents channel, dead letter on delete
│   └── middleware.go        # MODIFY: Renamed OptionalAuthMiddlewareWithOAuth → RequiredAuthMiddlewareWithOAuth (MCP requires auth, returns 401 with WWW-Authenticate header)
├── channels/
│   └── service.go           # MODIFY: Prevent deletion of system channels
├── messaging/
│   └── service.go           # EXISTING: No changes needed (DLQ is a DB query)
├── api/
│   ├── agents_handler.go    # MODIFY: Dead letter capture on DELETE
│   ├── messages_handler.go  # MODIFY: Force human agent as sender
│   ├── channels_handler.go  # MODIFY: Block system channel deletion
│   └── deadletters_handler.go  # NEW: Dead letter queue API endpoints
├── mcp/
│   └── server.go            # MODIFY: OAuth metadata endpoint, removed agent management tools (register_agent, update_agent, deregister_agent)
└── storage/
    └── schema/
        └── 008_dead_letters.sql  # NEW: Dead letters table migration

# Frontend (Svelte)
web/src/
├── routes/
│   ├── channels/[name]/+page.svelte  # MODIFY: Remove send-as dropdown
│   ├── dm/[name]/+page.svelte        # MODIFY: Remove send-as dropdown
│   ├── agents/+page.svelte           # MODIFY: Remove type selector
│   ├── dead-letters/+page.svelte     # NEW: Dead letter queue view
│   └── oauth/
│       └── authorize/+page.svelte    # NEW: OAuth agent selection page
├── lib/
│   ├── api/client.ts                 # MODIFY: Add dead letter API methods
│   └── components/
│       └── Sidebar.svelte            # MODIFY: Add dead letters nav item
```

**Structure Decision**: Existing Go `internal/` + Svelte `web/src/` structure. No new packages — changes distributed across existing modules with one new handler file and one new Svelte route.

## Complexity Tracking

No constitution violations. No complexity justification needed.
