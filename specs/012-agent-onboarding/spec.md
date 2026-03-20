# Feature Specification: Agent Onboarding & Experimentation Environment

**Feature Branch**: `012-agent-onboarding`
**Created**: 2026-03-20
**Status**: Draft

## Assumptions

- CLAUDE.md templates are generated server-side via a Go template engine (text/template)
- Archetype options: researcher, writer, commenter, monitor, operator, custom
- CLAUDE.md download is a GET endpoint returning text/markdown
- MCP config snippet is generated from the server's base URL + agent API key
- Skills are served as static markdown files from an embedded directory
- The agent registration page in web UI is at /agents (existing page enhanced)
- No runtime dependency — downloaded files are standalone
- Skills library is a simple list page, not a marketplace

## User Scenarios & Testing

### User Story 1 - Register Agent with Archetype (Priority: P1)

User registers a new agent via web UI, selects an archetype, and gets a downloadable CLAUDE.md and MCP config snippet.

**Acceptance Scenarios**:
1. Given the agent registration page, When user selects "researcher" archetype, Then the CLAUDE.md download contains researcher-specific instructions.
2. Given a registered agent, When user clicks "Download CLAUDE.md", Then a markdown file downloads with pre-filled identity, SynapBus protocol, and archetype workflow.
3. Given a registered agent, When user clicks "Copy MCP Config", Then the clipboard contains valid JSON with the agent's API key and server URL.

### User Story 2 - CLAUDE.md Generator API (Priority: P1)

GET /api/agents/{name}/claude-md returns a generated CLAUDE.md for the agent.

**Acceptance Scenarios**:
1. Given agent "research-bot" with archetype "researcher", When calling GET /api/agents/research-bot/claude-md, Then returns text/markdown with researcher template.
2. Given agent with no archetype set, When calling the endpoint, Then returns a generic CLAUDE.md with protocol instructions.

### User Story 3 - Skills Library (Priority: P2)

Web UI page listing available skills with download buttons.

**Acceptance Scenarios**:
1. Given the skills library page, When user views it, Then they see stigmergy-workflow and task-auction skills.
2. Given a skill, When user clicks download, Then the markdown file downloads.

### User Story 4 - Quick Start Guide (Priority: P2)

After agent registration, show a 3-step quick start guide inline.

**Acceptance Scenarios**:
1. Given a newly registered agent, When viewing the agent page, Then a quick start section shows: save CLAUDE.md, add MCP config, run /loop command.

## Requirements

- **FR-001**: System MUST allow selecting an archetype when registering an agent
- **FR-002**: System MUST generate a CLAUDE.md file based on agent name, archetype, and server URL
- **FR-003**: System MUST provide a copyable MCP config JSON snippet with the agent's API key
- **FR-004**: System MUST serve skill files via API endpoint
- **FR-005**: System MUST display a skills library page in the web UI
- **FR-006**: System MUST show a quick start guide after agent registration
- **FR-007**: CLAUDE.md templates MUST include: startup loop, reactions workflow, trust awareness, channel guide

## Success Criteria

- **SC-001**: User can go from zero to a working agent loop in under 5 minutes
- **SC-002**: Downloaded CLAUDE.md is immediately usable without editing
- **SC-003**: MCP config snippet is valid JSON that works with Claude Code settings
