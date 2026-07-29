"""
F7: поиск NTIN по текущему полю «Имя» и запись только NTIN.

python/ntin_only.py --query-file logs/query.txt --result logs/last_result.json
"""
from __future__ import annotations

import argparse
import json
import sys
import traceback
from pathlib import Path
from typing import Any


def _bootstrap() -> Path:
    here = Path(__file__).resolve().parent
    sys.path.insert(0, str(here))
    return here.parent


ROOT = _bootstrap()


def write_result(path: Path, payload: dict[str, Any]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(payload, ensure_ascii=False, indent=2), encoding="utf-8-sig")
    txt = path.parent / "last_result.txt"
    lines = [
        f"status={payload.get('status', '')}",
        f"message={payload.get('message', '')}",
        f"name={payload.get('name', '')}",
        f"barcode={payload.get('barcode', '')}",
        f"catalog_number={payload.get('catalog_number', '')}",
        f"ntin={payload.get('ntin', '')}",
        f"ntin_missing={str(payload.get('ntin_missing', True)).lower()}",
    ]
    txt.write_text("\n".join(lines) + "\n", encoding="utf-16")


def run_ntin(name: str, cfg: dict[str, Any], logger) -> dict[str, Any]:
    from ntin_search import search_ntin_by_full_name
    from progress_ui import ProgressUI
    from ui_select import select_ntin

    progress = ProgressUI("F7 — поиск NTIN")
    try:
        progress.set(5, "Разбираю наименование…")
        progress.set(20, "Параллельный поиск в НКТ…")
        cands = search_ntin_by_full_name(name, cfg)
        progress.set(85, f"Найдено вариантов: {len(cands)}")
        progress.close()

        if not cands:
            return {
                "status": "not_found",
                "message": f"NTIN по имени «{name}» не найден.",
                "name": name,
                "barcode": "",
                "catalog_number": "",
                "ntin": "",
                "ntin_missing": True,
            }

        if len(cands) == 1:
            picked = cands[0]
        else:
            picked = select_ntin(cands, name)
            if picked is None:
                return {
                    "status": "cancelled",
                    "message": "NTIN не выбран. Поле не изменено.",
                    "name": name,
                    "barcode": "",
                    "catalog_number": "",
                    "ntin": "",
                    "ntin_missing": True,
                }

        logger.info("F7 NTIN выбран: %s ← «%s»", picked.ntin, picked.query)
        return {
            "status": "ok",
            "message": "NTIN найден.",
            "name": name,
            "barcode": "",
            "catalog_number": "",
            "ntin": picked.ntin,
            "ntin_missing": False,
            "ntin_card": picked.name,
            "ntin_query": picked.query,
        }
    finally:
        progress.close()


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description="F7 NTIN-only search")
    parser.add_argument("--query", default=None)
    parser.add_argument("--query-file", default=None)
    parser.add_argument("--config", default=None)
    parser.add_argument("--result", default=None)
    args = parser.parse_args(argv)

    if args.query_file:
        args.query = Path(args.query_file).read_text(encoding="utf-8-sig").strip()
    if not args.query:
        print("Need --query or --query-file", file=sys.stderr)
        return 3

    result_path = Path(args.result) if args.result else (ROOT / "logs" / "last_result.json")

    try:
        from config_loader import load_config
        from logger_setup import setup_logger

        cfg = load_config(args.config)
        logger = setup_logger(cfg["log_dir"])
        if not args.result:
            result_path = cfg["result_file"]
    except Exception as exc:
        write_result(
            result_path,
            {
                "status": "error",
                "message": f"Ошибка конфигурации: {exc}",
                "name": args.query,
                "barcode": "",
                "catalog_number": "",
                "ntin": "",
                "ntin_missing": True,
            },
        )
        return 3

    try:
        payload = run_ntin(args.query, cfg, logger)
    except Exception as exc:
        logger.exception("F7 NTIN error: %s", exc)
        payload = {
            "status": "error",
            "message": f"Ошибка поиска NTIN: {exc}\n{traceback.format_exc()[:500]}",
            "name": args.query,
            "barcode": "",
            "catalog_number": "",
            "ntin": "",
            "ntin_missing": True,
        }

    write_result(result_path, payload)
    print(json.dumps(payload, ensure_ascii=False))
    if payload.get("status") == "ok":
        return 0
    if payload.get("status") == "cancelled":
        return 2
    if payload.get("status") == "error":
        return 3
    return 1


if __name__ == "__main__":
    raise SystemExit(main())
