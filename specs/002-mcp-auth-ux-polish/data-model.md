# Data Model: MCP Auth, UX Polish & Agent Lifecycle

**Date**: 2026-03-14
**Feature**: 002-mcp-auth-ux-polish

## Entity Changes

### New: Dead Letter

Captures unread messages for deleted agents, owned by the agent's human owner.

| Field | Type | Constraints | Description |
|-------|------|-------------|-------------|
| id | integer | PK, auto-increment | Unique identifier |
| owner_id | integer | FK → users.id, NOT NULL | Owner who deleted the agent |
| original_message_id | integer | NOT NULL | Reference to original message ID |
| to_agent | text | NOT NULL | Name of the deleted agent (preserved as text, not FK) |
| from_agent | text | NOT NULL | Original sender agent name |
| body | text | NOT NULL | Message body |
| subject | text | | Original subject |
| priority | integer | DEFAULT 5 | Original priority (1-10) |
| metadata | text | | Original metadata JSON |
| acknowledged | integer | DEFAULT 0 | 0=active, 1=acknowledged |
| created_at | datetime | DEFAULT CURRENT_TIMESTAMP | When dead letter was created |

**Indexes**:
- `idx_dead_letters_owner` on (owner_id, acknowledged)
- `idx_dead_letters_agent` on (to_agent)

### Modified: Channels

Add `is_system` flag to prevent deletion of auto-created channels.

| New Field | Type | Constraints | Description |
|-----------|------|-------------|-------------|
| is_system | integer | DEFAULT 0 | 1=system channel (cannot be deleted/left by owner) |

### Modified: OAuth Session Data

The `session_data` JSON field in `oauth_tokens` already exists. The structure is extended to include agent identity:

```json
{
  "user_id": 1,
  "username": "alice",
  "subject": "1",
  "agent_name": "alice-bot"
}
```

No schema change needed — `session_data` is already a JSON text field.

## State Transitions

### Dead Letter Lifecycle

```
Message (pending/processing) → [Agent Deleted] → Dead Letter (active)
Dead Letter (active) → [Owner Acknowledges] → Dead Letter (acknowledged)
```

### OAuth Agent Selection Flow

```
MCP Client (no auth) → 401 + metadata URL
  → Browser opens authorize URL
  → User logs in (if needed)
  → User selects agent from dropdown
  → Authorization code issued (with agent in session)
  → Code exchanged for token (agent bound to token)
  → MCP Client uses token → Authenticated as selected agent
```

### My Agents Channel Lifecycle

```
User registers → [Login] → my-agents-{username} channel created (if not exists)
  → Human agent set as owner
User creates agent → Agent auto-joins my-agents channel
User deletes agent → Agent removed from my-agents channel (+ dead letter capture)
```

## Relationships

```
users (1) ──── (N) agents
  │                 │
  │                 ├── (N) dead_letters.to_agent (preserved name)
  │                 └── (N) channel_members
  │
  ├── (1) dead_letters.owner_id
  └── (1) channels (my-agents-{username}, is_system=1)
```

## Migration: 008_dead_letters.sql

```sql
-- Dead letter queue for unread messages of deleted agents
CREATE TABLE IF NOT EXISTS dead_letters (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    owner_id INTEGER NOT NULL REFERENCES users(id),
    original_message_id INTEGER NOT NULL,
    to_agent TEXT NOT NULL,
    from_agent TEXT NOT NULL,
    body TEXT NOT NULL,
    subject TEXT DEFAULT '',
    priority INTEGER DEFAULT 5,
    metadata TEXT DEFAULT '',
    acknowledged INTEGER DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_dead_letters_owner ON dead_letters(owner_id, acknowledged);
CREATE INDEX IF NOT EXISTS idx_dead_letters_agent ON dead_letters(to_agent);

-- Add is_system flag to channels
ALTER TABLE channels ADD COLUMN is_system INTEGER DEFAULT 0;
```
