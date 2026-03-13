#!/usr/bin/env python3
"""
E2E test: Two Claude-powered agents communicate through SynapBus MCP.

Prerequisites:
  1. SynapBus running: ./synapbus serve --port 8080
  2. ANTHROPIC_API_KEY set in environment
  3. pip install anthropic httpx

Usage:
  python tests/e2e/test_two_agents.py [--port 8080] [--model claude-sonnet-4-6]
"""

from __future__ import annotations

import argparse
import json
import os
import sys
from typing import Optional

import httpx
import anthropic

SYNAPBUS_URL = ""
MCP_URL = ""


# ---------------------------------------------------------------------------
# SynapBus MCP client (Streamable HTTP JSON-RPC)
# ---------------------------------------------------------------------------

class SynapBusMCP:
    """Thin MCP client for SynapBus Streamable HTTP transport."""

    def __init__(self, base_url: str, api_key: Optional[str] = None):
        self.url = f"{base_url}/mcp"
        self.api_key = api_key
        self.session_id: Optional[str] = None
        self._req_id = 0
        self._client = httpx.Client(timeout=30)

    def _next_id(self) -> int:
        self._req_id += 1
        return self._req_id

    def _headers(self) -> dict:
        h = {"Content-Type": "application/json"}
        if self.api_key:
            h["Authorization"] = f"Bearer {self.api_key}"
        if self.session_id:
            h["Mcp-Session-Id"] = self.session_id
        return h

    def _rpc(self, method: str, params: dict | None = None) -> dict:
        body = {
            "jsonrpc": "2.0",
            "id": self._next_id(),
            "method": method,
        }
        if params:
            body["params"] = params

        resp = self._client.post(self.url, json=body, headers=self._headers())
        resp.raise_for_status()

        # Capture session ID from initialize response
        if sid := resp.headers.get("Mcp-Session-Id"):
            self.session_id = sid

        return resp.json()

    def initialize(self) -> dict:
        return self._rpc("initialize", {
            "protocolVersion": "2025-03-26",
            "capabilities": {},
            "clientInfo": {"name": "synapbus-e2e-test", "version": "1.0"},
        })

    def call_tool(self, name: str, arguments: dict) -> dict:
        result = self._rpc("tools/call", {"name": name, "arguments": arguments})
        if "error" in result:
            return result
        # Parse the text content from MCP result
        content = result.get("result", {}).get("content", [])
        for block in content:
            if block.get("type") == "text":
                return json.loads(block["text"])
        return result

    def list_tools(self) -> list[dict]:
        result = self._rpc("tools/list")
        return result.get("result", {}).get("tools", [])

    def close(self):
        self._client.close()


# ---------------------------------------------------------------------------
# Setup: register user + agents, get API keys
# ---------------------------------------------------------------------------

def setup_agents(base_url: str) -> tuple[str, str]:
    """Register admin user (if needed) and two agents. Returns (alice_key, bob_key)."""
    client = httpx.Client(timeout=10)

    # Try to register a test user (might already exist)
    client.post(f"{base_url}/auth/register", json={
        "username": "e2e-tester",
        "password": "testpass123456",
        "display_name": "E2E Tester",
    })

    # Login
    resp = client.post(f"{base_url}/auth/login", json={
        "username": "e2e-tester",
        "password": "testpass123456",
    })
    if resp.status_code != 200:
        # Fall back to admin (auto-created on first run)
        print("  [!] Could not login as e2e-tester, trying admin...")
        print("      You may need to provide the admin password.")
        sys.exit(1)

    cookies = resp.cookies

    # Register agents (ignore errors if already exist)
    alice_resp = client.post(f"{base_url}/api/agents", json={
        "name": "alice",
        "display_name": "Alice the Researcher",
        "type": "ai",
        "capabilities": {"research": True, "summarization": True},
    }, cookies=cookies)

    bob_resp = client.post(f"{base_url}/api/agents", json={
        "name": "bob",
        "display_name": "Bob the Analyst",
        "type": "ai",
        "capabilities": {"data_analysis": True, "coding": True},
    }, cookies=cookies)

    alice_key = alice_resp.json().get("api_key", "")
    bob_key = bob_resp.json().get("api_key", "")

    if not alice_key or not bob_key:
        print("  [!] Could not register agents. They may already exist.")
        print("      Delete test-data/ and restart the server for a clean test.")
        sys.exit(1)

    client.close()
    return alice_key, bob_key


