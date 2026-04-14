#!/bin/bash
# start.sh — launch an isolated synapbus instance and build the
# docgardener demo driver. Mirrors cold-topic-explainer layout.
#
# Exit codes:
#   0  everything came up
#   1  synapbus failed to start
#   2  admin socket never appeared
#   3  preflight failed

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

PORT="${SYNAPBUS_PORT:-18089}"
DATA_DIR="$SCRIPT_DIR/data"
BIN_DIR="$SCRIPT_DIR/bin"
BIN="$BIN_DIR/synapbus"
DOCGARDENER="$BIN_DIR/docgardener"
SOCKET="$DATA_DIR/synapbus.sock"
PID_FILE="$SCRIPT_DIR/.synapbus.pid"
LOG_FILE="$SCRIPT_DIR/synapbus.log"

cd "$SCRIPT_DIR"

say() { printf '\033[1;36m[start]\033[0m %s\n' "$*"; }
die() { printf '\033[1;31m[start][FAIL]\033[0m %s\n' "$*" >&2; exit "${2:-1}"; }

# --- preflight ---------------------------------------------------------
for cmd in go sqlite3 curl; do
    command -v "$cmd" >/dev/null || die "missing required CLI: $cmd" 3
done

if [ -f "$PID_FILE" ] && kill -0 "$(cat "$PID_FILE")" 2>/dev/null; then
    die "synapbus already running (pid $(cat "$PID_FILE")); run ./stop.sh first"
fi

# --- build -------------------------------------------------------------
say "building synapbus + docgardener binaries..."
mkdir -p "$BIN_DIR"

# The Web UI is embedded via go:embed at compile time. If
# internal/web/dist is missing or stale the binary will serve an
# old/empty SPA (symptom: channel messages UI stuck on loading
# skeletons). Rebuild it from web/build whenever the source is newer
# than the embedded copy, or if the embedded copy is missing.
DIST_DIR="$REPO_ROOT/internal/web/dist"
WEB_SRC="$REPO_ROOT/web/build"
if [ ! -d "$DIST_DIR/_app" ] || [ -z "$(ls -A "$DIST_DIR" 2>/dev/null)" ]; then
    say "embedded web dist missing — running 'npm run build' and syncing"
    if command -v npm >/dev/null && [ -d "$REPO_ROOT/web/node_modules" ]; then
        (cd "$REPO_ROOT/web" && npm run build >/dev/null 2>&1) || true
    fi
    if [ -d "$WEB_SRC/_app" ]; then
        rm -rf "$DIST_DIR"
        mkdir -p "$DIST_DIR"
        cp -r "$WEB_SRC/"* "$DIST_DIR/"
    else
        say "WARNING: $WEB_SRC not found — embedded web dist may be stale"
    fi
fi

(cd "$REPO_ROOT" && CGO_ENABLED=0 go build -o "$BIN" ./cmd/synapbus)
(cd "$REPO_ROOT" && CGO_ENABLED=0 go build -o "$DOCGARDENER" ./cmd/docgardener)

# --- fresh data dir ----------------------------------------------------
say "wiping data dir $DATA_DIR"
rm -rf "$DATA_DIR"
mkdir -p "$DATA_DIR"

# --- launch synapbus ---------------------------------------------------
say "starting synapbus on port $PORT"
# Disable the legacy background workers that hold the single-connection
# write pool long enough to wedge interactive sessions. They manage
# features (task-auction expiry, message retention, stalemate reminders)
# that the doc-gardener demo doesn't use, so skipping them here is safe.
export SYNAPBUS_DISABLE_EXPIRY_WORKER=1
export SYNAPBUS_DISABLE_RETENTION_WORKER=1
export SYNAPBUS_DISABLE_STALEMATE_WORKER=1
nohup "$BIN" serve \
    --port "$PORT" \
    --data "$DATA_DIR" \
    > "$LOG_FILE" 2>&1 &
echo $! > "$PID_FILE"
say "pid $(cat "$PID_FILE") → $LOG_FILE"

# Wait for the admin socket + HTTP to appear.
for i in $(seq 1 100); do
    if [ -S "$SOCKET" ]; then break; fi
    if ! kill -0 "$(cat "$PID_FILE")" 2>/dev/null; then
        die "synapbus crashed during boot — see $LOG_FILE" 1
    fi
    sleep 0.1
done
if [ ! -S "$SOCKET" ]; then
    die "admin socket $SOCKET never appeared after 10s" 2
