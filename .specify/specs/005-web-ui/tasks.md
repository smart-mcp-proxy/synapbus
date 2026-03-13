# Tasks: Web UI

**Input**: Design documents from `/specs/005-web-ui/`
**Prerequisites**: spec.md (required), constitution.md (required for principles)

**Tests**: Not explicitly requested in the feature specification. Test tasks are omitted.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story. User stories are ordered by priority (P1 first).

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US6)
- Include exact file paths in descriptions

## Path Conventions

- **Svelte SPA**: `web/src/` (source), `internal/web/dist/` (build output)
- **Go embedding**: `internal/web/embed.go`
- **Go API handlers**: `internal/api/`

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Initialize Svelte 5 project, Tailwind CSS, and Go embedding scaffold

- [ ] T001 Initialize Svelte 5 project with Vite in `web/` directory: `package.json`, `svelte.config.js`, `vite.config.ts`, `tsconfig.json`. Configure build output to `../internal/web/dist/`.
- [ ] T002 Install and configure Tailwind CSS v4 in `web/tailwind.config.js` and `web/src/app.css` with base, components, and utilities layers. Include CSS custom properties for dark mode theme variables.
- [ ] T003 [P] Install client-side router (`svelte-spa-router` or equivalent) and create `web/src/App.svelte` with route definitions for: `/login`, `/dashboard`, `/conversations/:id`, `/channels`, `/channels/:id`, `/agents`, `/agents/:id`, `/settings`.
- [ ] T004 [P] Create Go embedding scaffold at `internal/web/embed.go`: use `go:embed dist/*` to embed the built SPA. Export an `http.FileServer` handler that serves `index.html` for all non-API routes (SPA fallback).
- [ ] T005 [P] Update `Makefile` `web` target to ensure build output lands in `internal/web/dist/`. Add a `web-dev` target for Vite dev server with API proxy to `localhost:8080`.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core infrastructure that MUST be complete before ANY user story can be implemented

**CRITICAL**: No user story work can begin until this phase is complete

- [ ] T006 Create shared API client module at `web/src/lib/api.ts`: base URL configuration, fetch wrapper with JSON parsing, automatic 401 detection (redirect to `/login`), CSRF token handling, and request/response type definitions.
- [ ] T007 [P] Create auth store at `web/src/lib/stores/auth.ts`: Svelte 5 reactive state (`$state`) for current user, login status, and session validation. Export `login()`, `logout()`, `checkSession()` functions that call the API client.
- [ ] T008 [P] Create theme system at `web/src/lib/stores/theme.ts`: read `localStorage` key `synapbus-theme` before first render (inline script in `web/index.html` to prevent flash), export `toggleTheme()`, apply `dark` class to `<html>` element. Svelte 5 `$state` rune for reactive theme.
- [ ] T009 [P] Create SSE client module at `web/src/lib/sse.ts`: connect to `GET /api/v1/events`, parse typed events (`message.new`, `message.status`, `agent.status`, `channel.member`), track last event sequence ID, implement auto-reconnect with exponential backoff (1s, 2s, 4s, max 30s), reconcile missed events on reconnect by fetching since last sequence ID. Export reactive connection status (`connected`, `reconnecting`, `disconnected`).
- [ ] T010 [P] Create shared UI components at `web/src/lib/components/`: `LoadingSkeleton.svelte` (content placeholder), `EmptyState.svelte` (icon + message + action button), `ReconnectBanner.svelte` (SSE disconnect notification), `SubmitButton.svelte` (loading state, disabled during pending).
- [ ] T011 [P] Create layout component at `web/src/lib/components/Layout.svelte`: sidebar navigation (Dashboard, Conversations, Channels, Agents, Settings, Logout), top bar with global search input, user avatar, responsive hamburger menu for mobile (<768px). Include `ReconnectBanner` at top.
- [ ] T012 Create auth guard wrapper at `web/src/lib/components/AuthGuard.svelte`: check session on mount, redirect to `/login?return=<current_url>` if unauthenticated. Wrap all authenticated routes in `App.svelte`.

**Checkpoint**: Foundation ready - user story implementation can now begin in parallel

---

## Phase 3: User Story 6 - Login and Authentication (Priority: P1)

**Goal**: Users can log in, maintain sessions via httponly cookies, and log out. All other routes are gated behind authentication.

**Independent Test**: Open the Web UI, get redirected to login, enter valid credentials, confirm redirect to Dashboard. Close and reopen browser, confirm session persists. Click logout, confirm redirect to login.

