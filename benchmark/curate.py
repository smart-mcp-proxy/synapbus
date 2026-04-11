#!/usr/bin/env python3
"""
Curate a deterministic trio of MuSiQue 4-hop questions that share a
pivot entity. For MVP we pivot on the United States.

Input:  benchmark/data/musique_ans_v1.0_dev.jsonl
Output: benchmark/trio.jsonl

Each output record:
    {
      "id": str,
      "question": str,
      "answer": str,
      "decomposition": [{"question": str, "answer": str}, ...],
      "paragraphs": [str, ...]          # up to 20 distractor snippets
    }

MuSiQue dev records typically look like::

    {
      "id": "4hop1__...",
      "question": "...",
      "question_decomposition": [
         {"id": N, "question": "...", "answer": "...",
          "paragraph_support_idx": int},
         ...
      ],
      "answer": "...",
      "answer_aliases": [...],
      "paragraphs": [
         {"idx": int, "title": "...", "paragraph_text": "...",
          "is_supporting": bool},
         ...
      ]
    }

The curation rule is deterministic (fixed input ordering; first 3 matches).
"""

from __future__ import annotations

import json
import sys
from pathlib import Path

DATA_FILE = Path(__file__).resolve().parent / "data" / "musique_ans_v1.0_dev.jsonl"
OUT_FILE = Path(__file__).resolve().parent / "trio.jsonl"

PIVOT_TOKENS = ("united states", "u.s.", " us ", "america", "american")
N_QUESTIONS = 3
MAX_PARAGRAPHS = 20


def _normalized(s: str) -> str:
    return f" {s.lower()} "


def _mentions_pivot(record: dict) -> bool:
    blob_parts = [record.get("question", ""), record.get("answer", "")]
    for sub in record.get("question_decomposition", []) or []:
        blob_parts.append(sub.get("question", ""))
        blob_parts.append(sub.get("answer", ""))
    blob = _normalized(" ".join(str(x) for x in blob_parts if x))
    return any(tok in blob for tok in PIVOT_TOKENS)


def _is_4hop(record: dict) -> bool:
    rid = record.get("id", "")
    if isinstance(rid, str) and rid.startswith("4hop"):
        return True
    # Fall back: count decomposition hops.
    decomp = record.get("question_decomposition") or []
    return len(decomp) == 4


def _trim_paragraphs(record: dict, limit: int) -> list[str]:
    out: list[str] = []
    for p in record.get("paragraphs", []) or []:
        title = (p.get("title") or "").strip()
        text = (p.get("paragraph_text") or "").strip()
        if not text:
            continue
        snippet = f"[{title}] {text}" if title else text
        out.append(snippet)
        if len(out) >= limit:
            break
    return out


def _simplify_decomp(record: dict) -> list[dict]:
    out = []
    for sub in record.get("question_decomposition", []) or []:
        out.append(
            {
                "question": sub.get("question", ""),
                "answer": sub.get("answer", ""),
            }
        )
    return out


def curate() -> int:
    if not DATA_FILE.exists():
        print(
            f"[curate] ERROR: {DATA_FILE} not found. Run setup.py first.",
            file=sys.stderr,
        )
        return 2

    selected: list[dict] = []
    total_scanned = 0
    total_4hop = 0
    total_pivot = 0

    with open(DATA_FILE, "r", encoding="utf-8") as f:
        for line in f:
            line = line.strip()
            if not line:
                continue
            total_scanned += 1
            try:
                rec = json.loads(line)
            except json.JSONDecodeError:
                continue
            if not _is_4hop(rec):
                continue
            total_4hop += 1
            if not _mentions_pivot(rec):
                continue
            total_pivot += 1

            trio_record = {
                "id": rec.get("id", f"q{len(selected)+1}"),
                "question": rec.get("question", ""),
                "answer": rec.get("answer", ""),
                "answer_aliases": rec.get("answer_aliases", []),
                "decomposition": _simplify_decomp(rec),
                "paragraphs": _trim_paragraphs(rec, MAX_PARAGRAPHS),
            }
            selected.append(trio_record)
            if len(selected) >= N_QUESTIONS:
                break

    print(
        f"[curate] scanned={total_scanned} 4hop={total_4hop} "
        f"pivot-matches={total_pivot} kept={len(selected)}"
    )

    if len(selected) < N_QUESTIONS:
        print(
            f"[curate] ERROR: wanted {N_QUESTIONS} questions, "
            f"found {len(selected)}",
            file=sys.stderr,
        )
        return 3

    OUT_FILE.parent.mkdir(parents=True, exist_ok=True)
    with open(OUT_FILE, "w", encoding="utf-8") as f:
        for i, rec in enumerate(selected, start=1):
            # Attach a stable short id q1/q2/q3 in addition to MuSiQue's id.
            rec["short_id"] = f"q{i}"
            f.write(json.dumps(rec, ensure_ascii=False) + "\n")

    print(f"[curate] wrote {OUT_FILE} ({len(selected)} records)")
    return 0


if __name__ == "__main__":
    raise SystemExit(curate())
