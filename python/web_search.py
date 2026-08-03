"""Быстрый поиск автозапчасти: сначала Excel, при отсутствии — интернет (FAPI + бренды)."""
from __future__ import annotations

import logging
import re
from concurrent.futures import ThreadPoolExecutor, as_completed
from dataclasses import dataclass, field
from typing import Any, Callable

import requests

logger = logging.getLogger("microinvest_assistant")

ProgressFn = Callable[[int, str], None]

DEFAULT_BRANDS = [
    "WXQP",
    "ESEE",
    "OSSCA",
    "CNAB",
    "LEON",
    "LADA",
    "Superzing",
    "Bear",
    "BOSCH",
    "CHECKSTAR",
]
DEFAULT_KEYWORDS = ["автозапчасть", "запчасть", "авто"]


@dataclass
class PartCandidate:
    brand: str
    article: str
    title: str
    category: str = ""
    source: str = ""
    model: str = ""
    catalog_number: str = ""
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
            base = f"{head} — {self.title}".strip(" —")
        else:
            base = head or "Без названия"
        if self.catalog_number:
            return f"{base}  [кат. {self.catalog_number}]"
        return base

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


def _preferred_brands(cfg: dict[str, Any]) -> list[str]:
    brands = cfg.get("preferred_brands") or DEFAULT_BRANDS
    return [b.strip() for b in brands if str(b).strip()]


def _web_keywords(cfg: dict[str, Any]) -> list[str]:
    kws = cfg.get("web_keywords") or DEFAULT_KEYWORDS
    return [k.strip() for k in kws if str(k).strip()]


def search_fapi(
    query: str,
    cfg: dict[str, Any],
    progress: ProgressFn | None = None,
) -> list[PartCandidate]:
    key = cfg.get("fapi_key") or ""
    base = cfg.get("fapi_base") or "https://fapi.iisis.ru/fapi/v2"
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
                catalog_number=article,
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
    """1) Excel. 2) Если пусто — интернет: FAPI + приоритетные бренды + ключевые слова."""
    from excel_search import search_excel_candidates

    query = variants[0] if variants else ""
    if not query:
        return []

    limit = int(cfg.get("max_candidates") or 12)
    brands = _preferred_brands(cfg)
    keywords = _web_keywords(cfg)

    if progress:
        progress(5, "Старт: сначала ваши Excel…")

    excel = search_excel_candidates(query, cfg, progress=progress)
    if excel:
        if progress:
            progress(90, f"Готово из Excel: {len(excel)} вариант(ов)")
        return excel[:limit]

    if progress:
        progress(74, "В Excel нет — быстрый поиск в интернете…")

    # Параллельно: базовый код + несколько бренд-запросов (ограничено для скорости)
    brand_queries = [f"{query} {b}" for b in brands[:6]]
    kw_queries = [f"{query} {k}" for k in keywords[:2]]

    fapi_all: list[PartCandidate] = []
    with ThreadPoolExecutor(max_workers=6) as pool:
        futs = {pool.submit(search_fapi, query, cfg, None): query}
        for bq in brand_queries[:4]:
            futs[pool.submit(search_fapi, bq, cfg, None)] = bq
        done = 0
        total = len(futs)
        for fut in as_completed(futs):
            done += 1
            try:
                fapi_all.extend(fut.result())
            except Exception as exc:
                logger.error("FAPI parallel: %s", exc)
            if progress:
                progress(78 + int(10 * done / max(total, 1)), f"Интернет {done}/{total}…")

    web: list[PartCandidate] = []
    if cfg.get("enable_web_enrichment", True):
        if progress:
            progress(86, "Доп. веб-поиск по брендам/словам…")
        web = _quick_web_branded(query, brands, keywords, cfg)

    prefer = soft_norm_article(query)
    brand_rank = {b.lower(): i for i, b in enumerate(brands)}

    def rank(c: PartCandidate) -> tuple:
        src = {"excel": 0, "fapi": 1, "web": 2, "manual": 0, "omega": 3}.get(c.source, 9)
        exact = 0 if soft_norm_article(c.article) == prefer else 1
        bl = (c.brand or "").lower()
        # приоритетные фирмы сверху
        br = 0
        for name, idx in brand_rank.items():
            if name and name in bl:
                br = idx
                break
        else:
            br = 50
        # в title тоже ищем бренд
        title_l = (c.title or "").lower()
        for name, idx in brand_rank.items():
            if name and name in title_l:
                br = min(br, idx)
        return (src, br, exact, c.brand, c.title)

    merged = fapi_all + web
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


def _quick_web_branded(
    query: str,
    brands: list[str],
    keywords: list[str],
    cfg: dict[str, Any],
) -> list[PartCandidate]:
    try:
        from ddgs import DDGS  # type: ignore
    except ImportError:
        try:
            from duckduckgo_search import DDGS  # type: ignore
        except ImportError:
            return []

    # Короткий набор запросов для скорости
    searches = [f"{query} {kw}" for kw in (keywords or DEFAULT_KEYWORDS)[:2]]
    for b in (brands or DEFAULT_BRANDS)[:4]:
        searches.append(f"{query} {b}")

    out: list[PartCandidate] = []
    seen_titles: set[str] = set()
    try:
        ddgs = DDGS()
        for q in searches:
            try:
                results = list(ddgs.text(q, max_results=3))
            except Exception:
                continue
            for r in results:
                title = (r.get("title") or "").strip()
                if len(title) < 5:
                    continue
                clean = re.split(r"\s[\|\-–—]\s", title)[0].strip()[:160]
                key = clean.lower()
                if key in seen_titles:
                    continue
                seen_titles.add(key)
                # угадать бренд из списка
                brand = ""
                cl = clean.lower()
                for b in brands:
                    if b.lower() in cl:
                        brand = b
                        break
                out.append(
                    PartCandidate(
                        brand=brand,
                        article=query,
                        title=clean,
                        source="web",
                    )
                )
    except Exception as exc:
        logger.error("Web ошибка: %s", exc)
        return out

    logger.info("Web branded: %s по «%s»", len(out), query)
    return out


def _quick_web(query: str, cfg: dict[str, Any]) -> list[PartCandidate]:
    """Совместимость: один запрос с первым ключевым словом."""
    kws = _web_keywords(cfg)
    kw = kws[0] if kws else "автозапчасть"
    return _quick_web_branded(query, _preferred_brands(cfg), [kw], cfg)
