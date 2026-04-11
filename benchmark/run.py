#!/usr/bin/env python3
"""
Main entry point for the MuSiQue MAS benchmark.

Usage::

    python benchmark/run.py --mode single-shot --question q1
    python benchmark/run.py --mode single-shot --question q1 --dry-run

Flow (single-shot):
    1. Load trio.jsonl, find the requested question (by short_id).
    2. Marketplace run:
         a. post_auction(task, domain, max_budget)
         b. each agent in the pool submits a bid
         c. marketplace awards best bid
         d. winner executes (Anthropic call or dry-run stub)
         e. marketplace.mark_done records reputation
    3. Baseline run: one Sonnet call with all distractors.
    4. Score both, compute Pareto verdict.
    5. Write results/latest.json and results/latest.html.
"""

from __future__ import annotations

import argparse
import json
import sys
import time
from pathlib import Path
from typing import Any

# Allow running as ``python benchmark/run.py`` from the repo root.
_HERE = Path(__file__).resolve().parent
if str(_HERE) not in sys.path:
    sys.path.insert(0, str(_HERE))

from agents import default_pool  # noqa: E402
from baseline import run_baseline, BASELINE_MODEL  # noqa: E402
from marketplace import Marketplace  # noqa: E402
from report import render_report  # noqa: E402
from score import best_f1_against_aliases, pareto_verdict  # noqa: E402


TRIO_FILE = _HERE / "trio.jsonl"
RESULTS_DIR = _HERE / "results"
DEFAULT_DOMAIN = "multi-hop-qa"
DEFAULT_BUDGET = 50_000


def _load_trio() -> list[dict[str, Any]]:
    if not TRIO_FILE.exists():
        raise SystemExit(
            f"[run] trio.jsonl not found at {TRIO_FILE}. "
            "Run curate.py first."
        )
    out: list[dict[str, Any]] = []
    with open(TRIO_FILE, "r", encoding="utf-8") as f:
        for line in f:
            line = line.strip()
            if not line:
                continue
            out.append(json.loads(line))
    return out


def _pick_question(
    trio: list[dict[str, Any]], want: str
) -> dict[str, Any]:
    for rec in trio:
        if rec.get("short_id") == want or rec.get("id") == want:
            return rec
    raise SystemExit(
        f"[run] question {want!r} not found. Available: "
        + ", ".join(r.get("short_id", r.get("id", "?")) for r in trio)
    )


