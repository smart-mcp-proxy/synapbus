#!/usr/bin/env python3
"""
Single-agent baseline: one Anthropic API call to claude-sonnet-4-6 with
the question and all 20 distractor paragraphs plus chain-of-thought
instructions. No decomposition, no marketplace, no tools.

Returns {"answer": str, "tokens": int, "raw_text": str}.
"""

from __future__ import annotations

from typing import Any

from sdk_backend import call_model


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

    result = call_model(
        model=BASELINE_MODEL,
        system=BASELINE_SYSTEM,
        user=prompt,
        max_tokens=max_output_tokens,
    )
    text = result["text"]
    tokens = int(result["total_tokens"])
    return {
        "answer": _extract_answer(text),
        "tokens": tokens,
        "raw_text": text,
        "model": BASELINE_MODEL,
    }
