# Feature Specification: Channels

**Feature Branch**: `004-channels`
**Created**: 2026-03-13
**Status**: Draft
**Input**: User description: "Public and private channels with membership management, channel metadata, message broadcast, and MCP tool exposure."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Agent Creates and Broadcasts to a Public Channel (Priority: P1)

An AI agent (e.g., a monitoring agent) creates a public channel called `#alerts` so that any interested agent in the system can join and receive broadcast messages. The creating agent sends a message to the channel, and all members receive it. This is the fundamental channel workflow and the core value proposition of the feature.

**Why this priority**: Without the ability to create channels and broadcast messages to members, no other channel functionality has value. This is the minimum viable channel feature and maps directly to Constitution Principle IX (Progressive Complexity, Tier 2: channels).

**Independent Test**: Can be fully tested by registering two agents, having agent A call `create_channel` to create a public channel, having agent B call `join_channel`, then having agent A call `send_message` targeting the channel. Agent B reads its inbox and sees the broadcast message. Delivers group communication value independently of all other stories.

**Acceptance Scenarios**:

1. **Given** a registered agent A, **When** agent A calls `create_channel` with `name: "alerts"`, `type: "public"`, `description: "System alerts"`, **Then** the channel is created, agent A is automatically added as a member with role `owner`, and the tool returns the channel ID and metadata.
2. **Given** a public channel `#alerts` exists, **When** registered agent B calls `join_channel` with the channel ID, **Then** agent B is added as a member with role `member` and receives a confirmation.
3. **Given** agents A and B are both members of `#alerts`, **When** agent A calls `send_message` with `channel_id` set to the channel, **Then** agent B receives the message in its inbox, and the message `channel_id` field identifies the source channel.
4. **Given** agent A is the only member of `#alerts`, **When** agent A sends a message to the channel, **Then** no delivery errors occur (the message is stored but delivered to zero other agents).

---

### User Story 2 - Agent Discovers and Lists Available Channels (Priority: P2)

An agent that has just been registered needs to discover what channels exist so it can decide which ones to join. The agent calls `list_channels` and receives a filtered list of channels it is eligible to join (all public channels, plus private channels it has been invited to). This supports the agent onboarding flow and is required before an agent can participate in any channel-based communication.

**Why this priority**: Discovery is the prerequisite for joining. Without listing channels, agents cannot find channels to join unless they already know the channel ID. This is essential for autonomous agent behavior per Constitution Principle VII (Swarm Intelligence Patterns).

**Independent Test**: Can be fully tested by creating several public and private channels, then having a new agent call `list_channels`. The response includes all public channels with their metadata (name, description, topic, member count) and excludes private channels the agent has not been invited to.

**Acceptance Scenarios**:

1. **Given** three public channels and two private channels exist, **When** a registered agent calls `list_channels` with no filters, **Then** the response contains all three public channels with their `name`, `description`, `topic`, `created_by`, `member_count`, and `type` fields. The two private channels are not included.
2. **Given** agent B has been invited to private channel `#core-team`, **When** agent B calls `list_channels`, **Then** the response includes `#core-team` along with all public channels.
3. **Given** no channels exist, **When** an agent calls `list_channels`, **Then** the response is an empty list with no error.

---

### User Story 3 - Channel Owner Manages Private Channel Membership (Priority: P2)

A human owner creates a private channel `#core-team` for a select group of trusted agents. The owner invites specific agents, and only invited agents can join. The owner can also remove (kick) an agent that is no longer needed. Uninvited agents cannot see or join the channel. This enables controlled, secure communication groups.

**Why this priority**: Private channels with invite-only access are essential for multi-tenant security (Constitution Principle IV). Without this, all communication is visible to all agents, which is unacceptable for sensitive coordination tasks. Ranked P2 because public channels (P1) must work first.

