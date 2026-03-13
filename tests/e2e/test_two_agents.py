#!/usr/bin/env python3
"""
E2E test: Two Claude-powered agents communicate through SynapBus MCP.

Authentication (3-tier fallback, no API key required):
  1. ANTHROPIC_API_KEY env var
  2. CLAUDE_CODE_OAUTH_TOKEN env var
  3. macOS Keychain (Claude Code subscription credentials)

Prerequisites:
  pip install anthropic httpx

Usage:
  # Start server first (or use --auto-server):
  python tests/e2e/test_two_agents.py --auto-server
  python tests/e2e/test_two_agents.py --port 8080  # if server already running
"""

from __future__ import annotations

import argparse
import json
import os
import shutil
import signal
import socket
import subprocess
import sys
import tempfile
import time
from dataclasses import dataclass, field
from typing import Optional

import httpx
import anthropic


# ---------------------------------------------------------------------------
# Authentication (3-tier fallback from dialog-engine)
# ---------------------------------------------------------------------------

def create_anthropic_client() -> anthropic.Anthropic:
    """Create Anthropic client with 3-tier auth fallback.

    1. ANTHROPIC_API_KEY env var (standard API key)
    2. CLAUDE_CODE_OAUTH_TOKEN env var (OAuth token)
    3. macOS Keychain (Claude Code subscription credentials)
    """
    # Tier 1: Standard API key
    api_key = os.environ.get("ANTHROPIC_API_KEY")
    if api_key:
        print("  Auth: using ANTHROPIC_API_KEY")
        return anthropic.Anthropic(api_key=api_key)

    # Tier 2: OAuth token from env
    oauth_token = os.environ.get("CLAUDE_CODE_OAUTH_TOKEN")

    # Tier 3: macOS Keychain (Claude Code stores credentials as JSON)
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
                if oauth_token:
                    print("  Auth: using macOS Keychain (Claude Code subscription)")
        except (subprocess.CalledProcessError, FileNotFoundError, json.JSONDecodeError):
            pass

    if oauth_token:
        return anthropic.Anthropic(
            auth_token=oauth_token,
            default_headers={"anthropic-beta": "oauth-2025-04-20"},
        )

    print("ERROR: No Anthropic credentials found.")
    print("  Set ANTHROPIC_API_KEY, CLAUDE_CODE_OAUTH_TOKEN,")
    print("  or ensure Claude Code is logged in (macOS Keychain).")
    sys.exit(1)


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

    def _rpc(self, method: str, params: Optional[dict] = None) -> dict:
        body = {
            "jsonrpc": "2.0",
            "id": self._next_id(),
            "method": method,
        }
        if params:
            body["params"] = params

        resp = self._client.post(self.url, json=body, headers=self._headers())
        resp.raise_for_status()

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
        content = result.get("result", {}).get("content", [])
        for block in content:
            if block.get("type") == "text":
                return json.loads(block["text"])
        return result

    def list_tools(self) -> list:
        result = self._rpc("tools/list")
        return result.get("result", {}).get("tools", [])

    def close(self):
        self._client.close()


# ---------------------------------------------------------------------------
# Server lifecycle management
# ---------------------------------------------------------------------------

def find_free_port() -> int:
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as s:
        s.bind(("", 0))
        return s.getsockname()[1]


def start_server(port: int) -> tuple:
    """Start SynapBus server, return (process, data_dir)."""
    data_dir = tempfile.mkdtemp(prefix="synapbus-e2e-")
    proc = subprocess.Popen(
        ["./synapbus", "serve", "--port", str(port), "--data", data_dir],
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
        text=True,
    )

    # Wait for server to be healthy
    for i in range(30):
        try:
            resp = httpx.get(f"http://localhost:{port}/health", timeout=2)
            if resp.status_code == 200:
                return proc, data_dir
        except httpx.ConnectError:
            pass
        time.sleep(0.5)

    proc.terminate()
    shutil.rmtree(data_dir, ignore_errors=True)
    print("ERROR: Server failed to start within 15 seconds")
    sys.exit(1)


def stop_server(proc: subprocess.Popen, data_dir: str):
    """Stop server and clean up."""
    proc.send_signal(signal.SIGTERM)
    proc.wait(timeout=10)
    shutil.rmtree(data_dir, ignore_errors=True)


# ---------------------------------------------------------------------------
# Setup: register user + agents
# ---------------------------------------------------------------------------

def setup_agents(base_url: str) -> tuple:
    """Register test user + two agents. Returns (alice_key, bob_key)."""
    client = httpx.Client(timeout=10)

    # Register test user (ignore if exists)
    client.post(f"{base_url}/auth/register", json={
        "username": "e2e_tester",
        "password": "testpass123456",
        "display_name": "E2E Tester",
    })

    # Login
    resp = client.post(f"{base_url}/auth/login", json={
        "username": "e2e_tester",
        "password": "testpass123456",
    })
    if resp.status_code != 200:
        print("  [!] Login failed. Server may need a fresh data directory.")
        sys.exit(1)

    cookies = resp.cookies

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
        print("  [!] Agent registration failed.")
        print(f"      Alice: {alice_resp.json()}")
        print(f"      Bob: {bob_resp.json()}")
        sys.exit(1)

    client.close()
    return alice_key, bob_key


