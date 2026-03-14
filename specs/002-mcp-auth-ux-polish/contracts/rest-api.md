# REST API Contracts: MCP Auth, UX Polish & Agent Lifecycle

**Date**: 2026-03-14
**Feature**: 002-mcp-auth-ux-polish

## New Endpoints

### Dead Letters

#### GET /api/dead-letters

List dead letters for the authenticated user.

**Auth**: Session cookie (Web UI)

**Query Parameters**:
| Param | Type | Default | Description |
|-------|------|---------|-------------|
| acknowledged | boolean | false | Include acknowledged dead letters |
| limit | integer | 50 | Max results |

**Response** (200):
```json
{
  "dead_letters": [
    {
      "id": 1,
      "to_agent": "my-bot",
      "from_agent": "other-agent",
      "body": "Hello, are you there?",
      "subject": "Task update",
      "priority": 5,
      "metadata": {},
      "acknowledged": false,
      "created_at": "2026-03-14T10:00:00Z"
    }
  ],
  "total": 1
}
```

#### POST /api/dead-letters/{id}/acknowledge

Mark a dead letter as acknowledged.

**Auth**: Session cookie (Web UI)

**Response** (200):
```json
{ "acknowledged": true }
```

**Error** (404): Dead letter not found or not owned by user.

### OAuth Discovery

#### GET /.well-known/oauth-authorization-server

OAuth 2.0 Authorization Server Metadata (RFC 8414).

**Auth**: None

**Response** (200):
```json
{
  "issuer": "http://localhost:8080",
  "authorization_endpoint": "http://localhost:8080/oauth/authorize",
  "token_endpoint": "http://localhost:8080/oauth/token",
  "token_endpoint_auth_methods_supported": ["none"],
  "response_types_supported": ["code"],
  "grant_types_supported": ["authorization_code", "refresh_token"],
  "code_challenge_methods_supported": ["S256"],
  "scopes_supported": ["mcp"]
}
```

### OAuth Authorization Page

#### GET /oauth/authorize

Server-rendered HTML page for OAuth authorization.

**Query Parameters** (standard OAuth):
| Param | Type | Required | Description |
|-------|------|----------|-------------|
| response_type | string | yes | Must be "code" |
| client_id | string | yes | OAuth client ID |
| redirect_uri | string | yes | Callback URL |
| state | string | yes | CSRF state parameter |
| code_challenge | string | yes | PKCE challenge |
| code_challenge_method | string | yes | Must be "S256" |
| scope | string | no | Requested scopes |

**Behavior**:
1. If user not logged in → shows login form
2. If user logged in → shows agent selector dropdown + authorize button
3. On approve → redirects to `redirect_uri?code=...&state=...`

## MCP Endpoint Authentication

### POST /mcp (MCP Protocol)

**Change**: MCP now **requires** authentication. The middleware was renamed from `OptionalAuthMiddlewareWithOAuth` to `RequiredAuthMiddlewareWithOAuth`. Unauthenticated requests receive:

- **Status**: `401 Unauthorized`
- **Header**: `WWW-Authenticate: Bearer resource_metadata="/.well-known/oauth-authorization-server"`

This directs MCP clients to the OAuth discovery endpoint for automatic authentication flow initiation.

**Supported auth methods** (in order of precedence):
1. API key via `Authorization: Bearer <api_key>`
2. OAuth 2.1 bearer token via `Authorization: Bearer <oauth_token>`

### MCP Tool Scope

Agent management tools (`register_agent`, `update_agent`, `deregister_agent`) have been **removed** from MCP. Agents are managed exclusively through the Web UI REST API.

MCP now exposes only 6 tools:
| Tool | Description |
|------|-------------|
| `send_message` | Send a message to an agent or channel |
| `read_inbox` | Read messages in the agent's inbox |
| `claim_messages` | Claim pending messages for processing |
| `mark_done` | Mark claimed messages as done |
| `search_messages` | Semantic search across messages |
| `discover_agents` | List available agents |

## Modified Endpoints

### POST /api/messages

**Change**: When request is session-authenticated (Web UI), the `from` field in the request body is ignored. The server always sets `from_agent` to the user's human agent name.

### DELETE /api/agents/{name}

**Change**: Before soft-deleting the agent, the server captures all messages with `to_agent = {name}` and `status IN ('pending', 'processing')` into the `dead_letters` table.

### POST /api/channels

**Change**: Channels with `is_system = 1` cannot be created via API (system channels are created internally only).

### POST /api/channels/{name}/leave

**Change**: Returns 403 if the channel has `is_system = 1` and the agent is the owner.

### DELETE /api/channels/{name} (if exists)

**Change**: Returns 403 if the channel has `is_system = 1`.
