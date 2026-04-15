# doc-gardener — docker-isolated doc verification demo

A real, working multi-agent example that:

1. Takes a goal like *"Verify the CLI commands on docs.mcpproxy.app/cli/command-reference still exist in the current mcpproxy binary"*.
2. Routes it through `doc-coordinator`, which calls SynapBus MCP tools (`create_goal`, `propose_task_tree`, `send_message`) to record the goal and dispatch work.
3. Spawns `docs-inspector` inside an **isolated Docker container** to actually `curl` the docs, install/run `mcpproxy`, parse output, and tabulate drift.
4. Forwards the findings to `docs-critic` — a separate container with its own MCP key — for an independent audit.
5. Returns a `FINAL:` summary back to the human.

Every agent runs in its own ephemeral container with `--cap-drop=ALL`, `--security-opt=no-new-privileges`, `--read-only` root + tmpfs `/tmp`, `--pids-limit`, memory + CPU quotas, and `--user` set to your host UID. The container can reach the SynapBus MCP server on the host at `host.docker.internal:18089` but nothing else of yours unless you mount it in.

## Architecture

```
algis ──DM──▶ doc-coordinator      (Gemini Pro, container)
                  │
                  │  MCP tools: create_goal, propose_task_tree, send_message
                  ▼
              ┌── reply ──▶ algis                                (TRIVIAL)
              ├── refuse ─▶ algis  (CANNOT: …)                   (INFEASIBLE)
              └── delegate ──▶ docs-inspector  (Gemini Flash, container)
                                   │
                                   │  shell tools: curl, jq, mcpproxy …
                                   │  MCP: send_message
                                   ▼
                               docs-critic    (Gemini Flash, container)
                                   │
                                   │  spot-checks evidence; MCP: send_message
                                   ▼
                               algis (FINAL: … or REVISING: …)
```

Three independent agents, three MCP API keys, three containers. The critic is structurally separate from the inspector — it has its own `config_hash` and reputation, and reads only the inspector's findings JSON, not its reasoning trace.

## What's actually real (not synthetic)

| Piece | Status |
|---|---|
| Three Docker-isolated agent containers (`--cap-drop=ALL`, read-only root, pids/mem/cpu limits) | ✅ |
| MCP-native dispatch — every agent calls `send_message` directly via Gemini's MCP client | ✅ |
| `create_goal` + `propose_task_tree` materialize real rows in `goals` / `goal_tasks` | ✅ |
| Inspector has shell access inside the sandbox to fetch docs and run CLIs | ✅ |
| Coordinator/inspector/critic each get their own SynapBus API key | ✅ |
| Trust model (`config_hash`, delegation cap, reputation ledger) | ✅ (covered by `internal/trust/` tests) |
| Atomic task claim, cost rollup via recursive CTE | ✅ (covered by `internal/goaltasks/` tests) |
| Rich HTML report (goal tree / agents / spend / timeline) | ✅ via `./report.sh` |
| Secret encryption + scoped env injection | ✅ via `internal/secrets/` |
| Svelte `/goals` UI | ✅ at `http://localhost:18089/goals` |

## Prerequisites

- Docker daemon running (`docker version` works)
- `go`, `jq`, `sqlite3`, `curl` on PATH
- A Gemini API key from <https://aistudio.google.com/apikey>:
  ```bash
  export GEMINI_API_KEY=...
  ```

The first `./start.sh` builds the canonical `synapbus-agent` image (`image-build/synapbus-agent/Dockerfile`) — Debian slim + Node 22 + `gemini`, `claude`, `jq`, `sqlite3`, `curl`, `git`, `python3`, `tini`. ~2-5 minutes the first time, cached afterwards.

## Run

```bash
export GEMINI_API_KEY=...

./start.sh                                  # builds binary + image, provisions agents
./run_task.sh                               # default brief: verify mcpproxy CLI flags
./run_task.sh "what does this demo do?"     # TRIVIAL path — coordinator answers directly
./run_task.sh "Transfer money from my bank" # INFEASIBLE — coordinator refuses
./report.sh                                 # render rich HTML report
./stop.sh
```

