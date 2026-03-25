# MCP Tool Contracts: Reactive Agent Triggering

**Note**: These are owner-only admin tools, not agent-callable tools (Constitution Principle IV).

## configure_triggers

Configure reactive trigger settings for an agent.

**Parameters**:
```json
{
  "agent_name": "research-mcpproxy",
  "trigger_mode": "reactive",
  "cooldown_seconds": 600,
  "daily_trigger_budget": 8,
  "max_trigger_depth": 5
}
```

All fields except `agent_name` are optional — only provided fields are updated.

**Returns**:
```json
{
  "status": "ok",
  "agent_name": "research-mcpproxy",
  "trigger_mode": "reactive",
  "cooldown_seconds": 600,
  "daily_trigger_budget": 8,
  "max_trigger_depth": 5
}
```

## set_agent_image

Set the K8s container image and environment variables for reactive runs.

**Parameters**:
```json
{
  "agent_name": "research-mcpproxy",
  "k8s_image": "localhost:32000/universal-agent:latest",
  "k8s_env_json": {
    "AGENT_GIT_REPO": "Dumbris/agent-research-mcpproxy",
    "SYNAPBUS_API_KEY": {"secretRef": "synapbus-agent-keys", "key": "RESEARCH_MCPPROXY_API_KEY"}
  },
  "k8s_resource_preset": "default"
}
```

**Returns**:
```json
{
  "status": "ok",
  "agent_name": "research-mcpproxy",
  "k8s_image": "localhost:32000/universal-agent:latest"
}
```

## list_runs

List recent reactive runs for an agent.

**Parameters**:
```json
{
  "agent_name": "research-mcpproxy",
  "status": "failed",
  "limit": 20
}
```

All fields optional. Without `agent_name`, lists all agents' runs.

**Returns**:
```json
{
  "runs": [
    {
      "id": 1,
      "agent_name": "research-mcpproxy",
      "trigger_event": "message.received",
      "trigger_from": "algis",
      "status": "failed",
      "duration_ms": 45000,
      "error_log": "Exit code 1 — OOMKilled...",
      "created_at": "2026-03-25T10:00:00Z"
    }
  ]
}
```

## get_run_logs

Get full error log for a specific run.

**Parameters**:
```json
{
  "run_id": 1
}
```

**Returns**:
```json
{
  "run_id": 1,
  "agent_name": "research-mcpproxy",
  "status": "failed",
  "error_log": "... last 100 lines of pod logs ..."
}
```

## retry_run

Retry a failed run.

**Parameters**:
```json
{
  "run_id": 1
}
```

**Returns**:
```json
{
  "new_run_id": 43,
  "status": "running"
}
```
