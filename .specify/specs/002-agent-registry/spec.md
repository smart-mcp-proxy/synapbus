# Feature Specification: Agent Registry & Auth

**Feature Branch**: `002-agent-registry`
**Created**: 2026-03-13
**Status**: Draft
**Input**: User description: "Agent self-registration, API key auth, capability cards, owner-scoped access, CRUD via MCP tools"

## Constitution Compliance

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Local-First, Single Binary | Compliant | Agent registry stored in embedded SQLite. No external auth service. |
| II. MCP-Native | Compliant | All agent operations exposed as MCP tools. REST API used only by Web UI. |
| III. Pure Go, Zero CGO | Compliant | No new dependencies requiring CGO. API key generation uses `crypto/rand`. |
| IV. Multi-Tenant with Ownership | Compliant | Every agent has an `owner_id`. Agents only access own messages + joined channels. |
| V. Embedded OAuth 2.1 | Not Applicable | OAuth is for human users. Agents use API key auth. Future spec will integrate agent keys with OAuth token flow. |
| VIII. Observable by Default | Compliant | All registration, update, and deregistration actions generate trace entries. |
| IX. Progressive Complexity | Compliant | Registration is Tier 1 (basic). Capability cards and discovery are Tier 2 (intermediate). |

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Agent Self-Registration (Priority: P1)

An AI agent connects to SynapBus via MCP and calls `register_agent` with its name, display name, type, and owner credentials. SynapBus creates the agent record, generates a unique API key, and returns it in the response. The agent stores this key and uses it for all subsequent MCP tool calls. The API key is shown exactly once and never retrievable again.

**Why this priority**: Without registration, no agent can authenticate or use any other SynapBus feature. This is the foundational capability that everything else depends on.

**Independent Test**: Can be fully tested by calling `register_agent` via an MCP client and verifying the returned API key works for a subsequent `read_inbox` call. Delivers value as a standalone identity system.

**Acceptance Scenarios**:

1. **Given** a running SynapBus instance with a registered human owner (owner_id: "alice"), **When** an agent calls `register_agent` with `{name: "research-bot", display_name: "Research Bot", type: "ai", owner_id: "alice", capabilities: {"skills": ["web-search", "summarization"]}}`, **Then** the system returns `{agent_id: "<uuid>", api_key: "sk-synapbus-<random>", name: "research-bot", created_at: "<timestamp>"}` and the agent record is persisted in SQLite.

2. **Given** an agent "research-bot" already exists, **When** another agent calls `register_agent` with `{name: "research-bot", ...}`, **Then** the system returns an error: `{"error": "agent_name_taken", "message": "An agent with name 'research-bot' already exists"}` and no record is created.

3. **Given** a registered agent with a valid API key, **When** the agent includes the API key in the MCP connection headers (`Authorization: Bearer sk-synapbus-...`), **Then** all subsequent MCP tool calls are authenticated as that agent and scoped to its permissions.

4. **Given** a registered agent, **When** any tool call is made with an invalid or missing API key, **Then** the system returns `{"error": "unauthorized", "message": "Invalid or missing API key"}` and the request is rejected.

---

### User Story 2 - Agent Discovery by Capability (Priority: P2)

A coordinating agent needs to find other agents that can perform a specific task (e.g., "sentiment analysis"). It calls `discover_agents` with a capability query, and SynapBus returns a list of matching agents with their capability cards. The coordinating agent can then send messages or assign tasks to the discovered agents.

**Why this priority**: Discovery enables multi-agent coordination, which is core to SynapBus's swarm intelligence value proposition. Without discovery, agents must be hardcoded to know about each other.

**Independent Test**: Can be tested by registering 3-4 agents with different capabilities, then calling `discover_agents` with various queries and verifying correct filtering. Delivers value as a standalone agent directory.

**Acceptance Scenarios**:

1. **Given** three registered agents: "sentiment-bot" (capabilities: `{"skills": ["sentiment-analysis", "text-classification"]}`), "search-bot" (capabilities: `{"skills": ["web-search", "summarization"]}`), and "translate-bot" (capabilities: `{"skills": ["translation", "text-classification"]}`), **When** an agent calls `discover_agents` with `{capability: "text-classification"}`, **Then** the system returns `[{name: "sentiment-bot", display_name: "Sentiment Bot", type: "ai", capabilities: {...}}, {name: "translate-bot", display_name: "Translate Bot", type: "ai", capabilities: {...}}]`.

2. **Given** registered agents exist, **When** an agent calls `discover_agents` with `{capability: "quantum-computing"}`, **Then** the system returns an empty list `[]` with no error.

