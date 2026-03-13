<!--
Sync Impact Report
==================
- Version change: N/A → 1.0.0 (initial ratification)
- Added principles: I through X (10 total)
- Added sections: Technology Decisions, Non-Goals
- Removed sections: None (initial creation)
- Templates requiring updates:
  - .specify/templates/plan-template.md — ✅ compatible (Constitution Check section exists)
  - .specify/templates/spec-template.md — ✅ compatible (no constitution-specific sections needed)
  - .specify/templates/tasks-template.md — ✅ compatible (phase structure supports Go layout)
- Follow-up TODOs: None
-->

# SynapBus Constitution

## Core Principles

### I. Local-First, Single Binary

Everything ships in one Go binary. There MUST be no external runtime
dependencies — no separate database server, no message broker, no
external auth service. A user runs `synapbus serve` and the full
system starts: MCP server, REST API, Web UI, embedded storage.

**Rationale**: Eliminates deployment complexity and ensures any
developer or agent operator can run SynapBus with zero infrastructure
setup.

### II. MCP-Native

Agents interact with SynapBus exclusively through MCP protocol tools
(SSE and Streamable HTTP transports). The REST API exists solely for
the embedded Web UI and MUST NOT be advertised as an external agent
interface. All agent-facing operations MUST be exposed as MCP tools
with JSON Schema descriptions.

**Rationale**: MCP is the emerging standard for AI tool interaction.
By committing to MCP-only for agents, SynapBus avoids fragmenting
its interface and ensures compatibility with any MCP-capable client.

### III. Pure Go, Zero CGO

All dependencies MUST be pure Go. CGO MUST NOT be enabled for any
build target. This means:
- `modernc.org/sqlite` (not `mattn/go-sqlite3`)
- `TFMV/hnsw` (not C-based FAISS or Annoy)
- No C library bindings of any kind

The binary MUST cross-compile cleanly for at minimum:
`linux/amd64`, `darwin/arm64`, `darwin/amd64`.

**Rationale**: CGO breaks cross-compilation, complicates CI, and
introduces platform-specific build failures. Pure Go guarantees
`GOOS=X GOARCH=Y go build` works everywhere.

### IV. Multi-Tenant with Ownership

Every agent MUST have a human owner (`owner_id`). Owners control
their agents' access and can view all traces of agent activity.
Agents MUST only access their own messages and channels they have
joined. No agent may impersonate another or access another owner's
data without explicit grants.

**Rationale**: AI agents act on behalf of humans. Humans need
visibility and control over what their agents do. This principle
ensures accountability and prevents runaway agent behavior.

### V. Embedded OAuth 2.1

The OAuth 2.1 authorization server MUST be built into SynapBus
using `ory/fosite`. There MUST NOT be a dependency on an external
identity provider for core functionality. Local accounts (username
+ bcrypt-hashed password) MUST be supported as the default auth
method. PKCE MUST be required for all authorization code flows.

**Rationale**: Requiring an external auth service violates
Principle I (single binary). Embedding OAuth 2.1 keeps the system
self-contained while providing standards-compliant security.

### VI. Semantic-Ready Storage

SQLite handles all relational data. HNSW (via `TFMV/hnsw`) handles
vector search. Both MUST be embedded and store data within a single
`--data` directory. The system MUST function fully without an
embedding provider configured (falling back to full-text search).
When a provider is configured, messages MUST be embedded
asynchronously in the background.

**Rationale**: Semantic search is a key differentiator but MUST NOT
be a hard dependency. Progressive enhancement: basic → full-text →
semantic.

### VII. Swarm Intelligence Patterns

SynapBus MUST provide first-class support for:
- **Stigmergy**: Tagged messages on blackboard channels that agents
  read and react to (tags: `#finding`, `#task`, `#decision`, `#trace`)
- **Task Auction**: Post task → agents bid → poster selects winner
- **Agent Discovery**: Search agent capability cards by keyword or
  semantic match

Channel types (`standard`, `blackboard`, `auction`) MUST enforce
the appropriate interaction patterns.

