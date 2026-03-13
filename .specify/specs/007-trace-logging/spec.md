# Feature Specification: Trace Logging & Observability

**Feature Branch**: `007-trace-logging`
**Created**: 2026-03-13
**Status**: Draft
**Input**: User description: "All agent actions logged: tool calls, messages sent/received, channel joins, errors. Traces stored in SQLite with agent_name, action, details, timestamp. Owner can view traces for their agents via Web UI. Filterable by agent, action type, time range. Exportable as JSON/CSV. Optional Prometheus metrics endpoint (/metrics). Structured logging (slog) to stdout."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Owner Inspects Agent Activity Traces (Priority: P1)

An owner logs into the SynapBus Web UI and navigates to the "Traces" view to see everything their agents have done. They see a reverse-chronological log of all actions (tool calls, messages sent, messages received, channel joins/leaves, errors) across all their agents. They click on an individual trace entry to expand the full JSON details. This is the foundational use case: a human gaining visibility into what their agents are doing.

**Why this priority**: Without trace storage and a basic viewing interface, none of the other stories (filtering, exporting, metrics) have anything to build on. This directly implements Constitution Principle VIII (Observable by Default) and Principle IV (owners control and view agent activity).

**Independent Test**: Can be fully tested by registering an agent, performing several MCP tool calls (send_message, join_channel, read_inbox), then logging into the Web UI as the agent's owner and verifying each action appears in the trace view with correct agent name, action type, timestamp, and JSON details.

**Acceptance Scenarios**:

1. **Given** an agent "research-bot" owned by user "alice" has sent 3 messages and joined 1 channel, **When** alice opens the Traces view in the Web UI, **Then** she sees 4 trace entries in reverse chronological order, each showing agent_name="research-bot", the action type, and a human-readable timestamp.
2. **Given** alice is viewing the Traces list, **When** she clicks on a trace entry for a `send_message` action, **Then** she sees the full JSON details including the recipient, channel (if any), message body preview, and message ID.
3. **Given** agent "research-bot" encounters an error (e.g., sending a message to a non-existent agent), **When** alice views the Traces, **Then** she sees a trace entry with action="error", and the details JSON includes the error message and the originating tool call.
4. **Given** user "bob" also has agents, **When** bob opens the Traces view, **Then** he sees only traces for his own agents and never sees traces belonging to alice's agents.

---

### User Story 2 - Owner Filters and Searches Traces (Priority: P2)

An owner has accumulated hundreds or thousands of trace entries across multiple agents. They need to narrow down to specific activity: a particular agent, a specific action type (e.g., only errors), or a specific time window. The Web UI provides filter controls for agent name, action type, and time range. Filters can be combined. Results update in real time as filters change.

**Why this priority**: Trace viewing (P1) becomes unwieldy at scale without filtering. This story makes traces operationally useful for debugging and monitoring. It depends on P1's trace storage being in place.

**Independent Test**: Can be tested by generating 50+ trace entries across 2 agents with mixed action types, then applying each filter individually and in combination, verifying the result set matches expectations.

**Acceptance Scenarios**:

1. **Given** alice has two agents ("research-bot" and "writer-bot") with 100 combined trace entries, **When** she selects agent="research-bot" in the filter, **Then** only traces for "research-bot" are displayed and the count updates accordingly.
2. **Given** alice is viewing traces with no filters, **When** she selects action_type="error" from the action type dropdown, **Then** only error traces are shown.
3. **Given** alice selects a time range of "last 1 hour", **When** the filter is applied, **Then** only traces with timestamps within the last 60 minutes appear, and older entries are excluded.
4. **Given** alice has set agent="research-bot" AND action_type="send_message" AND time range="today", **When** results are displayed, **Then** only traces matching all three criteria are shown.

---

### User Story 3 - Owner Exports Traces as JSON or CSV (Priority: P3)

An owner needs to share agent activity logs with a colleague, feed them into an external analysis tool, or archive them for compliance. They apply filters (or leave them unfiltered) and click an export button. They choose JSON or CSV format. The file downloads to their browser containing all matching trace entries with full details.

**Why this priority**: Export is a value-add on top of viewing and filtering. It enables external workflows (compliance, analytics, debugging outside the UI) but is not required for core observability.

**Independent Test**: Can be tested by generating trace entries, optionally applying filters, then exporting as JSON and CSV separately, and verifying both files parse correctly and contain the expected number of entries with all fields.

**Acceptance Scenarios**:

1. **Given** alice is viewing unfiltered traces (50 entries), **When** she clicks "Export as JSON", **Then** a file `traces-YYYY-MM-DD.json` downloads containing a JSON array of 50 trace objects, each with fields: id, agent_name, action, details (object), timestamp.
2. **Given** alice has filtered traces to agent="research-bot" (20 entries), **When** she clicks "Export as CSV", **Then** a file `traces-YYYY-MM-DD.csv` downloads with 20 data rows plus a header row. Columns: id, agent_name, action, details (JSON-encoded string), timestamp.
3. **Given** alice exports traces with no matching results (empty filter), **When** the export completes, **Then** the JSON file contains an empty array `[]` and the CSV file contains only the header row.

---

### User Story 4 - Operator Enables Prometheus Metrics (Priority: P3)

A SynapBus operator running the service on a team server wants to monitor system health using their existing Prometheus + Grafana stack. They start SynapBus with `--metrics` (or set `SYNAPBUS_METRICS=true`). A `/metrics` endpoint becomes available on the HTTP server, exposing counters and histograms for trace actions, message throughput, active agents, and error rates. This endpoint is unauthenticated (standard for Prometheus scrape targets behind a firewall).

**Why this priority**: Prometheus metrics are explicitly optional and serve operators with existing monitoring infrastructure. Core observability (traces in SQLite + Web UI) works without this.

**Independent Test**: Can be tested by starting SynapBus with `--metrics`, performing several agent actions, then curling `/metrics` and verifying Prometheus-formatted output includes expected metric names and non-zero counters.

**Acceptance Scenarios**:

1. **Given** SynapBus is started with `--metrics`, **When** an operator sends `GET /metrics`, **Then** the response is `text/plain` in Prometheus exposition format and includes at least: `synapbus_traces_total` (counter), `synapbus_traces_by_action` (counter vec with action label), `synapbus_active_agents` (gauge).
2. **Given** SynapBus is started without `--metrics`, **When** an operator sends `GET /metrics`, **Then** the server returns 404 Not Found.
3. **Given** SynapBus is running with `--metrics` and agent "research-bot" has sent 5 messages, **When** an operator scrapes `/metrics`, **Then** `synapbus_traces_by_action{action="send_message"}` reports a value of at least 5.

---

### User Story 5 - Structured Logging to stdout (Priority: P2)

A SynapBus operator wants structured, machine-parseable logs on stdout for integration with log aggregation systems (Loki, CloudWatch, ELK). All server-side log output uses Go's `slog` package with JSON format. Each log line includes timestamp, level, message, and relevant context fields (agent_name, action, request_id, error). Log level is configurable via `--log-level` flag (debug, info, warn, error).

**Why this priority**: Structured logging is foundational infrastructure that benefits both development and production. It is required by Constitution Principle VIII and improves debuggability from day one, independent of the Web UI trace viewer.

**Independent Test**: Can be tested by starting SynapBus with `--log-level=debug`, performing agent actions, and piping stdout through `jq` to verify each line is valid JSON with the expected fields.

**Acceptance Scenarios**:

1. **Given** SynapBus is started with `--log-level=info`, **When** an agent sends a message via MCP, **Then** stdout emits a JSON log line with keys: `time`, `level` ("INFO"), `msg`, `agent_name`, `action` ("send_message"), and `request_id`.
2. **Given** SynapBus is started with `--log-level=error`, **When** an agent sends a message successfully, **Then** no log line is emitted for that action (since it is info-level). Only errors appear on stdout.
3. **Given** SynapBus is started with `--log-level=debug`, **When** any MCP tool call is received, **Then** a debug-level log line is emitted containing the full tool call parameters as a JSON field.

---

### Edge Cases

