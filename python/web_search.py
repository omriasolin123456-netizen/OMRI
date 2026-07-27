"""Поиск автозапчасти в интернете (FAPI + опциональное веб-обогащение)."""
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
        parts = [p for p in (self.brand, self.article, "—", self.title) if p]
        # brand article — title
        if self.brand and self.article:
            head = f"{self.brand} {self.article}"
        else:
            head = self.brand or self.article or ""
        if self.title:
            return f"{head} — {self.title}".strip(" —")
        return head or "Без названия"

    def key(self) -> str:
        return f"{self.brand}|{self.article}|{self.title}".upper()


def _session(timeout: float) -> requests.Session:
    s = requests.Session()
    s.headers.update(
        {
            "User-Agent": (
                "Mozilla/5.0 (Windows NT 10.0; Win64; x64) "
                "AppleWebKit/537.36 (KHTML, like Gecko) "
                "Chrome/126.0.0.0 Safari/537.36"
            ),
            "Accept": "application/json,text/html,*/*",
        }
    )
    s.request_timeout = timeout  # type: ignore[attr-defined]
    return s


def search_fapi(query: str, cfg: dict[str, Any]) -> list[PartCandidate]:
    """Поиск по артикулу через FAPI productList (без кэша — каждый вызов новый)."""
    key = cfg.get("fapi_key") or ""
    base = cfg.get("fapi_base") or "https://fapi.iisis.ru/fapi/v2"
    timeout = float(cfg.get("http_timeout") or 25)
    if not key:
        logger.warning("FAPI ключ не задан — пропускаем FAPI")
        return []

    url = f"{base}/productList"
    params = {"ui": key, "n": query}
    try:
        resp = requests.get(url, params=params, timeout=timeout)
        resp.raise_for_status()
        data = resp.json()
    except Exception as exc:
        logger.error("FAPI productList ошибка для %s: %s", query, exc)
        return []

    manufacturers = {
        m["i"]: m.get("ds") or m.get("da") or ""
        for m in (data.get("manufacturerList") or {}).get("mf") or []
    }
    products = (data.get("productList") or {}).get("p") or []
    out: list[PartCandidate] = []
    for p in products:
        brand = manufacturers.get(p.get("mfi"), "") or ""
        article = str(p.get("n") or p.get("ns") or query).strip()
        title = str(p.get("d") or "").strip()
        if not title and not brand:
            continue
        out.append(
            PartCandidate(
                brand=brand.strip(),
                article=article,
                title=title or "Автозапчасть",
                category=title,
                source="fapi",
                extra={"dbi": p.get("dbi"), "sr": p.get("sr")},
            )
        )
    logger.info("FAPI: найдено %s вариантов по «%s»", len(out), query)
    return out


def search_web_enrichment(query: str, cfg: dict[str, Any]) -> list[PartCandidate]:
    """Дополнительный поиск через DuckDuckGo: название / производитель / категория."""
    if not cfg.get("enable_web_enrichment"):
        return []

    DDGS = None
    try:
        from ddgs import DDGS as DDGS  # type: ignore
    except ImportError:
        try:
            from duckduckgo_search import DDGS as DDGS  # type: ignore
        except ImportError:
            logger.warning("Пакет ddgs/duckduckgo-search не установлен — веб-обогащение отключено")
            return []

    queries = [
        f"{query} автозапчасть",
        f"{query} OEM",
        f"{query} запчасть производитель",
    ]
    found: list[PartCandidate] = []
    seen: set[str] = set()

    try:
        ddgs = DDGS()
        for q in queries:
            try:
                results = list(ddgs.text(q, max_results=5))
            except Exception as exc:
                logger.error("DDG ошибка (%s): %s", q, exc)
                continue
            for r in results:
                title = (r.get("title") or "").strip()
                body = (r.get("body") or r.get("href") or "").strip()
                cand = _parse_web_result(query, title, body)
                if cand and cand.key() not in seen:
                    seen.add(cand.key())
                    found.append(cand)
    except Exception as exc:
        logger.error("Веб-обогащение недоступно: %s", exc)

    logger.info("Web: найдено %s кандидатов по «%s»", len(found), query)
    return found


