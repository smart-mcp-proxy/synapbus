# Implementation Plan: Reactive Agent Triggering System

**Branch**: `014-reactive-agent-triggers` | **Date**: 2026-03-25 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/014-reactive-agent-triggers/spec.md`

## Summary

Add a reactor engine to SynapBus that automatically triggers K8s Jobs when agents receive DMs or @mentions. The reactor enforces per-agent rate limits (cooldown, daily budget, trigger depth), ensures sequential execution with coalescing, and provides visibility through a Web UI Agent Runs panel, failure DM notifications, and admin CLI commands.

## Technical Context

**Language/Version**: Go 1.25+ (per go.mod)
**Primary Dependencies**: go-chi/chi (HTTP), mark3labs/mcp-go (MCP), spf13/cobra (CLI), modernc.org/sqlite (storage), k8s.io/client-go (K8s Jobs)
**Storage**: SQLite via modernc.org/sqlite — new migration 015_reactive_triggers.sql
**Testing**: `go test ./...` (table-driven tests, Go standard)
**Target Platform**: linux/amd64 (kubic deployment), darwin/arm64 (development)
**Project Type**: Web service (single binary) with embedded Svelte 5 SPA
**Performance Goals**: Trigger evaluation < 100ms, K8s Job creation < 5s after evaluation, Job status polling every 15s
**Constraints**: Zero CGO, single binary, single `--data` directory, all state in SQLite
**Scale/Scope**: ~4 reactive agents, ~8 triggers/day each, single K8s cluster

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Local-First, Single Binary | PASS | Reactor lives inside SynapBus binary. K8s client is optional (no-op when not in-cluster). |
| II. MCP-Native | PASS | Admin tools exposed via MCP. Agents interact through existing MCP tools only. |
| III. Pure Go, Zero CGO | PASS | k8s.io/client-go is pure Go. No new CGO deps. |
| IV. Multi-Tenant with Ownership | PASS | Trigger config scoped to agent's owner. Run visibility restricted to owner. |
| V. Embedded OAuth 2.1 | N/A | No auth changes needed. |
| VI. Semantic-Ready Storage | N/A | No vector search changes. |
| VII. Swarm Intelligence | N/A | Not affected. |
| VIII. Observable by Default | PASS | Every trigger evaluation recorded in reactive_runs. Failed runs send DM + show in Web UI. |
| IX. Progressive Complexity | PASS | Reactive triggers are opt-in per agent (trigger_mode='reactive'). Default is 'passive' — no behavior change for existing agents. |
| X. Web UI First-Class | PASS | New Agent Runs page with real-time status, filtering, expandable logs. |

**GATE RESULT: PASS** — No violations.

## Project Structure

### Documentation (this feature)

```text
specs/014-reactive-agent-triggers/
├── plan.md              # This file
├── research.md          # Phase 0: research findings
├── data-model.md        # Phase 1: schema design
├── quickstart.md        # Phase 1: developer onboarding
├── contracts/           # Phase 1: API contracts
│   ├── rest-api.md      # REST endpoints for Web UI
│   ├── mcp-tools.md     # MCP admin tools
│   └── cli-commands.md  # Admin CLI commands
└── tasks.md             # Phase 2: implementation tasks
```

### Source Code (repository root)

```text
internal/
├── reactor/             # NEW: reactive trigger engine
│   ├── reactor.go       # Core decision logic
│   ├── store.go         # SQLite persistence for reactive_runs
│   ├── poller.go        # K8s Job status polling goroutine
│   └── reactor_test.go  # Unit tests
├── agents/              # MODIFIED: add trigger fields to agent model
│   ├── model.go         # Add trigger_mode, cooldown, budget, etc.
│   └── store.go         # Add trigger config CRUD
├── k8s/                 # MODIFIED: extend job creation with trigger env vars
│   └── runner.go        # Add SYNAPBUS_MESSAGE_* env vars
├── dispatcher/          # MODIFIED: add reactor as dispatch target
│   └── dispatcher.go    # Wire reactor into MultiDispatcher
├── webhooks/            # MODIFIED: add trigger block to payloads
│   └── delivery.go      # Enrich payload with depth/run_id
├── messaging/           # MODIFIED: propagate trigger depth on agent messages
│   └── service.go       # Track depth in message metadata
├── mcp/                 # MODIFIED: add admin MCP tools
│   └── bridge.go        # Register configure_triggers, list_runs, etc.
├── api/                 # MODIFIED: add REST endpoints for Web UI
│   └── runs.go          # NEW: /api/runs endpoints
└── web/                 # MODIFIED: embed updated SPA
    └── dist/            # Rebuilt after Svelte changes

web/                     # Svelte source
└── src/
    ├── routes/
    │   └── runs/        # NEW: Agent Runs page
    │       └── +page.svelte
    └── lib/
        └── components/
            └── RunCard.svelte  # NEW: run row component

schema/
└── 015_reactive_triggers.sql   # NEW: migration

cmd/synapbus/
└── runs.go              # NEW: CLI commands for runs
└── agent_triggers.go    # NEW: CLI commands for trigger config
```

**Structure Decision**: Follows existing SynapBus layout. New `internal/reactor/` package for core logic. All other changes extend existing packages.

## Complexity Tracking

> No violations — section not needed.
