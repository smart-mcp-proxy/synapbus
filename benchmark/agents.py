#!/usr/bin/env python3
"""
Agent pool for the MuSiQue benchmark.

Two agents:
    - haiku-agent (claude-haiku-4-5-20251001)
    - sonnet-agent (claude-sonnet-4-6)

Each agent exposes:
    - name, model, skill_card
    - bid(task) -> {estimated_tokens, confidence, approach}
    - execute(task, paragraphs) -> {answer, actual_tokens}

Design notes:
- We use the official ``anthropic`` Python SDK directly (NOT the
  Claude Agent SDK). Simpler, no subprocesses, reliable token accounting.
- ``bid()`` is pure Python — it is a cheap heuristic so the marketplace
  has something to pick from. Real 016 agents would emit a structured
  reply. For MVP, heuristic bids are sufficient to exercise the auction
  primitive.
- ``execute()`` is the only thing that actually burns tokens.
- ``--dry-run`` in run.py never calls execute(); it uses stub responses.
"""

from __future__ import annotations

from dataclasses import dataclass
from typing import Any

from sdk_backend import call_model


HAIKU_MODEL = "claude-haiku-4-5-20251001"
SONNET_MODEL = "claude-sonnet-4-6"


HAIKU_SKILL_CARD = """\
# haiku-agent

A fast, cheap agent best for single-hop fact lookups and short
extractive answers. Accepts multi-paragraph context but may miss
subtle bridging entities on 4-hop questions. Very low cost per call.

Domains: factual-lookup, extraction, summarization
"""

SONNET_SKILL_CARD = """\
# sonnet-agent

A deliberate mid-tier agent well-suited to multi-hop reasoning with
explicit chain-of-thought. Handles 4-hop MuSiQue questions with
decomposition when the context fits in one prompt. Higher cost per call
than Haiku but meaningfully better F1 on bridging questions.

Domains: multi-hop-qa, decomposition, reasoning
"""


SYSTEM_PROMPT = """\
You are a careful question-answering agent working on a MuSiQue
multi-hop benchmark. You are given a question and a set of numbered
paragraphs. Only a few of the paragraphs are relevant; the rest are
distractors.

Think step by step and cite the paragraphs you used. Then output a
final line starting with exactly:

ANSWER: <your short final answer>

Your final answer must be a short entity or phrase — not a sentence.
"""


@dataclass
class BidResult:
    estimated_tokens: int
    confidence: float
    approach: str

    def to_dict(self) -> dict[str, Any]:
        return {
            "estimated_tokens": self.estimated_tokens,
            "confidence": self.confidence,
            "approach": self.approach,
        }


@dataclass
class ExecuteResult:
    answer: str
    actual_tokens: int
    raw_text: str = ""

    def to_dict(self) -> dict[str, Any]:
        return {
            "answer": self.answer,
            "actual_tokens": self.actual_tokens,
        }


class Agent:
    name: str
    model: str
    skill_card: str

    def __init__(self, name: str, model: str, skill_card: str) -> None:
        self.name = name
        self.model = model
        self.skill_card = skill_card

    # ---- bidding -----------------------------------------------------------

    def bid(self, task: dict[str, Any]) -> BidResult:
        raise NotImplementedError

    # ---- execution ---------------------------------------------------------

    def execute(
        self,
        task: dict[str, Any],
        paragraphs: list[str],
        *,
        dry_run: bool = False,
        max_budget_tokens: int = 100_000,
    ) -> ExecuteResult:
        question = task["question"]
        prompt = self._build_prompt(question, paragraphs)

        if dry_run:
            stub = (
                "Thinking step by step... [dry-run stub]\n"
                f"ANSWER: [stub answer from {self.name}]"
            )
            # Rough estimate: 1 token ~= 4 characters.
            est = max(256, len(prompt) // 4 + 64)
            return ExecuteResult(
                answer=self._extract_answer(stub),
                actual_tokens=est,
                raw_text=stub,
            )

        # Cap max_tokens to min(1024, budget/2) so the worst case is tame.
        max_tokens = min(1024, max(128, max_budget_tokens // 2))
        result = call_model(
            model=self.model,
            system=SYSTEM_PROMPT,
            user=prompt,
            max_tokens=max_tokens,
        )
        text = result["text"]
        actual = int(result["total_tokens"])
        return ExecuteResult(
            answer=self._extract_answer(text),
            actual_tokens=actual,
            raw_text=text,
        )

    # ---- helpers -----------------------------------------------------------

    def _build_prompt(
        self, question: str, paragraphs: list[str]
    ) -> str:
        body = ["Paragraphs:"]
        for i, p in enumerate(paragraphs, start=1):
            body.append(f"[{i}] {p}")
        body.append("")
        body.append(f"Question: {question}")
        body.append("")
        body.append("Think step by step, then output your final ANSWER: line.")
        return "\n".join(body)

    def _extract_answer(self, text: str) -> str:
        if not text:
            return ""
        for line in reversed(text.splitlines()):
            line = line.strip()
            if line.upper().startswith("ANSWER:"):
                return line.split(":", 1)[1].strip()
        # Fallback: last non-empty line.
        for line in reversed(text.splitlines()):
            line = line.strip()
            if line:
                return line
        return ""


class HaikuAgent(Agent):
    def __init__(self) -> None:
        super().__init__(
            name="haiku-agent",
            model=HAIKU_MODEL,
            skill_card=HAIKU_SKILL_CARD,
        )

    def bid(self, task: dict[str, Any]) -> BidResult:
        # Cheap, low confidence on multi-hop bridging.
        return BidResult(
            estimated_tokens=4_000,
            confidence=0.45,
            approach=(
                "Extract candidate entities from the paragraphs and "
                "answer directly; may miss 4-hop bridges."
            ),
        )


class SonnetAgent(Agent):
    def __init__(self) -> None:
        super().__init__(
            name="sonnet-agent",
            model=SONNET_MODEL,
            skill_card=SONNET_SKILL_CARD,
        )

    def bid(self, task: dict[str, Any]) -> BidResult:
        # More expensive, higher confidence on multi-hop.
        return BidResult(
            estimated_tokens=12_000,
            confidence=0.80,
            approach=(
                "Decompose the question into sub-questions, resolve each "
                "sub-answer against the paragraphs, then compose the final "
                "bridged answer."
            ),
        )


def default_pool() -> list[Agent]:
    return [HaikuAgent(), SonnetAgent()]
