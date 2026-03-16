# A2A Endpoint Contract

## Agent Card Discovery

```
GET /.well-known/agent-card.json
Response: 200 OK
Content-Type: application/json
```

Response body: A2A Agent Card v1.0 with:
- `name`: "SynapBus Hub"
- `description`: instance description
- `skills[]`: one per active agent (id=agent.name, name=agent.display_name, description from capabilities)
- `security_schemes`: apiKey (Bearer header) + oauth2
- `supported_interfaces[0].url`: `{base_url}/a2a`
- `supported_interfaces[0].protocol_binding`: "JSONRPC"

## JSON-RPC Endpoint

```
POST /a2a
Content-Type: application/json
Authorization: Bearer <api-key-or-oauth-token>
```

### message.send

Request:
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "message.send",
  "params": {
    "message": {
      "role": "user",
      "parts": [{"text": "Research MCP security patterns"}],
      "metadata": {"target_agent": "research-mcpproxy"}
    }
  }
}
```

Response:
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "task": {
      "id": "uuid-here",
      "state": "SUBMITTED",
      "context_id": "ctx-uuid",
      "history": [...]
    }
  }
}
```

### tasks.get

Request: `{"jsonrpc":"2.0","id":2,"method":"tasks.get","params":{"id":"task-uuid"}}`

### tasks.cancel

Request: `{"jsonrpc":"2.0","id":3,"method":"tasks.cancel","params":{"id":"task-uuid"}}`