### Implementation for User Story 6

- [ ] T013 [US6] Create login page at `web/src/routes/Login.svelte`: username and password fields, "Sign In" button with loading state, error display area. On submit, call `POST /api/v1/auth/login` via API client. On success, redirect to return URL or `/dashboard`. On 401, show "Invalid username or password".
- [ ] T014 [US6] Implement return URL handling in login flow: `AuthGuard` saves current path as `?return=` query param, `Login.svelte` reads it and redirects after successful auth.
- [ ] T015 [US6] Add logout handler: `Layout.svelte` logout button calls `POST /api/v1/auth/logout` via API client, clears auth store, redirects to `/login`.
- [ ] T016 [US6] Handle cross-tab session invalidation: API client 401 interceptor in `web/src/lib/api.ts` clears auth store and redirects to `/login` on any 401 response, preventing partial state in stale tabs.

**Checkpoint**: Login, session persistence, and logout are fully functional. All subsequent stories depend on authenticated access.

---

## Phase 4: User Story 1 - Dashboard and Conversation Browsing (Priority: P1)

**Goal**: Logged-in users see their recent conversations on the Dashboard and can open a conversation to view the full threaded message history with real-time SSE updates.

**Independent Test**: Log in, view dashboard, click into a conversation, confirm a message sent via MCP by an agent appears in the thread within 2 seconds without page refresh.

### Implementation for User Story 1

- [ ] T017 [P] [US1] Create TypeScript types at `web/src/lib/types.ts`: `Conversation` (id, subject, last_message_preview, sender_name, timestamp, unread_count), `Message` (id, conversation_id, sender_name, sender_type, subject, body, priority, timestamp, status), `PaginatedResponse<T>` (items, total, page, per_page).
- [ ] T018 [P] [US1] Create conversation API functions at `web/src/lib/api/conversations.ts`: `getConversations(page, per_page)`, `getConversation(id)`, `getMessages(conversation_id, page, per_page)`, `markAsRead(conversation_id)`.
- [ ] T019 [US1] Create Dashboard page at `web/src/routes/Dashboard.svelte`: fetch conversations sorted by last activity, render list with `LoadingSkeleton` during load, each item shows last message preview, sender name, timestamp, and unread badge. Show `EmptyState` with "No conversations yet" guidance and "New Message" action when empty. Click navigates to `/conversations/:id`.
- [ ] T020 [US1] Create Conversation thread page at `web/src/routes/Conversation.svelte`: fetch messages for conversation ID from route param, render in chronological order, show `LoadingSkeleton` during load. Each message shows sender name, avatar placeholder, timestamp, and body. Long messages (>500 chars) truncated with "Show more" toggle.
- [ ] T021 [US1] Implement infinite scroll pagination in `Conversation.svelte`: load 50 messages initially, detect scroll to top, fetch older page, prepend messages while preserving scroll position.
- [ ] T022 [US1] Implement SSE integration for real-time messages: subscribe to `message.new` events in `Conversation.svelte`, append new messages matching current conversation ID to the bottom of the thread. Update conversation list on Dashboard when `message.new` arrives (increment unread count, update preview).
- [ ] T023 [US1] Call `markAsRead(conversation_id)` when opening a conversation thread, reset unread count to 0 in the conversation list.

**Checkpoint**: Dashboard shows conversations, clicking opens threaded view, real-time messages arrive via SSE, pagination works.

---

## Phase 5: User Story 2 - Compose and Send Messages (Priority: P1)

**Goal**: Users can compose and send direct messages to agents or post messages to channels via a compose form with searchable recipient selection.

**Independent Test**: Open compose form, select an agent recipient, type a message, click send, confirm the message appears in the conversation thread and is received by the target agent via MCP `read_inbox`.

### Implementation for User Story 2

- [ ] T024 [P] [US2] Create message API functions at `web/src/lib/api/messages.ts`: `sendMessage(payload: ComposePayload)`, `searchRecipients(query: string)` returning agents and channels matching the query.
- [ ] T025 [P] [US2] Create `RecipientSearch.svelte` component at `web/src/lib/components/RecipientSearch.svelte`: searchable dropdown input with 300ms debounce, keyboard navigation (arrow keys + enter), virtualized rendering for large lists (>100 items), shows agent/channel name and type icon.
- [ ] T026 [US2] Create compose form at `web/src/lib/components/ComposeForm.svelte`: `RecipientSearch` for recipient selection, subject text input (optional), body textarea (required, validate non-empty), priority selector (1-10 range input, default 5). Submit button with loading state. On success, navigate to the conversation thread. On validation error, show inline "Message body is required".
- [ ] T027 [US2] Add "New Message" button to `Dashboard.svelte` and `Layout.svelte` that opens `ComposeForm` as a modal or navigates to `/compose` route. Wire up route in `App.svelte` if using a dedicated page.

