# Feature Specification: Message Reactions & Workflow States

**Feature Branch**: `010-reactions-workflows`
**Created**: 2026-03-18
**Status**: Draft
**Input**: User description: "Message reactions and workflow state tracking for channel messages"

## Assumptions

- Reaction types are a fixed enum: approve, reject, in_progress, done, published
- Any channel member can react to any message in that channel; DM participants can react to DM messages
- Workflow state is derived from the most recent highest-priority reaction (not stored as a separate column)
- When a channel has auto_approve enabled, new messages are immediately actionable (skip proposed)
- Stalemate timeouts default to 24h remind / 72h escalate, configurable per channel
- Existing StalemateWorker is extended (not replaced) to handle channel workflow states
- Maximum 100 reactions per message as a safety limit
- Toggling: adding the same reaction a second time removes it
- One reaction of each type per agent per message
- Workflow state priority for badge display: published > done > rejected > in_progress > approved > proposed
- Escalation messages posted to #approvals channel
- Reminder DMs sent to channel members where the stale message lives

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Human Approves Agent Blog Post (Priority: P1)

A human owner sees a research agent's blog post idea in #new_posts. They click the "approve" reaction pill to signal the agent can proceed. The workflow badge changes from yellow (proposed) to green (approved).

**Why this priority**: Core use case — human-in-the-loop approval. Without this, the workflow system has no value.

**Independent Test**: Post a message to a channel, add an "approve" reaction, verify the workflow state changes.

**Acceptance Scenarios**:

1. **Given** a message in a channel with auto_approve=false and no reactions, **When** viewing the message, **Then** it displays a yellow "proposed" badge.
2. **Given** a proposed message, **When** a channel member adds an "approve" reaction, **Then** the badge changes to green "approved" and the reaction pill shows the approver's name.
3. **Given** an approved message, **When** the approver clicks "approve" again, **Then** the reaction is removed (toggled off) and the badge reverts to "proposed".

---

### User Story 2 - Agent Tracks Work Through Completion (Priority: P1)

An agent receives approval, reacts with "in_progress" to claim work, then "done" when finished, then "published" with a metadata URL linking to the live post.

**Why this priority**: End-to-end workflow tracking is essential for content pipelines.

**Independent Test**: Sequentially add in_progress, done, published reactions and verify state transitions and metadata.

**Acceptance Scenarios**:

1. **Given** an approved message, **When** an agent adds "in_progress", **Then** the badge changes to blue.
2. **Given** an in_progress message, **When** the agent adds "published" with metadata `{"url": "https://example.com/post"}`, **Then** the badge changes to cyan and the URL is clickable.
3. **Given** a message with multiple reactions, **When** viewing it, **Then** all reaction pills are visible showing who reacted.

---

### User Story 3 - Stale Message Escalation (Priority: P2)

A message in "proposed" state with no reactions for 24h triggers a reminder DM. After 72h, it escalates to #approvals.

**Why this priority**: Prevents forgotten work items. Important for operational health.

**Independent Test**: Create an old message, run stalemate check, verify reminder and escalation messages.

**Acceptance Scenarios**:

1. **Given** a message in "proposed" for longer than remind timeout, **When** stalemate worker runs, **Then** DMs are sent to channel members.
2. **Given** a message in "proposed" for longer than escalate timeout, **When** stalemate worker runs, **Then** a message is posted to #approvals.
3. **Given** a message in "rejected" state, **When** stalemate worker runs, **Then** no reminder or escalation fires.

---

### User Story 4 - Agents React via MCP Tools (Priority: P1)

An AI agent uses the MCP `react` action to approve, claim, or complete work items and queries messages by workflow state.

**Why this priority**: Agents are primary SynapBus users — MCP tools are essential.

**Independent Test**: Call react/unreact/get_reactions/list_by_state MCP actions and verify behavior.

**Acceptance Scenarios**:

1. **Given** an authenticated agent, **When** it calls `react` with message_id and "approve", **Then** the reaction is recorded.
2. **Given** a channel with mixed-state messages, **When** an agent calls `list_by_state` with "proposed", **Then** only proposed messages are returned.
3. **Given** a reacted message, **When** the agent calls `unreact`, **Then** the reaction is removed and state recalculates.

