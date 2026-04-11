#!/usr/bin/env python3
"""
In-process stub of the 016-agent-marketplace primitives.

*** IMPORTANT ***
This module is an in-process stand-in for the SynapBus-hosted 016
marketplace. The follow-up deliverable after this MVP is to replace the
bodies of these functions with calls to the real SynapBus MCP tools
(``post_auction``, ``bid``, ``award``, ``mark_done``, and
``query_reputation``) once 016 lands. The public API here deliberately
mirrors those tool names so the swap is mechanical.

Scope for MVP:
- In-memory auctions, bids, awards, and done records
- Domain-scoped reputation ledger stored in a dict
- No persistence, no concurrency — single process, single thread
- No schema enforcement beyond a couple of shape checks

The harness (``run.py``) holds a single ``Marketplace`` instance.
"""

from __future__ import annotations

import itertools
import time
from dataclasses import dataclass, field
from typing import Any


@dataclass
class Auction:
    auction_id: str
    task: dict[str, Any]
    domain: str
    max_budget_tokens: int
    posted_at: float
    bids: list[dict[str, Any]] = field(default_factory=list)
    awarded_to: str | None = None
    result: dict[str, Any] | None = None


@dataclass
class ReputationEntry:
    agent: str
    domain: str
    runs: int = 0
    correct: int = 0
    tokens_spent: int = 0

    def score(self) -> float:
        if self.runs == 0:
            return 0.5  # prior
        quality = self.correct / self.runs
        avg_tokens = self.tokens_spent / self.runs
        # Arbitrary: quality dominates, tokens slightly penalize.
        return quality - min(avg_tokens / 200_000.0, 0.3)


class Marketplace:
    """In-process marketplace stub."""

    def __init__(self) -> None:
        self._auctions: dict[str, Auction] = {}
        self._reputation: dict[tuple[str, str], ReputationEntry] = {}
        self._counter = itertools.count(1)

    # ---- auction lifecycle -------------------------------------------------

    def post_auction(
        self,
        task: dict[str, Any],
        domain: str,
        max_budget_tokens: int,
    ) -> str:
        auction_id = f"auction-{next(self._counter)}"
        self._auctions[auction_id] = Auction(
            auction_id=auction_id,
            task=dict(task),
            domain=domain,
            max_budget_tokens=max_budget_tokens,
            posted_at=time.time(),
        )
        return auction_id

    def bid(
        self,
        auction_id: str,
        agent: str,
        estimated_tokens: int,
        confidence: float,
        approach: str,
    ) -> None:
        auction = self._auctions[auction_id]
        if auction.awarded_to is not None:
            raise RuntimeError(f"auction {auction_id} already awarded")
        auction.bids.append(
            {
                "agent": agent,
                "estimated_tokens": int(estimated_tokens),
                "confidence": float(confidence),
                "approach": approach,
                "submitted_at": time.time(),
            }
        )

    def list_bids(self, auction_id: str) -> list[dict[str, Any]]:
        return list(self._auctions[auction_id].bids)

    def score_bid(self, auction_id: str, bid: dict[str, Any]) -> float:
        """
        Lower is better (we're minimizing tokens per unit confidence),
        but we add a reputation adjustment that rewards agents with a
        track record in this domain.
        """
        auction = self._auctions[auction_id]
        rep = self._reputation.get((bid["agent"], auction.domain))
        rep_score = rep.score() if rep else 0.5
        conf = max(bid["confidence"], 1e-3)
        # Cost per confidence, lightly discounted by reputation.
        raw = bid["estimated_tokens"] / conf
        return raw * (1.15 - 0.3 * rep_score)

    def award(self, auction_id: str) -> dict[str, Any]:
        auction = self._auctions[auction_id]
        if not auction.bids:
            raise RuntimeError(f"auction {auction_id} has no bids")
        if auction.awarded_to is not None:
            raise RuntimeError(f"auction {auction_id} already awarded")
        best = min(
            auction.bids,
            key=lambda b: self.score_bid(auction_id, b),
        )
        auction.awarded_to = best["agent"]
        return best

    def mark_done(
        self,
        auction_id: str,
        answer: str,
        actual_tokens: int,
        correct: bool,
    ) -> None:
        auction = self._auctions[auction_id]
        if auction.awarded_to is None:
            raise RuntimeError(f"auction {auction_id} not awarded yet")
        auction.result = {
            "answer": answer,
            "actual_tokens": int(actual_tokens),
            "correct": bool(correct),
        }
        key = (auction.awarded_to, auction.domain)
        entry = self._reputation.get(key) or ReputationEntry(
            agent=auction.awarded_to, domain=auction.domain
        )
        entry.runs += 1
        entry.tokens_spent += int(actual_tokens)
        if correct:
            entry.correct += 1
        self._reputation[key] = entry

    # ---- reputation --------------------------------------------------------

    def query_reputation(
        self, agent: str, domain: str
    ) -> dict[str, Any]:
        rep = self._reputation.get((agent, domain))
        if rep is None:
            return {
                "agent": agent,
                "domain": domain,
                "runs": 0,
                "correct": 0,
                "tokens_spent": 0,
                "score": 0.5,
            }
        return {
            "agent": rep.agent,
            "domain": rep.domain,
            "runs": rep.runs,
            "correct": rep.correct,
            "tokens_spent": rep.tokens_spent,
            "score": rep.score(),
        }

    def all_reputation(self) -> list[dict[str, Any]]:
        return [
            self.query_reputation(rep.agent, rep.domain)
            for rep in self._reputation.values()
        ]

    def auction(self, auction_id: str) -> Auction:
        return self._auctions[auction_id]
