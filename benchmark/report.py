#!/usr/bin/env python3
"""
Self-contained HTML report generator.

Renders a single HTML file with inline styles and an inline SVG scatter
plot. No external assets, no CDN calls, nothing to fetch. Safe to open
directly in a browser.
"""

from __future__ import annotations

import html
from pathlib import Path
from typing import Any


def _esc(s: Any) -> str:
    return html.escape(str(s if s is not None else ""))


def _scatter_svg(
    market_tokens: int,
    market_f1: float,
    baseline_tokens: int,
    baseline_f1: float,
    *,
    width: int = 520,
    height: int = 320,
) -> str:
    pad_l, pad_r, pad_t, pad_b = 70, 30, 30, 50
    plot_w = width - pad_l - pad_r
    plot_h = height - pad_t - pad_b

    max_tokens = max(market_tokens, baseline_tokens, 1)
    # Give a little headroom so points aren't on the axis.
    max_tokens_axis = max_tokens * 1.15
    min_tokens_axis = 0

    def sx(tokens: float) -> float:
        frac = (tokens - min_tokens_axis) / max(
            max_tokens_axis - min_tokens_axis, 1
        )
        return pad_l + frac * plot_w

    def sy(f1: float) -> float:
        # y=0 at top of plot, y=1 at bottom -> invert
        return pad_t + (1.0 - max(0.0, min(1.0, f1))) * plot_h

    axis_color = "#555"
    grid_color = "#eee"
    market_color = "#2563eb"
    baseline_color = "#dc2626"

    parts: list[str] = []
    parts.append(
        f'<svg xmlns="http://www.w3.org/2000/svg" width="{width}" '
        f'height="{height}" viewBox="0 0 {width} {height}" '
        f'role="img" aria-label="Pareto scatter: tokens vs F1">'
    )
    parts.append(
        f'<rect x="0" y="0" width="{width}" height="{height}" '
        f'fill="white"/>'
    )
    # Gridlines at F1 = 0, 0.25, 0.5, 0.75, 1.0
    for f in (0.0, 0.25, 0.5, 0.75, 1.0):
        y = sy(f)
        parts.append(
            f'<line x1="{pad_l}" y1="{y:.1f}" x2="{width-pad_r}" '
            f'y2="{y:.1f}" stroke="{grid_color}" stroke-width="1"/>'
        )
        parts.append(
            f'<text x="{pad_l-8}" y="{y+4:.1f}" font-family="sans-serif" '
            f'font-size="11" fill="{axis_color}" text-anchor="end">'
            f'{f:.2f}</text>'
        )
    # X-axis ticks
    for frac in (0.0, 0.25, 0.5, 0.75, 1.0):
        t_val = frac * max_tokens_axis
        x = sx(t_val)
        parts.append(
            f'<line x1="{x:.1f}" y1="{height-pad_b}" x2="{x:.1f}" '
            f'y2="{height-pad_b+4}" stroke="{axis_color}"/>'
        )
        parts.append(
            f'<text x="{x:.1f}" y="{height-pad_b+18}" '
            f'font-family="sans-serif" font-size="11" fill="{axis_color}" '
            f'text-anchor="middle">{int(t_val)}</text>'
        )
    # Axis lines
    parts.append(
        f'<line x1="{pad_l}" y1="{pad_t}" x2="{pad_l}" '
        f'y2="{height-pad_b}" stroke="{axis_color}"/>'
    )
    parts.append(
        f'<line x1="{pad_l}" y1="{height-pad_b}" x2="{width-pad_r}" '
        f'y2="{height-pad_b}" stroke="{axis_color}"/>'
    )
    # Axis labels
    parts.append(
        f'<text x="{width/2:.1f}" y="{height-10}" '
        f'font-family="sans-serif" font-size="12" fill="{axis_color}" '
        f'text-anchor="middle">tokens</text>'
    )
    parts.append(
        f'<text x="15" y="{height/2:.1f}" font-family="sans-serif" '
        f'font-size="12" fill="{axis_color}" text-anchor="middle" '
        f'transform="rotate(-90 15 {height/2:.1f})">F1</text>'
    )
    # Baseline point
    bx, by = sx(baseline_tokens), sy(baseline_f1)
    parts.append(
        f'<circle cx="{bx:.1f}" cy="{by:.1f}" r="7" '
        f'fill="{baseline_color}"/>'
    )
    parts.append(
        f'<text x="{bx+10:.1f}" y="{by+4:.1f}" font-family="sans-serif" '
        f'font-size="11" fill="{baseline_color}">baseline</text>'
    )
    # Market point
    mx, my = sx(market_tokens), sy(market_f1)
    parts.append(
        f'<circle cx="{mx:.1f}" cy="{my:.1f}" r="7" '
        f'fill="{market_color}"/>'
    )
    parts.append(
        f'<text x="{mx+10:.1f}" y="{my+4:.1f}" font-family="sans-serif" '
        f'font-size="11" fill="{market_color}">marketplace</text>'
    )

    parts.append("</svg>")
    return "".join(parts)


