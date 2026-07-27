"""Быстрый поиск автозапчасти: FAPI (+ опционально веб)."""
from __future__ import annotations

import logging
import re
from dataclasses import dataclass, field
from typing import Any

import requests

logger = logging.getLogger("microinvest_assistant")


@dataclass
class PartCandidate:
    brand: str
    article: str
    title: str
    category: str = ""
    source: str = ""
    model: str = ""
    extra: dict[str, Any] = field(default_factory=dict)

    @property
    def display(self) -> str:
        if self.brand and self.article:
            head = f"{self.brand} {self.article}"
        else:
            head = self.brand or self.article or ""
        if self.model:
            head = f"{head} ({self.model})" if head else self.model
        if self.title:
            return f"{head} — {self.title}".strip(" —")
        return head or "Без названия"

    def key(self) -> str:
        return f"{self.brand}|{self.article}|{self.title}|{self.model}".upper()


def soft_norm_article(value: str) -> str:
    return re.sub(r"[\s\-.]+", "", (value or "")).upper()


def compact_article(value: str) -> str:
    return re.sub(r"[^A-Za-z0-9]", "", (value or "")).upper()


def article_compatible(article: str, query: str) -> bool:
    if not article or not query:
        return False
    q_soft = soft_norm_article(query)
    a_soft = soft_norm_article(article)
    if a_soft == q_soft:
        return True
    q_c = compact_article(query)
    a_c = compact_article(article)
    if q_c and a_c == q_c:
        if "/" in article:
            return a_soft == q_soft
        return True
    if q_c and len(q_c) >= 5 and q_c in a_c:
        if "/" in article and a_soft != q_soft:
            return False
        return True
    return False


def search_fapi(query: str, cfg: dict[str, Any]) -> list[PartCandidate]:
    key = cfg.get("fapi_key") or ""
    base = cfg.get("fapi_base") or "https://fapi.iisis.ru/fapi/v2"
    timeout = float(cfg.get("http_timeout") or 8)
    if not key:
        return []

    try:
        resp = requests.get(
            f"{base}/productList",
            params={"ui": key, "n": query},
            timeout=timeout,
        )
        resp.raise_for_status()
        data = resp.json()
    except Exception as exc:
        logger.error("FAPI ошибка для %s: %s", query, exc)
        return []

    manufacturers = {
        m["i"]: m.get("ds") or m.get("da") or ""
        for m in (data.get("manufacturerList") or {}).get("mf") or []
    }
    out: list[PartCandidate] = []
    skipped = 0
    for p in (data.get("productList") or {}).get("p") or []:
        brand = (manufacturers.get(p.get("mfi"), "") or "").strip()
        article = str(p.get("n") or p.get("ns") or query).strip()
        title = str(p.get("d") or "").strip()
        if not title and not brand:
            continue
        if not article_compatible(article, query):
            skipped += 1
            continue
        out.append(
            PartCandidate(
                brand=brand,
                article=article,
                title=title or "Автозапчасть",
                category=title,
                source="fapi",
            )
        )
    logger.info("FAPI: %s шт. (отброшено %s) по «%s»", len(out), skipped, query)
    return out


def search_parts(variants: list[str], cfg: dict[str, Any]) -> list[PartCandidate]:
    """Сначала прайс Excel, затем Omega API, затем FAPI."""
    from excel_search import search_excel_candidates
    from omega_search import search_omega

    query = variants[0] if variants else ""
    if not query:
        return []

    excel = search_excel_candidates(query, cfg)
    omega = search_omega(query, cfg)
    fapi = search_fapi(query, cfg)

    web: list[PartCandidate] = []
    if not excel and not omega and not fapi and cfg.get("enable_web_enrichment", False):
        web = _quick_web(query, cfg)

    prefer = soft_norm_article(query)

    def rank(c: PartCandidate) -> tuple:
        # excel → omega (ваши штрихкоды) → fapi → web
        src = {"excel": 0, "omega": 1, "fapi": 2, "web": 3, "manual": 0}.get(c.source, 9)
        exact = 0 if soft_norm_article(c.article) == prefer else 1
        return (src, exact, c.brand, c.title)

    merged = excel + omega + fapi + web
    seen: set[str] = set()
    uniq: list[PartCandidate] = []
    for c in merged:
        k = c.key()
        if k in seen:
            continue
        seen.add(k)
        uniq.append(c)
    uniq.sort(key=rank)
    return uniq[: int(cfg.get("max_candidates") or 12)]


def _quick_web(query: str, cfg: dict[str, Any]) -> list[PartCandidate]:
    try:
        from ddgs import DDGS  # type: ignore
    except ImportError:
        return []
    timeout_note = float(cfg.get("http_timeout") or 8)
    out: list[PartCandidate] = []
    try:
        ddgs = DDGS()
        results = list(ddgs.text(f"{query} автозапчасть", max_results=5))
    except Exception as exc:
        logger.error("Web ошибка: %s", exc)
        return []
    for r in results:
        title = (r.get("title") or "").strip()
        if len(title) < 5:
            continue
        clean = re.split(r"\s[\|\-–—]\s", title)[0].strip()[:160]
        out.append(PartCandidate(brand="", article=query, title=clean, source="web"))
    logger.info("Web fallback: %s", len(out))
    return out
