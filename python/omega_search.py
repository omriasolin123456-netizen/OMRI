"""Быстрый поиск по артикулу/штрихкоду через API Omega Auto Parts.

Документация: https://omega-auto-parts.kz/shop/api-docs
База: https://omega-auto-parts.kz/api/v1/

Нужны реквизиты оптового кабинета:
  /shop/api → userlogin + API-ключ (userpsw)

GET /search/brands/?number=...  — бренды и названия по номеру (~0.6с)
GET /search/articles/           — уточнение наличия/цены (опционально)
"""
from __future__ import annotations

import logging
import re
from typing import Any

import requests

from web_search import PartCandidate

logger = logging.getLogger("microinvest_assistant")

API_BASE = "https://omega-auto-parts.kz/api/v1"


def search_omega(query: str, cfg: dict[str, Any]) -> list[PartCandidate]:
    if not cfg.get("omega_enabled", True):
        return []

    login = (cfg.get("omega_login") or "").strip()
    api_key = (cfg.get("omega_api_key") or "").strip()
    if not login or not api_key:
        logger.info(
            "Omega: нет логина/API-ключа — пропуск "
            "(укажите [omega] login и api_key в config.ini)"
        )
        return []

    number = re.sub(r"\s+", "", (query or "").strip())
    if len(number) < 3:
        return []

    timeout = float(cfg.get("omega_timeout") or 5)
    base = (cfg.get("omega_api_base") or API_BASE).rstrip("/")

    try:
        brands = _get_brands(base, login, api_key, number, timeout)
    except Exception as exc:
        logger.error("Omega search/brands ошибка: %s", exc)
        return []

    if not brands:
        logger.info("Omega: по «%s» ничего не найдено", number)
        return []

    out: list[PartCandidate] = []
    seen: set[str] = set()

    # Опционально уточняем первые N брендов через articles (чуть дольше, но богаче)
    enrich = bool(cfg.get("omega_enrich_articles", False))
    enrich_limit = int(cfg.get("omega_enrich_limit") or 3)

    for i, row in enumerate(brands):
        brand = str(row.get("brand") or "").strip()
        art = str(row.get("number") or number).strip()
        title = str(row.get("description") or "").strip()
        model = _guess_model(title)

        if enrich and i < enrich_limit:
            try:
                arts = _get_articles(base, login, api_key, art, brand, timeout)
                if arts:
                    a0 = arts[0]
                    title = str(a0.get("description") or title).strip()
                    art = str(a0.get("number") or a0.get("numberFix") or art).strip()
                    model = _guess_model(title) or model
            except Exception as exc:
                logger.debug("Omega articles skip: %s", exc)

        if not title:
            title = f"{brand} {art}".strip() or number

        # Убрать дубль бренда/артикула из начала title
        title = _clean_title(title, brand, art)

        key = f"{brand}|{art}|{title}".upper()
        if key in seen:
            continue
        seen.add(key)
        out.append(
            PartCandidate(
                brand=brand,
                article=art,
                title=title,
                model=model,
                source="omega",
                extra={"raw": row},
            )
        )
        if len(out) >= int(cfg.get("max_candidates") or 12):
            break

    logger.info("Omega: %s вариантов по «%s»", len(out), number)
    return out


def _get_brands(
    base: str,
    login: str,
    api_key: str,
    number: str,
    timeout: float,
) -> list[dict]:
    url = f"{base}/search/brands/"
    resp = requests.get(
        url,
        params={"userlogin": login, "userpsw": api_key, "number": number},
        timeout=timeout,
        headers={"Accept": "application/json", "User-Agent": "MicroinvestPartsAssistant/1.0"},
    )
    if resp.status_code >= 400:
        try:
            err = resp.json()
        except Exception:
            err = {"errorMessage": resp.text[:200]}
        raise RuntimeError(f"HTTP {resp.status_code}: {err}")
    data = resp.json()
    if isinstance(data, dict) and data.get("errorCode"):
        raise RuntimeError(str(data))
    if not isinstance(data, list):
        return []
    return data


def _get_articles(
    base: str,
    login: str,
    api_key: str,
    number: str,
    brand: str,
    timeout: float,
) -> list[dict]:
    url = f"{base}/search/articles/"
    resp = requests.get(
        url,
        params={
            "userlogin": login,
            "userpsw": api_key,
            "number": number,
            "brand": brand,
            "withOutAnalogs": 1,
        },
        timeout=timeout,
        headers={"Accept": "application/json", "User-Agent": "MicroinvestPartsAssistant/1.0"},
    )
    if resp.status_code >= 400:
        return []
    data = resp.json()
    return data if isinstance(data, list) else []


def _guess_model(title: str) -> str:
    """Грубая модель из описания вида «... подходит на VW Golf III ...»."""
    if not title:
        return ""
    m = re.search(
        r"подходит\s+на\s+(.+)$",
        title,
        re.I,
    )
    if m:
        tail = m.group(1).strip()
        # взять 2–4 слова
        words = re.findall(r"[A-Za-zА-Яа-я0-9\-]+", tail)
        return " ".join(words[:4])
    return ""


def _clean_title(title: str, brand: str, article: str) -> str:
    t = title.strip()
    for piece in (brand, article):
        if not piece:
            continue
        t = re.sub(rf"^\s*{re.escape(piece)}\s*", "", t, flags=re.I)
        t = re.sub(rf"\s*{re.escape(piece)}\s*$", "", t, flags=re.I)
    t = re.sub(r"\s+", " ", t).strip(" -–—")
    return t or title.strip()


def probe_omega(cfg: dict[str, Any]) -> dict[str, Any]:
    """Проверка доступности API (для test_omega.py)."""
    import time

    base = (cfg.get("omega_api_base") or API_BASE).rstrip("/")
    login = (cfg.get("omega_login") or "").strip()
    api_key = (cfg.get("omega_api_key") or "").strip()
    number = (cfg.get("omega_test_number") or "120281").strip()

    result: dict[str, Any] = {
        "base": base,
        "has_credentials": bool(login and api_key),
        "login": login[:3] + "***" if login else "",
    }

    t0 = time.time()
    try:
        r = requests.get(
            f"{base}/search/brands/",
            params={"number": number},
            timeout=5,
            headers={"Accept": "application/json"},
        )
        result["noauth_status"] = r.status_code
        result["noauth_body"] = r.text[:200]
        result["noauth_ms"] = int((time.time() - t0) * 1000)
    except Exception as exc:
        result["noauth_error"] = str(exc)

    if login and api_key:
        t0 = time.time()
        try:
            rows = _get_brands(base, login, api_key, number, 5)
            result["auth_ms"] = int((time.time() - t0) * 1000)
            result["auth_count"] = len(rows)
            result["auth_sample"] = rows[:3]
        except Exception as exc:
            result["auth_error"] = str(exc)
            result["auth_ms"] = int((time.time() - t0) * 1000)

    return result