CSS = """\
body { font-family: -apple-system, system-ui, sans-serif;
       max-width: 960px; margin: 2rem auto; padding: 0 1rem;
       color: #1f2937; line-height: 1.55; }
h1, h2, h3 { color: #111827; }
h1 { border-bottom: 2px solid #2563eb; padding-bottom: .4rem; }
.verdict-pass { display: inline-block; background: #dcfce7;
                color: #166534; padding: .3rem .8rem; border-radius: 6px;
                font-weight: 600; }
.verdict-fail { display: inline-block; background: #fee2e2;
                color: #991b1b; padding: .3rem .8rem; border-radius: 6px;
                font-weight: 600; }
table { border-collapse: collapse; margin: .8rem 0; width: 100%; }
th, td { border: 1px solid #e5e7eb; padding: .4rem .6rem;
         text-align: left; vertical-align: top; }
th { background: #f9fafb; }
pre, code { background: #f3f4f6; border-radius: 4px;
            padding: .1rem .4rem; font-size: .9rem; }
pre { padding: .8rem; white-space: pre-wrap; word-break: break-word; }
.card { border: 1px solid #e5e7eb; border-radius: 8px;
        padding: 1rem 1.2rem; margin: 1rem 0; background: #fff; }
.kv { display: grid; grid-template-columns: 180px 1fr; gap: .3rem .8rem; }
.small { color: #6b7280; font-size: .88rem; }
"""


