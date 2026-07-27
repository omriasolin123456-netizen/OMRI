"""
Точка входа помощника Microinvest Склад Pro.

Читает исходный запрос из поля «Имя», ищет автозапчасть, сверяет с Excel,
получает NTIN и пишет JSON-результат для AutoHotkey.
Кэш товаров не используется — каждый запуск выполняет новый поиск.
"""
from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path
from typing import Any

# Чтобы импорты работали при запуске python python/main.py
sys.path.insert(0, str(Path(__file__).resolve().parent))

from config_loader import load_config
from excel_search import search_excel
from logger_setup import setup_logger
from ntin_search import search_ntin
from query_parser import clean_query, looks_like_part_number
from ui_select import select_candidate
from web_search import PartCandidate, search_parts


def write_result(path: Path, payload: dict[str, Any]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(
        json.dumps(payload, ensure_ascii=False, indent=2),
        encoding="utf-8",
    )


def build_internet_name(cand: PartCandidate) -> str:
    """Имя из интернет-источника, пока нет приоритетного Excel."""
    # Порядок близок к целевому: Наименование [артикул] Бренд
    title = cand.title.strip()
    brand = cand.brand.strip()
    article = cand.article.strip()
    # Если артикул уже есть в title — не дублируем
    bits: list[str] = []
    if title:
        bits.append(title)
    if article and article.lower() not in title.lower():
        bits.append(article)
    if brand and brand.lower() not in " ".join(bits).lower():
        bits.append(brand)
    return " ".join(bits).strip()


def run(query: str, cfg: dict[str, Any], logger) -> dict[str, Any]:
    parsed = clean_query(query)
    barcode = parsed.raw  # исходное значение поля «Имя» → Штрихкод
    logger.info("Старт поиска. raw=%r cleaned=%r variants=%s", parsed.raw, parsed.cleaned, parsed.variants)

    if not parsed.cleaned:
        return {
            "status": "not_found",
            "message": f"Товар по запросу {query} не найден.",
            "name": "",
            "barcode": barcode,
            "ntin": "",
            "ntin_missing": True,
        }

    if not looks_like_part_number(parsed.cleaned):
        logger.warning("Запрос не похож на номер запчасти: %s", parsed.cleaned)

    candidates = search_parts(parsed.variants, cfg)
    if not candidates:
        return {
            "status": "not_found",
            "message": f"Товар по запросу {parsed.cleaned} не найден.",
            "name": "",
            "barcode": barcode,
            "ntin": "",
            "ntin_missing": True,
        }

    # Выбор варианта
    mode = (cfg.get("selection_mode") or "auto_single").lower()
    chosen: PartCandidate | None
    if len(candidates) == 1 and mode == "auto_single":
        chosen = candidates[0]
    else:
        chosen = select_candidate(candidates, parsed.cleaned)

    if chosen is None:
        return {
            "status": "cancelled",
            "message": "Выбор отменён пользователем.",
            "name": "",
            "barcode": barcode,
            "ntin": "",
            "ntin_missing": True,
        }

    logger.info("Выбран: %s", chosen.display)

    # Excel — приоритетный источник названия (Наименование Модель Бренд)
    excel_hit = search_excel(chosen.title, chosen.brand, cfg)
    if excel_hit:
        final_name = excel_hit.formatted_name
        brand_for_ntin = excel_hit.brand or chosen.brand
        model = excel_hit.model
    else:
        final_name = build_internet_name(chosen)
        brand_for_ntin = chosen.brand
        model = chosen.model

    if not final_name:
        return {
            "status": "not_found",
            "message": f"Товар по запросу {parsed.cleaned} не найден.",
            "name": "",
            "barcode": barcode,
            "ntin": "",
            "ntin_missing": True,
        }

    # Для NTIN передаём наименование без повторного бренда в конце
    ntin_name = excel_hit.name if excel_hit else chosen.title
    ntin = search_ntin(ntin_name, brand_for_ntin, cfg) or ""
    ntin_missing = not bool(ntin)

    payload = {
        "status": "ok",
        "message": (
            "Название найдено, NTIN отсутствует."
            if ntin_missing
            else "Карточка готова к заполнению."
        ),
        "name": final_name,
        "barcode": barcode,
        "ntin": ntin,
        "ntin_missing": ntin_missing,
        "brand": brand_for_ntin,
        "model": model,
        "source": "excel" if excel_hit else chosen.source,
        "query": parsed.cleaned,
    }
    logger.info("Результат: %s", payload)
    return payload


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description="Microinvest parts assistant")
    parser.add_argument("--query", required=True, help="Значение из поля «Имя»")
    parser.add_argument("--config", default=None, help="Путь к config.ini")
    parser.add_argument("--result", default=None, help="Путь к JSON-результату")
    args = parser.parse_args(argv)

    cfg = load_config(args.config)
    logger = setup_logger(cfg["log_dir"])
    result_path = Path(args.result) if args.result else cfg["result_file"]

    try:
        payload = run(args.query, cfg, logger)
    except Exception as exc:
        logger.exception("Критическая ошибка: %s", exc)
        payload = {
            "status": "error",
            "message": f"Ошибка поиска: {exc}",
            "name": "",
            "barcode": args.query,
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
