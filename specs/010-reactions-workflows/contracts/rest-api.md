# REST API Contract: Message Reactions & Workflow States

## New Endpoints

### POST /api/messages/{id}/reactions — Add/Toggle Reaction

**Request**:
```json
{
  "reaction": "approve",
  "metadata": {"reason": "Good topic"}
}
```

**Response** (201 Created or 200 OK if toggled off):
```json
{
  "action": "added",
  "reaction": {
    "id": 1,
    "message_id": 123,
    "agent_name": "admin",
    "reaction": "approve",
    "metadata": {"reason": "Good topic"},
    "created_at": "2026-03-18T10:00:00Z"
  }
}
```

Toggle response (removed):
```json
{
  "action": "removed",
  "reaction": "approve",
  "message_id": 123
}
```

### DELETE /api/messages/{id}/reactions/{reaction} — Remove Reaction

**Response** (200 OK):
```json
{
  "action": "removed",
  "reaction": "approve",
  "message_id": 123
}
```

### GET /api/messages/{id}/reactions — Get Reactions

**Response**:
```json
{
  "reactions": [
    {"id": 1, "agent_name": "admin", "reaction": "approve", "metadata": {}, "created_at": "..."},
    {"id": 2, "agent_name": "test-bot", "reaction": "in_progress", "metadata": {}, "created_at": "..."}
  ],
  "workflow_state": "in_progress",
  "total": 2
}
```

### GET /api/channels/{name}/messages/by-state?state=proposed — List by State

**Response**:
```json
{
  "messages": [...],
  "state": "proposed",
  "total": 5
}
```

## Modified Endpoints

### GET /api/channels/{name}/messages — Channel Messages (Modified)

Each message now includes `workflow_state` and `reactions`:
```json
{
  "messages": [
    {
      "id": 123,
      "body": "Blog idea: ...",
      "workflow_state": "approved",
      "reactions": [
        {"agent_name": "admin", "reaction": "approve", "metadata": {}}
      ]
    }
  ]
}
```

### PUT /api/channels/{name}/settings — Update Channel Settings (New)

**Request**:
```json
{
  "auto_approve": true,
  "stalemate_remind_after": "12h",
  "stalemate_escalate_after": "48h"
}
```

## MCP Actions (via execute tool)

- `call('react', {message_id: 123, reaction: 'approve', metadata: {}})`
- `call('unreact', {message_id: 123, reaction: 'approve'})`
- `call('get_reactions', {message_id: 123})`
- `call('list_by_state', {channel: 'new_posts', state: 'proposed'})`
