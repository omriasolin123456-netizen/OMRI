"""Поиск NTIN в Национальном каталоге товаров Казахстана (nct.gov.kz / nct.kz)."""
from __future__ import annotations

import logging
import re
from typing import Any

import requests

logger = logging.getLogger("microinvest_assistant")

NTIN_RE = re.compile(r"\b(\d{13,14})\b")
NTIN_LABEL_RE = re.compile(
    r"(?:NTIN|НТИН|KZTIN|XTIN)\s*[:=]?\s*(\d{13,14})",
    re.I,
)


def search_ntin(product_name: str, brand: str, cfg: dict[str, Any]) -> str | None:
    if not cfg.get("ntin_enabled", True):
        return None

    query = " ".join(p for p in (product_name, brand) if p).strip()
    if not query:
        return None

    token = (cfg.get("ntin_api_token") or "").strip()
    if token:
        ntin = _search_ntin_api(query, token, cfg)
        if ntin:
            return ntin

    if cfg.get("ntin_web_search", True):
        ntin = _search_ntin_web(query, cfg)
        if ntin:
            return ntin

    logger.info("NTIN не найден для «%s»", query)
    return None


def _search_ntin_api(query: str, token: str, cfg: dict[str, Any]) -> str | None:
    """Попытка запроса к API НКТ (если выдан токен в config.ini)."""
    base = (cfg.get("ntin_api_base") or "https://nct.kz").rstrip("/")
    timeout = float(cfg.get("http_timeout") or 25)
    headers = {
        "Authorization": f"Bearer {token}",
        "Accept": "application/json",
        "User-Agent": "MicroinvestPartsAssistant/1.0",
    }
    endpoints = [
        f"{base}/rest/api/v3/products",
        f"{base}/gwp/rest/api/v3/products",
        f"{base}/rest/v3/products",
    ]
    for url in endpoints:
        try:
            resp = requests.get(
                url,
                params={"name": query, "query": query, "search": query},
                headers=headers,
                timeout=timeout,
            )
            if resp.status_code >= 400:
                logger.debug("НКТ API %s → %s", url, resp.status_code)
                continue
            data = resp.json()
            ntin = _extract_ntin_from_json(data)
            if ntin:
                logger.info("NTIN из API: %s", ntin)
                return ntin
        except Exception as exc:
            logger.error("НКТ API ошибка (%s): %s", url, exc)
    return None


def _extract_ntin_from_json(data: Any) -> str | None:
    if isinstance(data, dict):
        for key in ("ntin", "NTIN", "ntIn", "ntin_code", "code", "gtin", "GTIN"):
            val = data.get(key)
            if val and re.fullmatch(r"\d{13,14}", str(val).strip()):
                return str(val).strip()
        for v in data.values():
            found = _extract_ntin_from_json(v)
            if found:
                return found
    elif isinstance(data, list):
        for item in data[:20]:
            found = _extract_ntin_from_json(item)
            if found:
                return found
    return None


def _search_ntin_web(query: str, cfg: dict[str, Any]) -> str | None:
    """Публичный поиск упоминаний NTIN через DuckDuckGo по сайтам НКТ."""
    DDGS = None
    try:
        from ddgs import DDGS as DDGS  # type: ignore
    except ImportError:
        try:
            from duckduckgo_search import DDGS as DDGS  # type: ignore
        except ImportError:
            logger.warning("Пакет ddgs/duckduckgo-search не установлен — NTIN web-поиск недоступен")
            return None

    searches = [
        f'site:nct.gov.kz "{query}" NTIN',
        f'site:nationalcatalog.kz "{query}" NTIN',
        f'site:nct.kz "{query}" NTIN',
        f'"{query}" NTIN Национальный каталог товаров',
    ]
    query_tokens = {t.lower() for t in re.findall(r"[A-Za-zА-Яа-я0-9]{3,}", query)}

    try:
        ddgs = DDGS()
        for q in searches:
            try:
                results = list(ddgs.text(q, max_results=8))
            except Exception as exc:
                logger.error("DDG NTIN ошибка (%s): %s", q, exc)
                continue
            for r in results:
                blob = " ".join(
                    str(r.get(k) or "") for k in ("title", "body", "href")
                )
                blob_l = blob.lower()
                # Требуем пересечение с названием товара, чтобы не брать чужой NTIN
                if query_tokens:
                    hit_tokens = sum(1 for t in query_tokens if t in blob_l)
                    if hit_tokens < max(1, min(2, len(query_tokens) // 2)):
                        continue
                on_nct = any(
                    s in blob_l
                    for s in ("nct.gov", "nct.kz", "nationalcatalog", "нкт", "ntin")
                )
                m = NTIN_LABEL_RE.search(blob)
                if m and on_nct:
                    logger.info("NTIN из веб-поиска: %s", m.group(1))
                    return m.group(1)
                if on_nct:
                    m2 = NTIN_RE.search(blob)
                    if m2 and "ntin" in blob_l:
                        logger.info("NTIN-кандидат из веб-поиска: %s", m2.group(1))
                        return m2.group(1)
    except Exception as exc:
        logger.error("NTIN web-поиск недоступен: %s", exc)
    return None
