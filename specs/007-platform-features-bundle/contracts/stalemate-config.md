# StalemateWorker Configuration Contract

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| SYNAPBUS_STALEMATE_PROCESSING_TIMEOUT | 24h | Auto-fail processing DMs older than this |
| SYNAPBUS_STALEMATE_REMINDER_AFTER | 4h | Send reminder for pending DMs older than this |
| SYNAPBUS_STALEMATE_ESCALATE_AFTER | 48h | Escalate pending DMs to #approvals after this |
| SYNAPBUS_STALEMATE_INTERVAL | 15m | How often the worker checks (minimum 1m) |

## Duration Format

Supports Go `time.ParseDuration` format: `24h`, `30m`, `1h30m`, etc.
Also supports day suffix: `7d` = 168h.

## Behavior

1. Worker runs every INTERVAL (default 15m)
2. Queries DMs only (`to_agent IS NOT NULL`)
3. Skips messages where `from_agent = 'system'` or `to_agent = 'system'`
4. For processing messages: checks `claimed_at + PROCESSING_TIMEOUT < now()`
5. For pending messages: checks `created_at + REMINDER_AFTER < now()` (reminder) and `created_at + ESCALATE_AFTER < now()` (escalation)
6. Avoids duplicate reminders by checking if a system reminder for this message already exists
7. Escalation message includes: original sender, original body (truncated), age, target agent
