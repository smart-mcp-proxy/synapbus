# Data Model: Reactive Agent Triggering System

**Feature**: 014-reactive-agent-triggers
**Date**: 2026-03-25

## Entity Changes

### Agent (extended)

Existing `agents` table gains new columns for reactive trigger configuration.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `trigger_mode` | TEXT | `'passive'` | `passive` (polls only), `reactive` (auto-triggered), `disabled` (no triggers) |
| `cooldown_seconds` | INTEGER | `600` | Minimum seconds between reactive runs |
| `daily_trigger_budget` | INTEGER | `8` | Max reactive runs per UTC calendar day |
| `max_trigger_depth` | INTEGER | `5` | Max agent-to-agent cascade depth |
| `k8s_image` | TEXT | `NULL` | Container image for reactive K8s Jobs |
| `k8s_env_json` | TEXT | `NULL` | JSON object of env vars (plain + secret refs) |
| `k8s_resource_preset` | TEXT | `'default'` | Resource limits: `default` (256Mi/100m) or `large` (2Gi/1CPU) |
| `pending_work` | BOOLEAN | `0` | True if triggers arrived while agent was busy |

### Reactive Run (new)

New `reactive_runs` table tracking every trigger evaluation.

| Field | Type | Nullable | Description |
|-------|------|----------|-------------|
| `id` | INTEGER PK | No | Auto-increment |
| `agent_name` | TEXT FK | No | References `agents(name)` |
| `trigger_message_id` | INTEGER FK | Yes | References `messages(id)` — the message that caused the trigger |
| `trigger_event` | TEXT | No | `message.received` or `message.mentioned` |
| `trigger_depth` | INTEGER | No | Depth in the cascade chain (0 = human-initiated) |
| `trigger_from` | TEXT | Yes | Agent/user who sent the trigger message |
| `status` | TEXT | No | `queued`, `running`, `succeeded`, `failed`, `cooldown_skipped`, `budget_exhausted`, `depth_exceeded` |
| `k8s_job_name` | TEXT | Yes | K8s Job name (set when job is created) |
| `k8s_namespace` | TEXT | Yes | K8s namespace |
| `started_at` | DATETIME | Yes | When the K8s Job was created |
| `completed_at` | DATETIME | Yes | When the K8s Job finished |
| `duration_ms` | INTEGER | Yes | Computed: completed_at - started_at |
| `error_log` | TEXT | Yes | Last 100 lines of pod logs on failure |
| `token_cost_json` | TEXT | Yes | Optional: `{"input": N, "output": N}` |
| `created_at` | DATETIME | No | When the trigger evaluation happened |

**Indexes**:
- `idx_reactive_runs_agent_created` on `(agent_name, created_at)` — for budget counting and cooldown checks
- `idx_reactive_runs_status` on `(status)` — for poller to find active runs
- `idx_reactive_runs_agent_status` on `(agent_name, status)` — for coalescing checks

### State Transitions

```
Trigger evaluation:
  → [all checks pass, agent idle]     → status: 'running'
  → [all checks pass, agent busy]     → status: 'queued' (sets pending_work)
  → [cooldown not elapsed]            → status: 'cooldown_skipped'
  → [daily budget exhausted]          → status: 'budget_exhausted'
  → [depth exceeded]                  → status: 'depth_exceeded'
  → [no k8s_image configured]         → status: 'failed'
  → [K8s cluster unreachable]         → status: 'failed'

Job completion (via poller):
  → [exit code 0]                     → status: 'succeeded'
  → [exit code != 0 / OOM / timeout]  → status: 'failed'
  → [pending_work set]                → new run launched (back to evaluation)
```

### Message Metadata (extended)

Messages sent by triggered agents carry `trigger_depth` in their metadata JSON field. When the dispatcher evaluates mentions from such a message, it reads the depth and increments it for the next trigger evaluation.

| Metadata Key | Type | Description |
|-------------|------|-------------|
| `trigger_depth` | INTEGER | Current depth in cascade chain |

## Migration: 015_reactive_triggers.sql

```sql
-- Extend agents table with reactive trigger configuration
ALTER TABLE agents ADD COLUMN trigger_mode TEXT NOT NULL DEFAULT 'passive';
ALTER TABLE agents ADD COLUMN cooldown_seconds INTEGER NOT NULL DEFAULT 600;
ALTER TABLE agents ADD COLUMN daily_trigger_budget INTEGER NOT NULL DEFAULT 8;
ALTER TABLE agents ADD COLUMN max_trigger_depth INTEGER NOT NULL DEFAULT 5;
ALTER TABLE agents ADD COLUMN k8s_image TEXT;
ALTER TABLE agents ADD COLUMN k8s_env_json TEXT;
ALTER TABLE agents ADD COLUMN k8s_resource_preset TEXT NOT NULL DEFAULT 'default';
ALTER TABLE agents ADD COLUMN pending_work INTEGER NOT NULL DEFAULT 0;

-- New table: reactive trigger runs
CREATE TABLE reactive_runs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    agent_name TEXT NOT NULL REFERENCES agents(name),
    trigger_message_id INTEGER,
    trigger_event TEXT NOT NULL,
    trigger_depth INTEGER NOT NULL DEFAULT 0,
    trigger_from TEXT,
    status TEXT NOT NULL DEFAULT 'queued',
    k8s_job_name TEXT,
    k8s_namespace TEXT,
    started_at DATETIME,
    completed_at DATETIME,
    duration_ms INTEGER,
    error_log TEXT,
    token_cost_json TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_reactive_runs_agent_created ON reactive_runs(agent_name, created_at);
CREATE INDEX idx_reactive_runs_status ON reactive_runs(status);
CREATE INDEX idx_reactive_runs_agent_status ON reactive_runs(agent_name, status);
```

## k8s_env_json Format

```json
{
  "AGENT_GIT_REPO": "Dumbris/agent-research-mcpproxy",
  "AGENT_PROMPT": "You are research-mcpproxy...",
  "MCPPROXY_URL": "http://kubic.home.arpa:30080",
  "SYNAPBUS_API_KEY": {
    "secretRef": "synapbus-agent-keys",
    "key": "RESEARCH_MCPPROXY_API_KEY"
  }
}
```

Plain string values become `env[].value`. Objects with `secretRef` become `env[].valueFrom.secretKeyRef`.
