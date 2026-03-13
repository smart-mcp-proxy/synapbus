# Feature Specification: Web UI

**Feature Branch**: `005-web-ui`
**Created**: 2026-03-13
**Status**: Draft
**Input**: User description: "Svelte 5 + Tailwind CSS embedded SPA with login, dashboard, conversations, channels, agents, settings pages. Real-time SSE updates, compose, search, agent management, dark mode, responsive, embedded in Go binary via go:embed."

## Constitution Compliance

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Local-First, Single Binary | Compliant | SPA built at compile time, embedded via `go:embed` into the Go binary. No external CDN or asset server. |
| II. MCP-Native | Compliant | Web UI consumes the internal REST API only. No MCP tools are exposed for UI operations. |
| III. Pure Go, Zero CGO | Compliant | Svelte build produces static assets; Go embedding uses stdlib `embed` package. No CGO involved. |
| IV. Multi-Tenant with Ownership | Compliant | UI enforces owner-scoped views: users see only their own agents, traces, and authorized channels. |
| V. Embedded OAuth 2.1 | Compliant | Login page authenticates against the embedded OAuth 2.1 server. Session managed via httponly cookies. |
| VIII. Observable by Default | Compliant | Trace viewer page lets owners inspect all agent activity. |
| IX. Progressive Complexity | Compliant | Dashboard shows basic messaging by default; channels, traces, and search are separate pages users navigate to when needed. |
| X. Web UI as First-Class Citizen | Compliant | This spec directly implements Principle X. |

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Dashboard and Conversation Browsing (Priority: P1)

A human owner logs in and lands on the Dashboard, which shows their most recent messages across all conversations. They click a conversation to open the thread view, read the full history, and see real-time updates as new messages arrive via SSE without refreshing the page.

**Why this priority**: The primary reason humans open the Web UI is to see what their agents are doing. Without message browsing and real-time updates, the UI provides no value. This is the core loop that makes SynapBus observable.

**Independent Test**: Can be fully tested by logging in, viewing the dashboard, clicking into a conversation, and confirming that a message sent via MCP by an agent appears in the thread within 2 seconds without a page refresh. Delivers the core value of human oversight over agent communication.

**Acceptance Scenarios**:

1. **Given** a logged-in user with 3 conversations containing messages, **When** they load the Dashboard, **Then** they see the 3 conversations sorted by most recent activity, each showing the last message preview, sender name, timestamp, and unread count.
2. **Given** a user viewing a conversation thread with 10 messages, **When** an agent sends a new message to that conversation via MCP, **Then** the message appears at the bottom of the thread within 2 seconds via SSE, without a page refresh.
3. **Given** a user on the Dashboard, **When** they click a conversation with 5 unread messages, **Then** the thread opens, all messages display in chronological order, and the unread count resets to 0.
4. **Given** a user viewing a conversation, **When** they scroll up past the initial 50 messages, **Then** older messages load incrementally (pagination) without losing their scroll position.

---

### User Story 2 - Compose and Send Messages (Priority: P1)

A human owner composes and sends a direct message to a specific agent or posts a message to a channel. They select the recipient from a searchable dropdown, type the message body, optionally set a subject and priority, and send. The message appears in the conversation immediately.

**Why this priority**: Two-way communication is essential. Without compose, the UI is read-only and humans cannot participate in agent workflows. This is tied with Story 1 as the minimum viable product.

**Independent Test**: Can be fully tested by opening the compose form, selecting an agent recipient, typing a message, clicking send, and confirming the message appears in the conversation thread and is received by the target agent via MCP `read_inbox`.

**Acceptance Scenarios**:

1. **Given** a logged-in user on the Dashboard, **When** they click "New Message", **Then** a compose form opens with a searchable recipient dropdown (populated from registered agents and channels), a subject field, a message body textarea, and a priority selector (1-10, default 5).
2. **Given** a user in the compose form who has typed "deploy" in the recipient field, **When** the dropdown filters, **Then** only agents and channels whose name or display_name contains "deploy" appear in the results.
3. **Given** a user who has filled in recipient (agent "researcher-01"), subject ("Analysis request"), body ("Please analyze Q1 data"), and priority (7), **When** they click Send, **Then** the message is created via the REST API, the compose form closes, and the user is navigated to the conversation thread showing their sent message.
4. **Given** a user composing a message, **When** they submit with an empty body, **Then** the form shows a validation error "Message body is required" and does not submit.

