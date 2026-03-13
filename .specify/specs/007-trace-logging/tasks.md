# Tasks: Trace Logging & Observability

**Input**: Design documents from `/specs/007-trace-logging/`
**Prerequisites**: spec.md (required)

**Tests**: Tests are included where specified by the feature specification acceptance scenarios.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Project initialization, dependencies, and schema changes for trace logging

- [ ] T001 Add `--log-level` flag and `SYNAPBUS_LOG_LEVEL` env var to `cmd/synapbus/main.go`. Accept values: `debug`, `info`, `warn`, `error`. Default: `info`.
- [ ] T002 Add `--metrics` flag and `SYNAPBUS_METRICS` env var to `cmd/synapbus/main.go`. Boolean, default false.
- [ ] T003 Add `--trace-retention` flag and `SYNAPBUS_TRACE_RETENTION` env var to `cmd/synapbus/main.go`. Accept duration strings like `30d`, `90d`, `0` (unlimited). Default: `0`.
- [ ] T004 [P] Add `prometheus/client_golang` dependency to `go.mod` for optional metrics support.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core infrastructure that MUST be complete before ANY user story can be implemented

**CRITICAL**: No user story work can begin until this phase is complete

- [ ] T005 Create SQLite migration `schema/002_trace_logging.sql`: add `owner_id TEXT NOT NULL DEFAULT ''` column to existing `traces` table, add composite indexes `(owner_id, timestamp)`, `(owner_id, agent_name, timestamp)`, `(owner_id, action, timestamp)`. Update `schema_migrations`.
- [ ] T006 Configure slog JSON handler as the global logger in `cmd/synapbus/main.go`. Initialize `slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: parsedLevel}))` and set as default. Wire log level from the `--log-level` flag.
- [ ] T007 [P] Define the `Trace` domain struct in `internal/trace/model.go` with fields: `ID int64`, `OwnerID string`, `AgentName string`, `Action string`, `Details json.RawMessage`, `Timestamp time.Time`.
- [ ] T008 [P] Define the `TraceFilter` struct in `internal/trace/model.go` with fields: `OwnerID string`, `AgentName string`, `Action string`, `Since *time.Time`, `Until *time.Time`, `Page int`, `PageSize int`.
- [ ] T009 Define the `TraceStore` interface in `internal/trace/store.go` with methods: `Insert(ctx context.Context, t *Trace) error`, `Query(ctx context.Context, f TraceFilter) ([]Trace, int, error)` (returns traces + total count), `DeleteOlderThan(ctx context.Context, before time.Time) (int64, error)`.
- [ ] T010 Implement `SQLiteTraceStore` in `internal/trace/sqlite_store.go` satisfying the `TraceStore` interface. Use `modernc.org/sqlite` via the existing storage layer. Insert must be fast (single row insert). Query must use indexed columns, enforce `PageSize` max 200, default 50. `DeleteOlderThan` for retention cleanup.
- [ ] T011 Implement the async `Tracer` service in `internal/trace/tracer.go`. Accepts trace entries via a buffered channel, batches writes to SQLite in a background goroutine (flush every 100ms or when buffer reaches 64 entries). Expose `Record(ctx context.Context, ownerID, agentName, action string, details any)` method that serializes details to JSON and enqueues without blocking. On SQLite write failure, log error via slog but do not propagate to caller. Provide `Close()` for graceful shutdown (flush remaining).
- [ ] T012 Write table-driven tests for `SQLiteTraceStore` in `internal/trace/sqlite_store_test.go`: test insert, query with each filter combination, pagination, `DeleteOlderThan`, owner isolation (query for owner A must not return owner B traces).
- [ ] T013 Write tests for `Tracer` in `internal/trace/tracer_test.go`: test async recording (Record returns immediately), batch flush behavior, graceful shutdown flushes pending traces.

**Checkpoint**: Foundation ready — trace storage, async tracer, slog configured, schema migrated. User story implementation can now begin in parallel.

---

## Phase 3: User Story 1 — Owner Inspects Agent Activity Traces (Priority: P1) MVP

**Goal**: Owners can view a reverse-chronological list of all their agents' traced actions via REST API and Web UI, with expandable JSON details.

**Independent Test**: Register an agent, perform several MCP tool calls, then query `GET /api/traces` as the owner and verify each action appears with correct agent_name, action, timestamp, and details.

### Implementation for User Story 1

