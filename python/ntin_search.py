"""Поиск NTIN через API Национального каталога товаров Казахстана.

Основной endpoint (без токена):
  POST https://nationalcatalog.kz/gw/search/api/v1/search

Тело запроса (как на сайте):
  {
    "query": "…",
    "withAttributesFilter": false,
    "withCategoriesFilter": true,
    "attributes": {},
    "baseNtin": null,
    "page": 0,
    "size": 20,
    "sort": "relevance"
  }

Ответ: items[].ntin + items[].nameRu
"""
from __future__ import annotations

import logging
import re
from dataclasses import dataclass
from typing import Any

import requests

logger = logging.getLogger("microinvest_assistant")

NTIN_RE = re.compile(r"\b(02\d{11,12}|\d{13,14})\b")
NTIN_LABEL_RE = re.compile(
    r"(?:NTIN|НТИН|ntin_code)\s*[:=]?\s*(\d{13,14})",
    re.I,
)

DEFAULT_GW_SEARCH = "https://nationalcatalog.kz/gw/search/api/v1/search"


@dataclass
class NtinCandidate:
    ntin: str
    name: str
    source: str = ""

    @property
    def display(self) -> str:
        return f"{self.ntin} — {self.name}" if self.name else self.ntin


def short_product_name(title: str) -> str:
    """Короткое название для поиска NTIN: без артикулов и длинных хвостов."""
    text = (title or "").strip()
    if not text:
        return ""
    text = re.split(r"\s*/\s*|\(|\[", text, maxsplit=1)[0].strip()
    words: list[str] = []
    for w in text.split():
        if re.search(r"\d{4,}", w):
            break
        if re.fullmatch(r"[\d\.,\-]+мм", w, re.I):
            break
        words.append(w)
        if len(words) >= 4:
            break
    return " ".join(words).strip() or text.split()[0]


def build_ntin_query(name: str, model: str, brand: str) -> str:
    """Формат как при ручном поиске: Название Модель Бренд."""
    short = short_product_name(name)
    parts = [p.strip() for p in (short, model, brand) if p and str(p).strip()]
    out: list[str] = []
    for p in parts:
        if out and p.lower() in " ".join(out).lower():
            continue
        out.append(p)
    return " ".join(out).strip()


def search_ntin_candidates(
    product_name: str,
    model: str,
    brand: str,
    cfg: dict[str, Any],
) -> list[NtinCandidate]:
    if not cfg.get("ntin_enabled", True):
        return []

    query = build_ntin_query(product_name, model, brand)
    if not query:
        return []

    logger.info("NTIN-запрос: «%s»", query)
    timeout = float(cfg.get("ntin_timeout") or cfg.get("http_timeout") or 8)
    found: list[NtinCandidate] = []
    seen: set[str] = set()

    # 1) Публичный gateway search (основной, без токена)
    for c in _search_ntin_gateway(query, cfg, timeout):
        if c.ntin not in seen:
            seen.add(c.ntin)
            found.append(c)

    # 2) Официальный API с Bearer-токеном (если задан)
    token = (cfg.get("ntin_api_token") or "").strip()
    if token and not found:
        for c in _search_ntin_api_list(query, token, cfg, timeout):
            if c.ntin not in seen:
                seen.add(c.ntin)
                found.append(c)

    # 3) Веб-fallback (медленный, по умолчанию выкл. если gateway работает)
    if not found and cfg.get("ntin_web_search", False):
        for c in _search_ntin_web_list(query, timeout):
            if c.ntin not in seen:
                seen.add(c.ntin)
                found.append(c)

    found = _rank_ntin(found, query, brand)
    logger.info("NTIN кандидатов: %s", len(found))
    return found[:12]


def search_ntin(product_name: str, brand: str, cfg: dict[str, Any], model: str = "") -> str | None:
    """Совместимость: вернуть один NTIN или None."""
    cands = search_ntin_candidates(product_name, model, brand, cfg)
    return cands[0].ntin if cands else None


def _rank_ntin(items: list[NtinCandidate], query: str, brand: str) -> list[NtinCandidate]:
    q_tokens = {t.lower() for t in re.findall(r"[A-Za-zА-Яа-я0-9]{2,}", query)}
    b = (brand or "").strip().lower()

    def key(c: NtinCandidate) -> tuple:
        name_l = (c.name or "").lower()
        hits = sum(1 for t in q_tokens if t in name_l)
        brand_hit = 0 if (b and b in name_l) else 1
        return (-hits, brand_hit, c.name)

    return sorted(items, key=key)


