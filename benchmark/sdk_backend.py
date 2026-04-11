#!/usr/bin/env python3
"""
Model call backend for the MuSiQue benchmark.

Two backends are supported, selected at runtime:

- anthropic SDK (requires ANTHROPIC_API_KEY) — preferred for production.
- claude-agent-sdk (runs inside Claude Code, inherits session auth) —
  used when ANTHROPIC_API_KEY is not available (e.g. in an interactive
  Claude Code autonomous run).

Both backends share the same `call_model(model, system, user, max_tokens)`
signature and return the same shape: `(text, input_tokens, output_tokens, cost_usd)`.
"""

from __future__ import annotations

import asyncio
import os
from typing import Any

# ---------------------------------------------------------------------------
# Backend selection
# ---------------------------------------------------------------------------

_BACKEND = None  # "anthropic" | "claude_agent_sdk" | None


def detect_backend() -> str:
    """Return the name of the best available backend."""
    global _BACKEND
    if _BACKEND is not None:
        return _BACKEND

    api_key = os.environ.get("ANTHROPIC_API_KEY", "").strip()
    if api_key:
        try:
            import anthropic  # type: ignore  # noqa: F401
            _BACKEND = "anthropic"
            return _BACKEND
        except ImportError:
            pass

    try:
        import claude_agent_sdk  # type: ignore  # noqa: F401
        _BACKEND = "claude_agent_sdk"
        return _BACKEND
    except ImportError:
        pass

    raise RuntimeError(
        "No model backend available. Set ANTHROPIC_API_KEY + install "
        "anthropic, OR install claude-agent-sdk inside a Claude Code session."
    )


# ---------------------------------------------------------------------------
# Unified call signature
# ---------------------------------------------------------------------------


def call_model(
    model: str,
    system: str,
    user: str,
    max_tokens: int = 1024,
) -> dict[str, Any]:
    """
    Call the model with a system prompt and a user message.
    Returns {text, input_tokens, output_tokens, total_tokens, cost_usd, backend}.
    """
    backend = detect_backend()
    if backend == "anthropic":
        return _call_anthropic(model, system, user, max_tokens)
    if backend == "claude_agent_sdk":
        return _call_agent_sdk(model, system, user, max_tokens)
    raise RuntimeError(f"unknown backend: {backend}")


# ---------------------------------------------------------------------------
# anthropic SDK backend
# ---------------------------------------------------------------------------


def _call_anthropic(model: str, system: str, user: str, max_tokens: int) -> dict[str, Any]:
    import anthropic  # type: ignore

    client = anthropic.Anthropic()  # reads ANTHROPIC_API_KEY
    msg = client.messages.create(
        model=model,
        max_tokens=max_tokens,
        system=system,
        messages=[{"role": "user", "content": user}],
    )
    text_parts = []
    for block in msg.content:
        t = getattr(block, "text", None)
        if t:
            text_parts.append(t)
    text = "\n".join(text_parts).strip()
    usage = getattr(msg, "usage", None)
    input_t = getattr(usage, "input_tokens", 0) if usage else 0
    output_t = getattr(usage, "output_tokens", 0) if usage else 0
    return {
        "text": text,
        "input_tokens": int(input_t),
        "output_tokens": int(output_t),
        "total_tokens": int(input_t + output_t),
        "cost_usd": None,  # anthropic SDK does not return cost; caller can compute
        "backend": "anthropic",
    }


# ---------------------------------------------------------------------------
# claude-agent-sdk backend
# ---------------------------------------------------------------------------


def _call_agent_sdk(model: str, system: str, user: str, max_tokens: int) -> dict[str, Any]:
    from claude_agent_sdk import (  # type: ignore
        query,
        ClaudeAgentOptions,
        AssistantMessage,
        ResultMessage,
        TextBlock,
    )

    async def run() -> dict[str, Any]:
        opts = ClaudeAgentOptions(
            model=model,
            system_prompt=system,
            max_turns=1,
            allowed_tools=[],
            permission_mode="bypassPermissions",
        )
        text_parts: list[str] = []
        result: Any = None
        async for msg in query(prompt=user, options=opts):
            if isinstance(msg, AssistantMessage):
                for block in msg.content:
                    if isinstance(block, TextBlock):
                        text_parts.append(block.text)
            if isinstance(msg, ResultMessage):
                result = msg
        text = "\n".join(text_parts).strip()
        input_t = 0
        output_t = 0
        cost = None
        if result is not None:
            usage = getattr(result, "usage", None) or {}
            input_t = int(usage.get("input_tokens", 0))
            output_t = int(usage.get("output_tokens", 0))
            cost = getattr(result, "total_cost_usd", None)
        return {
            "text": text,
            "input_tokens": input_t,
            "output_tokens": output_t,
            "total_tokens": input_t + output_t,
            "cost_usd": cost,
            "backend": "claude_agent_sdk",
        }

    return asyncio.run(run())


# ---------------------------------------------------------------------------
# Self-test
# ---------------------------------------------------------------------------

if __name__ == "__main__":
    import sys

    print(f"backend: {detect_backend()}")
    result = call_model(
        model="claude-haiku-4-5-20251001",
        system="You are a concise assistant.",
        user="Respond with exactly: 'backend ok'",
        max_tokens=32,
    )
    print(result)
    sys.exit(0)
