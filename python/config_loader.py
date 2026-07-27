"""Загрузка config.ini и путей проекта."""
from __future__ import annotations

import configparser
from pathlib import Path
from typing import Any


ROOT = Path(__file__).resolve().parent.parent


def _as_bool(value: str | bool, default: bool = False) -> bool:
    if isinstance(value, bool):
        return value
    if value is None:
        return default
    return str(value).strip().lower() in {"1", "true", "yes", "on", "да"}


def load_config(config_path: str | Path | None = None) -> dict[str, Any]:
    path = Path(config_path) if config_path else ROOT / "config.ini"
    parser = configparser.ConfigParser()
    parser.read(path, encoding="utf-8")

    def get(section: str, key: str, fallback: str = "") -> str:
        return parser.get(section, key, fallback=fallback).strip()

    price_rel = get("paths", "price_dir", "price")
    log_rel = get("paths", "log_dir", "logs")
    result_rel = get("paths", "result_file", "logs/last_result.json")

    def resolve(p: str) -> Path:
        pp = Path(p)
        return pp if pp.is_absolute() else (path.parent / pp)

    cfg: dict[str, Any] = {
        "config_path": path,
        "root": path.parent,
        "hotkey": get("hotkey", "key", "F8"),
        "window_title": get("microinvest", "window_title", "Microinvest"),
        "field_name": get("microinvest", "field_name"),
        "field_barcode": get("microinvest", "field_barcode"),
        "field_ntin": get("microinvest", "field_ntin"),
        "price_dir": resolve(price_rel),
        "log_dir": resolve(log_rel),
        "result_file": resolve(result_rel),
        "fapi_key": get("search", "fapi_key"),
        "fapi_base": get("search", "fapi_base", "https://fapi.iisis.ru/fapi/v2").rstrip("/"),
        "enable_web_enrichment": _as_bool(get("search", "enable_web_enrichment", "false"), False),
        "max_candidates": int(get("search", "max_candidates", "10") or 10),
        "http_timeout": float(get("search", "http_timeout", "8") or 8),
        "ntin_enabled": _as_bool(get("ntin", "enabled", "true"), True),
        "ntin_api_token": get("ntin", "api_token"),
        "ntin_api_base": get("ntin", "api_base", "https://nct.kz").rstrip("/"),
        "ntin_web_search": _as_bool(get("ntin", "web_search", "true"), True),
        "ntin_timeout": float(get("ntin", "ntin_timeout", "6") or 6),
        "col_name": [x.strip() for x in get("excel", "col_name").split("|") if x.strip()],
        "col_brand": [x.strip() for x in get("excel", "col_brand").split("|") if x.strip()],
        "col_model": [x.strip() for x in get("excel", "col_model").split("|") if x.strip()],
        "ignore_article_columns": _as_bool(get("excel", "ignore_article_columns", "true"), True),
        "excel_search_article_columns": _as_bool(
            get("excel", "excel_search_article_columns", "false"), False
        ),
        "min_match_score": float(get("excel", "min_match_score", "0.55") or 0.55),
        "excel_override_score": float(get("excel", "excel_override_score", "0.8") or 0.8),
        "selection_mode": get("ui", "selection_mode", "always"),
    }
    cfg["log_dir"].mkdir(parents=True, exist_ok=True)
    cfg["price_dir"].mkdir(parents=True, exist_ok=True)
    cfg["result_file"].parent.mkdir(parents=True, exist_ok=True)
    return cfg