Web UI at `http://localhost:18089` (login `algis` / `algis-demo-pw`):

- `/runs` — every reactive harness run, captured prompts + responses, exit codes, durations
- `/goals` — goal tree + task state + spend per billing code
- `/agents` — three agents, each with its own `config_hash` and reputation
- `/dm/algis` — DM thread with `doc-coordinator`

## How it isolates

The `docker` block in each `configs/*.json` is what makes this happen:

```json
{
  "docker": {
    "image": "synapbus-agent:latest",
    "memory": "1g",
    "cpus": "1.0",
    "network": "bridge"
  }
}
```

The SynapBus reactor sees the `docker.image` field, picks the `docker` harness backend (via `internal/harness/docker/`), and runs:

```
docker run --rm \
    --workdir /workspace \
    --mount type=bind,source=<run-workdir>,target=/workspace \
    --security-opt no-new-privileges \
    --cap-drop ALL \
    --pids-limit 512 \
    --read-only --tmpfs /tmp:rw,size=64m \
    --memory 1g --memory-swap 1g \
    --cpus 1.0 \
    --network bridge \
    --add-host host.docker.internal:host-gateway \
    --user <host-uid>:<host-gid> \
    --env GEMINI_API_KEY=... \
    --env GEMINI_MODEL=... \
    [other -e flags] \
    synapbus-agent:latest
```

The container's CMD is the standard `/usr/local/bin/synapbus-agent-wrapper.sh` baked into the image — it reads the bind-mounted `message.json`, loads `GEMINI.md`, and invokes `gemini -p` once. Every side effect happens through MCP tool calls inside the Gemini session; the container never reaches the SynapBus admin Unix socket because it doesn't have access to it.

The `.gemini/settings.json` materialized by the harness already points at the host MCP server with the correct API key — the harness rewrites `127.0.0.1` to `host.docker.internal` for docker-backed agents automatically.

## Customize

| Variable | Default | What it does |
|---|---|---|
| `SYNAPBUS_PORT` | `18089` | Host HTTP port |
| `SYNAPBUS_COORDINATOR_MODEL` | `gemini-3.1-pro-preview` | Smart triage model (fall back to `gemini-2.5-pro` if rate-limited) |
| `SYNAPBUS_WORKER_MODEL` | `gemini-2.5-flash` | Fast inspector + critic model |
| `SYNAPBUS_AGENT_IMAGE` | `synapbus-agent:latest` | Container image to run agents in |
| `GEMINI_API_KEY` | (required) | Forwarded to every container as `-e` |

Override per-agent docker resources by editing `configs/*.json`:

- `docker.memory` — `512m`, `1g`, `2g`
- `docker.cpus` — `0.5`, `1.0`, `2.0`
- `docker.network` — `bridge` (default, internet OK), `none` (air-gapped)
- `docker.cap_add` — array of capabilities to grant on top of `--cap-drop=ALL`
- `docker.extra_mounts` — additional read-only host bind mounts
- `docker.read_only_root` — set to `false` if the agent CLI insists on writing outside `/tmp` and `/workspace`

## What got removed

The legacy `cmd/docgardener` Go binary used to contain ~2400 LOC of agent orchestration: a hardcoded 3-task tree, a `runDemo` flow that wrote directly to the DB, per-role subprocess entry points, a Gemini fallback for tree generation, channel bootstrap, etc. All of that is gone — replaced by:

- `configs/coordinator.json` + `configs/inspector.json` + `configs/critic.json` (declarative GEMINI.md + docker block)
- The standard `synapbus-agent-wrapper.sh` baked into the canonical image
- The 6 spec-018 MCP tools that ship with `synapbus serve`

`cmd/docgardener/` now contains only `report.go` + `template.go` + a tiny `main.go` cobra wrapper. The binary's only job is rendering the HTML snapshot you get from `./report.sh`.