---

### User Story 3 - Agent Management and Trace Viewing (Priority: P2)

A human owner navigates to the Agents page to see all agents they own. They click an agent to view its details, activity traces, and API key status. They can regenerate or revoke an agent's API key from this page.

**Why this priority**: Agent oversight is a constitutional requirement (Principles IV and VIII). Without this, owners cannot monitor or control their agents. It is P2 because the system is still useful for messaging without it, but it is essential for production trust.

**Independent Test**: Can be fully tested by navigating to the Agents page, clicking an agent, viewing its traces (filtered by time range and action type), and revoking its API key, then confirming the agent can no longer authenticate via MCP.

**Acceptance Scenarios**:

1. **Given** a logged-in user who owns 3 agents, **When** they navigate to the Agents page, **Then** they see a list of their 3 agents showing name, display_name, type (ai/human), status (active/inactive), and the number of messages sent in the last 24 hours.
2. **Given** a user viewing agent "researcher-01" details, **When** they click "Activity Traces", **Then** they see a paginated, reverse-chronological list of traces (tool calls, messages sent, channel joins, errors) with filters for action type and date range.
3. **Given** a user viewing agent "researcher-01" details, **When** they click "Revoke API Key" and confirm the dialog, **Then** the API key is invalidated, the agent status changes to "inactive", and subsequent MCP requests from that agent return 401 Unauthorized.
4. **Given** a user viewing agent "researcher-01" details, **When** they click "Regenerate API Key", **Then** a new API key is generated and displayed once in a copiable field with a warning that it will not be shown again.

---

### User Story 4 - Channel Management (Priority: P2)

A human owner browses available channels, views channel details and membership, and creates new channels. For channels they own, they can manage members (invite, remove) and update channel metadata.

**Why this priority**: Channels are a core messaging concept and the foundation for swarm patterns. Channel management through the UI is essential for humans to organize agent communication, but basic DM messaging (Stories 1-2) works without it.

**Independent Test**: Can be fully tested by navigating to the Channels page, creating a new public channel, viewing its member list, and confirming that an agent can join it via MCP `join_channel`.

**Acceptance Scenarios**:

1. **Given** a logged-in user, **When** they navigate to the Channels page, **Then** they see a list of all public channels and private channels they are a member of, showing channel name, description, member count, and last activity timestamp.
2. **Given** a user on the Channels page, **When** they click "Create Channel" and fill in name ("research-findings"), description ("Shared research results"), type (public), **Then** the channel is created, appears in the list, and the user is its owner.
3. **Given** a user viewing a channel they own, **When** they click "Members" and then "Invite", **Then** they see a searchable list of agents not already in the channel and can select one or more to invite.
4. **Given** a user viewing a public channel they do not own, **When** they view the channel, **Then** they can see messages and members but cannot see "Invite" or "Remove" controls.

---

### User Story 5 - Full-Text Search (Priority: P2)

A human owner uses the search bar to find messages across all their conversations and channels. Results show message previews with highlighted matches, grouped by conversation, and link directly to the message in its thread context.

**Why this priority**: Search is critical for operational use once message volume grows. Without it, finding past agent communications becomes impossible. It is P2 because a small number of conversations can be browsed manually.

**Independent Test**: Can be fully tested by sending several messages with known content via MCP, then searching for a keyword in the Web UI and confirming matching messages appear with highlighted terms and correct conversation links.

**Acceptance Scenarios**:

1. **Given** a logged-in user with messages containing the word "deployment" in 3 different conversations, **When** they type "deployment" in the global search bar and press Enter, **Then** they see results grouped by conversation, each showing the matching message preview with "deployment" highlighted, sender name, and timestamp.
2. **Given** search results displayed, **When** the user clicks a result, **Then** they are navigated to the conversation thread scrolled to the specific message, with the matched message visually highlighted.
3. **Given** a search for "xyznonexistent", **When** results load, **Then** an empty state is shown with the message "No messages found for 'xyznonexistent'".
4. **Given** a user who does not have access to a private channel, **When** they search for content that exists only in that channel, **Then** those messages do not appear in results (access control enforced server-side).

---

### User Story 6 - Login and Authentication (Priority: P1)

A user navigates to SynapBus in their browser. If not authenticated, they are redirected to the login page. They enter their username and password, submit, and are redirected to the Dashboard. Sessions persist across browser restarts via httponly cookies until explicitly logged out.

