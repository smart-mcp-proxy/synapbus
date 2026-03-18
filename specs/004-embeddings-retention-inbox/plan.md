# Implementation Plan: Embeddings Management, Message Retention & Agent Inbox

**Branch**: `004-embeddings-retention-inbox` | **Date**: 2026-03-14 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/004-embeddings-retention-inbox/spec.md`

## Summary

Three operational improvements to SynapBus: (1) CLI admin commands for embedding provider management (status, reindex, clear), (2) automated message retention with configurable TTL, archive warnings, cleanup with SQLite compaction, and manual purge CLI commands, (3) a unified `my_status` MCP tool that gives agents a complete overview in a single call. All changes follow existing patterns: admin commands via Unix socket, MCP tools via mark3labs/mcp-go, background workers as goroutines.

## Technical Context

**Language/Version**: Go 1.25+ (per go.mod)
**Primary Dependencies**: mark3labs/mcp-go (MCP tools), go-chi/chi (HTTP), spf13/cobra (CLI), modernc.org/sqlite (storage), TFMV/hnsw (vectors)
**Storage**: SQLite (modernc.org/sqlite, pure Go) — single DB file in `--data` directory
**Testing**: `go test ./...` — table-driven tests, existing test files in most packages
**Target Platform**: linux/amd64, darwin/arm64 (zero CGO)
**Project Type**: CLI + server (single binary)
**Performance Goals**: `my_status` response < 500ms; message purge of 100k messages < 30s
**Constraints**: Zero CGO, single binary, all data in `--data` directory
**Scale/Scope**: Single-instance deployments, up to 100k messages, up to 100 agents

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Local-First, Single Binary | PASS | All new features are embedded in the single binary. No external dependencies added. |
| II. MCP-Native | PASS | `my_status` is an MCP tool. Admin commands use Unix socket (non-MCP, for operators). |
| III. Pure Go, Zero CGO | PASS | No new dependencies. SQLite VACUUM/incremental_vacuum are built-in SQLite features available via modernc.org/sqlite. |
| IV. Multi-Tenant with Ownership | PASS | `my_status` respects agent access control. Retention cleanup only affects messages the system owns. System agent has an owner. |
| V. Embedded OAuth 2.1 | N/A | No auth changes in this feature. |
| VI. Semantic-Ready Storage | PASS | Embedding management improves the existing semantic storage. Cleanup properly cascades to embeddings. System still works without embedding provider. |
| VII. Swarm Intelligence Patterns | N/A | No changes to swarm patterns. |
| VIII. Observable by Default | PASS | Cleanup operations are logged. Embedding status is queryable. System notifications are traced. |
| IX. Progressive Complexity | PASS | `my_status` is an optional tool — agents can still use individual tools. Retention defaults to 12mo but can be disabled (0). Embedding CLI is opt-in. |
| X. Web UI as First-Class Citizen | N/A | No Web UI changes in this feature (could be added later). |

No violations. All gates pass.

## Project Structure

### Documentation (this feature)

```text
specs/004-embeddings-retention-inbox/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output
│   ├── mcp-tools.md     # my_status MCP tool schema
│   └── admin-commands.md # New admin socket commands
└── tasks.md             # Phase 2 output (created by /speckit.tasks)
```

### Source Code (repository root)

```text
cmd/synapbus/
├── main.go              # Add --message-retention flag, retention worker startup
└── admin.go             # Add embeddings, retention, messages purge, db vacuum CLI commands

internal/
├── admin/
│   └── socket.go        # Add handlers: embeddings.*, retention.*, messages.purge, db.vacuum
├── mcp/
│   └── tools.go         # Add my_status tool definition and handler
├── messaging/
│   ├── retention.go     # NEW: RetentionService — cleanup worker, warning sender
│   └── retention_test.go # NEW: Tests for retention logic
├── search/
│   └── store.go         # Add EmbeddingStats() method
└── agents/
    └── service.go       # Add EnsureSystemAgent() method

schema/
└── 010_retention.sql    # NEW: system_notifications tracking table (optional, may use existing messages table)
```

**Structure Decision**: Follows existing Go package layout. New code goes into existing packages where it belongs. Only one new file pair (retention.go/retention_test.go) is truly new. Everything else extends existing files.

## Complexity Tracking

No violations to justify. All changes follow existing patterns.