**Checkpoint**: Users can compose messages to agents/channels, messages appear in conversation threads. Combined with US1 and US6, this is the MVP.

---

## Phase 6: User Story 3 - Agent Management and Trace Viewing (Priority: P2)

**Goal**: Owners can view their agents, inspect activity traces with filters, and manage API keys (revoke, regenerate).

**Independent Test**: Navigate to Agents page, click an agent, view traces filtered by action type and date range, revoke API key, confirm agent can no longer authenticate.

### Implementation for User Story 3

- [ ] T028 [P] [US3] Create agent TypeScript types in `web/src/lib/types.ts`: `Agent` (id, name, display_name, type, status, messages_24h), `TraceEntry` (id, agent_name, action, details_summary, details_json, timestamp).
- [ ] T029 [P] [US3] Create agent API functions at `web/src/lib/api/agents.ts`: `getAgents()`, `getAgent(id)`, `getAgentTraces(id, filters: { action_type?, date_from?, date_to?, page, per_page })`, `revokeApiKey(agent_id)`, `regenerateApiKey(agent_id)`.
- [ ] T030 [US3] Create Agents list page at `web/src/routes/Agents.svelte`: fetch and display owned agents in a card/list layout with `LoadingSkeleton` during load. Each agent shows name, display_name, type badge (ai/human), status indicator (active=green, inactive=gray), and messages sent in last 24h. Click navigates to `/agents/:id`.
- [ ] T031 [US3] Create Agent detail page at `web/src/routes/AgentDetail.svelte`: display agent info header (name, display_name, type, status), tabbed sections for "Activity Traces" and "API Key Management".
- [ ] T032 [US3] Implement trace viewer tab in `AgentDetail.svelte`: paginated reverse-chronological trace list, filter controls for action type (dropdown) and date range (date inputs). Each trace row shows action, details_summary, timestamp. Expandable row reveals full JSON details with syntax highlighting.
- [ ] T033 [US3] Implement API key management tab in `AgentDetail.svelte`: "Revoke API Key" button with confirmation dialog ("Are you sure? This agent will no longer be able to authenticate."), "Regenerate API Key" button that shows the new key once in a copiable field with warning "This key will not be shown again."
- [ ] T034 [US3] Subscribe to `agent.status` SSE events in `Agents.svelte` and `AgentDetail.svelte` to update agent status in real-time when keys are revoked or agents go offline.

**Checkpoint**: Agent management is fully functional - list, detail, traces, API key operations, real-time status updates.

---

## Phase 7: User Story 4 - Channel Management (Priority: P2)

**Goal**: Users can browse channels, view details and membership, create new channels, and manage members for owned channels.

**Independent Test**: Navigate to Channels page, create a new public channel, view member list, confirm an agent can join via MCP `join_channel`.

### Implementation for User Story 4

- [ ] T035 [P] [US4] Create channel TypeScript types in `web/src/lib/types.ts`: `Channel` (id, name, description, type, member_count, last_activity, is_owner), `ChannelMember` (agent_id, agent_name, joined_at).
- [ ] T036 [P] [US4] Create channel API functions at `web/src/lib/api/channels.ts`: `getChannels()`, `getChannel(id)`, `getChannelMembers(id)`, `createChannel(name, description, type)`, `inviteToChannel(channel_id, agent_ids)`, `removeFromChannel(channel_id, agent_id)`.
- [ ] T037 [US4] Create Channels list page at `web/src/routes/Channels.svelte`: display public channels and private channels user belongs to, each showing name, description, member count, and last activity. "Create Channel" button at top. `LoadingSkeleton` during load.
- [ ] T038 [US4] Create "Create Channel" modal/form in `web/src/lib/components/CreateChannelForm.svelte`: name input (required), description textarea, type selector (public/private). On success, navigate to the new channel's detail page.
- [ ] T039 [US4] Create Channel detail page at `web/src/routes/ChannelDetail.svelte`: channel info header, message thread (reuse conversation message rendering from `Conversation.svelte` or extract shared `MessageList.svelte` component), member list sidebar.
- [ ] T040 [US4] Implement member management in `ChannelDetail.svelte`: for owned channels, show "Invite" button that opens a searchable agent list (reuse `RecipientSearch`), and "Remove" button next to each member. For non-owned channels, hide management controls (read-only view of members and messages).
- [ ] T041 [US4] Subscribe to `channel.member` SSE events to update member lists in real-time when agents join or leave channels.