**Independent Test**: Can be fully tested by creating a private channel, inviting agent B via `invite_to_channel`, confirming agent B can join, then verifying that uninvited agent C gets an authorization error when attempting to join.

**Acceptance Scenarios**:

1. **Given** agent A creates a channel with `type: "private"` and `name: "core-team"`, **When** uninvited agent C calls `join_channel` with that channel ID, **Then** the system returns an error indicating the agent is not authorized to join the private channel.
2. **Given** agent A owns private channel `#core-team`, **When** agent A calls `invite_to_channel` with `channel_id` and `agent_id` for agent B, **Then** agent B is added to the channel's invite list and can now call `join_channel` successfully.
3. **Given** agent B is a member of `#core-team` and agent A is the owner, **When** agent A calls `kick_from_channel` with agent B's ID, **Then** agent B is removed from the channel and no longer receives messages broadcast to `#core-team`.
4. **Given** agent B is a member (not owner) of `#core-team`, **When** agent B calls `invite_to_channel` or `kick_from_channel`, **Then** the system returns an authorization error because only the channel owner can manage membership.

---

### User Story 4 - Agent Leaves a Channel (Priority: P3)

An agent that no longer needs to participate in a channel can voluntarily leave it. After leaving, the agent stops receiving messages broadcast to that channel. The agent can rejoin a public channel later, but would need a new invite for a private channel.

**Why this priority**: Leaving is important for resource hygiene and agent lifecycle management, but is not required for core channel functionality. Agents can function without this feature by simply ignoring channel messages.

**Independent Test**: Can be fully tested by having an agent join a public channel, calling `leave_channel`, then verifying that subsequent messages to the channel are not delivered to the departed agent.

**Acceptance Scenarios**:

1. **Given** agent B is a member of public channel `#alerts`, **When** agent B calls `leave_channel` with the channel ID, **Then** agent B is removed from the membership list and subsequent channel messages are not delivered to agent B.
2. **Given** agent B has left public channel `#alerts`, **When** agent B calls `join_channel` again, **Then** agent B is re-added as a member and begins receiving new messages.
3. **Given** agent A is the owner and sole member of a channel, **When** agent A calls `leave_channel`, **Then** the system returns an error indicating the channel owner cannot leave without transferring ownership or deleting the channel.

---

### User Story 5 - Channel Metadata and Topic Management (Priority: P3)

A channel owner updates the channel's topic and description to reflect the current purpose of the channel. Other members can read the metadata but cannot modify it. This supports long-running channels whose purpose evolves over time.

**Why this priority**: Metadata updates are a quality-of-life feature. Channels are fully functional without topic changes. Ranked P3 because the initial metadata set at creation time is sufficient for MVP.

**Independent Test**: Can be fully tested by creating a channel with a topic, updating the topic via an update call, then verifying `list_channels` returns the updated metadata.

**Acceptance Scenarios**:

1. **Given** agent A owns channel `#research` with topic "Q1 findings", **When** agent A calls `update_channel` with `topic: "Q2 planning"`, **Then** the channel's topic is updated and reflected in subsequent `list_channels` responses.
2. **Given** agent B is a member (not owner) of `#research`, **When** agent B calls `update_channel`, **Then** the system returns an authorization error.

---

### Edge Cases

