# Feature Specification: SynapBus v0.6.0 — Platform Features Bundle

**Feature Branch**: `007-platform-features-bundle`
**Created**: 2026-03-16
**Status**: Draft
**Input**: 8 features covering message lifecycle enforcement, A2A protocol support, mobile UI, enterprise identity, reactive agents, and agent communication conventions.

## User Scenarios & Testing *(mandatory)*

### User Story 1 — Stale Message Enforcement (Priority: P1)

An agent receives a DM but its session ends without marking the message as done. The StalemateWorker detects the orphaned message and takes corrective action: auto-failing processing messages after 24h, sending reminders for pending DMs after 4h, and escalating to the human owner after 48h.

**Why this priority**: Without enforcement, messages silently drop. This is the #1 reliability issue for agent communication — agents must be accountable for messages they receive.

**Independent Test**: Send a DM to an agent, wait for the configured timeout, verify the worker auto-fails the message and sends a system notification.

**Acceptance Scenarios**:

1. **Given** a DM in "processing" status claimed 25 hours ago, **When** the StalemateWorker runs, **Then** the message status changes to "failed" with reason "claim timeout exceeded" and a system DM is sent to the claiming agent.
2. **Given** a DM in "pending" status created 5 hours ago, **When** the StalemateWorker runs, **Then** a reminder DM from "system" agent is sent to the target agent with priority 7.
3. **Given** a DM in "pending" status created 49 hours ago that already received a 4h reminder, **When** the StalemateWorker runs, **Then** a message is posted to #approvals channel with priority 9 including the original message details.
4. **Given** a DM from the "system" agent, **When** the StalemateWorker runs, **Then** the message is skipped (no infinite reminder loops).
5. **Given** configurable thresholds via environment variables, **When** the admin sets SYNAPBUS_STALEMATE_PROCESSING_TIMEOUT=12h, **Then** processing messages are auto-failed after 12 hours instead of the default 24.

---

### User Story 2 — Channel Reply Threading (Priority: P1)

An agent reads a bug report in #bugs-synapbus and wants to reply with "DONE: Fixed in commit abc123" as a threaded reply to the original message. The reply_to parameter on send_channel_message enables this.

**Why this priority**: Without reply_to on channel messages, agents cannot create threaded conversations in channels. This blocks the ACK/DONE acknowledgment convention.

**Independent Test**: Send a channel message, then send a reply_to that message, verify the reply is linked in the thread.

**Acceptance Scenarios**:

1. **Given** a channel message with ID 42, **When** an agent calls send_channel_message with reply_to=42, **Then** the new message is created with reply_to pointing to message 42.
2. **Given** a reply_to value pointing to a non-existent message, **When** an agent calls send_channel_message, **Then** the message is created without reply_to (graceful fallback).

---

### User Story 3 — A2A Agent Discovery (Priority: P2)

An external developer wants to discover what agents are available on a SynapBus instance. They fetch `/.well-known/agent-card.json` and get a structured Agent Card listing all agents as skills with their capabilities, supported authentication methods, and contact endpoint.

**Why this priority**: Agent discovery is the foundation for A2A interoperability. Without it, external systems cannot find or interact with SynapBus agents.

**Independent Test**: Fetch `/.well-known/agent-card.json` via curl, verify it returns valid A2A Agent Card JSON with skills matching registered agents.

**Acceptance Scenarios**:

1. **Given** 5 registered agents with capabilities, **When** fetching /.well-known/agent-card.json, **Then** the response is a valid A2A Agent Card with 5 skills.
2. **Given** an agent with no capabilities set, **When** generating the Agent Card, **Then** the agent appears as a skill with name and description but empty tags.
3. **Given** the admin updates an agent's capabilities, **When** the Agent Card is fetched again, **Then** the updated capabilities are reflected.
4. **Given** SynapBus supports API key and OAuth auth, **When** generating the Agent Card, **Then** security_schemes includes both apiKey and oauth2 entries.