**Why this priority**: Authentication gates all other functionality. Without login, no other story can be tested. This is a foundational P1 alongside Stories 1 and 2.

**Independent Test**: Can be fully tested by opening the Web UI in a browser, being redirected to login, entering valid credentials, and confirming redirect to the Dashboard. Then close and reopen the browser, confirm the session persists. Then click logout and confirm redirect back to login.

**Acceptance Scenarios**:

1. **Given** an unauthenticated user, **When** they navigate to any UI route (e.g., `/dashboard`), **Then** they are redirected to `/login` with the original URL preserved as a return parameter.
2. **Given** a user on the login page, **When** they enter valid username and password and click "Sign In", **Then** they are authenticated, a session cookie is set (httponly, secure, SameSite=Strict), and they are redirected to the Dashboard (or the original requested URL).
3. **Given** a user on the login page, **When** they enter an incorrect password, **Then** they see the error "Invalid username or password" without revealing which field is wrong. The form is not rate-limited visually but the server enforces rate limiting.
4. **Given** an authenticated user, **When** they click "Logout" in the navigation, **Then** the session is invalidated server-side, the cookie is cleared, and they are redirected to `/login`.

---

### User Story 7 - Settings and Dark Mode (Priority: P3)

A user navigates to Settings to change their password, toggle dark/light mode, and configure notification preferences. Dark mode preference persists in local storage and applies immediately without a page reload.

**Why this priority**: Settings and dark mode are quality-of-life features. The system is fully functional without them. Dark mode is important for developer comfort but does not block any core workflow.

**Independent Test**: Can be fully tested by toggling dark mode in Settings and confirming all pages render correctly in both themes. Change password and confirm login works with the new password.

**Acceptance Scenarios**:

1. **Given** a logged-in user on the Settings page, **When** they toggle the "Dark Mode" switch, **Then** the entire UI switches to dark theme immediately (no reload), and the preference is saved to `localStorage`.
2. **Given** a user who previously enabled dark mode, **When** they close and reopen the browser, **Then** the UI loads in dark mode directly (no flash of light theme).
3. **Given** a user on the Settings page, **When** they enter their current password, a new password, and confirm the new password, then click "Change Password", **Then** the password is updated and they see a success message. Their session remains active.
4. **Given** a user changing their password, **When** the new password is shorter than 8 characters, **Then** a validation error is shown: "Password must be at least 8 characters".

---

### Edge Cases

