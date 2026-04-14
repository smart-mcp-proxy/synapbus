#!/bin/bash
# run_task.sh — kick off the real multi-agent doc-gardener demo.
#
# Sends a single DM from algis to doc-gardener-coordinator. The
# reactor picks it up, fires a subprocess running `docgardener agent`,
# which creates the goal, materializes the task tree, spawns the 3
# specialists (dynamically — they didn't exist before this run), and
# DMs each to claim its task. Every specialist fires its own reactor
# run, does real subprocess work, posts artifacts, and DMs the
# coordinator back. The coordinator's completion handler waits until
# all tasks resolve, then DMs algis with FINAL: summary.
#
# Nothing in this script touches the DB directly — the whole demo is
# driven by SynapBus messaging + the reactor.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BIN="$SCRIPT_DIR/bin/synapbus"
SOCKET="$SCRIPT_DIR/data/synapbus.sock"

cd "$SCRIPT_DIR"

say() { printf '\033[1;36m[run]\033[0m %s\n' "$*"; }
die() { printf '\033[1;31m[run][FAIL]\033[0m %s\n' "$*" >&2; exit 1; }

if [ ! -x "$BIN" ]; then
    die "synapbus binary not found at $BIN — run ./start.sh first"
fi
if [ ! -S "$SOCKET" ]; then
    die "admin socket not found at $SOCKET — is synapbus running?"
fi

GOAL_DESC='Verify every CLI flag and config option mentioned in docs.mcpproxy.app actually exists in the mcpproxy binary, flag any drift, and propose doc patches.'

say "sending kickoff DM: algis → doc-gardener-coordinator"
printf '%s' "$GOAL_DESC" | "$BIN" --socket "$SOCKET" messages send \
    --from algis \
    --to doc-gardener-coordinator \
    --priority 8 >&2

say "waiting for coordinator's FINAL: reply to algis (up to 120s)..."
deadline=$(( $(date +%s) + 120 ))
while [ $(date +%s) -lt $deadline ]; do
    FINAL=$("$BIN" --socket "$SOCKET" messages list --to algis --limit 10 2>/dev/null \
        | awk '/^FINAL: / {print; exit}')
    if [ -n "${FINAL:-}" ]; then
        say "coordinator reported: $FINAL"
        break
    fi
    sleep 1
done

# Find the latest goal id for the report step.
GOAL_ID=$(sqlite3 "$SCRIPT_DIR/data/synapbus.db" 'SELECT id FROM goals ORDER BY id DESC LIMIT 1')
if [ -n "$GOAL_ID" ]; then
    echo "$GOAL_ID" > "$SCRIPT_DIR/.last_goal_id"
    say "goal id = $GOAL_ID"
fi

say "done. render the report with:"
echo "  ./report.sh"
echo
say "Agent Runs:  http://localhost:18089/runs"
say "Agents:      http://localhost:18089/agents"
