# Feature Specification: E2E Agent Testing Framework

**Feature Branch**: `011-e2e-agent-testing`
**Created**: 2026-03-13
**Status**: Draft
**Input**: End-to-end testing framework where Claude-powered AI agents communicate through SynapBus MCP, using the Anthropic Python SDK with subscription-based OAuth authentication (macOS Keychain / CLAUDE_CODE_OAUTH_TOKEN fallback).

## Overview

A Python test harness that spins up SynapBus locally, registers AI agents,
and runs Claude-powered agents that autonomously interact through SynapBus
MCP tools (Streamable HTTP transport). Each test scenario validates a
different SynapBus feature — direct messaging, channels, task auctions,
agent discovery, attachments, and semantic search.

### Authentication Strategy (from dialog-engine pattern)

The test harness authenticates with the Anthropic API using a **3-tier
fallback** — no dedicated API key required:

1. `ANTHROPIC_API_KEY` env var (if set)
2. `CLAUDE_CODE_OAUTH_TOKEN` env var (OAuth token)
3. **macOS Keychain** — `security find-generic-password -s "claude-code-credentials"`

This uses the same subscription token as Claude Code / Claude Desktop,
requiring the `anthropic-beta: oauth-2025-04-20` header for OAuth paths.

### Architecture

```
┌─────────────────────────────────────────────────┐
│                Test Harness (Python)             │
│                                                  │
│  ┌──────────┐    ┌──────────┐    ┌──────────┐   │
│  │ Agent A   │    │ Agent B   │    │ Agent C   │  │
│  │ (Claude)  │    │ (Claude)  │    │ (Claude)  │  │
│  └────┬─────┘    └────┬─────┘    └────┬─────┘   │
│       │               │               │          │
│       └───────┬───────┴───────┬───────┘          │
│               │               │                  │
│         ┌─────┴─────┐  ┌─────┴─────┐            │
│         │  MCP Client│  │  MCP Client│            │
│         │  (httpx)   │  │  (httpx)   │            │
│         └─────┬─────┘  └─────┬─────┘            │
└───────────────┼───────────────┼──────────────────┘
                │               │
         ┌──────┴───────────────┴──────┐
         │     SynapBus Server         │
         │   localhost:PORT/mcp        │
         │  (Streamable HTTP + Auth)   │
         └─────────────────────────────┘
```

Each agent is a Python async function that:
1. Holds its own MCP session (Streamable HTTP + Bearer token)
2. Runs a Claude tool-use loop (vanilla `anthropic.AsyncAnthropic`)
3. Has access to SynapBus tools via inline JSON schema definitions
4. Iterates up to N tool rounds, then forces a text-only final response

## User Scenarios & Testing *(mandatory)*

### Scenario 1 — Direct Messaging Round-Trip (Priority: P0)

Two AI agents (Alice, Bob) exchange messages. Alice discovers Bob via
`discover_agents`, sends a research question via `send_message`. Bob
reads inbox via `read_inbox`, claims the message via `claim_messages`,
sends a reply, and marks the original done via `mark_done`. Alice then
reads Bob's reply from her inbox.

**Why P0**: This validates the core SynapBus value proposition — AI agents
autonomously communicating through MCP tools. If this doesn't work,
nothing else matters.

**Acceptance Criteria**:

1. **Given** SynapBus is running and two agents are registered, **When**
   Alice's Claude instance calls `discover_agents`, **Then** the result
   contains Bob with his capabilities.
2. **Given** Alice sent a message to Bob, **When** Bob's Claude instance
   calls `read_inbox`, **Then** the message appears with status `pending`,
   correct sender, body, and subject.
3. **Given** Bob claimed and replied to Alice's message, **When** Alice's
   Claude instance calls `read_inbox`, **Then** Bob's reply appears.
4. **Given** the full round-trip completes, **Then** the test harness
   verifies at least 2 messages exist in the conversation and both agents
   produced coherent responses.

---

### Scenario 2 — Channel Group Communication (Priority: P1)

Three agents join a shared channel. One agent creates a `standard`
channel, invites the others, and broadcasts a question via
`send_channel_message`. The other two agents read their inboxes and
reply on the channel.

**Why P1**: Channels are the primary group communication mechanism.
Multi-agent collaboration patterns depend on this.

**Acceptance Criteria**:

1. **Given** Agent A creates channel "research-team" via `create_channel`,
   **When** agents B and C call `join_channel`, **Then** all three appear
   in the member list.
2. **Given** Agent A broadcasts a message on the channel, **When** agents
   B and C call `read_inbox`, **Then** both receive the channel message.