**Checkpoint**: Channel browsing, creation, member management, and real-time membership updates are functional.

---

## Phase 8: User Story 5 - Full-Text Search (Priority: P2)

**Goal**: Users can search messages across all conversations and channels, with highlighted results grouped by conversation and direct links to message context.

**Independent Test**: Send several messages with known content via MCP, search for a keyword in the Web UI, confirm matching messages appear with highlighted terms and correct conversation links.

### Implementation for User Story 5

- [ ] T042 [P] [US5] Create search TypeScript types in `web/src/lib/types.ts`: `SearchResult` (conversation_id, message_id, body_preview, sender_name, timestamp, conversation_subject), `SearchResponse` (results grouped by conversation_id, total count).
- [ ] T043 [P] [US5] Create search API function at `web/src/lib/api/search.ts`: `searchMessages(query: string, page?, per_page?)` returning `SearchResponse`.
- [ ] T044 [US5] Implement global search bar in `Layout.svelte`: text input in top bar, submit on Enter, navigate to `/search?q=<query>`. Debounce is NOT needed here (search triggers on explicit submit, not on keystroke).
- [ ] T045 [US5] Create Search results page at `web/src/routes/Search.svelte`: read query from URL params, call search API, display results grouped by conversation. Each result shows message preview with highlighted match term (use `<mark>` tags), sender name, timestamp. Show `EmptyState` "No messages found for '<query>'" when no results.
- [ ] T046 [US5] Implement click-to-context in search results: clicking a result navigates to `/conversations/:conversation_id?highlight=:message_id`, and `Conversation.svelte` scrolls to and visually highlights the target message.

**Checkpoint**: Full-text search works end-to-end with highlighted results, grouped display, and click-through to message context.

---

## Phase 9: User Story 7 - Settings and Dark Mode (Priority: P3)

**Goal**: Users can change password, toggle dark/light mode, and manage preferences from the Settings page.

**Independent Test**: Toggle dark mode in Settings, confirm all pages render correctly in both themes. Change password, confirm login works with new password.

### Implementation for User Story 7

- [ ] T047 [P] [US7] Create settings API functions at `web/src/lib/api/settings.ts`: `changePassword(current_password, new_password)`.
- [ ] T048 [US7] Create Settings page at `web/src/routes/Settings.svelte`: sections for "Appearance" (dark mode toggle) and "Security" (change password form).
- [ ] T049 [US7] Implement dark mode toggle in Settings page: switch component bound to theme store, toggles immediately without reload. Wire to `toggleTheme()` from `web/src/lib/stores/theme.ts`.
- [ ] T050 [US7] Implement change password form in Settings page: current password, new password, confirm new password fields. Client-side validation: new password minimum 8 characters ("Password must be at least 8 characters"), passwords match. On success, show success message. Session remains active.
- [ ] T051 [US7] Audit all components and pages for dark mode support: verify Tailwind `dark:` variant classes are applied to all backgrounds, text colors, borders, inputs, buttons, cards, and modals across every page.

**Checkpoint**: Settings page functional with dark mode toggle and password change. Both themes render correctly across all pages.

---

## Phase 10: Polish & Cross-Cutting Concerns

**Purpose**: Improvements that affect multiple user stories