fi
for i in $(seq 1 100); do
    if curl -fsS "http://localhost:$PORT/health" >/dev/null 2>&1; then break; fi
    sleep 0.1
done

say "synapbus is up"

# --- shorthand for admin calls -----------------------------------------
admin() { "$BIN" --socket "$SOCKET" "$@"; }

# --- user + human agent ------------------------------------------------
say "creating user algis / algis-demo-pw"
admin user create --username algis --password 'algis-demo-pw' --display-name Algis >/dev/null 2>&1 || true

OWNER_ID=$(sqlite3 "$DATA_DIR/synapbus.db" "SELECT id FROM users WHERE username='algis'")
if [ -z "$OWNER_ID" ] || [ "$OWNER_ID" = "1" ]; then
    die "failed to resolve algis user id (got '$OWNER_ID')" 3
fi
say "algis user id = $OWNER_ID"

admin agent create --name algis --display-name "Algis (human)" --type human --owner "$OWNER_ID" >/dev/null 2>&1 || true

# --- coordinator agent -------------------------------------------------
# The coordinator is reactive and runs via the subprocess harness as
# `docgardener agent`. The reactor sets up the workdir with message.json
# and env vars; docgardener reads SYNAPBUS_AGENT to dispatch.
say "creating coordinator agent doc-gardener-coordinator"
admin agent create \
    --name doc-gardener-coordinator \
    --display-name "Doc-gardener Coordinator" \
    --type ai \
    --owner "$OWNER_ID" >/dev/null 2>&1 || true

# Configure reactive trigger mode, harness_name, local_command,
# harness_config_json, and feature-018 trust columns (config_hash,
# system_prompt, autonomy_tier, tool_scope_json) via sqlite3 — the
# admin CLI doesn't expose the new columns yet.
DG_ABS="$(cd "$SCRIPT_DIR" && pwd)/bin/docgardener"
DB_ABS="$DATA_DIR/synapbus.db"
LOCAL_CMD_JSON="[\"$DG_ABS\",\"agent\",\"--db\",\"$DB_ABS\"]"
# SYNAPBUS_BIN + SYNAPBUS_SOCKET let the coordinator shell out to the
# admin CLI for follow-up DMs (the real MessagingService.Send path
# that fires the reactor dispatcher). SYNAPBUS_AGENT tells docgardener
# which role it's running as.
HARNESS_CFG_JSON="{\"env\":{\"SYNAPBUS_AGENT\":\"doc-gardener-coordinator\",\"SYNAPBUS_BIN\":\"$BIN\",\"SYNAPBUS_SOCKET\":\"$SOCKET\"}}"
COORD_PROMPT='You are the doc-gardener coordinator. Your job is to decompose a high-level goal ("keep docs accurate against the source code") into a tree of sub-tasks, propose specialist agents to carry out the leaf tasks, monitor progress via the goal channel, and iterate. You never act on leaf tasks directly. You communicate via SynapBus DMs.'
COORD_TOOL_SCOPE='["messages:read","messages:send","channels:read","reactions:add","goals:create","tasks:propose_tree","agents:propose"]'

sqlite3 "$DATA_DIR/synapbus.db" <<SQL
UPDATE agents SET
    trigger_mode         = 'reactive',
    cooldown_seconds     = 0,
    daily_trigger_budget = 50,
    max_trigger_depth    = 8,
    harness_name         = 'subprocess',
    local_command        = '$LOCAL_CMD_JSON',
    harness_config_json  = '$HARNESS_CFG_JSON',
    system_prompt        = '$COORD_PROMPT',
    autonomy_tier        = 'assisted',
    tool_scope_json      = '$COORD_TOOL_SCOPE',
    config_hash          = '70a9a06e9595ade4edc527a792e857792d17af9819c80850cf53bea7ff3887ef'
WHERE name = 'doc-gardener-coordinator';
SQL

# --- base channels -----------------------------------------------------
say "ensuring approvals and requests channels"
admin channels create --name approvals --type blackboard --description 'Approval queue' >/dev/null 2>&1 || true
admin channels create --name requests  --type blackboard --description 'Resource requests' >/dev/null 2>&1 || true

echo
echo "  Web UI:        http://localhost:$PORT   (login: algis / algis-demo-pw)"
echo "  Log:           tail -f $LOG_FILE"
echo "  Admin socket:  $SOCKET"
echo
echo "Next: ./run_task.sh"
