# Research: Reactive Agent Triggering System

**Feature**: 014-reactive-agent-triggers
**Date**: 2026-03-25

## R1: K8s Job Status Polling vs Callbacks

**Decision**: Polling via background goroutine every 15 seconds.

**Rationale**: SynapBus's K8s runner already uses in-cluster client-go. Polling is simpler than setting up K8s watch streams or webhooks back to SynapBus. With ~4 agents and max 8 runs/day each, polling is trivially cheap. The poller queries active runs from SQLite, then checks each K8s Job status via client-go.

**Alternatives considered**:
- K8s Watch API: More responsive but requires long-lived connections, reconnect logic, and is overkill for <10 concurrent jobs.
- K8s Job completion callbacks (via init containers or sidecars): Complex, adds container dependencies, violates single-binary principle.
- Argo Events sensor: External dependency, violates Principle I.

## R2: Pending Work Flag Storage

**Decision**: Boolean `pending_work` column on the `agents` table.

**Rationale**: Simplest approach. The flag is set to true when a trigger arrives while the agent is busy, and cleared when the coalesced run launches. No need for a separate queue table since the agent's `claim_messages` workflow handles message ordering.

**Alternatives considered**:
- Separate queue table tracking individual trigger messages: Unnecessary complexity — the agent processes all pending messages anyway via `claim_messages`.
- In-memory flag: Lost on restart. SQLite is authoritative.
- Field on the latest reactive_run record: Complicates queries; cleaner as agent field.

## R3: Trigger Depth Propagation

**Decision**: Depth is tracked at two levels: (1) K8s env var `SYNAPBUS_TRIGGER_DEPTH` for the agent to know its depth, (2) stored on each message sent by a triggered agent as metadata, so the reactor can read it when evaluating the next hop.

**Rationale**: When agent A is triggered at depth N and sends a message mentioning agent B, the message needs to carry depth N+1. The reactor reads this from message metadata when evaluating agent B's trigger. This aligns with the existing `X-SynapBus-Depth` header pattern used for webhooks.

**Alternatives considered**:
- Global depth counter per conversation chain: Complex, requires conversation tracking.
- Only counting via webhook headers: Doesn't work for MCP-originated messages.

## R4: Cooldown Timer — From Start or From Completion

**Decision**: Cooldown starts from the most recent run's `created_at` timestamp (i.e., when the job was launched, not when it completed).

**Rationale**: Simpler and more predictable. If an agent runs for 30 minutes, the cooldown is already partially elapsed by completion time. Starting from launch prevents rapid re-triggering even if the previous run was fast.

**Alternatives considered**:
- From completion time: Could lead to very long effective cooldowns for long-running jobs. A 10-minute cooldown + 30-minute run = 40 minutes between runs.
- Configurable (start vs completion): Over-engineering for current needs.

## R5: CronJob vs Reactive Job Overlap Detection

**Decision**: The reactor checks for any running K8s Job with the agent's label (`synapbus-agent=<name>`), regardless of whether it's a CronJob-spawned or reactor-spawned job. If any is running, `pending_work` is set.

**Rationale**: The sequential execution constraint applies to all runs, not just reactive ones. Using K8s label selectors is clean and already supported by client-go.

**Alternatives considered**:
- Only tracking reactive runs in SQLite: Misses CronJob runs, could cause concurrent execution.
- Requiring agents to report "busy" status via MCP: Adds agent-side complexity, unreliable if agent crashes.

## R6: Self-Mention Detection

**Decision**: When extracting mentions from a message, filter out the sender's own agent name. The reactor never triggers an agent based on its own message.

**Rationale**: Prevents trivial infinite loops where an agent mentions itself in its response.

**Alternatives considered**:
- Relying on depth limit to catch self-loops: Too permissive — wastes budget on preventable triggers.
- No self-mention filtering: Dangerous with reactive agents.

## R7: Web UI Polling vs SSE for Agent Runs

**Decision**: The Agent Runs page uses polling (every 10 seconds) to refresh run statuses, same as other SynapBus Web UI pages.

**Rationale**: Consistent with existing Web UI patterns. SSE is already used for message notifications but adding a new SSE channel for run status adds complexity. Polling at 10s intervals is adequate for runs that take minutes.

**Alternatives considered**:
- SSE push: More responsive but adds server-side event infrastructure for a page that's not time-critical.
- WebSocket: Overkill, not used elsewhere in SynapBus.