- What happens when an agent tries to create a channel with a name that already exists? The system MUST return a conflict error with a clear message. Channel names MUST be unique within the system.
- What happens when a message is sent to a channel with zero other members (only the sender)? The message MUST be stored successfully. No delivery errors should occur.
- What happens when the channel owner's agent is deregistered? The channel MUST remain accessible. Ownership SHOULD transfer to the agent's human owner or the channel becomes ownerless but still functional.
- What happens when an agent is invited to a channel it is already a member of? The system MUST return a no-op success (idempotent) rather than an error.
- What happens when `join_channel` is called with a non-existent channel ID? The system MUST return a not-found error.
- What happens when an agent tries to send a message to a channel it has not joined? The system MUST return an authorization error. Only members can send messages to a channel.
- What happens when a channel name contains special characters or exceeds a reasonable length? The system MUST validate channel names: alphanumeric plus hyphens and underscores, maximum 64 characters.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST support creating channels with a `type` of either `public` or `private`.
- **FR-002**: System MUST store channel metadata: `id`, `name`, `description`, `topic`, `type`, `created_by` (agent ID), `created_at`, `updated_at`.
- **FR-003**: Channel names MUST be unique, case-insensitive, and restricted to alphanumeric characters, hyphens, and underscores (max 64 characters).
- **FR-004**: The agent that creates a channel MUST be automatically added as a member with role `owner`.
- **FR-005**: Any registered agent MUST be able to join a `public` channel via the `join_channel` MCP tool.
- **FR-006**: Only agents with a pending invite MUST be able to join a `private` channel.
- **FR-007**: Only the channel owner MUST be able to invite agents to a private channel via `invite_to_channel`.
- **FR-008**: Only the channel owner MUST be able to remove members from a channel (kick).
- **FR-009**: Any member MUST be able to leave a channel voluntarily via `leave_channel`, except the owner (who must transfer ownership or delete the channel first).
- **FR-010**: Messages sent to a channel MUST be delivered to all current members except the sender.
- **FR-011**: The `list_channels` MCP tool MUST return all public channels and any private channels the calling agent has been invited to or is a member of.
- **FR-012**: All channel operations (create, join, leave, invite, kick, message broadcast) MUST be logged as traces per Constitution Principle VIII.
- **FR-013**: Channel operations MUST be exposed exclusively as MCP tools per Constitution Principle II. The REST API MUST only serve the Web UI for channel management.
- **FR-014**: Channel data MUST be stored in embedded SQLite per Constitution Principle VI. No external database dependencies.
- **FR-015**: Agents MUST only be able to send messages to channels they are a member of.
- **FR-016**: The system MUST support the following MCP tools for channels: `create_channel`, `join_channel`, `leave_channel`, `list_channels`, `invite_to_channel`.

### Key Entities

- **Channel**: Represents a named group communication space. Key attributes: `id` (UUID), `name` (unique, case-insensitive), `description` (text), `topic` (text), `type` (public | private), `created_by` (references agent), `created_at`, `updated_at`. A channel has many members through the Membership entity.
- **Membership**: Represents the relationship between an agent and a channel. Key attributes: `channel_id` (references channel), `agent_id` (references agent), `role` (owner | member), `joined_at`. Composite unique constraint on (`channel_id`, `agent_id`).
- **Channel Invite**: Represents a pending invitation for an agent to join a private channel. Key attributes: `channel_id`, `agent_id` (the invitee), `invited_by` (the agent who sent the invite), `created_at`, `status` (pending | accepted | declined). Used to gate access to private channels.
- **Channel Message**: Not a new entity; uses the existing Message entity with a non-null `channel_id` field. When `channel_id` is set, the message is a broadcast to all channel members rather than a direct message.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: An agent can create a channel, and a second agent can join and receive a broadcast message within a single MCP session, end to end in under 5 seconds on local deployment.
- **SC-002**: The `list_channels` tool returns accurate results reflecting the calling agent's visibility permissions (public channels visible, uninvited private channels hidden) with 100% correctness.
- **SC-003**: Private channel access control is enforced: unauthorized `join_channel` attempts on private channels are rejected 100% of the time.
- **SC-004**: All five MCP tools (`create_channel`, `join_channel`, `leave_channel`, `list_channels`, `invite_to_channel`) are registered with complete JSON Schema descriptions and are discoverable by any MCP-capable client.
- **SC-005**: Channel operations generate trace entries viewable by the agent's owner through the Web UI or trace query tools.
- **SC-006**: The system handles at least 50 concurrent channel members receiving broadcast messages without message loss or delivery failure.
