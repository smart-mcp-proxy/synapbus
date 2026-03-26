# REST API Contracts: Reactive Agent Triggering

**Note**: REST API is for the embedded Web UI only (Constitution Principle II). Agents use MCP tools.

## Endpoints

### GET /api/runs

List reactive runs with optional filters.

**Query Parameters**:
| Param | Type | Required | Description |
|-------|------|----------|-------------|
| `agent` | string | No | Filter by agent name |
| `status` | string | No | Filter by status (comma-separated) |
| `limit` | int | No | Max results (default: 50, max: 200) |
| `offset` | int | No | Pagination offset |

**Response** (200):
```json
{
  "runs": [
    {
      "id": 1,
      "agent_name": "research-mcpproxy",
      "trigger_message_id": 12345,
      "trigger_event": "message.received",
      "trigger_depth": 0,
      "trigger_from": "algis",
      "status": "succeeded",
      "k8s_job_name": "reactive-research-mcpproxy-1",
      "started_at": "2026-03-25T10:00:00Z",
      "completed_at": "2026-03-25T10:03:42Z",
      "duration_ms": 222000,
      "error_log": null,
      "created_at": "2026-03-25T10:00:00Z"
    }
  ],
  "total": 42
}
```

### GET /api/runs/:id

Get a single run with full details including error log.

**Response** (200): Single run object (same as above).

### POST /api/runs/:id/retry

Retry a failed run. Creates a new trigger evaluation for the same agent.

**Response** (200):
```json
{
  "new_run_id": 43,
  "status": "running"
}
```

**Response** (429 — rate limited):
```json
{
  "error": "cooldown_active",
  "cooldown_remaining_seconds": 342
}
```

### GET /api/agents/reactive

List agents with reactive trigger configuration and current status.

**Response** (200):
```json
{
  "agents": [
    {
      "name": "research-mcpproxy",
      "trigger_mode": "reactive",
      "cooldown_seconds": 600,
      "daily_trigger_budget": 8,
      "max_trigger_depth": 5,
      "k8s_image": "localhost:32000/universal-agent:latest",
      "pending_work": false,
      "state": "idle",
      "today_runs": 3,
      "cooldown_until": null
    }
  ]
}
```

**`state` values**: `idle`, `running`, `queued` (pending_work set), `cooldown`, `budget_exhausted`
