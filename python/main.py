"""
Точка входа помощника Microinvest Склад Pro.

Читает исходный запрос из поля «Имя», ищет автозапчасть:
  1) супербыстро в Excel (ваши прайсы)
  2) если нет подходящего — быстрый интернет (FAPI)
показывает живой прогресс % и пишет JSON для AutoHotkey.
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
            "catalog_number": "",
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
            f"catalog_number=",
            f"ntin=",
            f"ntin_missing=true",
        ]
        txt.write_text("\n".join(lines) + "\n", encoding="utf-16")
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
        f"catalog_number={payload.get('catalog_number', '')}",
        f"ntin={payload.get('ntin', '')}",
        f"ntin_missing={str(payload.get('ntin_missing', True)).lower()}",
    ]
    txt_path.write_text("\n".join(lines) + "\n", encoding="utf-16")


def build_final_name(name: str, model: str, brand: str) -> str:
    """Строго: Название Модель Бренд (без артикула в имени)."""
    from ntin_search import short_product_name

    short = short_product_name(name)
    parts: list[str] = []
    for p in (short, model, brand):
        p = (p or "").strip()
        if not p:
            continue
        if parts and p.lower() in " ".join(parts).lower():
            continue
        parts.append(p)
    return " ".join(parts).strip()


def run(query: str, cfg: dict[str, Any], logger) -> dict[str, Any]:
    from excel_search import search_excel
    from ntin_search import search_ntin_candidates, short_product_name
    from progress_ui import ProgressUI
    from query_parser import clean_query, looks_like_part_number
    from ui_select import select_candidate, select_ntin
    from web_search import search_parts

    progress = ProgressUI("F8 — поиск запчасти")
    try:
        return _run_with_progress(
            query,
            cfg,
            logger,
            progress,
            search_excel,
            search_ntin_candidates,
            short_product_name,
            select_candidate,
            select_ntin,
            search_parts,
            clean_query,
            looks_like_part_number,
        )
    finally:
        progress.close()


def _run_with_progress(
    query: str,
    cfg: dict[str, Any],
    logger,
    progress,
    search_excel,
    search_ntin_candidates,
    short_product_name,
    select_candidate,
    select_ntin,
    search_parts,
    clean_query,
    looks_like_part_number,
) -> dict[str, Any]:
    progress.set(2, "Читаю запрос…")
    parsed = clean_query(query)
    barcode = parsed.raw
    logger.info(
        "Старт поиска. raw=%r cleaned=%r variants=%s",
        parsed.raw,
        parsed.cleaned,
        parsed.variants,
    )

    if not parsed.cleaned:
        progress.set(100, "Пустой запрос")
        return {
            "status": "not_found",
            "message": f"Товар по запросу {query} не найден.",
            "name": "",
            "barcode": barcode,
            "catalog_number": "",
            "ntin": "",
            "ntin_missing": True,
        }

    if not looks_like_part_number(parsed.cleaned):
        logger.warning("Запрос не похож на номер запчасти: %s", parsed.cleaned)

    def on_progress(pct: int, text: str) -> None:
        progress.set(pct, text)

    candidates = search_parts(parsed.variants, cfg, progress=on_progress)
    if not candidates:
        progress.set(100, "Ничего не найдено")
        return {
            "status": "not_found",
            "message": f"Товар по запросу {parsed.cleaned} не найден.",
            "name": "",
            "barcode": barcode,
            "catalog_number": "",
            "ntin": "",
            "ntin_missing": True,
        }

    # Скрыть прогресс перед окном выбора
    progress.set(92, "Выберите товар…")
    progress.close()

    chosen = select_candidate(candidates, parsed.cleaned)
    if chosen is None:
        return {
            "status": "cancelled",
            "message": "Подходящий товар не выбран. Данные в Microinvest не изменены.",
            "name": "",
            "barcode": barcode,
            "catalog_number": "",
            "ntin": "",
            "ntin_missing": True,
        }

    logger.info("Выбран: %s", chosen.display)

    # Новый прогресс для NTIN
    from progress_ui import ProgressUI as ProgressUI2

    progress2 = ProgressUI2("F8 — NTIN и финал")
    try:
        progress2.set(93, "Формирую наименование…")

        base_name = chosen.title
        brand = (chosen.brand or "").strip()
        model = (chosen.model or "").strip()
        source = chosen.source

        if chosen.source in {"excel", "manual"}:
            final_name = build_final_name(chosen.title, chosen.model, chosen.brand)
        else:
            excel_match = search_excel(chosen.title, chosen.brand, cfg, query=None)
            min_override = float(cfg.get("excel_override_score") or 0.8)
            if excel_match and excel_match.score >= min_override:
                logger.info(
                    "Excel подтвердил выбор score=%.2f → %s",
                    excel_match.score,
                    excel_match.formatted_name,
                )
                base_name = excel_match.name
                model = excel_match.model or model
                brand = excel_match.brand or brand
                source = "excel"
                final_name = build_final_name(base_name, model, brand)
            else:
                if excel_match:
                    if excel_match.brand and brand and excel_match.brand.lower() == brand.lower():
                        model = model or excel_match.model
                final_name = build_final_name(base_name, model, brand)

        if not final_name:
            progress2.set(100, "Пустое имя")
            return {
                "status": "not_found",
                "message": f"Товар по запросу {parsed.cleaned} не найден.",
                "name": "",
                "barcode": barcode,
            "catalog_number": "",
                "ntin": "",
                "ntin_missing": True,
            }

        ntin = ""
        ntin_missing = True
        if cfg.get("ntin_enabled", True):
            progress2.set(95, "Ищу NTIN (несколько запросов)…")
            ntin_cands = search_ntin_candidates(
                base_name,
                model,
                brand,
                cfg,
                barcode=barcode,
            )
            if len(ntin_cands) == 1:
                ntin = ntin_cands[0].ntin
                ntin_missing = False
            elif len(ntin_cands) > 1:
                progress2.close()
                picked = select_ntin(
                    ntin_cands,
                    build_final_name(short_product_name(base_name), model, brand),
                )
                if picked:
                    ntin = picked.ntin
                    ntin_missing = False
                else:
                    ntin_missing = True
                # прогресс уже закрыт
                progress2 = ProgressUI2("F8 — запись результата")
            else:
                ntin_missing = True

        progress2.set(99, "Сохраняю результат…")
        catalog_number = (
            (getattr(chosen, "catalog_number", None) or "")
            or str((chosen.extra or {}).get("catalog") or "")
        ).strip()

        payload = {
            "status": "ok",
            "message": (
                "Название найдено, NTIN отсутствует."
                if ntin_missing
                else "Карточка готова к заполнению."
            ),
            "name": final_name,
            "barcode": barcode,
            "catalog_number": catalog_number,
            "ntin": ntin,
            "ntin_missing": ntin_missing,
            "brand": brand,
            "model": model,
            "source": source,
            "query": parsed.cleaned,
        }
        progress2.set(100, "Готово!")
        logger.info("Результат: %s", payload)
        return payload
    finally:
        progress2.close()


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
    try:
        args = _parse_argv(argv)
    except SystemExit as exc:
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
        msg = (
            "Не удалось импортировать модули. Установите зависимости: "
            f"pip install -r requirements.txt\n{exc}\n{traceback.format_exc()}"
        )
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
            "catalog_number": "",
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