_BRAND_HINTS = re.compile(
    r"\b(SKF|FAG|KOYO|INA|SNR|NTN|NSK|TIMKEN|BOSCH|GATES|CONTITECH|"
    r"DAYCO|FEBI|SWAG|TRW|SACHS|LEMFOERDER|LEMFÖRDER|MANN|MAHLE|"
    r"VAG|OEM|WXQP|MONROE|MOOG|DENSO|NGK|VALEO|LUK|RUVILLE|"
    r"PILENGA|CTR|GMB|SNR|OPTIMAL|MEYLE|HENGST)\b",
    re.I,
)


def _parse_web_result(query: str, title: str, body: str) -> PartCandidate | None:
    text = f"{title} {body}"
    if not re.search(re.escape(query), text, re.I) and not re.search(
        re.sub(r"[^A-Za-z0-9]", "", query), re.sub(r"[^A-Za-z0-9]", "", text), re.I
    ):
        # Слабая связь с запросом — пропускаем
        if query.lower() not in text.lower():
            return None

    brand = ""
    m = _BRAND_HINTS.search(text)
    if m:
        brand = m.group(1).upper()
        if brand == "LEMFÖRDER":
            brand = "LEMFOERDER"

    # Убрать сайт из заголовка
    clean_title = re.split(r"\s[\|\-–—]\s", title)[0].strip()
    clean_title = re.sub(r"\s+", " ", clean_title)
    if len(clean_title) < 4:
        return None

    return PartCandidate(
        brand=brand,
        article=query,
        title=clean_title[:180],
        category="",
        source="web",
    )


def merge_candidates(
    primary: list[PartCandidate],
    secondary: list[PartCandidate],
    max_candidates: int,
    prefer_query: str = "",
) -> list[PartCandidate]:
    """Объединяет кандидатов без автовыбора. Точные совпадения артикула из FAPI приоритетнее."""
    merged: list[PartCandidate] = []
    seen: set[str] = set()
    for group in (primary, secondary):
        for c in group:
            k = c.key()
            if k in seen:
                continue
            # Для веб-результатов без бренда — не дублировать одинаковые title
            soft = f"{c.brand}|{c.title}".upper()
            if soft in seen:
                continue
            seen.add(k)
            seen.add(soft)
            merged.append(c)

    prefer = (prefer_query or "").strip().upper()
    prefer_compact = re.sub(r"[^A-Za-z0-9]", "", prefer)

    def rank(c: PartCandidate) -> tuple:
        art_raw = (c.article or "").strip().upper()
        art = re.sub(r"[^A-Za-z0-9]", "", art_raw)
        if prefer and art_raw == prefer:
            exact = 0
        elif prefer_compact and art == prefer_compact:
            exact = 1
        else:
            exact = 2
        src = 0 if c.source == "fapi" else 1
        return (exact, src, c.brand.upper(), art_raw)

    merged.sort(key=rank)
    return merged[: int(max_candidates)]


def search_parts(variants: list[str], cfg: dict[str, Any]) -> list[PartCandidate]:
    """Полный интернет-поиск по вариантам запроса. Без кэша."""
    all_primary: list[PartCandidate] = []
    seen: set[str] = set()
    for v in variants:
        for c in search_fapi(v, cfg):
            if c.key() not in seen:
                seen.add(c.key())
                all_primary.append(c)

    web: list[PartCandidate] = []
    # Веб-обогащение только если каталог кроссов ничего не дал
    if not all_primary and variants and cfg.get("enable_web_enrichment"):
        web = search_web_enrichment(variants[0], cfg)

    return merge_candidates(
        all_primary,
        web,
        int(cfg.get("max_candidates") or 12),
        prefer_query=variants[0] if variants else "",
    )