- [ ] T014 [US1] Instrument MCP tool handlers to call `Tracer.Record()` after every tool call in `internal/mcp/`. For each MCP tool (send_message, read_inbox, claim_messages, mark_done, search_messages, create_channel, join_channel, list_channels, register_agent, discover_agents, post_task, bid_task, upload_attachment, read_attachment), add a `tracer.Record(ctx, ownerID, agentName, toolName, detailsMap)` call capturing input params and result summary. On tool error, record a separate trace with action="error" including the error message and originating tool name.
- [ ] T015 [US1] Implement `GET /api/traces` handler in `internal/api/traces_handler.go`. Accept query params: `agent_name`, `action`, `since`, `until`, `page`, `page_size`. Extract `owner_id` from authenticated session. Call `TraceStore.Query()` with owner-scoped filter. Return JSON response: `{ "traces": [...], "total": N, "page": N, "page_size": N }`. Return 200 with empty array if no results.
- [ ] T016 [US1] Register the `/api/traces` route in `internal/api/router.go` (or equivalent). Apply authentication middleware so only authenticated owners can access. Wire the `TraceStore` dependency.
- [ ] T017 [P] [US1] Create the Svelte Traces list page in `web/src/routes/traces/+page.svelte`. Display a paginated, reverse-chronological table with columns: timestamp (human-readable), agent name, action, details preview (truncated to 80 chars). Clicking a row expands an inline panel showing the full JSON details formatted with syntax highlighting or `<pre>` block.
- [ ] T018 [P] [US1] Add "Traces" navigation link to the Web UI sidebar/nav in `web/src/lib/components/Nav.svelte` (or equivalent layout component).
- [ ] T019 [US1] Handle empty state in the Traces view: when zero traces exist, show a helpful message: "No traces found. Agent activity will appear here once your agents start performing actions."
- [ ] T020 [US1] Write integration test in `internal/api/traces_handler_test.go`: create two owners with agents, generate traces for both, verify `GET /api/traces` returns only the authenticated owner's traces (owner isolation). Verify response structure matches expected JSON format.

**Checkpoint**: User Story 1 complete. An owner can log into the Web UI, navigate to Traces, and see all their agents' actions in reverse-chronological order with expandable details. Owner isolation is enforced.

---

## Phase 4: User Story 5 — Structured Logging to stdout (Priority: P2)

**Goal**: All server-side log output uses slog JSON format with structured fields. Log level is configurable.

**Independent Test**: Start SynapBus with `--log-level=debug`, perform agent actions, pipe stdout through `jq` to verify every line is valid JSON with expected fields.

### Implementation for User Story 5

- [ ] T021 [US5] Create a slog middleware for chi in `internal/api/middleware_logging.go`. Log every HTTP request with fields: `method`, `path`, `status`, `duration_ms`, `request_id`. Generate `request_id` (UUID) per request and store in context.
- [ ] T022 [US5] Add structured slog calls to MCP tool handlers in `internal/mcp/`. Each tool call logs at `info` level with fields: `agent_name`, `action` (tool name), `request_id`. At `debug` level, include full tool call parameters as a JSON field. Errors log at `error` level with the error message.
- [ ] T023 [US5] Audit all existing `fmt.Printf` / `fmt.Println` calls in `cmd/synapbus/main.go` and any other files. Replace with `slog.Info()`, `slog.Debug()`, or `slog.Error()` calls with appropriate structured fields. Ensure zero unstructured log lines are emitted.
- [ ] T024 [US5] Write test in `internal/api/middleware_logging_test.go`: capture stdout, make HTTP requests at various log levels, parse each line as JSON, verify required fields are present. Verify `--log-level=error` suppresses info-level output.

**Checkpoint**: User Story 5 complete. All stdout output is valid JSON parseable by `jq`. Log level is configurable. No unstructured log lines.

---

## Phase 5: User Story 2 — Owner Filters and Searches Traces (Priority: P2)

**Goal**: Owners can filter traces by agent name, action type, and time range. Filters combine with AND logic. Results update as filters change.

**Independent Test**: Generate 50+ traces across 2 agents with mixed action types. Apply each filter individually and in combination. Verify result sets match expectations.

### Implementation for User Story 2