def single_shot(
    question: str,
    *,
    dry_run: bool,
    verbose: bool = True,
) -> dict[str, Any]:
    trio = _load_trio()
    rec = _pick_question(trio, question)

    task = {
        "question": rec["question"],
        "short_id": rec.get("short_id"),
    }
    paragraphs = rec.get("paragraphs", []) or []
    gold_answer = rec.get("answer", "")
    aliases = rec.get("answer_aliases", []) or []

    market = Marketplace()
    pool = default_pool()

    if verbose:
        print(f"[run] question {rec.get('short_id')}: {rec['question']!r}")
        print(
            f"[run] agents: "
            + ", ".join(f"{a.name}({a.model})" for a in pool)
        )
        print(f"[run] paragraphs: {len(paragraphs)}")

    # --- Marketplace path ------------------------------------------------
    auction_id = market.post_auction(
        task=task,
        domain=DEFAULT_DOMAIN,
        max_budget_tokens=DEFAULT_BUDGET,
    )
    if verbose:
        print(f"[run] posted auction {auction_id}")

    for agent in pool:
        bid = agent.bid(task)
        market.bid(
            auction_id=auction_id,
            agent=agent.name,
            estimated_tokens=bid.estimated_tokens,
            confidence=bid.confidence,
            approach=bid.approach,
        )
        if verbose:
            print(
                f"[run]   bid {agent.name}: "
                f"est={bid.estimated_tokens} conf={bid.confidence:.2f}"
            )

    winning_bid = market.award(auction_id)
    winner_name = winning_bid["agent"]
    winner = next(a for a in pool if a.name == winner_name)
    if verbose:
        print(f"[run] awarded to {winner_name}")

    start = time.time()
    result = winner.execute(
        task=task,
        paragraphs=paragraphs,
        dry_run=dry_run,
        max_budget_tokens=DEFAULT_BUDGET,
    )
    market_wall = time.time() - start

    market_f1 = best_f1_against_aliases(
        result.answer, gold_answer, aliases
    )
    market.mark_done(
        auction_id=auction_id,
        answer=result.answer,
        actual_tokens=result.actual_tokens,
        correct=market_f1 >= 0.5,
    )
    if verbose:
        print(
            f"[run] market answer: {result.answer!r} "
            f"(tokens={result.actual_tokens}, f1={market_f1:.3f})"
        )

    # --- Baseline path ---------------------------------------------------
    start = time.time()
    baseline = run_baseline(
        question=rec["question"],
        paragraphs=paragraphs,
        dry_run=dry_run,
    )
    baseline_wall = time.time() - start
    baseline_f1 = best_f1_against_aliases(
        baseline["answer"], gold_answer, aliases
    )
    if verbose:
        print(
            f"[run] baseline answer: {baseline['answer']!r} "
            f"(tokens={baseline['tokens']}, f1={baseline_f1:.3f})"
        )

    verdict = pareto_verdict(
        market_tokens=result.actual_tokens,
        market_f1=market_f1,
        baseline_tokens=baseline["tokens"],
        baseline_f1=baseline_f1,
    )
    if verbose:
        print(f"[run] PARETO VERDICT: {verdict['verdict']}")

    return {
        "mode": "single-shot",
        "dry_run": dry_run,
        "question_id": rec.get("short_id"),
        "musique_id": rec.get("id"),
        "question": rec["question"],
        "gold_answer": gold_answer,
        "decomposition": rec.get("decomposition", []),
        "domain": DEFAULT_DOMAIN,
        "max_budget_tokens": DEFAULT_BUDGET,
        "awarded_to": winner_name,
        "bids": market.list_bids(auction_id),
        "market": {
            "agent": winner_name,
            "model": winner.model,
            "tokens": result.actual_tokens,
            "answer": result.answer,
            "f1": market_f1,
            "wall_seconds": market_wall,
            "raw_text": result.raw_text,
        },
        "baseline": {
            "model": baseline.get("model", BASELINE_MODEL),
            "tokens": baseline["tokens"],
            "answer": baseline["answer"],
            "f1": baseline_f1,
            "wall_seconds": baseline_wall,
            "raw_text": baseline.get("raw_text", ""),
        },
        "pareto": verdict,
        "reputation": market.all_reputation(),
    }


def _write_outputs(result: dict[str, Any]) -> tuple[Path, Path]:
    RESULTS_DIR.mkdir(parents=True, exist_ok=True)
    json_path = RESULTS_DIR / "latest.json"
    html_path = RESULTS_DIR / "latest.html"
    # Trim raw_text from json to keep it small and readable.
    trimmed = dict(result)
    for key in ("market", "baseline"):
        section = dict(trimmed.get(key, {}))
        if "raw_text" in section:
            section["raw_text"] = (section["raw_text"] or "")[:2000]
        trimmed[key] = section
    json_path.write_text(
        json.dumps(trimmed, indent=2, ensure_ascii=False), encoding="utf-8"
    )
    render_report(result, html_path)
    return json_path, html_path


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(
        description="MuSiQue multi-agent benchmark harness"
    )
    parser.add_argument(
        "--mode",
        choices=["single-shot"],
        default="single-shot",
        help="Run mode (only single-shot is implemented in MVP)",
    )
    parser.add_argument(
        "--question",
        default="q1",
        help="Question short_id from trio.jsonl (q1/q2/q3)",
    )
    parser.add_argument(
        "--dry-run",
        action="store_true",
        help="Skip real Anthropic API calls; use stub responses",
    )
    args = parser.parse_args(argv)

    if args.mode != "single-shot":
        print(f"[run] mode {args.mode} not implemented in MVP", file=sys.stderr)
        return 2

    result = single_shot(
        question=args.question,
        dry_run=args.dry_run,
        verbose=True,
    )
    json_path, html_path = _write_outputs(result)
    print(f"[run] wrote {json_path}")
    print(f"[run] wrote {html_path}")
    print(f"[run] verdict: {result['pareto']['verdict']}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
