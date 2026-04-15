#!/bin/bash
# run_task.sh — send a doc-verification goal DM from algis to
# doc-coordinator and wait for the FINAL: reply that flows back from
# docs-critic. The whole flow is driven by MCP tool calls inside three
# Docker-isolated agent containers — nothing here writes to the DB
# directly.
#
# Usage:
#   ./run_task.sh                                 # default doc-gardener brief
#   ./run_task.sh "your custom goal here"
#
# The default brief asks the inspector to verify mcpproxy CLI flag
# documentation against the actual binary. Override with any free-form
# brief — the coordinator triages it.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BIN="$SCRIPT_DIR/bin/synapbus"
SOCKET="$SCRIPT_DIR/data/synapbus.sock"

DEFAULT_GOAL='Verify the CLI commands listed on https://docs.mcpproxy.app/cli/command-reference still exist in the current mcpproxy binary. Install mcpproxy in the sandbox first (releases at https://github.com/smart-mcp-proxy/mcpproxy-go/releases — pick the linux-arm64 or linux-amd64 variant matching `uname -m`). For each documented command, check whether `mcpproxy --help` and `mcpproxy <command> --help` show it; flag any drift, missing commands, or doc claims that no longer match. Produce a patch suggestion list.'

GOAL="${1:-$DEFAULT_GOAL}"

say() { printf '\033[1;36m[run]\033[0m %s\n' "$*"; }
die() { printf '\033[1;31m[run][FAIL]\033[0m %s\n' "$*" >&2; exit 1; }

[ -x "$BIN" ] || die "synapbus binary not found at $BIN — run ./start.sh first"
[ -S "$SOCKET" ] || die "admin socket missing — is synapbus running?"

cd "$SCRIPT_DIR"
DB="$SCRIPT_DIR/data/synapbus.db"

# Snapshot the current max message id so we only look at replies from
# THIS run, not stale replies left from previous invocations.
BASELINE=$(sqlite3 "$DB" "SELECT COALESCE(MAX(id), 0) FROM messages" 2>/dev/null || echo 0)

say "sending goal DM: algis → doc-coordinator (baseline msg_id=$BASELINE)"
printf '%s' "$GOAL" | "$BIN" --socket "$SOCKET" messages send \
    --from algis \
    --to doc-coordinator \
    --priority 8 >&2

say "waiting for goal completion or FINAL:/CANNOT: reply to algis (up to 600s)..."
deadline=$(( $(date +%s) + 600 ))
last_seen_id=$BASELINE

while [ "$(date +%s)" -lt "$deadline" ]; do
    # Goal completion check (definitive signal — set by complete_goal MCP).
    # A goal in 'completed'/'stuck'/'cancelled' state with a
    # completion_summary means the critic finalized the verdict.
    COMPLETED=$(sqlite3 "$DB" "
        SELECT id FROM goals
        WHERE status IN ('completed','stuck','cancelled')
          AND completion_summary IS NOT NULL
        ORDER BY id DESC LIMIT 1
    " 2>/dev/null || true)
    if [ -n "$COMPLETED" ]; then
        say "goal $COMPLETED reached terminal state"
        GOAL_SUMMARY=$(sqlite3 "$DB" "SELECT status || ': ' || COALESCE(completion_summary,'') FROM goals WHERE id = $COMPLETED" 2>/dev/null)
        say "$GOAL_SUMMARY"
        echo "$COMPLETED" > "$SCRIPT_DIR/.last_goal_id"
        say "goal id = $COMPLETED — render with ./report.sh"
        exit 0
    fi

    # Message-based fallback (for TRIVIAL/CANNOT paths that skip the
    # task tree and never call complete_goal).
    NEW_LINES=$(sqlite3 -separator '|' "$DB" "
        SELECT id, from_agent, replace(substr(body, 1, 280), char(10), ' ')
        FROM messages
        WHERE to_agent = 'algis'
          AND from_agent != 'algis'
          AND id > $last_seen_id
        ORDER BY id ASC
    " 2>/dev/null || true)

    if [ -n "$NEW_LINES" ]; then
        while IFS='|' read -r id from body; do
            [ -z "$id" ] && continue
            say "← [$from #$id] $body"
            last_seen_id=$id
            case "$body" in
                DELEGATED:*|REVISING:*|Received\ system\ trigger*|Coalesced\ trigger*)
                    ;;  # informational, keep waiting
                FINAL:*|CANNOT:*)
                    say "terminal response received"
                    GOAL_ID=$(sqlite3 "$DB" 'SELECT id FROM goals ORDER BY id DESC LIMIT 1' 2>/dev/null || echo)
                    if [ -n "$GOAL_ID" ]; then
                        echo "$GOAL_ID" > "$SCRIPT_DIR/.last_goal_id"
                        say "goal id = $GOAL_ID — render with ./report.sh"
                    fi
                    exit 0
                    ;;
                *)
                    # A bare reply from doc-coordinator with no status
                    # prefix is a TRIVIAL-triage direct answer. Count
                    # it as terminal only if no goal was created (i.e.
                    # the coordinator didn't start a pipeline).
                    if [ "$from" = "doc-coordinator" ]; then
                        HAS_GOAL=$(sqlite3 "$DB" 'SELECT COUNT(*) FROM goals' 2>/dev/null || echo 0)
                        if [ "$HAS_GOAL" = "0" ]; then
                            say "direct (trivial) response received"
                            exit 0
                        fi
                        # Otherwise keep waiting — the coordinator
                        # already delegated and will finalize via
                        # complete_goal once the critic runs.
                    fi
                    ;;
            esac
        done <<EOF
$NEW_LINES
EOF
    fi
    sleep 1
done

say "timed out waiting for terminal response"
say "check http://localhost:18089/runs and http://localhost:18089/goals"
exit 2
