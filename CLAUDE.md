# SynapBus

Local-first, MCP-native agent-to-agent messaging service. Single Go binary with embedded storage, semantic search, and a Slack-like Web UI.

**Repo**: github.com/synapbus/synapbus
**License**: Apache 2.0

## Tech Stack

| Component | Technology | Notes |
|-----------|-----------|-------|
| Language | Go 1.23+ | Single binary, cross-compilation, zero CGO |
| Database | modernc.org/sqlite | Pure Go SQLite, no CGO required |
| Vectors | TFMV/hnsw | Pure Go HNSW vector index |
| MCP | mark3labs/mcp-go | MCP server library |
| HTTP | go-chi/chi | Lightweight router |
| Auth | ory/fosite | OAuth 2.1 framework |
| Web UI | Svelte 5 + Tailwind | Embedded via go:embed |
| Logging | slog | Structured logging |
| Attachments | Content-addressable FS | SHA-256 dedup |

**Critical constraint**: ZERO CGO. The binary must cross-compile cleanly for linux/amd64, darwin/arm64.

## Directory Structure

```
synapbus/
├── cmd/synapbus/         # main.go entry point (cobra CLI)
├── internal/
│   ├── auth/             # OAuth 2.1, API keys, sessions
│   ├── messaging/        # core message engine
│   ├── channels/         # channel management
│   ├── agents/           # agent registry
│   ├── search/           # semantic search (embeddings + HNSW)
│   ├── storage/          # SQLite + migrations
│   ├── attachments/      # content-addressable FS
│   ├── mcp/              # MCP server (tools, transport)
│   ├── api/              # REST API handlers (internal, for Web UI only)
│   ├── web/              # embedded Web UI (Svelte SPA)
│   └── trace/            # agent activity logging
├── web/                  # Svelte source (built → internal/web/dist/)
├── schema/               # SQLite migrations
├── docs/                 # documentation
├── .specify/             # speckit specs
└── Makefile
```

## Build & Run

```bash
make build          # Build Go binary
make test           # Run all tests
make dev            # Run with hot reload
make web            # Build Svelte SPA
make clean          # Clean build artifacts
make lint           # Run linters

./synapbus serve --port 8080 --data ./data
```

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `SYNAPBUS_PORT` | HTTP server port | `8080` |
| `SYNAPBUS_DATA_DIR` | Data directory (SQLite DB, attachments, vector index) | `./data` |
| `SYNAPBUS_BASE_URL` | Public base URL for OAuth metadata (required for LAN/remote) | auto-detect from Host header |
| `SYNAPBUS_EMBEDDING_PROVIDER` | Embedding provider: `openai`, `gemini`, `ollama` | (none) |
| `OPENAI_API_KEY` | OpenAI API key for embeddings | (none) |
| `GEMINI_API_KEY` | Google Gemini API key for embeddings | (none) |
| `SYNAPBUS_OLLAMA_URL` | Ollama server URL | `http://localhost:11434` |
| `SYNAPBUS_MESSAGE_RETENTION` | Message retention period (e.g. `12m`, `365d`, `0` to disable) | `12m` |

## Conventions

- Go standard project layout with `internal/` for non-public packages
- Table-driven tests
- Context propagation through all function signatures
- Structured logging via `slog`
- SQL migrations in `schema/` directory, numbered sequentially
- MCP is THE agent interface — REST API is for internal Web UI use only
- Every agent action must be traceable by the human owner
- All storage in a single `--data` directory (SQLite DB + attachments + vector index)

## Architecture Principles

1. **Local-first, single binary** — no external dependencies at runtime
2. **MCP-native** — agents interact exclusively through MCP protocol tools
3. **Pure Go, zero CGO** — all dependencies must be pure Go
4. **Multi-tenant with ownership** — every agent has a human owner
5. **Observable by default** — all agent actions traced, searchable, auditable
6. **Progressive complexity** — basic messaging first, advanced features layered on top

## Active Technologies
- Go 1.23+ + ory/fosite (OAuth 2.1), mark3labs/mcp-go (MCP server), go-chi/chi (HTTP), Svelte 5 + Tailwind (Web UI) (002-mcp-auth-ux-polish)
- modernc.org/sqlite (pure Go), TFMV/hnsw (vectors) (002-mcp-auth-ux-polish)
- Go 1.25+ (from go.mod) + mark3labs/mcp-go (MCP tools), go-chi/chi (HTTP), golang.org/x/time/rate (rate limiting), k8s.io/client-go (K8s Jobs — optional) (003-webhooks-k8s-runner)
- modernc.org/sqlite (pure Go), migration 009_webhooks.sql (003-webhooks-k8s-runner)
- Go 1.25+ (per go.mod) + mark3labs/mcp-go (MCP tools), go-chi/chi (HTTP), spf13/cobra (CLI), modernc.org/sqlite (storage), TFMV/hnsw (vectors) (004-embeddings-retention-inbox)
- SQLite (modernc.org/sqlite, pure Go) — single DB file in `--data` directory (004-embeddings-retention-inbox)

## Recent Changes
- 002-mcp-auth-ux-polish: Added Go 1.23+ + ory/fosite (OAuth 2.1), mark3labs/mcp-go (MCP server), go-chi/chi (HTTP), Svelte 5 + Tailwind (Web UI)