def _search_ntin_gateway(
    query: str,
    cfg: dict[str, Any],
    timeout: float,
) -> list[NtinCandidate]:
    """POST /gw/search/api/v1/search — как на nationalcatalog.kz."""
    url = (cfg.get("ntin_gw_search") or DEFAULT_GW_SEARCH).strip()
    size = int(cfg.get("ntin_page_size") or 20)
    payload = {
        "query": query,
        "withAttributesFilter": False,
        "withCategoriesFilter": True,
        "attributes": {},
        "baseNtin": None,
        "page": 0,
        "size": size,
        "sort": "relevance",
    }
    headers = {
        "Content-Type": "application/json",
        "Accept": "application/json",
        "User-Agent": "MicroinvestPartsAssistant/1.1",
    }
    try:
        resp = requests.post(url, json=payload, headers=headers, timeout=timeout)
        resp.raise_for_status()
        data = resp.json()
    except Exception as exc:
        logger.error("НКТ gateway ошибка: %s", exc)
        return []

    out: list[NtinCandidate] = []
    seen: set[str] = set()
    for item in data.get("items") or []:
        if not isinstance(item, dict):
            continue
        ntin = str(item.get("ntin") or "").strip()
        if not ntin or not re.fullmatch(r"\d{13,14}", ntin):
            continue
        if ntin in seen:
            continue
        seen.add(ntin)
        name = (
            str(item.get("nameRu") or item.get("shortNameRu") or item.get("nameKk") or "")
            .strip()
        )
        # Артикул из attributes — в хвост имени для выбора
        article = _attr_value(item, "article")
        brand_attr = _attr_value(item, "brand")
        extra = " ".join(p for p in (brand_attr, article) if p)
        if extra and name and extra.lower() not in name.lower():
            name = f"{name} ({extra})"
        out.append(NtinCandidate(ntin=ntin, name=name or ntin, source="nct-gw"))

    total = (data.get("pageInfo") or {}).get("totalSize")
    logger.info("НКТ gateway: %s шт. (всего %s) по «%s»", len(out), total, query)
    return out


def _attr_value(item: dict[str, Any], code: str) -> str:
    for a in item.get("attributes") or []:
        if not isinstance(a, dict):
            continue
        if str(a.get("code") or "").lower() != code.lower():
            continue
        for key in ("valueRu", "value", "valueEn", "valueKk"):
            val = a.get(key)
            if val is not None and str(val).strip():
                return str(val).strip()
    return ""


def _search_ntin_api_list(
    query: str,
    token: str,
    cfg: dict[str, Any],
    timeout: float,
) -> list[NtinCandidate]:
    base = (cfg.get("ntin_api_base") or "https://nct.kz").rstrip("/")
    headers = {
        "Authorization": f"Bearer {token}",
        "Accept": "application/json",
        "User-Agent": "MicroinvestPartsAssistant/1.1",
    }
    endpoints = [
        (f"{base}/rest/api/v3/products", {"name": query}),
        (f"{base}/gwp/rest/api/v3/products", {"name": query}),
    ]
    out: list[NtinCandidate] = []
    for url, params in endpoints:
        try:
            resp = requests.get(url, params=params, headers=headers, timeout=timeout)
            if resp.status_code >= 400:
                logger.debug("НКТ API %s → %s", url, resp.status_code)
                continue
            out.extend(_candidates_from_json(resp.json(), source="nct-api"))
            if out:
                break
        except Exception as exc:
            logger.error("НКТ API ошибка (%s): %s", url, exc)
    return out


def _candidates_from_json(data: Any, source: str) -> list[NtinCandidate]:
    out: list[NtinCandidate] = []

    def walk(node: Any):
        if isinstance(node, dict):
            ntin = None
            for key in ("ntin_code", "ntin", "NTIN", "ntIn", "gtin", "GTIN"):
                val = node.get(key)
                if val and re.fullmatch(r"\d{13,14}", str(val).strip()):
                    ntin = str(val).strip()
                    break
            name = str(
                node.get("name") or node.get("nameRu") or node.get("product_name") or node.get("title") or ""
            ).strip()
            if ntin:
                out.append(NtinCandidate(ntin=ntin, name=name or ntin, source=source))
            for v in node.values():
                walk(v)
        elif isinstance(node, list):
            for item in node[:50]:
                walk(item)

    walk(data)
    uniq: list[NtinCandidate] = []
    seen: set[str] = set()
    for c in out:
        if c.ntin in seen:
            continue
        seen.add(c.ntin)
        uniq.append(c)
    return uniq


def _search_ntin_web_list(query: str, timeout: float) -> list[NtinCandidate]:
    """Запасной DDG-поиск (медленный)."""
    try:
        from ddgs import DDGS  # type: ignore
    except ImportError:
        try:
            from duckduckgo_search import DDGS  # type: ignore
        except ImportError:
            logger.warning("ddgs не установлен — NTIN web недоступен")
            return []

    q = f'site:nct.gov.kz OR site:nationalcatalog.kz "{query}"'
    out: list[NtinCandidate] = []
    seen: set[str] = set()
    try:
        ddgs = DDGS()
        results = list(ddgs.text(q, max_results=10))
    except Exception as exc:
        logger.error("DDG NTIN ошибка: %s", exc)
        return []

    q_tokens = {t.lower() for t in re.findall(r"[A-Za-zА-Яа-я0-9]{3,}", query)}
    for r in results:
        title = str(r.get("title") or "").strip()
        body = str(r.get("body") or "").strip()
        href = str(r.get("href") or "").strip()
        blob = f"{title} {body} {href}"
        blob_l = blob.lower()
        if not any(s in blob_l for s in ("nct.gov", "nct.kz", "nationalcatalog", "ntin", "нкт")):
            continue
        if q_tokens:
            hits = sum(1 for t in q_tokens if t in blob_l)
            if hits < max(1, min(2, (len(q_tokens) + 1) // 2)):
                continue
        m = NTIN_LABEL_RE.search(blob) or NTIN_RE.search(blob)
        if not m:
            continue
        ntin = m.group(1)
        if ntin in seen:
            continue
        seen.add(ntin)
        card = re.split(r"\s[\|\-–—]\s", title)[0].strip() or query
        out.append(NtinCandidate(ntin=ntin, name=card[:160], source="nct-web"))
    return out
