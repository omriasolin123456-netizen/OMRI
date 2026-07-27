"""Быстрый поиск автозапчасти: сначала Excel, при отсутствии — интернет (FAPI)."""
from __future__ import annotations

import logging
import re
from dataclasses import dataclass, field
from typing import Any, Callable

import requests

logger = logging.getLogger("microinvest_assistant")

ProgressFn = Callable[[int, str], None]


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


def search_fapi(
    query: str,
    cfg: dict[str, Any],
    progress: ProgressFn | None = None,
) -> list[PartCandidate]:
    key = cfg.get("fapi_key") or ""
    base = cfg.get("fapi_base") or "https://fapi.iisis.ru/fapi/v2"
    # Короткий таймаут — «супербыстрый» интернет
    timeout = float(cfg.get("http_timeout") or 4)
    if not key:
        return []

    if progress:
        progress(78, "Интернет: запрос каталога…")

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
        if progress:
            progress(85, "Интернет: ошибка / таймаут")
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
    if progress:
        progress(88, f"Интернет: найдено {len(out)}")
    return out


def search_parts(
    variants: list[str],
    cfg: dict[str, Any],
    progress: ProgressFn | None = None,
) -> list[PartCandidate]:
    """1) Только Excel. 2) Если пусто — быстрый интернет (FAPI).

    Omega и медленный DDG по умолчанию не вызываются.
    """
    from excel_search import search_excel_candidates

    query = variants[0] if variants else ""
    if not query:
        return []

    limit = int(cfg.get("max_candidates") or 12)

    if progress:
        progress(5, "Старт: сначала ваши Excel…")

    excel = search_excel_candidates(query, cfg, progress=progress)
    if excel:
        if progress:
            progress(90, f"Готово из Excel: {len(excel)} вариант(ов)")
        return excel[:limit]

    # Нет подходящего в Excel → интернет
    if progress:
        progress(74, "В Excel нет — быстрый поиск в интернете…")

    fapi = search_fapi(query, cfg, progress=progress)

    web: list[PartCandidate] = []
    if not fapi and cfg.get("enable_web_enrichment", False):
        if progress:
            progress(86, "Доп. веб-поиск…")
        web = _quick_web(query, cfg)

    prefer = soft_norm_article(query)

    def rank(c: PartCandidate) -> tuple:
        src = {"excel": 0, "fapi": 1, "web": 2, "manual": 0, "omega": 3}.get(c.source, 9)
        exact = 0 if soft_norm_article(c.article) == prefer else 1
        return (src, exact, c.brand, c.title)

    merged = fapi + web
    seen: set[str] = set()
    uniq: list[PartCandidate] = []
    for c in merged:
        k = c.key()
        if k in seen:
            continue
        seen.add(k)
        uniq.append(c)
    uniq.sort(key=rank)

    if progress:
        progress(90, f"Интернет готов: {len(uniq)} вариант(ов)")
    return uniq[:limit]


def _quick_web(query: str, cfg: dict[str, Any]) -> list[PartCandidate]:
    try:
        from ddgs import DDGS  # type: ignore
    except ImportError:
        return []
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