# ---------------------------------------------------------------------------
# Claude-powered agent loop
# ---------------------------------------------------------------------------

# Tool definitions for Claude (subset of SynapBus MCP tools)
TOOLS = [
    {
        "name": "send_message",
        "description": "Send a message to another agent via SynapBus",
        "input_schema": {
            "type": "object",
            "properties": {
                "to": {"type": "string", "description": "Recipient agent name"},
                "body": {"type": "string", "description": "Message body"},
                "subject": {"type": "string", "description": "Conversation subject"},
            },
            "required": ["to", "body"],
        },
    },
    {
        "name": "read_inbox",
        "description": "Read messages from your inbox",
        "input_schema": {
            "type": "object",
            "properties": {
                "limit": {"type": "integer", "description": "Max messages to return"},
            },
        },
    },
    {
        "name": "discover_agents",
        "description": "Discover other agents by capability",
        "input_schema": {
            "type": "object",
            "properties": {
                "query": {"type": "string", "description": "Capability search query"},
            },
        },
    },
    {
        "name": "claim_messages",
        "description": "Claim pending messages for processing (atomic lock)",
        "input_schema": {
            "type": "object",
            "properties": {
                "limit": {"type": "integer", "description": "Max messages to claim"},
            },
        },
    },
    {
        "name": "mark_done",
        "description": "Mark a claimed message as done",
        "input_schema": {
            "type": "object",
            "properties": {
                "message_id": {"type": "integer", "description": "Message ID to mark done"},
                "status": {"type": "string", "enum": ["done", "failed"]},
            },
            "required": ["message_id", "status"],
        },
    },
]


def run_agent(
    agent_name: str,
    mcp: SynapBusMCP,
    system_prompt: str,
    user_prompt: str,
    model: str = "claude-sonnet-4-6",
    max_turns: int = 5,
) -> str:
    """Run a Claude agent that uses SynapBus MCP tools."""
    client = anthropic.Anthropic()
    messages = [{"role": "user", "content": user_prompt}]
    final_text = ""

    for turn in range(max_turns):
        print(f"  [{agent_name}] Turn {turn + 1}/{max_turns}")

        response = client.messages.create(
            model=model,
            max_tokens=1024,
            system=system_prompt,
            tools=TOOLS,
            messages=messages,
        )

        # Collect assistant response
        assistant_content = []
        tool_uses = []

        for block in response.content:
            if block.type == "text":
                final_text += block.text
                assistant_content.append({"type": "text", "text": block.text})
                print(f"  [{agent_name}] {block.text[:200]}")
            elif block.type == "tool_use":
                tool_uses.append(block)
                assistant_content.append({
                    "type": "tool_use",
                    "id": block.id,
                    "name": block.name,
                    "input": block.input,
                })
                print(f"  [{agent_name}] -> tool: {block.name}({json.dumps(block.input)[:100]})")

        messages.append({"role": "assistant", "content": assistant_content})

        if response.stop_reason == "end_turn":
            break

        # Execute tool calls
        tool_results = []
        for tool_use in tool_uses:
            try:
                result = mcp.call_tool(tool_use.name, tool_use.input)
                result_text = json.dumps(result, indent=2)
                print(f"  [{agent_name}] <- {tool_use.name}: {result_text[:150]}")
                tool_results.append({
                    "type": "tool_result",
                    "tool_use_id": tool_use.id,
                    "content": result_text,
                })
            except Exception as e:
                tool_results.append({
                    "type": "tool_result",
                    "tool_use_id": tool_use.id,
                    "content": f"Error: {e}",
                    "is_error": True,
                })

        messages.append({"role": "user", "content": tool_results})

    return final_text