3. **Given** Agent B replies on the channel, **When** Agent C reads inbox,
   **Then** Agent C sees Bob's channel message.

---

### Scenario 3 — Task Auction (Priority: P1)

An agent posts a task to an `auction` channel. Two other agents bid on
the task. The poster accepts one bid. The winning agent completes the
task.

**Why P1**: Task auctions are the core swarm intelligence pattern. They
enable dynamic work distribution among agents.

**Acceptance Criteria**:

1. **Given** an auction channel exists, **When** Agent A calls `post_task`
   with title, description, and requirements, **Then** the task is created
   with status `open`.
2. **Given** an open task, **When** agents B and C call `bid_task` with
   capabilities and estimates, **Then** both bids are recorded.
3. **Given** two bids exist, **When** Agent A calls `accept_bid` for
   Agent B's bid, **Then** the task status becomes `assigned` and Agent B
   is the assignee.
4. **Given** Agent B is assigned, **When** Agent B calls `complete_task`,
   **Then** the task status becomes `completed`.

---

### Scenario 4 — Agent Discovery (Priority: P2)

An agent with a specific need uses `discover_agents` to find agents
with matching capabilities, then initiates communication with the
best match.

**Why P2**: Discovery enables emergent agent collaboration without
hard-coded routing.

**Acceptance Criteria**:

1. **Given** agents registered with diverse capabilities (research,
   coding, analysis, translation), **When** an agent searches for
   "analysis", **Then** agents with analysis capability appear in results.
2. **Given** discovery results, **When** the agent sends a message to a
   discovered agent, **Then** the message is delivered successfully.

---

### Scenario 5 — Blackboard Stigmergy (Priority: P2)

Agents use a `blackboard` channel as shared state. One agent writes
findings, another reads them and builds on top.

**Why P2**: Stigmergy enables indirect coordination, a key swarm pattern.

**Acceptance Criteria**:

1. **Given** a blackboard channel exists, **When** Agent A writes findings
   via `send_channel_message` with metadata, **Then** Agent B can read the
   findings from the channel.
2. **Given** Agent B reads Agent A's findings, **When** Agent B writes
   additional analysis referencing Agent A's work, **Then** Agent A can
   read the combined state from the channel.

---

## Technical Design

### Project Structure

```
tests/e2e/
├── conftest.py           # Shared fixtures: server lifecycle, agent registration
├── synapbus_mcp.py       # MCP Streamable HTTP client wrapper
├── agent_runner.py       # Claude agent loop (Anthropic SDK + tool execution)
├── auth.py               # 3-tier auth (API key → OAuth token → Keychain)
├── tools.py              # SynapBus tool JSON schemas for Claude
├── test_direct_msg.py    # Scenario 1: Direct messaging
├── test_channels.py      # Scenario 2: Channel group communication
├── test_task_auction.py  # Scenario 3: Task auction
├── test_discovery.py     # Scenario 4: Agent discovery
├── test_blackboard.py    # Scenario 5: Blackboard stigmergy
└── pyproject.toml        # Python deps (anthropic, httpx, pytest, pytest-asyncio)
```

### Authentication Module (`auth.py`)

```python
import os
import subprocess
import anthropic

def create_client() -> anthropic.AsyncAnthropic:
    """Create Anthropic client with 3-tier auth fallback."""
    # Tier 1: Standard API key
    api_key = os.environ.get("ANTHROPIC_API_KEY")
    if api_key:
        return anthropic.AsyncAnthropic(api_key=api_key)

    # Tier 2: OAuth token from env
    oauth_token = os.environ.get("CLAUDE_CODE_OAUTH_TOKEN")

    # Tier 3: macOS Keychain (Claude Code stores JSON with OAuth token)
    if not oauth_token:
        try:
            raw = subprocess.check_output(
                ["security", "find-generic-password",
                 "-s", "Claude Code-credentials", "-w"],
                text=True, stderr=subprocess.DEVNULL,
            ).strip()
            if raw:
                creds = json.loads(raw)
                oauth_token = creds.get("claudeAiOauth", {}).get("accessToken")
        except (subprocess.CalledProcessError, FileNotFoundError, json.JSONDecodeError):
            pass

    if oauth_token:
        return anthropic.AsyncAnthropic(
            auth_token=oauth_token,
            default_headers={"anthropic-beta": "oauth-2025-04-20"},
        )

    raise RuntimeError(
        "No Anthropic credentials found. Set ANTHROPIC_API_KEY, "
        "CLAUDE_CODE_OAUTH_TOKEN, or ensure Claude Code credentials "
        "are in macOS Keychain."
    )
```

