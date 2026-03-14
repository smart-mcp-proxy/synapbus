# Research: MCP Auth, UX Polish & Agent Lifecycle

**Date**: 2026-03-14
**Feature**: 002-mcp-auth-ux-polish

## R1: MCP OAuth 2.1 Discovery Pattern

**Decision**: Use RFC 8414 OAuth 2.0 Authorization Server Metadata at `/.well-known/oauth-authorization-server` endpoint. When an unauthenticated MCP client connects, the server returns 401 with a `WWW-Authenticate` header pointing to the metadata URL.

**Rationale**: This is the standard MCP specification approach for auth discovery. MCP clients (like Claude Code) that support OAuth will automatically detect the authorization server metadata and initiate the OAuth flow. The metadata document provides `authorization_endpoint`, `token_endpoint`, and `registration_endpoint` URLs.

**Alternatives considered**:
- Custom auth negotiation protocol — rejected, non-standard
- HTTP 302 redirect to login — rejected, doesn't work for programmatic MCP clients
- Server-side auth prompt — rejected, MCP transport doesn't support interactive prompts

## R2: OAuth Token-to-Agent Binding

**Decision**: Store the selected agent name in the OAuth session data (`session_data` JSON field in `oauth_tokens` table). When a bearer token is introspected, extract the agent name from session data and set it in the request context.

**Rationale**: Fosite's session system already supports storing arbitrary data. The `fositeSession` struct can carry the agent name alongside user_id. This avoids adding new tables and reuses the existing token introspection flow.

**Alternatives considered**:
- Separate agent-token mapping table — rejected, over-engineering
- Token scope with agent name (e.g., `agent:mybot`) — rejected, scopes are for permissions not identity
- Custom JWT claim — rejected, fosite handles token format internally

## R3: OAuth Authorization Page Implementation

**Decision**: Implement the authorize page as a server-rendered Go template (not a Svelte SPA route) at `/oauth/authorize`. The page includes a login form (if not logged in) and an agent selector dropdown (if logged in). This is a standard OAuth consent page pattern.

**Rationale**: The OAuth authorize endpoint must work independently of the SPA. It's accessed directly by the user's browser during the OAuth redirect flow. Server-rendered HTML is simpler and avoids CORS/session issues that would arise from a SPA-based consent page.

**Alternatives considered**:
- Svelte SPA route — rejected, OAuth authorize must work without the SPA loaded
- Redirect to SPA login then back — rejected, adds unnecessary complexity and redirect chains
- Headless/API-only consent — rejected, user must visually see and approve agent selection

## R4: "My Agents" Channel Auto-Creation Strategy

**Decision**: Lazy initialization — create the "my-agents-{username}" channel on first login (in the `withHumanAgent` handler wrapper) if it doesn't exist. For existing users, the channel is created on their next login. The channel name uses `my-agents-{username}` format internally but displays as "My Agents" in the UI via a display name.

**Rationale**: Lazy init avoids a migration that creates channels for all existing users (some of whom may never log in again). It's idempotent — checking "does this channel exist?" on each login is cheap.

**Alternatives considered**:
- Eager migration (create for all users) — rejected, creates channels for inactive users
- Separate "system channels" table — rejected, over-engineering; standard channels with a `is_system` flag suffice
- On-demand at agent registration — rejected, channel should exist before first agent is created

## R5: Dead Letter Queue Storage Strategy

**Decision**: Add a `dead_letters` table with: id, owner_id, original_message_id, to_agent (the deleted agent name), from_agent, body, subject, priority, metadata, acknowledged (boolean), created_at. When an agent is deleted, INSERT INTO dead_letters SELECT from messages WHERE to_agent = ? AND status IN ('pending', 'processing').

**Rationale**: A separate table is cleaner than adding status flags to the messages table. Dead letters are an owner-level concept (not agent-level), and they need to survive agent deletion. The original message remains in the messages table but can be garbage-collected later.

**Alternatives considered**:
- Status flag on messages table (status = 'dead_letter') — rejected, messages.to_agent references the agent which is being deleted; keeping messages in the same table creates referential integrity issues
- Soft-copy into a JSON blob — rejected, loses queryability
- View-based approach (query messages for deleted agents) — rejected, requires knowing which agents were deleted and when

## R6: System Channel Protection

**Decision**: Add an `is_system` boolean column to the channels table. System channels cannot be deleted or left by the owner. The channel service checks this flag before allowing delete/leave operations.

**Rationale**: Simple boolean flag is more flexible than hardcoding channel name patterns. Can be used for future system channels beyond "my-agents".

**Alternatives considered**:
- Hardcode "my-agents-*" pattern check — rejected, not extensible
- Separate system_channels table — rejected, over-engineering
- Permission-based (remove delete permission) — rejected, permissions don't exist yet as a first-class concept for channels

## R7: Web UI Human-Only Messaging

**Decision**: Modify the message send API handler to always override the `from` field with the logged-in user's human agent name when the request comes from a session-authenticated context (Web UI). Remove the "Send as" dropdown from channel and DM page components.

**Rationale**: Server-side enforcement ensures security — even if the UI is bypassed, the API won't allow impersonation. Client-side removal of the dropdown is a UX simplification.

**Alternatives considered**:
- Client-side only enforcement — rejected, insecure
- Remove the `from` field entirely from API — rejected, MCP tools still need it
- Per-user setting to enable/disable send-as — rejected, user explicitly said to remove it
