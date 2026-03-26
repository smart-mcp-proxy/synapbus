# Feature Specification: Reactive Agent Triggering System

**Feature Branch**: `014-reactive-agent-triggers`
**Created**: 2026-03-25
**Status**: Draft
**Input**: Brainstormed and approved design from conversation — reactive agent triggering via DM/@mention with K8s Job orchestration.

## Assumptions

- Reactive triggers fire only on `message.received` (DM) and `message.mentioned` (@mention) events — not on `workflow.state_changed` or `channel.message` (deferred to a future feature)
- All agents are hybrid: they have existing K8s CronJob schedules and can additionally be triggered reactively by SynapBus
- The reactor engine lives inside SynapBus as `internal/reactor/` — no external coordinator service
- K8s is the only trigger mechanism for v1; webhook-based triggers are deferred (infrastructure exists but is not wired to the reactor)
- Agent K8s image and env config are stored on the agent registry record — the existing `k8s_handlers` table is for the legacy webhook-style K8s integration and remains unchanged
- The universal agent template (`searcher/agents/universal/run_agent.py`) reads `SYNAPBUS_MESSAGE_ID`, `SYNAPBUS_MESSAGE_BODY`, `SYNAPBUS_FROM_AGENT`, `SYNAPBUS_EVENT` env vars and prepends trigger context to the agent prompt
- Coalescing: when an agent is busy and new triggers arrive, a `pending_work` flag is set. On job completion, if the flag is set, a new run launches. The agent's `my_status` / `claim_messages` workflow handles processing all pending messages — SynapBus does not queue individual messages
- Per-agent configurable rate limits with defaults: cooldown = 600 seconds, daily budget = 8 runs, max trigger depth = 5
- Trigger depth is propagated via `SYNAPBUS_TRIGGER_DEPTH` env var and incremented on each agent-to-agent hop; if an agent sends a message via MCP that triggers another agent, the depth increases
- Failed reactive jobs send a system DM to the agent's owner when a reactive job fails, including agent name, trigger context, duration, and error summary
- The Web UI Agent Runs panel is a new page showing recent reactive triggers with status, trigger context, duration, and expandable error logs
- Token cost tracking is optional — agents may report it back but it is not required for v1
- Migration number: 015_reactive_triggers.sql (next after existing migrations)
- Admin CLI commands use the existing `synapbus` cobra command tree
- `k8s_env_json` stores both plain env vars and secret references (format: `{"AGENT_GIT_REPO": "value", "SYNAPBUS_API_KEY": {"secretRef": "secret-name", "key": "key-name"}}`)
- SYNAPBUS_MESSAGE_BODY is truncated to 4KB when passed as an env var

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Reactive Agent Trigger via DM (Priority: P1)

A human owner sends a DM to an agent (e.g., "research-mcpproxy") via SynapBus. SynapBus detects the agent has `trigger_mode='reactive'`, passes all rate-limit checks, and automatically launches a K8s Job running the agent's container image. The agent processes the DM as its first priority.

**Why this priority**: This is the core value proposition — agents respond to messages in near-real-time instead of waiting for the next cron cycle.

**Independent Test**: Send a DM to a reactive agent, verify a K8s Job is created with the correct env vars, and the agent responds to the message.

**Acceptance Scenarios**:

1. **Given** agent "research-mcpproxy" with `trigger_mode='reactive'` and no active runs, **When** a human sends it a DM, **Then** SynapBus creates a K8s Job within 5 seconds with `SYNAPBUS_MESSAGE_ID`, `SYNAPBUS_MESSAGE_BODY`, `SYNAPBUS_FROM_AGENT`, `SYNAPBUS_EVENT=message.received` env vars.
2. **Given** agent "research-mcpproxy" with `trigger_mode='passive'`, **When** a human sends it a DM, **Then** no reactive trigger fires; the agent picks up the message on its next scheduled run.
3. **Given** agent "research-mcpproxy" with `trigger_mode='reactive'`, **When** a DM is sent, **Then** a `reactive_runs` record is created with `status='running'` and `trigger_event='message.received'`.

---

### User Story 2 - Reactive Agent Trigger via @Mention (Priority: P1)

A human or agent @mentions another agent in a channel message (e.g., "@social-commenter check this thread"). SynapBus detects the mention, checks if the mentioned agent is reactive, and triggers it.

**Why this priority**: @mentions are the primary way to request agent attention in channel conversations — equally important as DMs.

**Independent Test**: Post a channel message mentioning a reactive agent, verify a K8s Job is created.

**Acceptance Scenarios**:

