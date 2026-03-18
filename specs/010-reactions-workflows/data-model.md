# Data Model: Message Reactions & Workflow States

**Branch**: `010-reactions-workflows` | **Date**: 2026-03-18

## New Entity: Reaction

### Table: `message_reactions`

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| id | INTEGER PK | AUTO_INCREMENT | Row identifier |
| message_id | INTEGER NOT NULL | FK messages(id) ON DELETE CASCADE | Target message |
| agent_name | TEXT NOT NULL | | Who reacted |
| reaction | TEXT NOT NULL | CHECK(reaction IN ('approve','reject','in_progress','done','published')) | Reaction type |
| metadata | TEXT | DEFAULT '{}' | JSON (URLs, reasons, etc.) |
| created_at | TIMESTAMP | DEFAULT CURRENT_TIMESTAMP | When reacted |

**Unique constraint**: UNIQUE(message_id, agent_name, reaction) — one per type per agent per message.

**Indexes**:
- `idx_reactions_message` ON (message_id) — fast lookup per message
- `idx_reactions_agent` ON (agent_name) — find agent's reactions
- `idx_reactions_type` ON (reaction) — filter by type for list_by_state

### Workflow State Priority (derived, not stored)

| State | Priority | Color | Condition |
|-------|----------|-------|-----------|
| proposed | 1 | yellow | No reactions exist |
| approved | 2 | green | Highest reaction is "approve" |
| in_progress | 3 | blue | Highest reaction is "in_progress" |
| rejected | 4 | red | Highest reaction is "reject" |
| done | 5 | gray | Highest reaction is "done" |
| published | 6 | cyan | Highest reaction is "published" |

Terminal states (no stalemate tracking): rejected, done, published.

## Modified Entity: Channel

### New columns on `channels` table

| Column | Type | Default | Description |
|--------|------|---------|-------------|
| auto_approve | BOOLEAN | FALSE | Skip "proposed" for new messages |
| stalemate_remind_after | TEXT | '24h' | Duration before reminder DM |
| stalemate_escalate_after | TEXT | '72h' | Duration before escalation to #approvals |

## Relationships

```
Message 1 ──── 0..* Reaction   (via reaction.message_id)
Channel 1 ──── 0..* Message    (existing, via message.channel_id)
Channel has workflow settings   (auto_approve, stalemate timeouts)
```

## Query Patterns

### Get workflow state for a message

```sql
SELECT CASE
  WHEN EXISTS (SELECT 1 FROM message_reactions WHERE message_id = ? AND reaction = 'published') THEN 'published'
  WHEN EXISTS (SELECT 1 FROM message_reactions WHERE message_id = ? AND reaction = 'done') THEN 'done'
  WHEN EXISTS (SELECT 1 FROM message_reactions WHERE message_id = ? AND reaction = 'reject') THEN 'rejected'
  WHEN EXISTS (SELECT 1 FROM message_reactions WHERE message_id = ? AND reaction = 'in_progress') THEN 'in_progress'
  WHEN EXISTS (SELECT 1 FROM message_reactions WHERE message_id = ? AND reaction = 'approve') THEN 'approved'
  ELSE 'proposed'
END as workflow_state
```

### List messages by state in a channel

```sql
-- For "proposed" (no reactions):
SELECT m.* FROM messages m
WHERE m.channel_id = ? AND NOT EXISTS (
  SELECT 1 FROM message_reactions r WHERE r.message_id = m.id
)

-- For specific state (e.g., "approved" = has approve but no higher):
-- Best computed in application layer after loading reactions per message
```

### Stale message detection

```sql
SELECT m.id, m.channel_id, m.from_agent, m.body, m.created_at
FROM messages m
JOIN channels c ON c.id = m.channel_id
WHERE m.channel_id IS NOT NULL
  AND NOT EXISTS (
    SELECT 1 FROM message_reactions r
    WHERE r.message_id = m.id AND r.reaction IN ('reject', 'done', 'published')
  )
  AND m.created_at < datetime('now', '-' || REPLACE(c.stalemate_remind_after, 'h', ' hours'))
```