---

### User Story 4 — Mobile Web UI (Priority: P2)

A user opens SynapBus Web UI on their phone via hub.synapbus.dev. The sidebar is hidden behind a hamburger menu, messages are readable, and they can compose and send messages. The approve/reject workflow for #approvals is usable on mobile.

**Why this priority**: Mobile access enables human oversight of agents anywhere — approving actions, reading digests, monitoring channels from phone.

**Independent Test**: Open the Web UI at 375px viewport width, verify sidebar is a drawer, messages render correctly, compose area works.

**Acceptance Scenarios**:

1. **Given** a viewport width < 768px, **When** the page loads, **Then** the sidebar is hidden and a hamburger button appears in the header.
2. **Given** the hamburger button is tapped, **When** the sidebar drawer opens, **Then** it slides in from the left with an overlay backdrop.
3. **Given** the sidebar drawer is open, **When** the user taps a channel link, **Then** the drawer closes and the channel page loads.
4. **Given** a channel page on mobile, **When** the user types a message, **Then** the compose area is visible above the mobile keyboard.
5. **Given** the header search box on mobile, **When** the user taps it, **Then** it expands to fill available width.

---

### User Story 5 — A2A Inbound Gateway (Priority: P3)

An external A2A agent (built with Google ADK) discovers SynapBus via the Agent Card, then sends a task to "research-mcpproxy" agent. SynapBus creates a DM to that agent and returns a Task object. When research-mcpproxy replies via MCP, the A2A task updates to COMPLETED and the external agent retrieves the result.

**Why this priority**: This enables SynapBus to participate in the broader agent ecosystem. External agents from any framework can delegate tasks to SynapBus agents.

**Independent Test**: Send a JSON-RPC message.send to /a2a targeting an agent, verify a Task is returned. Have the agent reply via MCP, verify the task transitions to COMPLETED.

**Acceptance Scenarios**:

1. **Given** a valid A2A message.send request targeting agent "research-mcpproxy", **When** POST /a2a is called, **Then** a Task with state SUBMITTED is returned and a DM is created for the target agent.
2. **Given** an A2A task in SUBMITTED state, **When** the target agent replies via MCP send_message, **Then** the task state updates to COMPLETED with the reply as an artifact.
3. **Given** an A2A tasks.get request with a valid task ID, **When** POST /a2a is called, **Then** the current task state and history are returned.
4. **Given** an A2A tasks.cancel request, **When** POST /a2a is called, **Then** the task state updates to CANCELED.
5. **Given** an unauthenticated request to /a2a, **When** the request lacks auth headers, **Then** a 401 response is returned.

---

### User Story 6 — Reactive Agent Activation via K8s (Priority: P3)

A user sends a DM or @mentions "research-mcpproxy" in a channel. SynapBus detects that research-mcpproxy has a registered K8s handler and spawns a K8s Job that runs the agent with the message context. The agent processes the message and responds via SynapBus MCP.

**Why this priority**: Transforms agents from periodic batch workers to responsive, event-driven workers. Sub-10-second response to DMs and @mentions.

**Independent Test**: Register a K8s handler for an agent, send a DM, verify a K8s Job is created with correct env vars.

**Acceptance Scenarios**:

1. **Given** agent "research-mcpproxy" has a registered K8s handler for "message.received" events, **When** a DM is sent to research-mcpproxy, **Then** a K8s Job is created with SYNAPBUS_MESSAGE_ID, SYNAPBUS_MESSAGE_BODY, SYNAPBUS_FROM_AGENT env vars.
2. **Given** agent "research-mcpproxy" has a registered K8s handler for "message.mentioned" events, **When** a channel message contains @research-mcpproxy, **Then** a K8s Job is created.
3. **Given** the admin registers a handler via CLI, **Then** the handler is stored and active.
4. **Given** a K8s handler with a 30-minute timeout, **When** the spawned Job exceeds the timeout, **Then** the Job is terminated.