**Rationale**: Multi-agent coordination requires higher-level
patterns beyond point-to-point messaging. These patterns are
proven in swarm intelligence literature and directly applicable
to AI agent orchestration.

### VIII. Observable by Default

All agent actions MUST be traced: tool calls, messages sent/received,
channel operations, errors. Traces MUST be stored in SQLite with
agent identity, action type, details (JSON), and timestamp. Owners
MUST be able to view, filter, and export traces for their agents.
Structured logging via `slog` MUST be used for all server-side logs.

**Rationale**: Agent systems are opaque by default. Observability
is not optional — it is a safety and debugging requirement. If a
human cannot see what an agent did, the system is not trustworthy.

### IX. Progressive Complexity

The system MUST be usable with just core messaging (send, read,
mark done). Advanced features MUST layer on top without requiring
configuration changes for basic usage:
1. Basic: messaging + agent registration
2. Intermediate: channels + full-text search + traces
3. Advanced: semantic search + attachments + swarm patterns

No feature in a higher tier MUST break or require features from
a lower tier.

**Rationale**: New users should not be overwhelmed. The simplest
use case (two agents exchanging messages) should require minimal
setup. Complexity is opt-in.

### X. Web UI as First-Class Citizen

The Svelte 5 + Tailwind CSS SPA MUST be embedded in the Go binary
via `go:embed`. It MUST provide full operational visibility:
message browsing, conversation threads, channel management, agent
monitoring, trace viewing, and search. The UI MUST support dark
mode and be responsive. It MUST receive real-time updates via SSE.

**Rationale**: The Web UI is how humans interact with SynapBus.
It is not an afterthought or admin panel — it is a primary
interface alongside MCP.

## Technology Decisions

| Component | Choice | Constraint |
|-----------|--------|------------|
| Language | Go 1.23+ | Single binary, cross-compilation |
| Database | modernc.org/sqlite | Pure Go, zero CGO (Principle III) |
| Vectors | TFMV/hnsw | Pure Go HNSW index (Principle III) |
| MCP | mark3labs/mcp-go | SSE + Streamable HTTP transports |
| HTTP Router | go-chi/chi | Lightweight, stdlib-compatible |
| Auth | ory/fosite | OAuth 2.1 framework (Principle V) |
| Web UI | Svelte 5 + Tailwind | Embedded via go:embed (Principle X) |
| Logging | slog | Structured, stdlib (Principle VIII) |
| Attachments | Content-addressable FS | SHA-256 hash, dedup storage |
| CLI | spf13/cobra | Standard Go CLI framework |

## Non-Goals

These are explicitly out of scope for the current version:

- **No external database**: No PostgreSQL, Redis, Kafka, or any
  external data store dependency.
- **No framework lock-in**: No LangChain, CrewAI, AutoGen, or any
  agent framework dependency. SynapBus is framework-agnostic.
- **No A2A protocol**: Google's Agent-to-Agent protocol is not
  supported in v1. MCP is the sole agent interface.
- **No cloud-specific features**: No AWS/GCP/Azure service
  integrations. SynapBus is local-first, always.
- **No multi-node clustering**: Single binary, single instance.
  Horizontal scaling is a future concern.

## Governance

This constitution is the authoritative source for architectural
decisions in SynapBus. All implementation work MUST comply with
these principles.

### Amendment Process

1. Propose amendment with rationale and impact analysis.
2. Update constitution version per semantic versioning:
   - **MAJOR**: Principle removal or backward-incompatible redefinition
   - **MINOR**: New principle or material expansion
   - **PATCH**: Clarification, wording, typo fix
3. Propagate changes to dependent templates and documentation.
4. Document changes in Sync Impact Report (HTML comment at top).

### Compliance

- All specs MUST include a constitution compliance check.
- All implementation plans MUST verify alignment with principles
  before Phase 0 research begins.
- Code reviews SHOULD verify adherence to relevant principles.

**Version**: 1.0.0 | **Ratified**: 2026-03-13 | **Last Amended**: 2026-03-13
