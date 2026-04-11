#!/usr/bin/env python3
"""
MuSiQue dataset downloader.

Downloads ``musique_v1.0.zip`` from the canonical source used by the
upstream project (https://github.com/StonyBrookNLP/musique). The zip is
hosted on Google Drive (file id ``1tGdADlNjWFaHLeZZGShh2IRcpO6Lv24h``);
this mirrors the behavior of the project's ``download_data.sh`` which
uses ``gdown`` under the hood.

Idempotent — skips download if the target dev-set jsonl already exists.
Run: ``python benchmark/setup.py``
"""

from __future__ import annotations

import os
import re
import sys
import zipfile
from pathlib import Path

import requests

GDRIVE_FILE_ID = "1tGdADlNjWFaHLeZZGShh2IRcpO6Lv24h"
GDRIVE_URL = "https://docs.google.com/uc?export=download"

DATA_DIR = Path(__file__).resolve().parent / "data"
ZIP_PATH = DATA_DIR / "musique_v1.0.zip"
TARGET_FILE = DATA_DIR / "musique_ans_v1.0_dev.jsonl"


def _write_stream(resp: requests.Response, dest: Path) -> int:
    total = int(resp.headers.get("Content-Length", 0))
    downloaded = 0
    dest.parent.mkdir(parents=True, exist_ok=True)
    with open(dest, "wb") as f:
        for chunk in resp.iter_content(chunk_size=1024 * 1024):
            if not chunk:
                continue
            f.write(chunk)
            downloaded += len(chunk)
            if total:
                pct = 100.0 * downloaded / total
                print(
                    f"\r  downloading: {downloaded/1e6:6.1f} MB "
                    f"/ {total/1e6:6.1f} MB ({pct:5.1f}%)",
                    end="",
                    file=sys.stderr,
                )
    print("", file=sys.stderr)
    return downloaded


def _download_gdrive(file_id: str, dest: Path) -> bool:
    """
    Download a large file from Google Drive, handling the virus-scan
    confirmation page that Drive injects for anything over ~100 MB.
    """
    session = requests.Session()
    try:
        resp = session.get(
            GDRIVE_URL,
            params={"id": file_id, "export": "download"},
            stream=True,
            timeout=60,
        )
    except requests.RequestException as exc:
        print(f"  -> request failed: {exc}", file=sys.stderr)
        return False

    # Case 1: Drive returns the file directly (small file or cached).
    ctype = resp.headers.get("Content-Type", "")
    if "text/html" not in ctype.lower():
        _write_stream(resp, dest)
        return dest.exists() and dest.stat().st_size > 0

    # Case 2: HTML confirmation page. Extract the confirm token and/or
    # the form action URL.
    html = resp.text
    # Newer Drive flow: a <form ...> with all the params we need.
    form_match = re.search(
        r'<form[^>]*id="download-form"[^>]*action="([^"]+)"', html
    )
    if form_match:
        action = form_match.group(1).replace("&amp;", "&")
        params = dict(
            re.findall(
                r'name="([^"]+)"[^>]*value="([^"]+)"', html
            )
        )
        try:
            resp2 = session.get(action, params=params, stream=True, timeout=120)
            if resp2.status_code == 200:
                _write_stream(resp2, dest)
                return dest.exists() and dest.stat().st_size > 0
        except requests.RequestException as exc:
            print(f"  -> form post failed: {exc}", file=sys.stderr)
            return False

    # Older flow: confirm cookie token.
    token = None
    for k, v in session.cookies.items():
        if k.startswith("download_warning"):
            token = v
            break
    if token is None:
        m = re.search(r'confirm=([0-9A-Za-z_-]+)', html)
        if m:
            token = m.group(1)
    if token:
        try:
            resp3 = session.get(
                GDRIVE_URL,
                params={
                    "id": file_id,
                    "export": "download",
                    "confirm": token,
                },
                stream=True,
                timeout=120,
            )
            if resp3.status_code == 200:
                _write_stream(resp3, dest)
                return dest.exists() and dest.stat().st_size > 0
        except requests.RequestException as exc:
            print(f"  -> confirm fetch failed: {exc}", file=sys.stderr)
            return False

    print("  -> could not navigate Google Drive download flow", file=sys.stderr)
    return False


def _extract(zip_path: Path, out_dir: Path) -> None:
    """Extract the dev set jsonl from the zip."""
    wanted_suffixes = (
        "musique_ans_v1.0_dev.jsonl",
        "musique_ans_v1.0_train.jsonl",
    )
    with zipfile.ZipFile(zip_path) as zf:
        members = zf.namelist()
        extracted_any = False
        for m in members:
            base = os.path.basename(m)
            if base in wanted_suffixes:
                with zf.open(m) as src, open(out_dir / base, "wb") as dst:
                    dst.write(src.read())
                print(f"  extracted: {base}")
                extracted_any = True
        if not extracted_any:
            # Fall back: extract everything so a human can inspect.
            zf.extractall(out_dir)
            print(
                "  could not find canonical filenames; extracted all",
                file=sys.stderr,
            )


def main() -> int:
    DATA_DIR.mkdir(parents=True, exist_ok=True)

    if TARGET_FILE.exists():
        size = TARGET_FILE.stat().st_size
        print(f"[setup] already present: {TARGET_FILE} ({size/1e6:.1f} MB)")
        return 0

    print(f"[setup] downloading Google Drive file id {GDRIVE_FILE_ID}")
    ok = _download_gdrive(GDRIVE_FILE_ID, ZIP_PATH)

    if not ok:
        print(
            "[setup] ERROR: failed to download MuSiQue. Please download "
            "manually from "
            f"https://drive.google.com/file/d/{GDRIVE_FILE_ID}/view "
            f"and place the zip at {ZIP_PATH}",
            file=sys.stderr,
        )
        return 2

    print(f"[setup] extracting {ZIP_PATH}")
    _extract(ZIP_PATH, DATA_DIR)

    if not TARGET_FILE.exists():
        print(
            f"[setup] WARNING: {TARGET_FILE.name} not found after extract. "
            f"Listing {DATA_DIR}:",
            file=sys.stderr,
        )
        for p in sorted(DATA_DIR.iterdir()):
            print(f"  - {p.name}", file=sys.stderr)
        return 3

    print(f"[setup] ready: {TARGET_FILE}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
