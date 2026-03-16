# Implementation Plan: SynapBus v0.6.0 — Platform Features Bundle

**Branch**: `007-platform-features-bundle` | **Date**: 2026-03-16 | **Spec**: [spec.md](spec.md)

## Summary

8 features adding message lifecycle enforcement (StalemateWorker), channel reply threading, A2A protocol support (Agent Cards + inbound gateway), mobile-responsive Web UI, reactive K8s agent activation, enterprise identity providers (GitHub/Google/Azure AD), and CLAUDE.md communication protocol. All features are additive — existing functionality remains unchanged.

## Technical Context

**Language/Version**: Go 1.25+ (per go.mod)
**Primary Dependencies**: go-chi/chi (HTTP), mark3labs/mcp-go (MCP), ory/fosite (OAuth), spf13/cobra (CLI), modernc.org/sqlite (storage), TFMV/hnsw (vectors). NEW: coreos/go-oidc/v3 (OIDC), golang.org/x/oauth2 (OAuth client)
**Storage**: SQLite (modernc.org/sqlite, pure Go) — single DB file in `--data` directory
**Testing**: `go test ./...` (Go), `npm run build` (Svelte)
**Target Platform**: linux/amd64, darwin/arm64, darwin/amd64
**Project Type**: web-service + CLI + embedded SPA
**Constraints**: Zero CGO, single binary, pure Go dependencies only

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Local-First, Single Binary | PASS | All features add to the single binary. No external dependencies. |
| II. MCP-Native | PASS | A2A is an additional protocol alongside MCP, not a replacement. MCP remains sole agent interface. |
| III. Pure Go, Zero CGO | PASS | coreos/go-oidc and golang.org/x/oauth2 are pure Go. a2a-go SDK is pure Go. |
| IV. Multi-Tenant with Ownership | PASS | All new features respect ownership model. |
| V. Embedded OAuth 2.1 | PASS | Enterprise IdP is additive — local auth remains default. External IdPs are optional login providers, not replacements. |
| VI. Semantic-Ready Storage | PASS | New tables use same SQLite DB. |
| VII. Swarm Intelligence Patterns | PASS | K8s handlers extend existing dispatcher. |
| VIII. Observable by Default | PASS | StalemateWorker actions are logged and traced. |
| IX. Progressive Complexity | PASS | All features are opt-in. Basic messaging unchanged. |
| X. Web UI as First-Class Citizen | PASS | Mobile-responsive improves the UI. |

## Complexity Tracking

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| A2A protocol (previously a Non-Goal) | User explicitly requested A2A support. Ecosystem has matured (v1.0, Linux Foundation). | Not adding A2A means SynapBus remains invisible to external agent frameworks. |
| Enterprise IdP (adds external dependency at runtime) | Required for organizational deployment (Gcore uses Azure AD). | External IdPs are optional — SynapBus fully functions without them. |

## Project Structure

### Documentation (this feature)

```text
specs/007-platform-features-bundle/
├── plan.md              # This file
├── research.md          # Phase 0: technical decisions
├── data-model.md        # Phase 1: entities and schema
├── quickstart.md        # Phase 1: developer setup guide
├── contracts/           # Phase 1: API contracts
│   ├── a2a-endpoint.md  # A2A JSON-RPC contract
│   ├── idp-routes.md    # IdP callback routes
│   └── stalemate-config.md # StalemateWorker config
└── tasks.md             # Phase 2: implementation tasks
```

### Source Code (repository root)

```text
internal/
├── messaging/
│   └── stalemate.go          # F1: StalemateWorker
├── channels/
│   └── service.go            # F2: reply_to in BroadcastMessage
├── a2a/                      # F3+F5: A2A support (NEW)
│   ├── agentcard.go          # Agent Card generation
│   ├── gateway.go            # JSON-RPC handler
│   ├── taskstore.go          # A2A task state tracking
│   └── routes.go             # HTTP endpoint registration
├── auth/
│   └── idp/                  # F7: Identity providers (NEW)
│       ├── provider.go       # Provider interface
│       ├── github.go         # GitHub OAuth
│       ├── oidc.go           # Generic OIDC (Google, Azure AD)
│       ├── handlers.go       # Callback HTTP handlers
│       └── store.go          # user_identities DB operations
├── actions/
│   └── registry.go           # F2: add reply_to to send_channel_message
├── mcp/
│   └── bridge.go             # F2: pass reply_to through bridge
├── k8s/                      # F6: extend existing handlers
│   └── dispatcher.go         # Add @mention event matching
├── api/
│   └── router.go             # Wire new routes
└── web/                      # F4: mobile-responsive (embedded)

web/src/
├── routes/
│   ├── +layout.svelte        # F4: responsive sidebar
│   └── login/+page.svelte    # F7: IdP buttons
├── lib/components/
│   ├── Sidebar.svelte        # F4: drawer mode
│   └── Header.svelte         # F4: hamburger button

schema/
├── 010_a2a_tasks.sql         # F5: A2A task tracking
└── 011_external_auth.sql     # F7: user_identities, identity_providers

cmd/synapbus/
├── main.go                   # Wire StalemateWorker, A2A, IdP
└── admin.go                  # F3/F8: agent capabilities CLI

CLAUDE.md                     # F8: communication protocol
```

**Structure Decision**: Follows existing Go project layout with `internal/` packages. Two new packages (`internal/a2a/`, `internal/auth/idp/`) and two new migrations. All other changes extend existing files.
