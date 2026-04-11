#!/usr/bin/env python3
"""
Scoring utilities for the MuSiQue benchmark.

- Normalized exact-match F1 (SQuAD-style): lowercase, strip articles,
  strip punctuation, collapse whitespace.
- Pareto verdict: the marketplace point is strictly northwest of the
  baseline iff it uses fewer tokens AND has F1 >= baseline, with at
  least one of those strict.
"""

from __future__ import annotations

import re
import string
from collections import Counter
from typing import Any

_ARTICLE_RE = re.compile(r"\b(a|an|the)\b", re.IGNORECASE)


def normalize(text: str) -> str:
    if text is None:
        return ""
    text = text.lower()
    text = _ARTICLE_RE.sub(" ", text)
    text = "".join(ch for ch in text if ch not in string.punctuation)
    text = " ".join(text.split())
    return text


def f1(prediction: str, gold: str) -> float:
    pred_tokens = normalize(prediction).split()
    gold_tokens = normalize(gold).split()
    if not pred_tokens and not gold_tokens:
        return 1.0
    if not pred_tokens or not gold_tokens:
        return 0.0
    common = Counter(pred_tokens) & Counter(gold_tokens)
    overlap = sum(common.values())
    if overlap == 0:
        return 0.0
    precision = overlap / len(pred_tokens)
    recall = overlap / len(gold_tokens)
    return 2 * precision * recall / (precision + recall)


def exact_match(prediction: str, gold: str) -> bool:
    return normalize(prediction) == normalize(gold)


def best_f1_against_aliases(
    prediction: str, gold: str, aliases: list[str] | None = None
) -> float:
    candidates = [gold] + list(aliases or [])
    return max(f1(prediction, c) for c in candidates if c is not None)


def pareto_verdict(
    market_tokens: int,
    market_f1: float,
    baseline_tokens: int,
    baseline_f1: float,
) -> dict[str, Any]:
    """
    Strictly northwest of baseline: fewer tokens AND higher-or-equal F1,
    with at least one strict inequality.
    """
    tokens_better = market_tokens < baseline_tokens
    quality_atleast = market_f1 >= baseline_f1
    quality_better = market_f1 > baseline_f1

    strictly_nw = (
        (tokens_better and quality_atleast)
        or (quality_better and market_tokens <= baseline_tokens)
    )
    return {
        "verdict": "PASS" if strictly_nw else "FAIL",
        "strictly_northwest": strictly_nw,
        "market_tokens": int(market_tokens),
        "market_f1": float(market_f1),
        "baseline_tokens": int(baseline_tokens),
        "baseline_f1": float(baseline_f1),
        "tokens_delta": int(market_tokens - baseline_tokens),
        "f1_delta": float(market_f1 - baseline_f1),
    }