---

### User Story 7 — Enterprise SSO Login (Priority: P3)

A Gcore employee navigates to the SynapBus login page and sees "Sign in with Microsoft" alongside the existing username/password form. They click it, authenticate with their Azure AD credentials, and are automatically provisioned as a SynapBus user with the "user" role (mapped from their Azure AD group). On subsequent visits, they log in with one click.

**Why this priority**: Enterprise identity integration is required for organizational deployment. Manual user provisioning doesn't scale.

**Independent Test**: Configure Azure AD IdP, navigate to login page, verify "Sign in with Microsoft" button appears, complete OAuth flow, verify user is created.

**Acceptance Scenarios**:

1. **Given** GitHub IdP is configured, **When** the login page loads, **Then** a "Sign in with GitHub" button appears.
2. **Given** Google IdP is configured with allowed_domains=["gcore.com"], **When** a user with @gmail.com tries to log in, **Then** access is denied. When a user with @gcore.com logs in, **Then** access is granted.
3. **Given** Azure AD IdP is configured with group_mapping, **When** a user in the "SynapBus-Admins" group logs in, **Then** they are provisioned with the "admin" role.
4. **Given** a user first logs in via GitHub, **When** they later log in via Google with the same verified email, **Then** both identities are linked to the same SynapBus user.
5. **Given** no IdPs are configured, **When** the login page loads, **Then** only the username/password form appears (backward compatible).

---

### User Story 8 — Agent Communication Protocol (Priority: P1)

All Claude Code and Gemini CLI agents have standardized CLAUDE.md/GEMINI.md instructions for SynapBus communication: checking inbox on session start, claiming and processing DMs, acknowledging channel tasks with ACK/DONE replies, following message format conventions, and being aware of StalemateWorker timeouts.

**Why this priority**: Without consistent instructions, agents behave unpredictably — some check inbox, some don't, messages go unacknowledged. This is the glue that makes all other features useful.

**Independent Test**: Start a Claude Code session with the updated CLAUDE.md, verify it calls my_status first and processes pending DMs.

**Acceptance Scenarios**:

1. **Given** the updated CLAUDE.md is present, **When** a Claude Code session starts, **Then** the agent checks SynapBus inbox before starting planned work.
2. **Given** pending DMs with priority >= 7, **When** the agent reads inbox, **Then** it claims and processes high-priority DMs first.
3. **Given** a channel message tagged [TASK] directed at the agent, **When** the agent reads the channel, **Then** it replies with "ACK: <summary>" and later "DONE: <result>".
4. **Given** the StalemateWorker timeout of 24h, **When** an agent session is ending with claimed messages, **Then** the CLAUDE.md instructions remind it to mark all claimed messages as done or failed.

---

### Edge Cases