1. **Given** agent "social-commenter" with `trigger_mode='reactive'`, **When** a message containing "@social-commenter" is posted in a channel, **Then** SynapBus triggers the agent with `SYNAPBUS_EVENT=message.mentioned`.
2. **Given** a message mentioning multiple reactive agents, **When** the message is sent, **Then** each mentioned agent is evaluated independently for triggering (subject to their own cooldown/budget).
3. **Given** agent "social-commenter" already running, **When** a new @mention arrives, **Then** the `pending_work` flag is set and no additional job is created until the current one completes.

---

### User Story 3 - Rate Limiting: Cooldown (Priority: P1)

To control costs, each agent has a configurable cooldown period. After a reactive run starts, no new reactive run can be triggered for that agent until the cooldown elapses.

**Why this priority**: Without cooldown, a burst of messages could trigger many expensive runs in rapid succession.

**Independent Test**: Trigger an agent, then immediately send another DM. Verify the second trigger is recorded as `cooldown_skipped`.

**Acceptance Scenarios**:

1. **Given** agent with `cooldown_seconds=600` and a run that started 3 minutes ago, **When** a new DM arrives, **Then** the trigger is recorded as `cooldown_skipped` and no K8s Job is created.
2. **Given** agent with `cooldown_seconds=600` and last run started 11 minutes ago, **When** a new DM arrives, **Then** the agent is triggered normally.
3. **Given** a trigger that was `cooldown_skipped`, **When** the cooldown elapses, **Then** if `pending_work` is set, a new run launches automatically.

---

### User Story 4 - Rate Limiting: Daily Budget (Priority: P1)

Each agent has a configurable daily limit on the number of reactive runs. Once exhausted, no more reactive triggers fire until the next day.

**Why this priority**: Hard cap on daily spend per agent prevents runaway costs.

**Independent Test**: Configure an agent with daily budget of 2, trigger it twice successfully, then send a third DM. Verify the third is recorded as `budget_exhausted`.

**Acceptance Scenarios**:

1. **Given** agent with `daily_trigger_budget=8` and 7 runs today, **When** a new DM arrives, **Then** the agent is triggered (8th run).
2. **Given** agent with `daily_trigger_budget=8` and 8 runs today, **When** a new DM arrives, **Then** the trigger is recorded as `budget_exhausted` and no job is created.
3. **Given** budget-exhausted agent, **When** a new calendar day begins (UTC), **Then** the budget resets and new triggers can fire.

---

### User Story 5 - Rate Limiting: Trigger Depth (Priority: P1)

