# Research: SynapBus v0.6.0 Platform Features

## R1: StalemateWorker Pattern

**Decision**: Follow ExpiryWorker/RetentionWorker pattern — background goroutine with configurable interval.
**Rationale**: Consistent with existing codebase. ExpiryWorker (`internal/channels/expiry.go`) runs every 1min. RetentionWorker (`internal/messaging/retention.go`) runs every 24h. StalemateWorker will run every 15min.
**Alternatives**: CRON-style scheduler (rejected: adds complexity for no benefit), webhook-based (rejected: requires external receiver).

**Key implementation detail**: Must use `messaging.MessagingService.SendMessage()` to send system reminders, ensuring SSE and embedding hooks fire. Must query messages with `to_agent IS NOT NULL` (DMs only) and `from_agent != 'system'` (avoid loops).

## R2: reply_to in Channel Messages

**Decision**: Add `reply_to` parameter to `send_channel_message` action definition in `internal/actions/registry.go`. Pass through to `BroadcastMessage` which already forwards to `SendMessage` with `SendOptions.ReplyTo`.
**Rationale**: The infrastructure already supports reply_to in the messaging layer — it's just missing from the channel action definition and bridge.
**Alternatives**: Separate threading system (rejected: overkill, reply_to already exists in schema).

## R3: A2A Agent Cards

**Decision**: Implement minimal Agent Card generation without the a2a-go SDK. The Agent Card is just a JSON document — no SDK needed for serving it.
**Rationale**: The a2a-go SDK adds a dependency for what amounts to JSON marshaling. A simple handler generating the JSON from the agent registry is lighter and avoids dependency risk.
**Alternatives**: Use a2a-go SDK (considered for gateway in R5).

**Agent Card structure**: One hub-level card at `/.well-known/agent-card.json`. Each agent maps to an `AgentSkill`. Capabilities stored in agents.capabilities JSON column (already exists, currently unused).

## R4: Mobile-Responsive UI

**Decision**: CSS/Svelte only changes. Use Tailwind `md:` breakpoint (768px). Sidebar becomes `fixed` drawer with `translate-x` transition. New `sidebarOpen` state in layout.
**Rationale**: No new dependencies. Tailwind already provides all needed utilities. The existing sidebar is `fixed` positioned — just needs conditional `transform: translateX(-100%)` below md breakpoint.
**Alternatives**: Separate mobile app (rejected: violates Principle I simplicity), Headless UI library (rejected: adds npm dependency).

## R5: A2A Inbound Gateway

**Decision**: Implement minimal JSON-RPC handler without the a2a-go SDK. Support only `message.send`, `tasks.get`, `tasks.cancel` methods. A2A Task maps to a SynapBus conversation with tracking metadata.
**Rationale**: The A2A JSON-RPC protocol is straightforward (3 methods). A dedicated gateway package (`internal/a2a/`) keeps it isolated. Using the SDK would add a dependency that may not be stable yet.
**Alternatives**: Use a2a-go SDK (rejected for v0.6.0: adds dependency complexity, can adopt later if needed).

**New SQLite table**: `a2a_tasks` with id (UUID), context_id, target_agent, conversation_id, state, created_at, updated_at.

## R6: K8s Job Handlers for Reactive Activation

**Decision**: Extend existing `internal/k8s/dispatcher.go` to match `message.mentioned` events. The K8s dispatcher already handles `message.received` and `channel.message` — add mention detection by parsing @agent-name patterns from message body.
**Rationale**: The infrastructure already exists. The gap is that `message.mentioned` isn't wired as an event type, and the admin CLI needs a more user-friendly registration command.
**Alternatives**: Custom CRD controller (rejected: overkill, SynapBus already has built-in K8s support).

## R7: Enterprise Identity Providers

**Decision**: Use `coreos/go-oidc/v3` for OIDC (Google, Azure AD) and `golang.org/x/oauth2` for all OAuth flows (including GitHub). New package `internal/auth/idp/` with `Provider` interface.
**Rationale**: go-oidc is the de facto Go OIDC library (pure Go, 8K stars). oauth2 is already an indirect dependency. This combination handles all three providers with minimal new code.
**Alternatives**: markbates/goth (rejected: too many transitive dependencies, opinionated session handling conflicts with SynapBus's existing model).

**New tables**: `user_identities` (links external IDs to local users), `identity_providers` (IdP configuration storage, admin-managed).

## R8: CLAUDE.md / GEMINI.md Protocol

**Decision**: Add a comprehensive "SynapBus Communication Protocol" section to the project CLAUDE.md. Include inbox check mandate, claim-done loop, ACK/DONE convention, channel routing rules, message format templates, and StalemateWorker awareness.
**Rationale**: Advisory instructions in CLAUDE.md are the primary mechanism for guiding agent behavior. Combined with SessionStart hooks, this provides both advisory and deterministic enforcement.
**Alternatives**: Skill-only approach (rejected: skills must be explicitly invoked, CLAUDE.md is always in context).