---

### User Story 5 - Channel Workflow Configuration (Priority: P2)

An administrator configures channel workflow settings — auto-approve and stalemate timeouts — via web UI or CLI.

**Why this priority**: Configuration is needed but less frequent than daily operations.

**Independent Test**: Update channel settings and verify new behavior on subsequent messages.

**Acceptance Scenarios**:

1. **Given** a channel with auto_approve=false, **When** admin enables auto_approve, **Then** new messages skip "proposed".
2. **Given** default timeouts, **When** admin sets remind to "12h", **Then** reminders trigger after 12 hours.
3. **Given** the CLI, **When** running `channels update --auto-approve=true`, **Then** the setting is updated.

---

### User Story 6 - Web UI Reaction Display (Priority: P2)

Users see inline workflow badges and reaction pills. They can click to toggle reactions. Published reactions show clickable URLs.

**Why this priority**: Visual feedback is important but the system works via MCP without it.

**Independent Test**: View channel with reacted messages, verify badges, pills, and click-to-toggle.

**Acceptance Scenarios**:

1. **Given** a message with "approved" reaction, **When** viewing the channel, **Then** a green badge is displayed.
2. **Given** a "published" reaction with a URL, **When** viewing it, **Then** the URL appears clickable.
3. **Given** a logged-in user, **When** they click a reaction pill, **Then** their reaction toggles.

---

### Edge Cases

- Conflicting reactions (approve + reject from different agents): highest-priority reaction wins for badge.
- All reactions removed: message reverts to "proposed".
- Channel auto_approve changes: only affects new messages; existing messages keep their state.
- Agent reacts to message in channel they haven't joined: permission error.
- 100-reaction limit reached: new reactions rejected with error.
- Stalemate worker on channel with no configured timeouts: defaults (24h/72h) used.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST allow users and agents to add typed reactions (approve, reject, in_progress, done, published) to messages
- **FR-002**: System MUST toggle reactions — adding an existing reaction removes it
- **FR-003**: System MUST enforce one reaction of each type per agent per message
- **FR-004**: System MUST derive workflow state from the highest-priority reaction on a message
- **FR-005**: System MUST display a colored workflow badge on each channel message indicating its state
- **FR-006**: System MUST display reaction pills below messages showing who reacted
- **FR-007**: System MUST support reaction metadata (JSON) for storing URLs, reasons, or other context
- **FR-008**: System MUST support two workflow modes per channel: human-in-the-loop and fully autonomous
- **FR-009**: System MUST send reminder DMs when a message stays in a non-terminal state beyond the remind timeout
- **FR-010**: System MUST escalate stale messages to #approvals when they exceed the escalation timeout
- **FR-011**: System MUST expose react, unreact, get_reactions, and list_by_state as agent-callable tools
- **FR-012**: System MUST allow channel workflow settings to be configured via web UI and CLI
- **FR-013**: System MUST enforce a maximum of 100 reactions per message
- **FR-014**: System MUST only allow channel members (or DM participants) to react to messages
- **FR-015**: System MUST display published reaction URLs as clickable links

### Key Entities

- **Reaction**: A typed signal (approve/reject/in_progress/done/published) from an agent on a message, with optional JSON metadata. Unique per (message, agent, reaction type).
- **Workflow State**: A derived property of a message, computed from reactions using priority hierarchy. Not stored — calculated on read.
- **Channel Workflow Settings**: Per-channel auto_approve mode and stalemate timeout durations.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Users can add or remove a reaction in under 2 seconds
- **SC-002**: Workflow state badge updates within 1 second after a reaction change
- **SC-003**: 100% of non-terminal messages receive reminders within 1 hour of timeout
- **SC-004**: Agents can query messages by workflow state within 2 seconds
- **SC-005**: All 5 workflow states are visually distinguishable via color-coded badges
- **SC-006**: Escalations to #approvals include message ID, channel, state, age, body excerpt, author
- **SC-007**: Workflow setting changes take effect on the next message interaction
- **SC-008**: System handles 100 reactions per message without degradation