### Agent Runner (`agent_runner.py`)

Core loop pattern (from dialog-engine):

```python
async def run_agent(
    client: anthropic.AsyncAnthropic,
    mcp: SynapBusMCP,
    name: str,
    system_prompt: str,
    user_prompt: str,
    tools: list[dict],
    model: str = "claude-sonnet-4-6",
    max_tool_rounds: int = 5,
) -> AgentResult:
    messages = [{"role": "user", "content": user_prompt}]

    for round_num in range(max_tool_rounds + 1):
        api_kwargs = dict(
            model=model,
            max_tokens=2048,
            system=system_prompt,
            messages=messages,
        )
        # Allow tool use except on final round (force text response)
        if round_num < max_tool_rounds:
            api_kwargs["tools"] = tools

        response = await client.messages.create(**api_kwargs)

        has_tool_use = any(b.type == "tool_use" for b in response.content)
        if not has_tool_use or round_num == max_tool_rounds:
            # Extract final text
            text = "\n".join(
                b.text for b in response.content if b.type == "text"
            )
            return AgentResult(text=text, ...)

        # Process tool calls → execute via MCP → feed results back
        assistant_content = [...]
        tool_results = [...]
        for block in response.content:
            if block.type == "tool_use":
                result = mcp.call_tool(block.name, block.input)
                tool_results.append({"type": "tool_result", ...})

        messages.append({"role": "assistant", "content": assistant_content})
        messages.append({"role": "user", "content": tool_results})
```

### MCP Client (`synapbus_mcp.py`)

Thin wrapper over Streamable HTTP JSON-RPC (already implemented in
`tests/e2e/test_two_agents.py`):

- `initialize()` → establishes session, captures `Mcp-Session-Id`
- `call_tool(name, args)` → JSON-RPC `tools/call`, returns parsed result
- `list_tools()` → JSON-RPC `tools/list`
- Bearer token auth via `Authorization` header on every request

### Tool Definitions (`tools.py`)

Inline JSON schemas for Claude's tool-use API, matching SynapBus MCP tools:

- `send_message` (to, body, subject, priority)
- `read_inbox` (limit, status_filter, from_agent)
- `claim_messages` (limit)
- `mark_done` (message_id, status)
- `discover_agents` (query)
- `create_channel` (name, description, type)
- `join_channel` (channel_name)
- `send_channel_message` (channel_name, body)
- `post_task` (channel_name, title, description, requirements)
- `bid_task` (task_id, capabilities, time_estimate, message)
- `accept_bid` (task_id, bid_id)
- `complete_task` (task_id)
- `list_tasks` (channel_name, status)
- `search_messages` (query, limit)

### Test Fixtures (`conftest.py`)

```python
@pytest.fixture(scope="session")
def synapbus_server():
    """Start SynapBus server, yield URL, stop on teardown."""
    port = find_free_port()
    data_dir = tempfile.mkdtemp()
    proc = subprocess.Popen(
        ["./synapbus", "serve", "--port", str(port), "--data", data_dir],
        stdout=subprocess.PIPE, stderr=subprocess.STDOUT,
    )
    wait_for_healthy(f"http://localhost:{port}")
    yield f"http://localhost:{port}"
    proc.terminate()
    shutil.rmtree(data_dir)

@pytest.fixture
async def registered_agents(synapbus_server):
    """Register test user + agents, return dict of {name: (mcp, api_key)}."""
    ...
```

### Test Model

Default model: `claude-sonnet-4-6` (fast, cheap for testing).
Override via `--model` flag or `SYNAPBUS_TEST_MODEL` env var.
Each test scenario costs ~$0.02-0.10 depending on conversation length.

## Non-Functional Requirements

- **No API key required**: MUST work with macOS Keychain subscription token
- **Self-contained**: Tests start/stop SynapBus automatically
- **Idempotent**: Each test uses a fresh data directory
- **Cost-aware**: Print token usage and estimated cost per scenario
- **Timeout**: Each agent turn has a 30s timeout; full test 5 min max
- **Model override**: Tests accept model parameter for cost control

## Dependencies

```toml
[project]
dependencies = [
    "anthropic>=0.49.0",
    "httpx>=0.27",
    "pytest>=8.0",
    "pytest-asyncio>=0.24",
]
```

## Out of Scope

- Streaming responses (batch mode sufficient for testing)
- Attachment upload/download (binary handling adds complexity)
- Semantic search (requires embedding provider configuration)
- Performance/load testing
- CI/CD integration (requires API key management)
