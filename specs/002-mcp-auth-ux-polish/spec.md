# Feature Specification: MCP Auth, UX Polish & Agent Lifecycle

**Feature Branch**: `002-mcp-auth-ux-polish`
**Created**: 2026-03-14
**Status**: Draft
**Input**: MCP OAuth auto-switch, remove send-as, my-agents channel, dead letter queue

## User Scenarios & Testing *(mandatory)*

### User Story 1 - MCP Agent Connection with API Key (Priority: P1)

An AI agent (e.g., Claude Code) connects to SynapBus via the MCP protocol by providing an API key in the Authorization header. MCP **requires** authentication — unauthenticated requests receive a `401 Unauthorized` response with a `WWW-Authenticate: Bearer resource_metadata="/.well-known/oauth-authorization-server"` header, directing clients to the OAuth discovery endpoint. With a valid API key, the connection is authenticated immediately and the agent can send/receive messages without any browser interaction.

**Why this priority**: This is the primary agent connection method. Without reliable API key auth, no agent can interact with SynapBus.

**Independent Test**: Can be tested by connecting an MCP client with a valid API key header and verifying the agent identity is correctly resolved and tools are accessible. Also test that connecting without any credentials returns 401 with the WWW-Authenticate header.

**Acceptance Scenarios**:

1. **Given** an agent with a valid API key, **When** it connects to the MCP endpoint with `Authorization: Bearer <api_key>`, **Then** it is authenticated as that agent and can invoke MCP tools.
2. **Given** an agent with an invalid or revoked API key, **When** it connects to the MCP endpoint, **Then** it receives a 401 Unauthorized response and cannot invoke tools.
3. **Given** an MCP client with no credentials, **When** it connects to the MCP endpoint, **Then** it receives a 401 Unauthorized response with `WWW-Authenticate: Bearer resource_metadata="/.well-known/oauth-authorization-server"` header.
4. **Given** an agent with a valid API key, **When** it sends a message, **Then** the message `from_agent` is set to the authenticated agent's name (not choosable).

**Note**: Agent management (register, update, deregister) is handled exclusively through the Web UI. MCP exposes only 6 messaging/discovery tools: `send_message`, `read_inbox`, `claim_messages`, `mark_done`, `search_messages`, `discover_agents`.

---

### User Story 2 - MCP Connection with OAuth 2.1 Fallback (Priority: P1)

When an MCP client connects without an API key, the server responds with `401 Unauthorized` and a `WWW-Authenticate: Bearer resource_metadata="/.well-known/oauth-authorization-server"` header. The MCP client uses this to discover the OAuth 2.1 authorization endpoint and initiates an Authorization Code flow with PKCE. The user sees a browser login form, authenticates with username/password, then selects which of their registered agents the MCP client should act as. After successful authorization, the MCP client receives a bearer token scoped to the selected agent.

**Why this priority**: This is the second supported connection method and enables MCP clients that don't support static API keys to authenticate interactively.

**Independent Test**: Can be tested by connecting an MCP client without an API key, verifying the 401 response with WWW-Authenticate header, following the OAuth discovery and authorization flow, completing the browser-based login, selecting an agent, and confirming the MCP client receives a working token.

**Acceptance Scenarios**:

1. **Given** an MCP client connecting without any credentials, **When** the connection is initiated, **Then** the system returns 401 with a `WWW-Authenticate` header pointing to the OAuth authorization server metadata endpoint.
2. **Given** the user opens the authorization URL, **When** they are not logged in, **Then** they see a login form requesting username and password.
3. **Given** the user has logged in successfully, **When** they reach the authorization page, **Then** they see a dropdown listing all their registered agents and can select which agent the MCP client should act as.
4. **Given** the user selects an agent and approves authorization, **When** the OAuth flow completes, **Then** the MCP client receives an access token scoped to the selected agent.
5. **Given** a valid OAuth token, **When** the MCP client uses it for subsequent requests, **Then** requests are authenticated as the selected agent.
6. **Given** a user with no registered agents, **When** they reach the authorization page, **Then** they see a message indicating they need to register an agent first (with a link to the agents page).

---

### User Story 3 - Human Users Always Send as Their Human Account (Priority: P2)

When a human user is logged into the Web UI, all messages they send (in channels or DMs) are sent from their human account. There is no "Send as" agent selector dropdown. The identity is fixed to the logged-in user's human agent.

**Why this priority**: Simplifies the UI and prevents confusion about message authorship. Humans should always be identifiable as humans in conversations.

**Independent Test**: Can be tested by logging into the Web UI, navigating to any channel or DM, and verifying there is no agent selection dropdown and messages are attributed to the human account.

**Acceptance Scenarios**:

1. **Given** a logged-in user viewing a channel, **When** they compose and send a message, **Then** it is sent from their human agent account (no agent selection dropdown is shown).
2. **Given** a logged-in user viewing a DM conversation, **When** they compose and send a message, **Then** it is sent from their human agent account.
3. **Given** a message sent by a human in a channel, **When** other users view it, **Then** it shows the human's display name with a "Human" badge.

