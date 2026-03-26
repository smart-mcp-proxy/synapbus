# Feature Specification: SQL Query Interface + Split Connection Pools

**Feature Branch**: `015-sql-query-split-pools`
**Created**: 2026-03-26
**Status**: Draft
**Input**: Architecture research from reactive agent triggering session

## Assumptions

- SQL query interface is exposed as a `query` action via the existing `execute` MCP tool, not a new top-level MCP tool
- Queries are read-only (enforced via `PRAGMA query_only=ON` on a dedicated connection)
- Agents query curated SQL views (not raw tables) that bake in per-agent access control
- Views: `my_messages`, `my_channels`, `channel_messages` — parameterized by the authenticated agent's name
- Results are automatically limited to 100 rows; agent can specify lower limit
- Query timeout: 5 seconds max
- Only SELECT statements allowed (validated before execution); WITH (CTEs) permitted
- Split connection pools: writeDB (MaxOpenConns=1) for all INSERT/UPDATE/DELETE, readDB (MaxOpenConns=8) for all SELECT
- Both pools share the same SQLite file with WAL mode
- The read pool uses `PRAGMA query_only=ON` for safety
- No schema changes needed — this is a runtime architecture change
- Agent SQL queries use the read pool

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Agent Queries Messages via SQL (Priority: P1)

An agent connected via MCP uses the `execute` tool to run a SQL query against its accessible messages. For example: "Show me all messages in #news-mcpproxy from the last 3 days with priority >= 7".

**Why this priority**: Removes the expressiveness ceiling — agents can compose arbitrary queries instead of being limited to fixed API endpoints.

**Independent Test**: Agent calls `execute` with `call('query', {sql: "SELECT * FROM my_messages WHERE channel_name = 'news-mcpproxy' AND priority >= 7 ORDER BY created_at DESC LIMIT 5"})` and gets results.

**Acceptance Scenarios**:

1. **Given** an authenticated agent, **When** it calls `query` with a valid SELECT, **Then** it receives JSON results with column names and rows.
2. **Given** an agent, **When** it runs a query referencing `my_messages`, **Then** it only sees messages it has access to (own DMs + joined channels).
3. **Given** an agent, **When** it runs `INSERT INTO messages ...`, **Then** the query is rejected with "only SELECT statements allowed".
4. **Given** an agent, **When** it runs a query without LIMIT, **Then** results are automatically capped at 100 rows.
5. **Given** an agent, **When** it runs a slow query (> 5s), **Then** the query is cancelled and an error is returned.

---

### User Story 2 - Split Read/Write Connection Pools (Priority: P1)

SynapBus uses separate connection pools for reads and writes to eliminate SQLITE_BUSY errors under concurrent agent load.

**Why this priority**: Directly fixes the SQLITE_BUSY errors observed during reactive agent runs.

**Independent Test**: Run concurrent read and write operations; verify no SQLITE_BUSY errors and writes serialize correctly.

**Acceptance Scenarios**:

1. **Given** concurrent agents sending messages, **When** writes happen simultaneously, **Then** they serialize through the single-writer pool without SQLITE_BUSY.
2. **Given** a write in progress, **When** a read query arrives, **Then** the read executes immediately on the read pool (WAL mode).
3. **Given** the read pool, **When** any write operation is attempted, **Then** it fails (query_only=ON enforcement).

---

### User Story 3 - Agent Queries Channel Messages (Priority: P2)

An agent queries messages from a specific channel with rich filtering — date ranges, keywords, reactions, workflow states.

**Why this priority**: Enables the social-commenter to query #opportunities channel structured data via SQL.

**Acceptance Scenarios**:

1. **Given** an agent that has joined #opportunities, **When** it queries `SELECT * FROM channel_messages WHERE channel_name = 'opportunities' AND created_at > datetime('now', '-3 days')`, **Then** it sees messages from that channel.
2. **Given** an agent that has NOT joined a private channel, **When** it queries that channel's messages, **Then** no results are returned.

---

### Edge Cases

- Query with syntax error returns a clear error message, not a crash
- Query referencing non-existent view returns "no such table" error
- Empty result set returns empty array, not null
- Very large result (>100 rows) is truncated with a warning
- Concurrent SQL queries from multiple agents don't interfere

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST provide a `query` action callable via the `execute` MCP tool that accepts a SQL string and returns results as JSON.
- **FR-002**: System MUST enforce read-only execution — no INSERT, UPDATE, DELETE, DROP, ALTER, or PRAGMA statements allowed.
- **FR-003**: System MUST expose curated views (`my_messages`, `my_channels`, `channel_messages`) that enforce per-agent access control.
- **FR-004**: System MUST automatically limit query results to 100 rows (or fewer if agent specifies).
- **FR-005**: System MUST cancel queries that exceed 5 seconds.
- **FR-006**: System MUST use a separate read-only connection pool (MaxOpenConns=8) for all SELECT operations.
- **FR-007**: System MUST use a single-writer connection pool (MaxOpenConns=1) for all write operations.
- **FR-008**: System MUST configure `PRAGMA query_only=ON` on the read pool connections.
- **FR-009**: System MUST validate SQL statements before execution — only SELECT and WITH (CTE) prefixes allowed.
- **FR-010**: System MUST return query results as `{columns: [...], rows: [[...], ...], row_count: N, truncated: bool}`.

### Key Entities

- **Read Pool**: SQLite connection pool with MaxOpenConns=8, query_only=ON, for all SELECT operations including agent SQL queries.
- **Write Pool**: SQLite connection pool with MaxOpenConns=1, for all INSERT/UPDATE/DELETE operations.
- **Agent Views**: SQL views parameterized by agent name that enforce access control.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Agents can execute arbitrary SELECT queries against curated views and receive structured JSON results within 5 seconds.
- **SC-002**: No SQLITE_BUSY errors under concurrent 4-agent workload (verified by running all 4 reactive agents simultaneously).
- **SC-003**: Write operations on the read pool are rejected at the SQLite engine level.
- **SC-004**: Query results are limited to 100 rows maximum.
- **SC-005**: All 28+ existing test packages continue to pass with the split pool architecture.