def render_report(data: dict[str, Any], out_path: Path) -> None:
    verdict = data.get("pareto", {})
    is_pass = verdict.get("verdict") == "PASS"
    verdict_html = (
        '<span class="verdict-pass">PASS — strictly northwest</span>'
        if is_pass
        else '<span class="verdict-fail">FAIL — not dominating baseline</span>'
    )

    bids_rows: list[str] = []
    for b in data.get("bids", []):
        conf_str = "{:.2f}".format(b.get("confidence", 0) or 0)
        bids_rows.append(
            f"<tr><td>{_esc(b.get('agent'))}</td>"
            f"<td>{_esc(b.get('estimated_tokens'))}</td>"
            f"<td>{_esc(conf_str)}</td>"
            f"<td>{_esc(b.get('approach'))}</td></tr>"
        )
    bids_table = "\n".join(bids_rows) or (
        "<tr><td colspan=4>no bids</td></tr>"
    )

    decomp_rows: list[str] = []
    for i, sub in enumerate(data.get("decomposition", []) or [], start=1):
        decomp_rows.append(
            f"<tr><td>{i}</td><td>{_esc(sub.get('question'))}</td>"
            f"<td>{_esc(sub.get('answer'))}</td></tr>"
        )
    decomp_table = "\n".join(decomp_rows) or (
        "<tr><td colspan=3>(none)</td></tr>"
    )

    rep_rows: list[str] = []
    for rep in data.get("reputation", []) or []:
        score_str = "{:.3f}".format(rep.get("score", 0) or 0)
        rep_rows.append(
            f"<tr><td>{_esc(rep.get('agent'))}</td>"
            f"<td>{_esc(rep.get('domain'))}</td>"
            f"<td>{_esc(rep.get('runs'))}</td>"
            f"<td>{_esc(rep.get('correct'))}</td>"
            f"<td>{_esc(rep.get('tokens_spent'))}</td>"
            f"<td>{_esc(score_str)}</td></tr>"
        )
    rep_table = "\n".join(rep_rows) or (
        "<tr><td colspan=6>(empty)</td></tr>"
    )

    svg = _scatter_svg(
        market_tokens=int(data.get("market", {}).get("tokens", 0)),
        market_f1=float(data.get("market", {}).get("f1", 0.0)),
        baseline_tokens=int(data.get("baseline", {}).get("tokens", 0)),
        baseline_f1=float(data.get("baseline", {}).get("f1", 0.0)),
    )

    market = data.get("market", {})
    baseline = data.get("baseline", {})

    market_f1_str = "{:.3f}".format(verdict.get("market_f1", 0) or 0)
    baseline_f1_str = "{:.3f}".format(verdict.get("baseline_f1", 0) or 0)
    f1_delta_str = "{:.3f}".format(verdict.get("f1_delta", 0) or 0)
    market_run_f1_str = "{:.3f}".format(market.get("f1", 0) or 0)
    baseline_run_f1_str = "{:.3f}".format(baseline.get("f1", 0) or 0)

    html_doc = f"""<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8"/>
<title>MuSiQue MAS Benchmark — {_esc(data.get('question_id', ''))}</title>
<style>{CSS}</style>
</head>
<body>
<h1>MuSiQue MAS Benchmark Report</h1>
<p class="small">
  Mode: <code>{_esc(data.get('mode', ''))}</code>
  &middot; Question: <code>{_esc(data.get('question_id', ''))}</code>
  &middot; Dry-run: <code>{_esc(data.get('dry_run', False))}</code>
</p>

<div class="card">
  <h2>Verdict</h2>
  <p>{verdict_html}</p>
  <div class="kv">
    <div>Market tokens</div><div>{_esc(verdict.get('market_tokens'))}</div>
    <div>Market F1</div><div>{_esc(market_f1_str)}</div>
    <div>Baseline tokens</div><div>{_esc(verdict.get('baseline_tokens'))}</div>
    <div>Baseline F1</div><div>{_esc(baseline_f1_str)}</div>
    <div>Tokens delta</div><div>{_esc(verdict.get('tokens_delta'))}</div>
    <div>F1 delta</div><div>{_esc(f1_delta_str)}</div>
  </div>
</div>

<div class="card">
  <h2>Pareto plot</h2>
  {svg}
  <p class="small">
    Lower-right = expensive and wrong. Upper-left = cheap and correct.
    Marketplace must sit strictly northwest of baseline to pass.
  </p>
</div>

<div class="card">
  <h2>Question</h2>
  <p><strong>{_esc(data.get('question', ''))}</strong></p>
  <p>Gold answer: <code>{_esc(data.get('gold_answer', ''))}</code></p>

  <h3>Gold decomposition</h3>
  <table>
    <tr><th>#</th><th>Sub-question</th><th>Sub-answer</th></tr>
    {decomp_table}
  </table>
</div>

<div class="card">
  <h2>Auction</h2>
  <p>Domain: <code>{_esc(data.get('domain', ''))}</code>
     &middot; Budget: <code>{_esc(data.get('max_budget_tokens', ''))}</code>
     &middot; Awarded to: <code>{_esc(data.get('awarded_to', ''))}</code></p>
  <h3>Bids received</h3>
  <table>
    <tr><th>Agent</th><th>Est. tokens</th><th>Confidence</th><th>Approach</th></tr>
    {bids_table}
  </table>
</div>

<div class="card">
  <h2>Marketplace run</h2>
  <div class="kv">
    <div>Winning agent</div><div>{_esc(market.get('agent'))}</div>
    <div>Model</div><div>{_esc(market.get('model'))}</div>
    <div>Tokens</div><div>{_esc(market.get('tokens'))}</div>
    <div>F1</div><div>{_esc(market_run_f1_str)}</div>
    <div>Answer</div><div><code>{_esc(market.get('answer'))}</code></div>
  </div>
</div>

<div class="card">
  <h2>Single-agent baseline</h2>
  <div class="kv">
    <div>Model</div><div>{_esc(baseline.get('model'))}</div>
    <div>Tokens</div><div>{_esc(baseline.get('tokens'))}</div>
    <div>F1</div><div>{_esc(baseline_run_f1_str)}</div>
    <div>Answer</div><div><code>{_esc(baseline.get('answer'))}</code></div>
  </div>
</div>

<div class="card">
  <h2>Reputation ledger (post-run)</h2>
  <table>
    <tr><th>Agent</th><th>Domain</th><th>Runs</th><th>Correct</th>
        <th>Tokens</th><th>Score</th></tr>
    {rep_table}
  </table>
</div>

<p class="small">
  Generated by <code>benchmark/report.py</code>.
  Marketplace primitives are currently stubbed in-process — see
  <code>benchmark/marketplace.py</code> for the migration plan to the
  real 016 SynapBus MCP tools.
</p>
</body>
</html>
"""
    out_path.parent.mkdir(parents=True, exist_ok=True)
    out_path.write_text(html_doc, encoding="utf-8")