# ---------------------------------------------------------------------------
# Tool definitions for Claude
# ---------------------------------------------------------------------------

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
        "description": "Discover other agents by capability keyword search",
        "input_schema": {
            "type": "object",
            "properties": {
                "query": {"type": "string", "description": "Capability search query"},
            },
        },
    },
    {
        "name": "claim_messages",
        "description": "Atomically claim pending messages for processing",
        "input_schema": {
            "type": "object",
            "properties": {
                "limit": {"type": "integer", "description": "Max messages to claim"},
            },
        },
    },
    {
        "name": "mark_done",
        "description": "Mark a previously claimed message as done or failed",
        "input_schema": {
            "type": "object",
            "properties": {
                "message_id": {"type": "integer", "description": "Message ID"},
                "status": {"type": "string", "enum": ["done", "failed"]},
            },
            "required": ["message_id", "status"],
        },
    },
]


# ---------------------------------------------------------------------------
# Agent runner (dialog-engine pattern: max tool rounds + forced text)
# ---------------------------------------------------------------------------

@dataclass
class AgentResult:
    text: str = ""
    tool_calls: list = field(default_factory=list)
    input_tokens: int = 0
    output_tokens: int = 0


def run_agent(
    claude: anthropic.Anthropic,
    mcp: SynapBusMCP,
    agent_name: str,
    system_prompt: str,
    user_prompt: str,
    model: str = "claude-sonnet-4-6",
    max_tool_rounds: int = 5,
) -> AgentResult:
    """Run a Claude agent with SynapBus MCP tools.

    Pattern from dialog-engine: iterate up to max_tool_rounds allowing
    tool use. On the final round, omit tools to force a text response.
    """
    messages = [{"role": "user", "content": user_prompt}]
    result = AgentResult()

    for round_num in range(max_tool_rounds + 1):
        api_kwargs = dict(
            model=model,
            max_tokens=2048,
            system=system_prompt,
            messages=messages,
        )
        # Allow tool use except on final round
        if round_num < max_tool_rounds:
            api_kwargs["tools"] = TOOLS

        response = claude.messages.create(**api_kwargs)
        result.input_tokens += response.usage.input_tokens
        result.output_tokens += response.usage.output_tokens

        has_tool_use = any(b.type == "tool_use" for b in response.content)

        if not has_tool_use or round_num == max_tool_rounds:
            # Extract final text
            text_parts = []
            for block in response.content:
                if block.type == "text":
                    text_parts.append(block.text)
            result.text = "\n".join(text_parts) if text_parts else "(no response)"
            print(f"  [{agent_name}] {result.text[:300]}")
            return result

        # Process tool calls
        assistant_content = []
        for block in response.content:
            if block.type == "text":
                assistant_content.append({"type": "text", "text": block.text})
                print(f"  [{agent_name}] {block.text[:150]}")
            elif block.type == "tool_use":
                assistant_content.append({
                    "type": "tool_use",
                    "id": block.id,
                    "name": block.name,
                    "input": block.input,
                })

        messages.append({"role": "assistant", "content": assistant_content})

        # Execute tools via MCP
        tool_results = []
        for block in response.content:
            if block.type == "tool_use":
                tool_input_str = json.dumps(block.input)[:80]
                print(f"  [{agent_name}] -> {block.name}({tool_input_str})")
                try:
                    tool_result = mcp.call_tool(block.name, block.input)
                    result_str = json.dumps(tool_result)
                    print(f"  [{agent_name}] <- {result_str[:150]}")
                    result.tool_calls.append({
                        "tool": block.name,
                        "input": block.input,
                        "output": tool_result,
                    })
                    tool_results.append({
                        "type": "tool_result",
                        "tool_use_id": block.id,
                        "content": result_str,
                    })
                except Exception as e:
                    print(f"  [{agent_name}] <- ERROR: {e}")
                    tool_results.append({
                        "type": "tool_result",
                        "tool_use_id": block.id,
                        "content": f"Error: {e}",
                        "is_error": True,
                    })

        messages.append({"role": "user", "content": tool_results})

    return result


# ---------------------------------------------------------------------------
# Main test scenario
# ---------------------------------------------------------------------------

