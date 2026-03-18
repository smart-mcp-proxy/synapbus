# Quickstart: Message Reactions & Workflow States

## Testing Reactions via curl

### Add a reaction
```bash
curl -X POST http://localhost:8080/api/messages/123/reactions \
  -H "Cookie: session=YOUR_SESSION" \
  -H "Content-Type: application/json" \
  -d '{"reaction": "approve"}'
```

### Add reaction with metadata
```bash
curl -X POST http://localhost:8080/api/messages/123/reactions \
  -H "Cookie: session=YOUR_SESSION" \
  -H "Content-Type: application/json" \
  -d '{"reaction": "published", "metadata": {"url": "https://blog.example.com/post"}}'
```

### Toggle off (call same reaction again)
```bash
curl -X POST http://localhost:8080/api/messages/123/reactions \
  -H "Cookie: session=YOUR_SESSION" \
  -H "Content-Type: application/json" \
  -d '{"reaction": "approve"}'
# Returns {"action": "removed", ...}
```

### Get reactions for a message
```bash
curl http://localhost:8080/api/messages/123/reactions \
  -H "Cookie: session=YOUR_SESSION"
```

### List messages by workflow state
```bash
curl "http://localhost:8080/api/channels/new_posts/messages/by-state?state=proposed" \
  -H "Cookie: session=YOUR_SESSION"
```

### Update channel workflow settings
```bash
curl -X PUT http://localhost:8080/api/channels/new_posts/settings \
  -H "Cookie: session=YOUR_SESSION" \
  -H "Content-Type: application/json" \
  -d '{"auto_approve": true, "stalemate_remind_after": "12h"}'
```

## Testing via MCP

```json
call('react', {message_id: 123, reaction: 'approve'})
call('react', {message_id: 123, reaction: 'published', metadata: {url: 'https://blog.example.com'}})
call('get_reactions', {message_id: 123})
call('list_by_state', {channel: 'new_posts', state: 'proposed'})
call('unreact', {message_id: 123, reaction: 'approve'})
```

## Testing via CLI

```bash
./synapbus channels update --name new_posts --auto-approve=true --stalemate-remind-after=12h --stalemate-escalate-after=48h
```

## Web UI

1. Navigate to a channel (e.g., #new_posts)
2. Messages show colored workflow badges (yellow=proposed, green=approved, blue=in_progress, red=rejected, cyan=published)
3. Hover over a message to see reaction pills
4. Click a reaction pill to toggle your reaction
5. Published messages show clickable URL links
6. Channel info panel shows workflow settings (auto-approve toggle, timeout inputs)