- [ ] T025 [US2] Add filter controls to the Traces Svelte page in `web/src/routes/traces/+page.svelte`: agent name dropdown (populated from owner's agents via `GET /api/agents`), action type dropdown (hardcoded list of known action types), time range selector (presets: "last hour", "last 24h", "last 7d", "custom" with date/time pickers). Filters update query params and re-fetch traces on change.
- [ ] T026 [US2] Implement `GET /api/agents` handler (if not already present) in `internal/api/agents_handler.go` to return the authenticated owner's agents. Used by the filter dropdown.
- [ ] T027 [US2] Write integration test in `internal/api/traces_handler_test.go`: insert 50+ traces across 2 agents with mixed actions and timestamps. Test each filter individually (agent_name, action, since/until) and combined filters. Verify correct counts and that no unmatched traces leak through.

**Checkpoint**: User Story 2 complete. Owners can narrow traces by agent, action type, and time range. All filters combine correctly.

---

## Phase 6: User Story 3 — Owner Exports Traces as JSON or CSV (Priority: P3)

**Goal**: Owners can export filtered or unfiltered traces as a JSON or CSV file download. Export streams results to avoid memory exhaustion on large datasets.

**Independent Test**: Generate traces, apply filters, export as JSON and CSV. Verify files parse correctly and contain expected entries.

### Implementation for User Story 3

- [ ] T028 [US3] Implement `GET /api/traces/export` handler in `internal/api/traces_export_handler.go`. Accept same filter params as `GET /api/traces` plus `format` query param (`json` or `csv`; also respect `Accept` header). Stream results using `Transfer-Encoding: chunked`. For JSON: open with `[`, stream each trace object comma-separated, close with `]`. For CSV: write header row (`id,agent_name,action,details,timestamp`), then stream each row with `details` as a JSON-encoded string. Set `Content-Disposition: attachment; filename="traces-YYYY-MM-DD.{json|csv}"`.
- [ ] T029 [US3] Add a streaming query method `QueryStream(ctx context.Context, f TraceFilter, fn func(Trace) error) error` to `TraceStore` interface and `SQLiteTraceStore` in `internal/trace/store.go` and `internal/trace/sqlite_store.go`. Iterates rows without loading all into memory. Calls `fn` for each row.
- [ ] T030 [US3] Register `/api/traces/export` route in `internal/api/router.go` with authentication middleware.
- [ ] T031 [P] [US3] Add export buttons ("Export JSON", "Export CSV") to the Traces Svelte page in `web/src/routes/traces/+page.svelte`. Buttons construct the export URL with current filter params and trigger browser download.
- [ ] T032 [US3] Write integration test in `internal/api/traces_export_handler_test.go`: export as JSON, parse the response body as `[]Trace`, verify count. Export as CSV, parse rows, verify header and data row count. Test with filters applied. Test empty result (empty array / header-only CSV).

**Checkpoint**: User Story 3 complete. Owners can export traces as JSON or CSV with current filters applied. Large exports stream without memory issues.

---

## Phase 7: User Story 4 — Operator Enables Prometheus Metrics (Priority: P3)

**Goal**: When `--metrics` is enabled, a `/metrics` endpoint exposes Prometheus-formatted counters and gauges. When disabled, `/metrics` returns 404.

**Independent Test**: Start with `--metrics`, perform agent actions, curl `/metrics`, verify Prometheus-formatted output with expected metric names.

### Implementation for User Story 4

- [ ] T033 [P] [US4] Create `internal/trace/metrics.go`. Define Prometheus metrics using `prometheus/client_golang`: `synapbus_traces_total` (counter), `synapbus_traces_by_action` (counter vec, label: `action`), `synapbus_active_agents` (gauge), `synapbus_errors_total` (counter). Provide a `Metrics` struct with methods `IncTrace(action string)`, `IncError()`, `SetActiveAgents(n int)`, and a no-op `NullMetrics` implementation for when metrics are disabled.
- [ ] T034 [US4] Conditionally register `/metrics` route in `internal/api/router.go`. When `--metrics` is enabled, register `promhttp.Handler()` at `/metrics`. When disabled, do not register the route (chi will 404 by default).
- [ ] T035 [US4] Wire metrics into the `Tracer` in `internal/trace/tracer.go`. After each successful trace batch write, call `metrics.IncTrace(action)` for each trace in the batch. On error traces, also call `metrics.IncError()`. Periodically update `metrics.SetActiveAgents()` by querying distinct active agent count.
- [ ] T036 [US4] Write test in `internal/trace/metrics_test.go`: verify counter increments, verify `NullMetrics` does not panic. Write integration test: start server with `--metrics`, perform actions, scrape `/metrics`, verify output contains expected metric names with correct values. Verify server without `--metrics` returns 404 on `/metrics`.

**Checkpoint**: User Story 4 complete. Operators with existing Prometheus+Grafana stacks can scrape SynapBus metrics. The endpoint is passive when no scraper connects.

---

## Phase 8: Polish & Cross-Cutting Concerns

**Purpose**: Improvements that span multiple user stories

- [ ] T037 [P] Implement trace retention cleanup in `internal/trace/retention.go`. Start a background goroutine that runs every hour (configurable). If `--trace-retention` is set to a non-zero duration, call `TraceStore.DeleteOlderThan()` with the computed cutoff time. Log deletions at info level via slog.
- [ ] T038 [P] Wire retention cleanup into the server startup in `cmd/synapbus/main.go`. Parse the `--trace-retention` flag, initialize the retention goroutine if duration > 0, ensure graceful shutdown cancels it.
- [ ] T039 Add context-propagated `request_id` to trace details in `internal/trace/tracer.go`. Extract `request_id` from context (set by logging middleware) and include it in the trace `details` JSON for cross-referencing logs and traces.
- [ ] T040 [P] Verify owner isolation end-to-end: write a multi-owner integration test in `internal/api/traces_handler_test.go` that creates 3 owners, generates traces for each, and verifies no API call or export leaks traces across owners. Covers SC-007.
- [ ] T041 Performance validation: write a benchmark test in `internal/trace/sqlite_store_test.go` using `testing.B`. Verify trace insertion adds < 5ms p99 latency (SC-004). Verify query on 100k traces with filters returns within 500ms (SC-002).
- [ ] T042 [P] Run `make lint` and fix any linting issues across all new files.
- [ ] T043 Graceful shutdown: ensure `Tracer.Close()` is called on server shutdown in `cmd/synapbus/main.go` so all buffered traces are flushed to SQLite before exit.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion — BLOCKS all user stories
- **User Stories (Phases 3–7)**: All depend on Foundational phase completion
  - US1 (Phase 3, P1) should be completed first as MVP
  - US5 (Phase 4, P2) and US2 (Phase 5, P2) can proceed in parallel after US1, or sequentially
  - US3 (Phase 6, P3) depends on US2 filter infrastructure in the API (already built in Phase 2 foundation)
  - US4 (Phase 7, P3) is fully independent of other user stories
- **Polish (Phase 8)**: Depends on all user stories being complete

### User Story Dependencies

- **User Story 1 (P1)**: Can start after Foundational (Phase 2) — No dependencies on other stories. This is the MVP.
- **User Story 5 (P2)**: Can start after Foundational (Phase 2) — Independent. Enhances logging infrastructure.
- **User Story 2 (P2)**: Can start after Foundational (Phase 2) — Independent from US5. Uses same API/store built in foundation. Builds on US1's Svelte page.
- **User Story 3 (P3)**: Can start after Foundational (Phase 2) — Adds export to the API. Adds streaming query to store. UI builds on US2's filter controls.
- **User Story 4 (P3)**: Can start after Foundational (Phase 2) — Fully independent. Only needs the `Tracer` from foundation.

### Within Each User Story

- Models/interfaces before service implementations
- Services before API handlers
- API handlers before Svelte UI components
- Core implementation before integration tests
- Story complete before moving to next priority

### Parallel Opportunities

- Phase 1: T001/T002/T003 touch the same file (`main.go`) — do sequentially. T004 is independent [P].
- Phase 2: T007/T008 are parallel [P] (both in `model.go` but logically grouped). T009 depends on T007/T008. T010 depends on T009. T011 depends on T010. T012/T013 are parallel [P] after their respective implementations.
- Phase 3: T017 and T018 are parallel [P] (different Svelte files). T014–T016 are sequential (instrument → handler → route).
- Phase 6: T031 is parallel [P] with T028–T030 (Svelte vs Go).
- Phase 7: T033 is parallel [P] with other Go work (new file, no dependencies).
- Phase 8: T037, T038, T040, T041, T042 are marked [P] where applicable.

---

## Notes

- All storage uses `modernc.org/sqlite` (pure Go, zero CGO per Constitution Principle III)
- Trace insertion is async via buffered channel — must not block MCP tool calls (FR-013, SC-004)
- Owner isolation is enforced at every layer: store queries always include `owner_id`, API handlers extract owner from session, tests verify no cross-owner leakage (FR-004, SC-007)
- The existing `traces` table in `schema/001_initial.sql` lacks `owner_id` — migration `002_trace_logging.sql` adds it
- Prometheus metrics use `prometheus/client_golang` with a `NullMetrics` no-op for when `--metrics` is disabled
- Structured logging via `slog` replaces all `fmt.Print*` calls — every stdout line must be valid JSON (SC-005)