- What happens when the SSE connection drops (e.g., network interruption)? The UI MUST detect the disconnect, show a "Reconnecting..." banner, attempt automatic reconnection with exponential backoff (1s, 2s, 4s, max 30s), and reconcile missed messages on reconnect by fetching updates since the last received event timestamp.
- What happens when a user has no conversations or agents yet? The Dashboard MUST show a meaningful empty state with guidance: "No conversations yet. Send your first message or register an agent to get started." with action buttons.
- What happens when two browser tabs are open and one logs out? The other tab MUST detect the invalidated session on the next API call (401 response) and redirect to login without data loss (no partial state).
- How does the UI handle very long messages (>10,000 characters)? Messages MUST be truncated to 500 characters with a "Show more" toggle that expands inline. Code blocks within messages MUST be syntax-highlighted.
- What happens when the agent list is very large (>100 agents) in the compose dropdown? The dropdown MUST use virtualized rendering and support keyboard navigation. Results MUST be debounced (300ms) to avoid excessive API calls during typing.
- What happens when a user tries to access an agent they do not own via direct URL manipulation (e.g., `/agents/other-owners-agent`)? The API MUST return 403 Forbidden and the UI MUST show "You do not have access to this agent."
- How does the UI behave on slow connections? All API calls MUST show loading skeletons (not spinners) for content areas. Actions (send, revoke) MUST disable the button and show a loading indicator to prevent double submission.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The Web UI MUST be a Svelte 5 single-page application styled with Tailwind CSS, built to static assets via `make web` and embedded in the Go binary using `go:embed`.
- **FR-002**: The UI MUST include these pages: Login, Dashboard, Conversations (thread view), Channels, Agents, Settings, each accessible via client-side routing with shareable URLs.
- **FR-003**: The Dashboard MUST display the user's recent conversations sorted by last activity, showing last message preview, sender, timestamp, and unread count.
- **FR-004**: The Conversations page MUST render a threaded message view with chronological ordering, pagination (50 messages per page), and infinite scroll for older messages.
- **FR-005**: The UI MUST receive real-time updates via Server-Sent Events (SSE) from the REST API. Events MUST include: new messages, message status changes, agent status changes, and channel membership changes.
- **FR-006**: The compose form MUST allow users to send direct messages or channel messages, with a searchable recipient dropdown, subject field, body textarea, and priority selector (1-10).
- **FR-007**: The search feature MUST perform full-text search across all messages the user has access to, returning results grouped by conversation with highlighted match terms.
- **FR-008**: The Agents page MUST list all agents owned by the authenticated user, showing name, type, status, and recent activity summary.
- **FR-009**: The agent detail view MUST display the agent's activity traces in a paginated, filterable list (by action type and date range).
- **FR-010**: Users MUST be able to revoke and regenerate API keys for their agents from the agent detail view.
- **FR-011**: The Channels page MUST list all public channels and private channels the user belongs to, with options to create new channels and manage membership for owned channels.
- **FR-012**: The UI MUST support dark mode and light mode, togglable from Settings, with preference persisted in `localStorage` and applied without page reload.
- **FR-013**: The UI MUST be responsive, functioning correctly on viewports from 375px (mobile) to 2560px (ultrawide) width.
- **FR-014**: All authenticated routes MUST redirect to `/login` when the session is invalid or expired. The login page MUST authenticate against the embedded OAuth 2.1 server.
- **FR-015**: The UI MUST show loading skeletons for content areas during data fetches and disable action buttons during pending requests to prevent double submission.
- **FR-016**: The SSE connection MUST automatically reconnect on disconnect with exponential backoff (1s, 2s, 4s, max 30s) and reconcile missed events on reconnection.
- **FR-017**: The Settings page MUST allow users to change their password with current password verification and minimum 8-character validation.
- **FR-018**: The UI MUST enforce access control client-side (hiding unauthorized actions) and the REST API MUST enforce it server-side (returning 403 for unauthorized access).

### Key Entities *(include if feature involves data)*

- **Page/Route**: Each UI page maps to a client-side route (e.g., `/dashboard`, `/conversations/:id`, `/channels`, `/channels/:id`, `/agents`, `/agents/:id`, `/settings`, `/login`). Routes are managed by the Svelte router with history-mode navigation.
- **SSE Event Stream**: A persistent server-to-client connection at `GET /api/v1/events` that emits typed events (`message.new`, `message.status`, `agent.status`, `channel.member`). Each event includes a monotonic sequence ID for reconnection reconciliation.
- **Compose Payload**: The data structure submitted when sending a message: `{ recipient_type: "agent" | "channel", recipient_id: string, subject?: string, body: string, priority: number }`.
- **Search Result**: A result object containing `{ conversation_id, message_id, body_preview (highlighted), sender_name, timestamp, conversation_subject }`.
- **Theme Preference**: A `localStorage` key (`synapbus-theme`) with values `"light"` or `"dark"`, read on app initialization before first render to prevent flash of wrong theme.
- **Trace Entry (UI representation)**: Displayed in the agent detail view: `{ id, agent_name, action, details_summary, timestamp }` with expandable JSON details.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A new user can create an account, log in, send a message, and read a response within 3 minutes without documentation, using only the Web UI.
- **SC-002**: Messages sent by agents via MCP appear in the Web UI within 2 seconds via SSE, without manual refresh.
- **SC-003**: The `make web` build produces a complete SPA bundle under 500KB gzipped (excluding source maps), embedded in the Go binary with zero additional runtime dependencies.
- **SC-004**: All UI pages render correctly on Chrome, Firefox, and Safari (latest 2 versions) at mobile (375px), tablet (768px), and desktop (1440px) viewport widths, in both light and dark modes.
- **SC-005**: Full-text search returns results within 500ms for a corpus of 10,000 messages.
- **SC-006**: The SSE connection automatically recovers from a network interruption within 30 seconds and displays all messages that arrived during the disconnection without duplicates.
- **SC-007**: An owner can view an agent's traces, filter by action type and date range, and revoke its API key entirely through the Web UI without using CLI or API tools directly.
- **SC-008**: The Lighthouse accessibility score for all pages MUST be 90 or above, ensuring the UI is usable with screen readers and keyboard navigation.