def main():
    parser = argparse.ArgumentParser(description="SynapBus two-agent E2E test")
    parser.add_argument("--port", type=int, default=0, help="SynapBus port (0 = auto)")
    parser.add_argument("--model", default="claude-sonnet-4-6", help="Claude model")
    parser.add_argument("--auto-server", action="store_true",
                        help="Auto-start and stop SynapBus server")
    args = parser.parse_args()

    server_proc = None
    data_dir = None

    if args.auto_server or args.port == 0:
        port = find_free_port() if args.port == 0 else args.port
        print(f"Starting SynapBus on port {port}...")
        server_proc, data_dir = start_server(port)
        args.auto_server = True
    else:
        port = args.port

    base_url = f"http://localhost:{port}"

    try:
        print(f"\n{'=' * 50}")
        print(f"  SynapBus E2E Agent Test")
        print(f"  Server: {base_url}")
        print(f"  Model:  {args.model}")
        print(f"{'=' * 50}")

        # 1. Authenticate with Anthropic
        print("\n[1/5] Authenticating with Anthropic...")
        claude = create_anthropic_client()

        # 2. Setup agents
        print("\n[2/5] Registering agents...")
        alice_key, bob_key = setup_agents(base_url)
        print(f"  Alice: {alice_key[:16]}...")
        print(f"  Bob:   {bob_key[:16]}...")

        # 3. Initialize MCP sessions
        print("\n[3/5] Initializing MCP sessions...")
        alice_mcp = SynapBusMCP(base_url, alice_key)
        bob_mcp = SynapBusMCP(base_url, bob_key)
        alice_mcp.initialize()
        bob_mcp.initialize()
        print(f"  Alice session: {alice_mcp.session_id}")
        print(f"  Bob session:   {bob_mcp.session_id}")

        # List available tools
        tools = alice_mcp.list_tools()
        print(f"  Available tools: {len(tools)}")
        for t in tools[:5]:
            print(f"    - {t['name']}: {t.get('description', '')[:60]}")
        if len(tools) > 5:
            print(f"    ... and {len(tools) - 5} more")

        # 4. Alice discovers agents and sends message
        print("\n[4/5] Alice discovers agents and sends research request...")
        alice_result = run_agent(
            claude, alice_mcp, "Alice",
            system_prompt=(
                "You are Alice, a research agent on the SynapBus messaging platform. "
                "You communicate with other agents using the provided tools. "
                "Be concise and direct. Complete your task in as few tool calls as possible."
            ),
            user_prompt=(
                "First, discover what other agents are available using discover_agents. "
                "Then send a message to 'bob' asking him to analyze the trade-offs "
                "of using MCP (Model Context Protocol) vs REST APIs for agent-to-agent "
                "communication. Ask a specific question."
            ),
            model=args.model,
        )

        # 5. Bob reads inbox, processes, and replies
        print("\n[5/5] Bob reads inbox and replies...")
        bob_result = run_agent(
            claude, bob_mcp, "Bob",
            system_prompt=(
                "You are Bob, a data analyst agent on the SynapBus messaging platform. "
                "You communicate with other agents using the provided tools. "
                "When you receive messages, process the request and send a thoughtful reply. "
                "Be concise and direct."
            ),
            user_prompt=(
                "Check your inbox for new messages using read_inbox. "
                "For each message: claim it with claim_messages, send a reply to the "
                "sender using send_message, then mark the original as done with mark_done."
            ),
            model=args.model,
        )

        # Verification
        print(f"\n{'=' * 50}")
        print("  VERIFICATION")
        print(f"{'=' * 50}")

        # Check Alice's inbox for Bob's reply
        alice_inbox = alice_mcp.call_tool("read_inbox", {"limit": 10})
        msgs = alice_inbox.get("messages", [])
        bob_replies = [m for m in msgs if m.get("from_agent") == "bob"]

        if bob_replies:
            print(f"\n  PASS: Alice received {len(bob_replies)} reply(ies) from Bob")
            for msg in bob_replies:
                body_preview = msg["body"][:200]
                print(f"    Subject: {msg.get('subject', 'N/A')}")
                print(f"    Body: {body_preview}")
        else:
            print("\n  FAIL: Alice did not receive a reply from Bob")
            print(f"    Alice inbox: {len(msgs)} total messages")

        # Cost summary
        total_input = alice_result.input_tokens + bob_result.input_tokens
        total_output = alice_result.output_tokens + bob_result.output_tokens
        # Sonnet pricing: $3/M input, $15/M output
        est_cost = (total_input * 3 + total_output * 15) / 1_000_000
        print(f"\n  Token usage:")
        print(f"    Alice: {alice_result.input_tokens} in / {alice_result.output_tokens} out")
        print(f"    Bob:   {bob_result.input_tokens} in / {bob_result.output_tokens} out")
        print(f"    Total: {total_input} in / {total_output} out")
        print(f"    Est. cost: ${est_cost:.4f}")

        print(f"\n  Tool calls: Alice={len(alice_result.tool_calls)}, Bob={len(bob_result.tool_calls)}")

        # Cleanup
        alice_mcp.close()
        bob_mcp.close()

        print(f"\n{'=' * 50}")
        print("  TEST COMPLETE")
        print(f"{'=' * 50}")

    finally:
        if server_proc:
            print("\nStopping server...")
            stop_server(server_proc, data_dir)


if __name__ == "__main__":
    main()