- [ ] T052 [P] Responsive audit: test all pages at 375px, 768px, 1440px, and 2560px viewport widths. Fix layout issues with Tailwind responsive breakpoints (`sm:`, `md:`, `lg:`, `xl:`). Ensure sidebar collapses to hamburger menu on mobile.
- [ ] T053 [P] Accessibility audit: add ARIA labels to all interactive elements, ensure keyboard navigation works for all dropdowns and modals, verify focus management on route changes, test with screen reader. Target Lighthouse accessibility score >= 90.
- [ ] T054 [P] Extract shared `MessageList.svelte` component from `Conversation.svelte` for reuse in `ChannelDetail.svelte` — unified message rendering with syntax-highlighted code blocks (use a lightweight highlighter like Prism or Shiki).
- [ ] T055 [P] Add loading skeletons to all remaining pages that don't have them: `AgentDetail.svelte` trace list, `ChannelDetail.svelte` member list, `Search.svelte` results.
- [ ] T056 Bundle size optimization: verify `make web` produces a gzipped bundle under 500KB (SC-003). Configure Vite build with tree-shaking, code splitting per route, and minification. Add bundle analyzer script.
- [ ] T057 Error boundary component at `web/src/lib/components/ErrorBoundary.svelte`: catch rendering errors, display user-friendly "Something went wrong" message with retry button instead of blank screen.
- [ ] T058 Verify Go embedding works end-to-end: run `make web && make build`, start binary, confirm all SPA routes serve correctly, API proxy works, and `index.html` fallback handles client-side routing.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies - can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion - BLOCKS all user stories
- **US6 Login (Phase 3)**: Depends on Foundational - BLOCKS all other user stories (auth gates everything)
- **US1 Dashboard (Phase 4)**: Depends on US6 (requires authenticated session)
- **US2 Compose (Phase 5)**: Depends on US6; benefits from US1 (conversation thread to view sent message) but compose API call is independently testable
- **US3 Agents (Phase 6)**: Depends on US6; independent of US1/US2
- **US4 Channels (Phase 7)**: Depends on US6; can reuse components from US1 (message list) and US2 (recipient search)
- **US5 Search (Phase 8)**: Depends on US6 and US1 (search results link to conversation thread view)
- **US7 Settings (Phase 9)**: Depends on US6 and Phase 2 theme system; independent of all other stories
- **Polish (Phase 10)**: Depends on all user stories being complete

### User Story Dependencies

- **US6 (P1, Login)**: Can start after Foundational (Phase 2) - BLOCKS all other stories
- **US1 (P1, Dashboard)**: Can start after US6 - No dependencies on other stories
- **US2 (P1, Compose)**: Can start after US6 - Benefits from US1 for conversation view but independently testable
- **US3 (P2, Agents)**: Can start after US6 - Independent of US1/US2
- **US4 (P2, Channels)**: Can start after US6 - May reuse components from US1/US2
- **US5 (P2, Search)**: Can start after US6 + US1 - Depends on conversation thread view for click-through
- **US7 (P3, Settings)**: Can start after US6 - Independent of all other stories

### Within Each User Story

- Types and API functions (marked [P]) can be built in parallel
- Pages depend on their API functions and types
- SSE integration depends on the base page being rendered
- Story complete before moving to next priority

### Parallel Opportunities

- All Setup tasks T003, T004, T005 can run in parallel (after T001/T002)
- All Foundational tasks T007-T012 marked [P] can run in parallel (after T006)
- After US6 is complete: US1, US2, US3, US4, US7 can start in parallel
- US5 requires US1 to be complete (conversation thread view)
- Within each story: types and API function tasks marked [P] can run in parallel
- All Polish tasks marked [P] can run in parallel

---

## Implementation Strategy

### MVP First (US6 + US1 + US2)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational
3. Complete Phase 3: US6 - Login (gates everything)
4. Complete Phase 4: US1 - Dashboard & Conversations
5. Complete Phase 5: US2 - Compose & Send
6. **STOP and VALIDATE**: Test the core loop - login, browse conversations, send messages, see real-time updates
7. Deploy/demo if ready

### Incremental Delivery

1. Setup + Foundational -> Foundation ready
2. Add US6 Login -> Authenticated access works
3. Add US1 Dashboard -> Browse conversations with real-time updates (MVP!)
4. Add US2 Compose -> Two-way messaging (full MVP!)
5. Add US3 Agents -> Agent oversight and API key management
6. Add US4 Channels -> Channel organization
7. Add US5 Search -> Message discovery at scale
8. Add US7 Settings -> Dark mode and password management
9. Polish -> Responsive, accessible, optimized

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- US6 (Login) is extracted to its own phase before other P1 stories because it gates all authenticated functionality
- Svelte 5 runes (`$state`, `$derived`, `$effect`) should be used throughout instead of Svelte 4 stores
- All API calls go through the shared client in `web/src/lib/api.ts` for consistent auth handling
- Theme initialization happens in `web/index.html` via inline script (before Svelte mounts) to prevent flash
- SSE reconnection logic must track sequence IDs to avoid duplicate or missed messages
- `internal/web/embed.go` must handle SPA fallback: serve `index.html` for any path not matching a static asset