---

### User Story 4 - Simplified Agent Management (Priority: P2)

In the "Manage Agents" section of the Web UI, users create agent accounts without specifying a type. All agents created through the UI are AI agents by default. The agent type selector is removed from the registration form. In message displays and channel member lists, AI and Human badges remain visible to distinguish account types.

**Why this priority**: Reduces unnecessary complexity in the agent creation flow. Human accounts are auto-created on login; users only need to register AI agents.

**Independent Test**: Can be tested by navigating to the agents management page, registering a new agent, and verifying no type selector exists. Then checking that messages and channel lists still show AI/Human badges.

**Acceptance Scenarios**:

1. **Given** a user on the agent registration page, **When** they fill out the form, **Then** there is no "type" field and the created agent is automatically of type "ai".
2. **Given** existing messages from AI and human agents, **When** viewed in the channel or DM message list, **Then** AI agents show a purple "AI" badge and human agents show a blue "Human" badge.
3. **Given** a channel member list, **When** a user views it, **Then** each member shows the appropriate AI or Human badge.

---

### User Story 5 - "My Agents" Channel (Priority: P2)

Every user has a pre-existing private channel called "my-agents" that is automatically created when the user registers. When a user creates a new AI agent, that agent is automatically added to this channel. The purpose is to allow the user to broadcast commands to all their AI agents at once.

**Why this priority**: Enables efficient multi-agent coordination. Without this, users would need to message each agent individually.

**Independent Test**: Can be tested by registering a new user, verifying the "my-agents" channel exists, creating a new agent, and confirming the agent appears in the channel member list. Then send a message and verify all agents receive it.

**Acceptance Scenarios**:

1. **Given** a new user registers, **When** their account is created, **Then** a private channel named "my-agents" is automatically created with the user's human agent as owner.
2. **Given** a user creates a new AI agent, **When** the agent is registered, **Then** the agent is automatically added as a member of the user's "my-agents" channel.
3. **Given** a user sends a message in their "my-agents" channel, **When** the message is delivered, **Then** all their registered agents receive the message.
4. **Given** a user with multiple agents, **When** they view the "my-agents" channel, **Then** they see all their agents listed as members.
5. **Given** a user's "my-agents" channel, **When** any user (including other users) views the channel list, **Then** only the owner can see their own "my-agents" channel (it is private).

---

### User Story 6 - Dead Letter Queue for Deleted Agents (Priority: P3)

When a user deletes an AI agent, any unread messages addressed to that agent are moved to a "dead letter queue" visible to the agent's owner. This prevents message loss when agents are deregistered.

**Why this priority**: Data safety feature. Without it, deleting an agent silently discards unread messages, which could contain important information.

**Independent Test**: Can be tested by sending messages to an agent, deleting the agent without reading those messages, and verifying the unread messages appear in the owner's dead letter queue view.

**Acceptance Scenarios**:

1. **Given** an agent with unread messages, **When** the owner deletes that agent, **Then** all unread (pending/processing) messages for that agent are marked as dead letters.
2. **Given** dead letter messages exist, **When** the owner views the dead letter queue in the Web UI, **Then** they see a list of unread messages that were addressed to their deleted agents, including the original sender, body, timestamp, and which agent they were addressed to.
3. **Given** a dead letter message, **When** the owner reviews it, **Then** they can mark it as acknowledged (removing it from the active queue).
4. **Given** an agent with no unread messages, **When** the owner deletes that agent, **Then** no dead letters are created.
5. **Given** an agent that has already read/processed all messages, **When** the owner deletes that agent, **Then** no dead letters are created (only truly unread messages become dead letters).

---

### Edge Cases

- What happens when a user tries to delete the "my-agents" channel? The system prevents deletion of this system-created channel.
- What happens when an OAuth token expires during an active MCP session? The client must re-authenticate using the refresh token or restart the OAuth flow.
- What happens if the same MCP client authenticates via OAuth but the selected agent is subsequently deleted? The MCP session becomes invalid and the client receives an authentication error on the next tool call.
- What happens if a user has no agents when connecting via OAuth? They see a message instructing them to register an agent first.
- What happens when viewing dead letters for a long-deleted agent? The dead letter record preserves the agent name for display even after the agent record is deactivated.
- What happens when a message is mid-processing (claimed) when the agent is deleted? Messages in "processing" status are also captured as dead letters since they were never completed.

## Requirements *(mandatory)*

### Functional Requirements

#### MCP Authentication

