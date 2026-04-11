#!/usr/bin/env python3
"""
Single-agent baseline: one Anthropic API call to claude-sonnet-4-6 with
the question and all 20 distractor paragraphs plus chain-of-thought
instructions. No decomposition, no marketplace, no tools.

Returns {"answer": str, "tokens": int, "raw_text": str}.
"""

from __future__ import annotations

import os
from typing import Any

try:
    import anthropic  # type: ignore
except ImportError:  # pragma: no cover
    anthropic = None  # type: ignore


BASELINE_MODEL = "claude-sonnet-4-6"

BASELINE_SYSTEM = """\
You are a careful multi-hop QA system. Given a question and a set of
numbered paragraphs (some irrelevant distractors), think step by step
and answer.

Output your reasoning first, then on a final line:

ANSWER: <short final answer>
"""


def _build_prompt(question: str, paragraphs: list[str]) -> str:
    parts = ["Paragraphs:"]
    for i, p in enumerate(paragraphs, start=1):
        parts.append(f"[{i}] {p}")
    parts.append("")
    parts.append(f"Question: {question}")
    parts.append("")
    parts.append(
        "Work through the reasoning step by step, then give your "
        "final ANSWER: line."
    )
    return "\n".join(parts)


def _extract_answer(text: str) -> str:
    if not text:
        return ""
    for line in reversed(text.splitlines()):
        line = line.strip()
        if line.upper().startswith("ANSWER:"):
            return line.split(":", 1)[1].strip()
    for line in reversed(text.splitlines()):
        line = line.strip()
        if line:
            return line
    return ""


def run_baseline(
    question: str,
    paragraphs: list[str],
    *,
    dry_run: bool = False,
    max_output_tokens: int = 1024,
) -> dict[str, Any]:
    prompt = _build_prompt(question, paragraphs)

    if dry_run:
        stub = (
            "Step 1: scanning paragraphs... [dry-run stub]\n"
            "Step 2: picking the most likely entity...\n"
            "ANSWER: [stub baseline answer]"
        )
        est = max(512, len(prompt) // 4 + 128)
        return {
            "answer": _extract_answer(stub),
            "tokens": est,
            "raw_text": stub,
            "model": BASELINE_MODEL,
        }

    if anthropic is None:
        raise RuntimeError("anthropic SDK not installed")
    api_key = os.environ.get("ANTHROPIC_API_KEY")
    if not api_key:
        raise RuntimeError("ANTHROPIC_API_KEY not set")

    client = anthropic.Anthropic(api_key=api_key)
    msg = client.messages.create(
        model=BASELINE_MODEL,
        max_tokens=max_output_tokens,
        system=BASELINE_SYSTEM,
        messages=[{"role": "user", "content": prompt}],
    )
    text_parts: list[str] = []
    for block in msg.content:
        t = getattr(block, "text", None)
        if t:
            text_parts.append(t)
    text = "\n".join(text_parts).strip()
    usage = getattr(msg, "usage", None)
    tokens = 0
    if usage is not None:
        tokens = (
            getattr(usage, "input_tokens", 0)
            + getattr(usage, "output_tokens", 0)
        )
    return {
        "answer": _extract_answer(text),
        "tokens": int(tokens),
        "raw_text": text,
        "model": BASELINE_MODEL,
    }