- What happens when the traces table grows very large (millions of rows)? Trace queries MUST use indexed columns (agent_name, action, timestamp) and paginate results. The system SHOULD support a configurable retention period (`--trace-retention=30d`) that auto-deletes traces older than the threshold.
- How does the system handle a burst of concurrent agent actions generating traces? Trace inserts MUST NOT block the MCP tool call response. Traces SHOULD be buffered in-memory and flushed to SQLite in batches to avoid write contention.
- What happens if the SQLite write fails during trace insertion (e.g., disk full)? The agent's tool call MUST still succeed. The failure MUST be logged to stderr/slog at error level. The trace is lost but the agent operation is not impacted.
- What happens when an owner has zero agents or zero traces? The Web UI MUST display an empty state with a helpful message ("No traces found. Agent activity will appear here once your agents start performing actions.").
- What happens if an agent is deleted but its traces remain? Traces MUST be retained even after agent deletion. The agent_name field in trace records is a denormalized string, not a foreign key, so traces survive agent removal. A filter for deleted agents SHOULD still work.
- How does export handle very large result sets (100k+ traces)? Export MUST stream the response rather than buffering the entire result in memory. The HTTP response SHOULD use `Transfer-Encoding: chunked` for large exports.
- What happens when Prometheus metrics are enabled but no scraper connects? No impact. The `/metrics` endpoint is passive; metrics are maintained in-memory regardless, with negligible overhead.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST record a trace entry for every MCP tool call received, including: agent_name, action (tool name), details (JSON object with call parameters and result summary), and timestamp (UTC).
- **FR-002**: System MUST record trace entries for the following action types at minimum: `send_message`, `read_inbox`, `claim_messages`, `mark_done`, `search_messages`, `create_channel`, `join_channel`, `list_channels`, `register_agent`, `discover_agents`, `post_task`, `bid_task`, `upload_attachment`, `read_attachment`, and `error`.
- **FR-003**: Trace entries MUST include an `owner_id` field derived from the agent's owner, enabling owner-scoped queries without joining the agents table.
- **FR-004**: System MUST enforce owner isolation: REST API trace endpoints and Web UI MUST only return traces belonging to the authenticated owner's agents. No cross-owner trace access is permitted.
- **FR-005**: System MUST provide a REST API endpoint (`GET /api/traces`) that returns paginated traces, accepting query parameters: `agent_name`, `action`, `since` (ISO 8601 timestamp), `until` (ISO 8601 timestamp), `page`, and `page_size` (default 50, max 200).
- **FR-006**: System MUST provide a REST API endpoint (`GET /api/traces/export`) that streams traces in the format specified by the `Accept` header or `format` query parameter (`json` or `csv`).
- **FR-007**: The Web UI MUST display a Traces view accessible from the main navigation, showing a paginated, reverse-chronological list of trace entries with columns: timestamp, agent name, action, and a details preview.
- **FR-008**: The Web UI Traces view MUST provide filter controls for agent name (dropdown of owner's agents), action type (dropdown), and time range (date/time pickers or preset ranges: last hour, last 24h, last 7d, custom).
- **FR-009**: System MUST use Go's `slog` package for all server-side logging, with JSON output format on stdout.
- **FR-010**: Log level MUST be configurable via `--log-level` CLI flag and `SYNAPBUS_LOG_LEVEL` environment variable, supporting values: `debug`, `info`, `warn`, `error`. Default: `info`.
- **FR-011**: When `--metrics` is enabled, the system MUST expose a Prometheus-compatible `/metrics` HTTP endpoint with at least: `synapbus_traces_total` (counter), `synapbus_traces_by_action` (counter vec, label: action), `synapbus_active_agents` (gauge), `synapbus_errors_total` (counter).
- **FR-012**: When `--metrics` is not enabled, the `/metrics` endpoint MUST NOT be registered (404 response).
- **FR-013**: Trace insertion MUST NOT block or slow down the MCP tool call that triggered it. Traces SHOULD be written asynchronously.
- **FR-014**: System SHOULD support a `--trace-retention` flag (e.g., `30d`, `90d`, `0` for unlimited) that triggers periodic cleanup of traces older than the specified duration.

### Key Entities

- **Trace**: Represents a single recorded agent action. Attributes: `id` (integer, auto-increment), `owner_id` (string, denormalized from agent), `agent_name` (string, denormalized), `action` (string, e.g. "send_message", "error"), `details` (JSON text, contains tool call parameters, result summary, error info), `timestamp` (UTC datetime). Indexed on: `(owner_id, timestamp)`, `(owner_id, agent_name, timestamp)`, `(owner_id, action, timestamp)`.
- **Metric**: In-memory Prometheus metric (counter, gauge, or histogram). Not persisted to SQLite. Registered conditionally when `--metrics` is enabled. Managed via `prometheus/client_golang` or a pure-Go metrics library.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Every MCP tool call results in a corresponding trace entry in SQLite within 1 second, with zero tool calls lost under normal operation (no disk-full or crash conditions).
- **SC-002**: An owner viewing traces in the Web UI can filter by agent, action type, and time range, and see results update within 500ms for datasets up to 100,000 trace entries.
- **SC-003**: Trace export (JSON/CSV) completes and initiates download for 10,000 entries in under 5 seconds.
- **SC-004**: Trace insertion adds less than 5ms of latency to MCP tool call response times (measured as p99).
- **SC-005**: All server log output on stdout is valid JSON parseable by `jq`, with no unstructured log lines emitted under any code path.
- **SC-006**: When `--metrics` is enabled, `/metrics` returns valid Prometheus exposition format that can be scraped by a standard Prometheus server without errors.
- **SC-007**: Owner isolation is enforced: no REST API call or Web UI interaction allows an owner to access traces belonging to another owner's agents, verified by automated tests with multi-owner scenarios.
