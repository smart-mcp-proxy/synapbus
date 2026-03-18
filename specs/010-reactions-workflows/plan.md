# Implementation Plan: Message Reactions & Workflow States

**Branch**: `010-reactions-workflows` | **Date**: 2026-03-18 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/010-reactions-workflows/spec.md`

## Summary

Add message reactions (approve/reject/in_progress/done/published) with toggle semantics and workflow state derivation. Extend channels with auto_approve and stalemate timeout settings. Add MCP tools for agent reaction management. Build web UI reaction pills and workflow badges. Extend StalemateWorker for channel message workflow escalation.

## Technical Context

**Language/Version**: Go 1.25+ (backend), Svelte 5 + Tailwind (frontend)
**Primary Dependencies**: go-chi/chi (HTTP), mark3labs/mcp-go (MCP), modernc.org/sqlite (storage), spf13/cobra (CLI)
**Storage**: SQLite (modernc.org/sqlite, pure Go) — new migration 013_reactions.sql
**Testing**: `go test ./...` (backend), curl (API), Chrome (UI)
**Target Platform**: linux/amd64, darwin/arm64 (cross-compiled single binary)
**Project Type**: Web service with embedded SPA
**Constraints**: Zero CGO, single binary, single --data directory

## Constitution Check

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Local-First, Single Binary | PASS | All within single binary. No external deps. |
| II. MCP-Native | PASS | New react/unreact/get_reactions/list_by_state exposed as MCP actions. |
| III. Pure Go, Zero CGO | PASS | No new CGO deps. Pure SQLite migration. |
| IV. Multi-Tenant with Ownership | PASS | Reactions track agent_name. Channel membership enforced. |
| V. Embedded OAuth 2.1 | N/A | No auth changes. |
| VI. Semantic-Ready Storage | N/A | No search changes. |
| VII. Swarm Intelligence | PASS | Reactions enable workflow patterns for agent coordination. |
| VIII. Observable by Default | PASS | Reactions are logged, queryable, traceable. |
| IX. Progressive Complexity | PASS | Reactions layer on existing messaging. Basic messaging unaffected. |
| X. Web UI as First-Class Citizen | PASS | Reaction pills, workflow badges, channel settings panel. |

## Project Structure

### Documentation (this feature)

```text
specs/010-reactions-workflows/
├── plan.md
├── spec.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   └── rest-api.md
├── checklists/
│   └── requirements.md
└── tasks.md
```

### Source Code (repository root)

```text
# Backend (Go)
internal/
├── reactions/
│   ├── model.go           # NEW: Reaction struct, types, state derivation
│   ├── store.go           # NEW: SQLite CRUD for reactions
│   └── service.go         # NEW: Business logic (toggle, validate, state calc)
├── channels/
│   └── service.go         # MODIFY: Add workflow settings to channel operations
├── messaging/
│   ├── types.go           # MODIFY: Add Reactions/WorkflowState to Message
│   └── service.go         # MODIFY: EnrichMessages adds reactions
├── api/
│   ├── reactions_handler.go  # NEW: REST endpoints for web UI
│   ├── channels_handler.go   # MODIFY: Return workflow settings
│   └── router.go             # MODIFY: Register reaction routes
├── mcp/
│   ├── bridge.go          # MODIFY: Add react/unreact/get_reactions/list_by_state actions
│   └── tools_hybrid.go    # MODIFY: (optional) if adding as hybrid tool
├── actions/
│   └── registry.go        # MODIFY: Register new reaction actions
└── stalemate/             # MODIFY: Extend for workflow state tracking

cmd/synapbus/
└── admin.go               # MODIFY: Add channel workflow update CLI

schema/
└── 013_reactions.sql      # NEW: reactions table + channel columns

# Frontend (Svelte)
web/src/lib/
├── components/
│   ├── ReactionPills.svelte     # NEW: Reaction display + toggle
│   └── WorkflowBadge.svelte     # NEW: Colored state badge
├── api/
│   └── client.ts                # MODIFY: Add reaction API methods
└── routes/channels/[name]/
    └── +page.svelte             # MODIFY: Integrate reactions + badges
```

**Structure Decision**: New `internal/reactions/` package for clean separation. Reactions are a distinct domain from messaging.

## Complexity Tracking

No constitution violations. No complexity justifications needed.
