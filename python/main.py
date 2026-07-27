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
import traceback
from pathlib import Path
from typing import Any


def _bootstrap_paths() -> Path:
    here = Path(__file__).resolve().parent
    sys.path.insert(0, str(here))
    return here


HERE = _bootstrap_paths()
ROOT = HERE.parent


def _emergency_result(result_path: Path | None, query: str, message: str) -> None:
    """Пишет результат даже при падении импортов — чтобы AHK не говорил «нет результата»."""
    path = result_path or (ROOT / "logs" / "last_result.json")
    try:
        path.parent.mkdir(parents=True, exist_ok=True)
        payload = {
            "status": "error",
            "message": message,
            "name": "",
            "barcode": query,
            "ntin": "",
            "ntin_missing": True,
        }
        path.write_text(
            json.dumps(payload, ensure_ascii=False, indent=2),
            encoding="utf-8-sig",
        )
        txt = path.parent / "last_result.txt"
        lines = [
            f"status={payload['status']}",
            f"message={payload['message']}",
            f"name=",
            f"barcode={query}",
            f"ntin=",
            f"ntin_missing=true",
        ]
        txt.write_text("\n".join(lines) + "\n", encoding="utf-16")
        # Также в лог
        log = path.parent / "python_crash.log"
        log.write_text(message, encoding="utf-8")
    except Exception:
        pass


def write_result(path: Path, payload: dict[str, Any]) -> None:
    """Пишет JSON (UTF-8 BOM) и параллельный TXT (UTF-16) для AutoHotkey."""
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(
        json.dumps(payload, ensure_ascii=False, indent=2),
        encoding="utf-8-sig",
    )
    txt_path = path.parent / "last_result.txt"
    lines = [
        f"status={payload.get('status', '')}",
        f"message={payload.get('message', '')}",
        f"name={payload.get('name', '')}",
        f"barcode={payload.get('barcode', '')}",
        f"ntin={payload.get('ntin', '')}",
        f"ntin_missing={str(payload.get('ntin_missing', True)).lower()}",
    ]
    txt_path.write_text("\n".join(lines) + "\n", encoding="utf-16")


def build_internet_name(cand: Any) -> str:
    title = (cand.title or "").strip()
    brand = (cand.brand or "").strip()
    article = (cand.article or "").strip()
    bits: list[str] = []
    if title:
        bits.append(title)
    if article and article.lower() not in title.lower():
        bits.append(article)
    if brand and brand.lower() not in " ".join(bits).lower():
        bits.append(brand)
    return " ".join(bits).strip()


def run(query: str, cfg: dict[str, Any], logger) -> dict[str, Any]:
    from excel_search import search_excel
    from ntin_search import search_ntin
    from query_parser import clean_query, looks_like_part_number
    from ui_select import select_candidate
    from web_search import PartCandidate, search_parts

    parsed = clean_query(query)
    barcode = parsed.raw
    logger.info(
        "Старт поиска. raw=%r cleaned=%r variants=%s",
        parsed.raw,
        parsed.cleaned,
        parsed.variants,
    )

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

    # Всегда показываем выбор — автовыбор по FAPI часто даёт не тот товар
    chosen = select_candidate(candidates, parsed.cleaned)

    if chosen is None:
        return {
            "status": "cancelled",
            "message": "Подходящий товар не выбран. Данные в Microinvest не изменены.",
            "name": "",
            "barcode": barcode,
            "ntin": "",
            "ntin_missing": True,
        }

    logger.info("Выбран: %s", chosen.display)

    # Если выбрали строку из Excel или ручной ввод — формат «Наименование Модель Бренд»
    if chosen.source in {"excel", "manual"}:
        final_name = " ".join(
            p for p in (chosen.title, chosen.model, chosen.brand) if p
        ).strip()
        brand_for_ntin = chosen.brand
        model = chosen.model
        excel_hit = chosen.source == "excel"
    else:
        excel_match = search_excel(chosen.title, chosen.brand, cfg, query=parsed.cleaned)
        if excel_match:
            final_name = excel_match.formatted_name
            brand_for_ntin = excel_match.brand or chosen.brand
            model = excel_match.model
            excel_hit = True
        else:
            final_name = build_internet_name(chosen)
            brand_for_ntin = chosen.brand
            model = chosen.model
            excel_hit = False

    if not final_name:
        return {
            "status": "not_found",
            "message": f"Товар по запросу {parsed.cleaned} не найден.",
            "name": "",
            "barcode": barcode,
            "ntin": "",
            "ntin_missing": True,
        }

    ntin_name = chosen.title
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


def _parse_argv(argv: list[str] | None) -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Microinvest parts assistant")
    parser.add_argument("--query", default=None, help="Значение из поля «Имя»")
    parser.add_argument(
        "--query-file",
        default=None,
        help="UTF-8 файл с запросом (надежнее путей с кириллицей)",
    )
    parser.add_argument("--config", default=None, help="Путь к config.ini")
    parser.add_argument("--result", default=None, help="Путь к JSON-результату")
    args = parser.parse_args(argv)
    if not args.query and not args.query_file:
        parser.error("Нужен --query или --query-file")
    if args.query_file:
        qpath = Path(args.query_file)
        args.query = qpath.read_text(encoding="utf-8-sig").strip()
    return args


def main(argv: list[str] | None = None) -> int:
    # Сначала парсим аргументы — чтобы даже при ошибке импорта знать куда писать
    try:
        args = _parse_argv(argv)
    except SystemExit as exc:
        # argparse error — попробуем всё же оставить след
        code = int(exc.code) if isinstance(exc.code, int) else 3
        if code != 0:
            _emergency_result(
                ROOT / "logs" / "last_result.json",
                "",
                "Ошибка аргументов. Нужен --query или --query-file.",
            )
        raise
    except Exception as exc:
        _emergency_result(ROOT / "logs" / "last_result.json", "", f"Ошибка аргументов: {exc}")
        return 3

    result_path = Path(args.result) if args.result else (ROOT / "logs" / "last_result.json")
    result_path.parent.mkdir(parents=True, exist_ok=True)

    try:
        from config_loader import load_config
        from logger_setup import setup_logger
    except Exception as exc:
        msg = f"Не удалось импортировать модули. Установите зависимости: pip install -r requirements.txt\n{exc}\n{traceback.format_exc()}"
        _emergency_result(result_path, args.query, msg)
        print(msg, file=sys.stderr)
        return 3

    try:
        cfg = load_config(args.config)
        logger = setup_logger(cfg["log_dir"])
        if args.result:
            result_path = Path(args.result)
        else:
            result_path = cfg["result_file"]
        result_path.parent.mkdir(parents=True, exist_ok=True)
        logger.info("Python OK. query=%r result=%s", args.query, result_path)
    except Exception as exc:
        msg = f"Ошибка конфигурации: {exc}\n{traceback.format_exc()}"
        _emergency_result(result_path, args.query, msg)
        print(msg, file=sys.stderr)
        return 3

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

    try:
        write_result(result_path, payload)
    except Exception as exc:
        logger.exception("Не удалось записать результат: %s", exc)
        _emergency_result(result_path, args.query, f"Не удалось записать результат: {exc}")
        return 3

    print(json.dumps(payload, ensure_ascii=False))
    if payload.get("status") == "ok":
        return 0
    if payload.get("status") == "cancelled":
        return 2
    if payload.get("status") == "error":
        return 3
    return 1


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except SystemExit:
        raise
    except Exception as exc:
        _emergency_result(
            ROOT / "logs" / "last_result.json",
            "",
            f"Фатальная ошибка: {exc}\n{traceback.format_exc()}",
        )
        raise