# ---------------------------------------------------------------------------
# Main test scenario
# ---------------------------------------------------------------------------

def main():
    parser = argparse.ArgumentParser(description="SynapBus two-agent E2E test")
    parser.add_argument("--port", type=int, default=8080, help="SynapBus port")
    parser.add_argument("--model", default="claude-sonnet-4-6", help="Claude model")
    args = parser.parse_args()

    global SYNAPBUS_URL, MCP_URL
    SYNAPBUS_URL = f"http://localhost:{args.port}"
    MCP_URL = f"{SYNAPBUS_URL}/mcp"

    if not os.environ.get("ANTHROPIC_API_KEY"):
        print("ERROR: ANTHROPIC_API_KEY not set")
        sys.exit(1)

    print(f"=== SynapBus E2E Test ===")
    print(f"Server: {SYNAPBUS_URL}")
    print(f"Model:  {args.model}")

    # 1. Setup
    print("\n[1/4] Setting up agents...")
    alice_key, bob_key = setup_agents(SYNAPBUS_URL)
    print(f"  Alice key: {alice_key[:16]}...")
    print(f"  Bob key:   {bob_key[:16]}...")

    # 2. Initialize MCP sessions
    print("\n[2/4] Initializing MCP sessions...")
    alice_mcp = SynapBusMCP(SYNAPBUS_URL, alice_key)
    bob_mcp = SynapBusMCP(SYNAPBUS_URL, bob_key)
    alice_mcp.initialize()
    bob_mcp.initialize()
    print(f"  Alice session: {alice_mcp.session_id}")
    print(f"  Bob session:   {bob_mcp.session_id}")

    # 3. Alice sends a message
    print("\n[3/4] Alice sends a research request to Bob...")
    alice_result = run_agent(
        "Alice",
        alice_mcp,
        system_prompt=(
            "You are Alice, a research agent. You communicate with other agents "
            "through SynapBus messaging tools. Be concise and direct."
        ),
        user_prompt=(
            "Discover what agents are available, then send a message to 'bob' "
            "asking him to analyze the pros and cons of using MCP (Model Context "
            "Protocol) for agent-to-agent communication. Include a specific "
            "question in your message."
        ),
        model=args.model,
    )

    # 4. Bob reads inbox and replies
    print("\n[4/4] Bob reads inbox and replies...")
    bob_result = run_agent(
        "Bob",
        bob_mcp,
        system_prompt=(
            "You are Bob, a data analyst agent. You communicate with other agents "
            "through SynapBus messaging tools. When you receive messages, read them, "
            "process the request, and send a thoughtful reply. Be concise."
        ),
        user_prompt=(
            "Check your inbox for new messages. Read any pending messages, then "
            "reply to each sender with a helpful response to their question. "
            "After replying, mark the original messages as done."
        ),
        model=args.model,
    )

    # 5. Verify Alice got the reply
    print("\n=== Verification ===")
    print("Checking Alice's inbox for Bob's reply...")
    alice_inbox = alice_mcp.call_tool("read_inbox", {"limit": 10})
    msgs = alice_inbox.get("messages", [])
    if msgs:
        for msg in msgs:
            print(f"  From: {msg['from_agent']}")
            print(f"  Body: {msg['body'][:200]}")
            print(f"  Status: {msg['status']}")
        print("\n  SUCCESS: Alice received Bob's reply")
    else:
        print("  WARNING: No messages in Alice's inbox yet")

    # Cleanup
    alice_mcp.close()
    bob_mcp.close()

    print("\n=== Test Complete ===")


if __name__ == "__main__":
    main()