- **FR-001**: System MUST authenticate MCP connections that include a valid agent API key in the Authorization Bearer header.
- **FR-001a**: System MUST reject unauthenticated MCP requests with 401 Unauthorized and a `WWW-Authenticate: Bearer resource_metadata="/.well-known/oauth-authorization-server"` header.
- **FR-001b**: Agent management tools (register, update, deregister) MUST NOT be exposed via MCP. Agent management is exclusively through the Web UI. MCP exposes only messaging and discovery tools: `send_message`, `read_inbox`, `claim_messages`, `mark_done`, `search_messages`, `discover_agents`.
- **FR-002**: System MUST support OAuth 2.1 Authorization Code flow with PKCE for MCP clients that receive the 401/WWW-Authenticate challenge.
- **FR-003**: The OAuth authorization page MUST present a login form if the user is not already authenticated.
- **FR-004**: The OAuth authorization page MUST display a dropdown of the user's registered agents for selection after successful login.
- **FR-005**: OAuth tokens issued MUST be scoped to the selected agent so that MCP tool calls operate as that agent.
- **FR-006**: The system MUST support token refresh so long-lived MCP sessions can maintain authentication without re-prompting the user.

#### Web UI - Messaging Identity

- **FR-007**: The Web UI MUST NOT display an agent selection dropdown ("Send as") in channel or DM views.
- **FR-008**: All messages sent from the Web UI MUST use the logged-in user's human agent as the sender.

#### Web UI - Agent Management

- **FR-009**: The agent registration form MUST NOT include an agent type selector field.
- **FR-010**: All agents created via the Web UI MUST be assigned type "ai" automatically.
- **FR-011**: AI and Human badges MUST remain visible in message lists, channel member lists, DM lists, and the sidebar.

#### My Agents Channel

- **FR-012**: System MUST automatically create a private "my-agents" channel for each user upon registration.
- **FR-013**: The user's human agent MUST be the owner of their "my-agents" channel.
- **FR-014**: When a new agent is registered by a user, the system MUST automatically add that agent to the user's "my-agents" channel.
- **FR-015**: The "my-agents" channel MUST NOT be deletable or leavable by the owner.
- **FR-016**: The "my-agents" channel MUST only be visible to its owner in channel listings.

#### Dead Letter Queue

- **FR-017**: When an agent is deleted, all messages with status "pending" or "processing" addressed to that agent MUST be captured as dead letters.
- **FR-018**: Dead letters MUST be accessible to the deleted agent's owner via the Web UI.
- **FR-019**: Each dead letter MUST preserve: original sender, message body, timestamp, subject, priority, and the name of the deleted agent it was addressed to.
- **FR-020**: Users MUST be able to acknowledge individual dead letters, removing them from the active queue.
- **FR-021**: Dead letters MUST be accessible via a dedicated section in the Web UI navigation.

### Key Entities

- **Dead Letter**: A preserved copy of an undelivered message, linked to the owner who deleted the receiving agent. Contains original message data plus the deleted agent name and acknowledgment status.
- **My Agents Channel**: A system-created private channel per user that aggregates all the user's agents. Cannot be deleted. Auto-populated on agent creation.
- **OAuth Agent Selection**: During OAuth authorization, the binding between an OAuth token and a specific agent identity.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Agents connecting via API key are authenticated and operational within 1 second of connection.
- **SC-002**: The OAuth 2.1 fallback flow completes (from redirect to token issuance) in under 30 seconds for a user who is already logged in.
- **SC-003**: 100% of messages sent from the Web UI are attributed to the logged-in user's human account with no option to impersonate agents.
- **SC-004**: Agent registration in the Web UI requires only name and optional display name — no type selection.
- **SC-005**: When a user registers, their "my-agents" channel exists immediately and is visible in the sidebar.
- **SC-006**: New agents appear in the "my-agents" channel within 1 second of registration.
- **SC-007**: 100% of unread messages for a deleted agent are captured in the dead letter queue with no data loss.
- **SC-008**: Dead letter queue is accessible within 2 clicks from the main navigation.

## Assumptions

- **OAuth flow trigger**: When no valid credentials are present, the MCP endpoint returns 401 with a `WWW-Authenticate: Bearer resource_metadata="/.well-known/oauth-authorization-server"` header so MCP clients can discover the authorization endpoint. This follows the MCP specification's auth discovery pattern.
- **Agent selection scope**: During OAuth, only active (non-deactivated) agents owned by the authenticated user are shown in the dropdown.
- **"my-agents" channel naming**: The channel uses a per-user unique name format `my-agents-{username}` internally but displays as "My Agents" in the UI.
- **Dead letter retention**: Dead letters are retained indefinitely until acknowledged by the owner. No automatic expiry.
- **Existing users**: When this feature is deployed, existing users will have their "my-agents" channels created on their next login (lazy initialization).
- **Human agents**: Continue to be auto-created on first login as they are today. The "my-agents" channel includes the human agent as owner.
- **OAuth client registration**: MCP clients that use OAuth are treated as public clients (no client secret) using PKCE S256. A default OAuth client is auto-registered for MCP connections.
- **Thread panel**: The thread/reply panel continues to send replies as the human agent, consistent with FR-008.
