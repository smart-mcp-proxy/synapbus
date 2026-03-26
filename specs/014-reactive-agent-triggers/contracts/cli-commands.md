# CLI Command Contracts: Reactive Agent Triggering

## Agent Trigger Configuration

### synapbus agent set-triggers

Configure reactive trigger settings for an agent.

```bash
synapbus agent set-triggers <agent-name> [flags]
```

**Flags**:
| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--mode` | string | - | Trigger mode: `passive`, `reactive`, `disabled` |
| `--cooldown` | int | 600 | Cooldown seconds between runs |
| `--daily-budget` | int | 8 | Max runs per UTC day |
| `--max-depth` | int | 5 | Max cascade depth |

**Example**:
```bash
synapbus agent set-triggers research-mcpproxy \
  --mode reactive --cooldown 600 --daily-budget 8 --max-depth 5
```

**Output**:
```
Updated trigger config for research-mcpproxy:
  mode:         reactive
  cooldown:     600s
  daily budget: 8
  max depth:    5
```

### synapbus agent set-image

Set the K8s container image and env vars for reactive runs.

```bash
synapbus agent set-image <agent-name> [flags]
```

**Flags**:
| Flag | Type | Description |
|------|------|-------------|
| `--image` | string | Container image (required) |
| `--env` | string[] | Plain env var: KEY=VALUE (repeatable) |
| `--secret-env` | string[] | Secret ref: KEY=secret-name:key-name (repeatable) |
| `--resource-preset` | string | `default` or `large` |

**Example**:
```bash
synapbus agent set-image research-mcpproxy \
  --image localhost:32000/universal-agent:latest \
  --env AGENT_GIT_REPO=Dumbris/agent-research-mcpproxy \
  --secret-env SYNAPBUS_API_KEY=synapbus-agent-keys:RESEARCH_MCPPROXY_API_KEY \
  --resource-preset default
```

## Run Management

### synapbus runs list

List recent reactive runs.

```bash
synapbus runs list [flags]
```

**Flags**:
| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--agent` | string | - | Filter by agent name |
| `--status` | string | - | Filter by status |
| `--limit` | int | 20 | Max results |

**Output**:
```
ID   AGENT                STATUS     TRIGGER          DURATION  CREATED
1    research-mcpproxy    succeeded  DM from algis    3m42s     2026-03-25 10:00
2    social-commenter     failed     @mention in #news 0m45s    2026-03-25 10:15
3    research-synapbus    running    DM from algis    -         2026-03-25 10:30
```

### synapbus runs logs

View error logs for a specific run.

```bash
synapbus runs logs <run-id>
```

**Output**: Last 100 lines of pod logs for the run.

### synapbus runs retry

Retry a failed run.

```bash
synapbus runs retry <run-id>
```

**Output**:
```
Retrying run 2 for social-commenter...
New run ID: 4, status: running
```
