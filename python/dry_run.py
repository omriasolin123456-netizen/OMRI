"""Неинтерактивный прогон пайплайна (первый кандидат без GUI)."""
from __future__ import annotations

import json
import sys
from pathlib import Path
from unittest.mock import patch

sys.path.insert(0, str(Path(__file__).resolve().parent))

from config_loader import load_config
from logger_setup import setup_logger
from main import run, write_result
from web_search import PartCandidate


def _pick_first(candidates, query):
    print("CANDIDATES:")
    for i, c in enumerate(candidates, 1):
        print(f"  {i}. {c.display} [{c.source}]")
    return candidates[0] if candidates else None


def main():
    query = sys.argv[1] if len(sys.argv) > 1 else "361337"
    cfg = load_config()
    logger = setup_logger(cfg["log_dir"])
    with patch("main.select_candidate", side_effect=_pick_first):
        payload = run(query, cfg, logger)
    out = cfg["result_file"]
    write_result(out, payload)
    print(json.dumps(payload, ensure_ascii=False, indent=2))
    return 0 if payload.get("status") == "ok" else 1


if __name__ == "__main__":
    raise SystemExit(main())