When agents trigger other agents (agent A's response mentions @agent-B), the depth counter increments. If depth exceeds the agent's `max_trigger_depth`, the cascade stops.

**Why this priority**: Prevents infinite agent-to-agent loops which could be extremely costly.

**Independent Test**: Set max_trigger_depth=2 on an agent, simulate a depth-3 trigger chain, verify the third hop is blocked.

**Acceptance Scenarios**:

1. **Given** agent with `max_trigger_depth=5` and an incoming trigger at depth 4, **When** evaluated, **Then** the trigger fires (depth 4 < max 5).
2. **Given** agent with `max_trigger_depth=5` and an incoming trigger at depth 5, **When** evaluated, **Then** the trigger is blocked and recorded as `depth_exceeded`.
3. **Given** a human-initiated DM (depth 0), **When** the triggered agent sends a message mentioning another agent, **Then** the second agent receives the trigger with depth 1.

---

### User Story 6 - Sequential Execution with Coalescing (Priority: P1)

Only one reactive K8s Job runs per agent at a time. If new triggers arrive while the agent is busy, they are coalesced — a single follow-up run launches when the current one completes, and the agent processes all accumulated messages.

**Why this priority**: Prevents concurrent modification of agent workspaces and saves tokens by avoiding redundant startups.

**Independent Test**: Trigger an agent, send 3 more DMs while it's running. Verify only one follow-up run launches after the first completes.

**Acceptance Scenarios**:

1. **Given** agent currently running a reactive job, **When** a new DM arrives, **Then** `pending_work` flag is set to true, no new job is created, and the trigger is recorded as `queued`.
2. **Given** agent finishes a run and `pending_work` is true, **When** the poller detects job completion, **Then** `pending_work` is cleared and a new coalesced run is launched (subject to cooldown/budget checks).
3. **Given** agent finishes a run and `pending_work` is false, **When** the poller detects job completion, **Then** no follow-up run launches.
4. **Given** 5 DMs arrive while agent is busy, **When** the follow-up run launches, **Then** only one K8s Job is created (not 5), and the agent uses `claim_messages` to process all pending messages.

---

### User Story 7 - Job Failure Notification (Priority: P2)

When a reactive K8s Job fails (exit code != 0, OOMKilled, timeout), SynapBus detects the failure, retrieves pod logs, records the error, and sends a system DM to the agent's human owner.

**Why this priority**: Visibility into failures is essential for debugging but not strictly required for the trigger mechanism to function.

**Independent Test**: Configure an agent with an image that exits with error, trigger it, verify owner receives a system DM with error details.

**Acceptance Scenarios**:

1. **Given** a reactive job that fails with exit code 1, **When** the poller detects failure, **Then** the `reactive_runs` record is updated with `status='failed'`, `error_log` containing the last 100 lines of pod logs, and `completed_at` timestamp.
2. **Given** a failed reactive run, **When** the failure is recorded, **Then** a system DM is sent to the agent's owner with agent name, trigger reason, duration, and error summary.
3. **Given** a reactive job that exceeds its timeout, **When** the pod is killed, **Then** the run is recorded as `failed` with error indicating timeout.

---

### User Story 8 - Web UI Agent Runs Panel (Priority: P2)

The SynapBus Web UI includes an "Agent Runs" page showing recent reactive triggers, their status, and details. Owners can filter by agent and status, view error logs, click through to the original trigger message, and retry failed runs.

**Why this priority**: Complements DM notifications with a historical, browsable view — important for day-to-day management but not blocking core functionality.

**Independent Test**: Trigger several agents (some succeed, some fail), navigate to Agent Runs page, verify all runs are listed with correct status and details.

**Acceptance Scenarios**:

1. **Given** several reactive runs have occurred, **When** the owner navigates to the Agent Runs page, **Then** runs are listed in reverse chronological order showing: status badge, agent name, trigger reason, duration, and timestamp.
2. **Given** a failed run, **When** the owner clicks to expand it, **Then** the error log and a "Retry" button are shown.
3. **Given** the owner clicks "Retry" on a failed run, **When** the retry fires, **Then** a new reactive run is created for the same agent (subject to cooldown/budget checks).
4. **Given** multiple reactive agents, **When** the owner views the page, **Then** agent summary cards at the top show: name, today's budget usage (e.g., "3/8 runs"), cooldown status, and current state (idle/running/queued).
5. **Given** a run with a trigger message, **When** the owner clicks the message link, **Then** they are navigated to the message in the Web UI.

---

### User Story 9 - Admin CLI for Trigger Configuration (Priority: P2)

System administrators can configure reactive triggers per agent via the CLI: set trigger mode, cooldown, daily budget, max depth, K8s image, and environment variables.

**Why this priority**: Required for initial setup and ongoing management, but can be done via direct DB manipulation as a workaround.

**Independent Test**: Use CLI to configure an agent as reactive, then verify the agent triggers on DM.

**Acceptance Scenarios**:

1. **Given** agent "research-mcpproxy", **When** admin runs `synapbus agent set-triggers ... --mode reactive --cooldown 600 --daily-budget 8 --max-depth 5`, **Then** the agent's trigger configuration is updated in the registry.
2. **Given** agent with no image configured, **When** admin runs `synapbus agent set-image ... --image <image> --env KEY=VALUE`, **Then** the image and env config are stored on the agent record.
3. **Given** admin wants to view recent runs, **When** they run `synapbus runs list --agent <name>`, **Then** recent runs are displayed with status, duration, and trigger reason.
4. **Given** a failed run, **When** admin runs `synapbus runs logs <run-id>`, **Then** the error log for that run is displayed.

---

### User Story 10 - Webhook Payload Enrichment (Priority: P3)

Webhook payloads for `message.received` and `message.mentioned` events include a `trigger` block with depth and run context, enabling future webhook-based agent triggers.

**Why this priority**: Future-proofing for webhook-based triggers. No immediate user need but prepares the infrastructure.

**Independent Test**: Register a webhook, send a message that triggers it, verify the payload includes the `trigger` block.

**Acceptance Scenarios**:

1. **Given** an agent with a registered webhook for `message.received`, **When** a DM is sent, **Then** the webhook payload includes a `trigger` object with `depth` and `triggered_by_run_id` fields.

---

### Edge Cases

- What happens when the K8s cluster is unreachable? The reactor records the run as `failed` with a connection error and sends a failure DM to the owner.
- What happens when a reactive agent's K8s image is not configured? The reactor skips the trigger and logs a warning. The trigger is recorded as `failed` with reason "no k8s_image configured".
- What happens when two DMs arrive simultaneously for the same agent? The reactor processes them sequentially (database-level locking on the agent). The first creates a job; the second sets `pending_work`.
- What happens when a scheduled CronJob and a reactive trigger overlap? The reactor checks for any running K8s Job for that agent (both scheduled and reactive). If one is running, it sets `pending_work` and waits.
- What happens when the daily budget resets while a coalesced run is pending? The pending run uses the new day's budget.
- What happens when an agent is mentioned in its own message (self-mention)? Self-mentions are ignored — an agent cannot trigger itself.
- What happens when the message body exceeds 4KB? It is truncated to 4KB in the `SYNAPBUS_MESSAGE_BODY` env var with a `[truncated]` suffix.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST detect DMs and @mentions to agents with `trigger_mode='reactive'` and initiate a reactive trigger evaluation.
- **FR-002**: System MUST enforce per-agent cooldown periods between reactive runs, rejecting triggers during cooldown.
- **FR-003**: System MUST enforce per-agent daily run budgets, rejecting triggers when the budget is exhausted.
- **FR-004**: System MUST track and enforce trigger depth limits to prevent infinite agent-to-agent cascades.
- **FR-005**: System MUST ensure only one reactive K8s Job runs per agent at any time (sequential execution).
- **FR-006**: System MUST coalesce pending triggers — when a new trigger arrives while an agent is busy, a `pending_work` flag is set and a single follow-up run launches after the current job completes.
- **FR-007**: System MUST pass trigger context to K8s Jobs via environment variables: `SYNAPBUS_MESSAGE_ID`, `SYNAPBUS_MESSAGE_BODY`, `SYNAPBUS_FROM_AGENT`, `SYNAPBUS_EVENT`, `SYNAPBUS_TRIGGER_DEPTH`.
- **FR-008**: System MUST record every trigger evaluation (successful or not) in the `reactive_runs` table with appropriate status.
- **FR-009**: System MUST poll K8s Job status and update `reactive_runs` records when jobs complete (succeed or fail).
- **FR-010**: System MUST retrieve and store the last 100 lines of pod logs for failed reactive runs.
- **FR-011**: System MUST send a system DM to the agent's owner when a reactive job fails, including agent name, trigger context, duration, and error summary.
- **FR-012**: System MUST provide a Web UI page listing reactive runs with filtering by agent and status.
- **FR-013**: System MUST display agent summary cards in the Web UI showing budget usage, cooldown status, and current state.
- **FR-014**: System MUST allow retrying failed runs from the Web UI (subject to rate limits).
- **FR-015**: System MUST provide CLI commands for configuring agent trigger settings (mode, cooldown, budget, depth, image, env vars).
- **FR-016**: System MUST provide CLI commands for listing and inspecting reactive runs.
- **FR-017**: System MUST ignore self-mentions (an agent cannot trigger itself).
- **FR-018**: System MUST truncate `SYNAPBUS_MESSAGE_BODY` to 4KB when passed as an env var.
- **FR-019**: System MUST include trigger context (`depth`, `triggered_by_run_id`) in webhook payloads for `message.received` and `message.mentioned` events.
- **FR-020**: System MUST support configurable rate limits per agent (cooldown, daily budget, max depth) with system-wide defaults.

### Key Entities

- **Agent (extended)**: Gains `trigger_mode` (passive/reactive/disabled), `cooldown_seconds`, `daily_trigger_budget`, `max_trigger_depth`, `k8s_image`, `k8s_env_json`, `k8s_resource_preset` fields.
- **Reactive Run**: A record of a trigger evaluation and its outcome. Tracks agent, trigger message, event type, depth, status (queued/running/succeeded/failed/cooldown_skipped/budget_exhausted/depth_exceeded), K8s job metadata, timing, error logs, and optional token cost.
- **Pending Work Flag**: A per-agent boolean indicating that new triggers arrived while the agent was busy. Stored on the agent record or in a dedicated field on the latest running reactive_run.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Reactive agents respond to DMs and @mentions within 30 seconds of message delivery (time from message sent to K8s Job created).
- **SC-002**: No more than one reactive K8s Job runs per agent at any time — verified by checking job state and run records.
- **SC-003**: Cooldown enforcement prevents back-to-back triggers — an agent triggered at time T cannot be triggered again before T + cooldown_seconds.
- **SC-004**: Daily budget enforcement caps reactive runs — after N runs in a calendar day (UTC), all further triggers are recorded as `budget_exhausted`.
- **SC-005**: Trigger depth enforcement prevents cascades beyond the configured limit — a trigger chain deeper than max_trigger_depth is blocked.
- **SC-006**: Agent owners receive failure notifications within 60 seconds of job failure detection.
- **SC-007**: The Web UI Agent Runs page accurately reflects all reactive runs with correct status, timing, and trigger context.
- **SC-008**: Admin CLI commands successfully configure trigger settings and display run history.
- **SC-009**: Coalesced runs process all pending messages in a single session — verified by checking that the agent handles all queued work.