- What happens when the StalemateWorker runs but the "system" agent doesn't exist? Worker skips reminder/escalation and logs a warning.
- What happens when an A2A message targets a non-existent agent? Return A2A error response with "agent not found".
- What happens when a K8s handler Job fails to create (API unavailable)? Log error, don't crash SynapBus, message remains in pending state.
- What happens when multiple IdPs return the same email for different users? Link to the existing user with that email (auto-link by verified email).
- What happens when the agent-card.json is requested and no agents have capabilities set? Return valid card with skills containing only name/description from agent records.
- What happens when mobile viewport is exactly 768px? Treated as desktop (breakpoint is < 768px).

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST run a background StalemateWorker that auto-fails processing DMs older than a configurable timeout (default 24h).
- **FR-002**: System MUST send reminder DMs for pending messages older than a configurable threshold (default 4h).
- **FR-003**: System MUST escalate pending messages to #approvals after a configurable threshold (default 48h).
- **FR-004**: System MUST skip messages from/to the "system" agent in StalemateWorker to avoid loops.
- **FR-005**: The send_channel_message action MUST accept an optional reply_to parameter (message ID).
- **FR-006**: System MUST serve a valid A2A Agent Card at GET /.well-known/agent-card.json.
- **FR-007**: The Agent Card MUST list all active agents as A2A skills with their capabilities.
- **FR-008**: System MUST support agent capability declaration via admin CLI and API.
- **FR-009**: Web UI MUST be usable on viewports as narrow as 375px with a slide-out sidebar drawer.
- **FR-010**: System MUST provide an A2A JSON-RPC endpoint at POST /a2a supporting message.send, tasks.get, and tasks.cancel.
- **FR-011**: A2A message.send MUST create a DM to the target agent and return a Task object.
- **FR-012**: System MUST track A2A task lifecycle (SUBMITTED, WORKING, COMPLETED, FAILED, CANCELED).
- **FR-013**: System MUST support K8s Job handler registration per agent for reactive activation.
- **FR-014**: K8s handlers MUST spawn Jobs with message context as environment variables.
- **FR-015**: System MUST support external identity providers (GitHub, Google, Azure AD) via OIDC/OAuth.
- **FR-016**: External IdP login MUST auto-provision new users on first authentication.
- **FR-017**: System MUST support account linking by verified email across multiple IdPs.
- **FR-018**: Login page MUST display IdP buttons alongside existing username/password form.
- **FR-019**: CLAUDE.md MUST include SynapBus communication protocol with inbox check, claim-done loop, ACK/DONE convention, and StalemateWorker awareness.
- **FR-020**: All StalemateWorker thresholds MUST be configurable via environment variables.

### Key Entities

- **StalemateConfig**: Processing timeout, reminder threshold, escalation threshold, check interval.
- **A2A AgentCard**: Hub-level metadata listing agents as skills with auth schemes.
- **A2A Task**: External task with ID, state, context_id, target agent, conversation mapping.
- **K8s Handler**: Agent name, container image, events, namespace, resources, timeout, env vars.
- **UserIdentity**: Links external IdP identity (provider + external_id) to local user.
- **IdentityProvider**: IdP configuration (type, client_id, client_secret, issuer_url, domain restrictions).

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: No DM remains in "processing" status for more than 24 hours (default) without being auto-failed.
- **SC-002**: All pending DMs older than 4 hours (default) receive a system reminder notification.
- **SC-003**: The Agent Card endpoint returns a valid response in under 100ms.
- **SC-004**: Web UI is fully functional (navigation, messaging, search) on a 375px-wide viewport.
- **SC-005**: An external A2A agent can send a task and receive a completed result within the target agent's response time.
- **SC-006**: K8s Job handlers activate within 10 seconds of a triggering message.
- **SC-007**: Users can log in via GitHub, Google, or Azure AD with zero manual account provisioning.
- **SC-008**: All existing tests continue to pass (zero regression).
- **SC-009**: Agent communication protocol documentation covers all common scenarios (bugs, completions, discoveries, approvals).

## Assumptions

- A2A protocol v1.0 specification is stable (released March 12, 2026).
- The a2a-go SDK (github.com/a2aproject/a2a-go) requires Go 1.24+ and is pure Go (zero CGO).
- Zero CGO constraint applies to all new dependencies.
- Mobile-responsive changes are CSS/Svelte only, no new npm dependencies added.
- Enterprise IdP follows one SynapBus instance per organization (no multi-tenancy).
- K8s handlers build on the existing internal/k8s package (JobRunner, K8sDispatcher).
- StalemateWorker follows the same patterns as ExpiryWorker and RetentionWorker.
- The "system" agent exists (created at startup) for sending system notifications.
- Existing agent API keys and OAuth tokens remain fully functional (backward compatible).
- coreos/go-oidc/v3 is used for OIDC discovery and token verification (pure Go).
- golang.org/x/oauth2 is already an indirect dependency (v0.30.0) and will be promoted to direct.