3. **Given** registered agents exist, **When** an agent calls `discover_agents` with `{}` (no filter), **Then** the system returns all agents visible to the caller, with each entry including the full capability card.

---

### User Story 3 - Agent Lifecycle Management (Priority: P2)

An agent owner needs to update an agent's capabilities as the agent evolves (e.g., after fine-tuning adds a new skill) or deregister an agent that is no longer needed. Only the agent's owner (the human who registered it) can perform these operations. The agent itself can update its own capabilities but cannot change its owner or deregister itself.

**Why this priority**: Agents evolve over time. Without update and deregister, the registry becomes stale and inaccurate, undermining discovery. Owner-only control is required by Constitution Principle IV.

**Independent Test**: Can be tested by registering an agent, updating its capabilities, verifying the change in `discover_agents` results, then deregistering and verifying removal. Delivers value as a complete lifecycle for agent identity management.

**Acceptance Scenarios**:

1. **Given** agent "research-bot" is registered with owner "alice" and capabilities `{"skills": ["web-search"]}`, **When** "research-bot" calls `update_agent` with `{capabilities: {"skills": ["web-search", "summarization", "citation-extraction"]}}`, **Then** the agent's capability card is updated and subsequent `discover_agents` calls reflect the new capabilities.

2. **Given** agent "research-bot" is registered with owner "alice", **When** owner "alice" calls `deregister_agent` with `{agent_name: "research-bot"}`, **Then** the agent record is soft-deleted (marked inactive), the API key is invalidated, and the agent no longer appears in `discover_agents` results.

3. **Given** agent "research-bot" is registered with owner "alice", **When** owner "bob" calls `deregister_agent` with `{agent_name: "research-bot"}`, **Then** the system returns `{"error": "forbidden", "message": "Only the agent's owner can deregister it"}` and the agent remains active.

4. **Given** agent "research-bot" is registered, **When** "research-bot" calls `update_agent` attempting to change `owner_id` to "bob", **Then** the system returns `{"error": "forbidden", "message": "Agents cannot change their own owner"}` and the owner remains unchanged.

---

### User Story 4 - Owner-Scoped Access Enforcement (Priority: P1)

When an agent makes any MCP tool call, SynapBus enforces that the agent can only access its own direct messages and channels it has explicitly joined. An agent cannot read another agent's inbox, send messages impersonating another agent, or list channels it has not joined (except public channel discovery).

**Why this priority**: Security isolation is a hard requirement from Constitution Principle IV. Without it, any agent could read or tamper with another agent's data, making the system untrustworthy.

**Independent Test**: Can be tested by registering two agents with different owners, having each send messages, and verifying that neither can read the other's inbox or access unauthorized channels. Delivers value as a security boundary.

**Acceptance Scenarios**:

1. **Given** agent "bot-a" (owner: "alice") and agent "bot-b" (owner: "bob") are registered, and "bot-a" has received a message, **When** "bot-b" calls `read_inbox`, **Then** "bot-b" sees only its own messages and cannot see "bot-a"'s messages.

2. **Given** agent "bot-a" is authenticated, **When** "bot-a" calls `send_message` with `{from: "bot-b", to: "bot-c", body: "..."}`, **Then** the system rejects the request with `{"error": "forbidden", "message": "Cannot send messages as another agent"}`. The `from` field is always set server-side from the authenticated identity.

3. **Given** a private channel "alpha-team" that "bot-a" has not joined, **When** "bot-a" calls `list_channels`, **Then** "alpha-team" does not appear in the results. Public channels are visible for discovery purposes regardless of membership.

---

### Edge Cases

- **What happens when an agent registers with an empty or whitespace-only name?** The system MUST reject registration with a validation error. Agent names MUST match the pattern `^[a-z0-9][a-z0-9._-]{0,62}[a-z0-9]$` (lowercase alphanumeric, dots, hyphens, underscores; 2-64 characters).
- **What happens when an agent's API key is compromised?** The owner MUST be able to rotate the API key via `update_agent` with `{rotate_key: true}`. The old key is immediately invalidated and a new key is returned (shown once).
- **What happens when an owner is deleted but still has registered agents?** All agents owned by the deleted owner MUST be soft-deleted (deregistered) and their API keys invalidated. This is a cascading operation.
- **What happens when `discover_agents` is called with a very broad query matching hundreds of agents?** Results MUST be paginated with a default limit of 50 and a maximum limit of 200. The response includes `total_count` and `next_cursor` for pagination.
- **What happens when an agent calls `register_agent` with capabilities exceeding the size limit?** The capabilities JSON MUST be capped at 64 KB. Requests exceeding this MUST be rejected with `{"error": "payload_too_large", "message": "Capabilities JSON must not exceed 64 KB"}`.
- **What happens when a deregistered agent's API key is used?** The system MUST return `{"error": "unauthorized", "message": "Agent has been deregistered"}` with no indication of whether the key was ever valid (to prevent enumeration).
- **How does the system handle concurrent registration of the same agent name?** The SQLite UNIQUE constraint on `agent.name` prevents duplicates. The second concurrent request receives an `agent_name_taken` error.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST allow agents to self-register via the `register_agent` MCP tool, providing name, display_name, type, capabilities, and owner_id.
- **FR-002**: System MUST generate a cryptographically random API key (minimum 256 bits of entropy) on registration and return it exactly once in the registration response.
- **FR-003**: System MUST store API keys as bcrypt hashes in SQLite. Raw API keys MUST NOT be stored or logged.
- **FR-004**: System MUST enforce unique agent names across the entire instance (case-insensitive).
- **FR-005**: System MUST authenticate all MCP tool calls via the `Authorization: Bearer <api_key>` header, matching against stored bcrypt hashes.
- **FR-006**: System MUST expose `discover_agents` as an MCP tool that accepts optional filters: capability keyword, agent type, and owner_id. Results MUST include the full capability card for each matching agent.
- **FR-007**: System MUST expose `update_agent` as an MCP tool allowing the authenticated agent to update its own `display_name` and `capabilities`. Owner_id and name MUST be immutable after registration.
- **FR-008**: System MUST expose `deregister_agent` as an MCP tool that soft-deletes the agent record. Only the agent's owner (authenticated as a human user) can deregister an agent.
- **FR-009**: System MUST enforce owner-scoped access: agents can only read their own inbox, send messages as themselves, and access channels they have joined.
- **FR-010**: System MUST support API key rotation via `update_agent` with `rotate_key: true`. The old key is immediately invalidated and a new key is returned.
- **FR-011**: System MUST log all registry operations (register, update, deregister, key rotation) as trace entries with agent identity, action type, and timestamp (Constitution Principle VIII).
- **FR-012**: System MUST validate the capabilities field as valid JSON conforming to a defined capability card schema. Invalid JSON or schema violations MUST be rejected.
- **FR-013**: System MUST support pagination for `discover_agents` results with cursor-based pagination (default limit: 50, max: 200).
- **FR-014**: System MUST cascade soft-delete all agents when their owner account is deleted, invalidating all associated API keys.

### Key Entities

- **Agent**: Represents a registered entity (AI or human) that can send/receive messages. Key attributes: `id` (UUID), `name` (unique, immutable), `display_name`, `type` (enum: "ai", "human"), `capabilities` (JSON capability card), `owner_id` (FK to human user), `api_key_hash` (bcrypt), `status` (enum: "active", "inactive"), `created_at`, `updated_at`, `deregistered_at`. An agent belongs to exactly one owner. An owner can have many agents.

- **Capability Card**: A structured JSON document describing what an agent can do. Key attributes: `skills` (array of string keywords for discovery matching), `description` (human-readable summary of the agent's purpose), `input_formats` (array of MIME types the agent can process), `output_formats` (array of MIME types the agent can produce), `version` (semver string for the agent's current version). Used by `discover_agents` for keyword matching and by other agents to understand how to interact.

- **Trace Entry** (registry-related): An audit record of a registry operation. Key attributes: `id`, `agent_id`, `action` (enum: "register", "update", "deregister", "key_rotate"), `details` (JSON with before/after state), `performed_by` (agent or owner who performed the action), `timestamp`. Stored in the shared traces table per Constitution Principle VIII.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: An agent can complete self-registration (call `register_agent` and receive a working API key) in under 500ms on commodity hardware.
- **SC-002**: API key authentication adds no more than 5ms of latency to any MCP tool call (bcrypt verify is cached for active sessions).
- **SC-003**: `discover_agents` returns results for a keyword query across 1,000 registered agents in under 200ms.
- **SC-004**: All four MCP tools (`register_agent`, `discover_agents`, `update_agent`, `deregister_agent`) have integration tests covering the acceptance scenarios above, with 100% pass rate.
- **SC-005**: No agent can access another agent's messages or channels it has not joined, verified by negative-path integration tests (at least 5 access-violation test cases).
- **SC-006**: API keys are never present in logs, traces, error messages, or database records in plaintext, verified by a grep-based audit of all log output during integration tests.
- **SC-007**: Owner cascade delete correctly deregisters all owned agents and invalidates their keys within a single SQLite transaction, verified by integration test.
