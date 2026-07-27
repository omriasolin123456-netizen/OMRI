"""Поиск автозапчасти в интернете (FAPI + веб) и отбор кандидатов."""
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
    """Нормализация артикула: убираем только пробелы/точки/дефисы, слэш сохраняем."""
    return re.sub(r"[\s\-.]+", "", (value or "")).upper()


def compact_article(value: str) -> str:
    return re.sub(r"[^A-Za-z0-9]", "", (value or "")).upper()


def article_compatible(article: str, query: str) -> bool:
    """
    Строгая совместимость артикула с запросом.
    361337 ~= 361-337 / 361.337
    но НЕ 36/1337 (другая группировка через слэш).
    """
    if not article or not query:
        return False
    q_soft = soft_norm_article(query)
    a_soft = soft_norm_article(article)
    if a_soft == q_soft:
        return True
    # Буквенно-цифровой префикс+номер: VKBA361337 содержит 361337
    q_c = compact_article(query)
    a_c = compact_article(article)
    if q_c and a_c == q_c:
        # Если в артикуле есть '/', требуем точного soft-совпадения
        if "/" in article:
            return a_soft == q_soft
        return True
    if q_c and q_c in a_c and len(q_c) >= 5:
        # Артикул вида VKBA361337
        if a_c.endswith(q_c) or q_c in a_c:
            if "/" in article and a_soft != q_soft:
                return False
            return True
    return False


def search_fapi(query: str, cfg: dict[str, Any]) -> list[PartCandidate]:
    """Поиск по артикулу через FAPI productList (без кэша)."""
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
    skipped = 0
    for p in products:
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
                extra={"dbi": p.get("dbi"), "sr": p.get("sr")},
            )
        )
    logger.info(
        "FAPI: принято %s / отброшено %s по «%s»",
        len(out),
        skipped,
        query,
    )
    return out


_BRAND_HINTS = re.compile(
    r"\b(SKF|FAG|KOYO|INA|SNR|NTN|NSK|TIMKEN|BOSCH|GATES|CONTITECH|"
    r"DAYCO|FEBI|SWAG|TRW|SACHS|LEMFOERDER|LEMFÖRDER|MANN|MAHLE|"
    r"VAG|WXQP|MONROE|MOOG|DENSO|NGK|VALEO|LUK|RUVILLE|"
    r"PILENGA|CTR|GMB|OPTIMAL|MEYLE|HENGST|BEAR|LEON|OSSCA|"
    r"ENGLIAN|ENGLIIAN|FENOX|TRUCKMAN|BAUTLER|SENBOOM|REMFCOM|РЕМКОМ|"
    r"TRIALLI|ТРИАЛЛИ|СТАРТВОЛЬТ|AVTOGRAD|OAT|КЕДР|АСТРО)\b",
    re.I,
)

_PART_WORDS = re.compile(
    r"подшипник|амортизатор|ступиц|фильтр|шаров|рычаг|сайлент|гранат|"
    r"генератор|стартер|помп|радиатор|сцеплен|диск|колодк|ремень|"
    r"наконечник|тяги|втулк|опора|bearing|shock|filter|joint|arm",
    re.I,
)


def _get_ddgs():
    try:
        from ddgs import DDGS  # type: ignore

        return DDGS
    except ImportError:
        try:
            from duckduckgo_search import DDGS  # type: ignore

            return DDGS
        except ImportError:
            return None


def search_web_enrichment(query: str, cfg: dict[str, Any]) -> list[PartCandidate]:
    """Веб-поиск автозапчасти (всегда, не только как fallback)."""
    if not cfg.get("enable_web_enrichment", True):
        return []

    DDGS = _get_ddgs()
    if DDGS is None:
        logger.warning("Пакет ddgs не установлен — веб-поиск отключён")
        return []

    queries = [
        f"{query} автозапчасть",
        f"{query} OEM запчасть",
        f"{query} подшипник OR амортизатор OR ступица",
        f"VKBA {query}",
        f"{query} SKF OR FAG OR KOYO OR INA",
        f"site:exist.ru {query}",
        f"site:autodoc.ru {query}",
        f"site:emex.ru {query}",
    ]
    found: list[PartCandidate] = []
    seen: set[str] = set()

    try:
        ddgs = DDGS()
        for q in queries:
            try:
                results = list(ddgs.text(q, max_results=6))
            except Exception as exc:
                logger.error("DDG ошибка (%s): %s", q, exc)
                continue
            for r in results:
                title = (r.get("title") or "").strip()
                body = (r.get("body") or "").strip()
                href = (r.get("href") or "").strip()
                cand = _parse_web_result(query, title, body, href)
                if cand and cand.key() not in seen:
                    seen.add(cand.key())
                    found.append(cand)
    except Exception as exc:
        logger.error("Веб-поиск недоступен: %s", exc)

    logger.info("Web: найдено %s кандидатов по «%s»", len(found), query)
    return found


def _parse_web_result(
    query: str,
    title: str,
    body: str,
    href: str = "",
) -> PartCandidate | None:
    text = f"{title} {body} {href}"
    q_c = compact_article(query)
    t_c = compact_article(text)
    if q_c and q_c not in t_c and query.lower() not in text.lower():
        return None

    # Отсечь химию / CAS / сельхоз / мусорные витрины
    if re.search(
        r"CAS|MFCD|carbamate|bromoaniline|PubChem|drug|John\s*Deere|"
        r"HYSTER|DRAG LINK|крючок|провода форсунок|не дал результата|"
        r"каталог запчастей для иномарок|купить на Дроме",
        text,
        re.I,
    ):
        return None

    brand = ""
    m = _BRAND_HINTS.search(text)
    if m:
        brand = m.group(1).upper().replace("LEMFÖRDER", "LEMFOERDER")
        if brand == "ENGLIIAN":
            brand = "ENGLIAN"

    article = query
    art_m = re.search(
        rf"\b([A-Z]{{2,6}}[\s\-]?{re.escape(query)})\b",
        text,
        re.I,
    )
    if art_m:
        article = re.sub(r"\s+", "", art_m.group(1)).upper()

    clean_title = re.split(r"\s[\|\-–—]\s", title)[0].strip()
    clean_title = re.sub(r"\s+", " ", clean_title)
    if len(clean_title) < 4:
        return None

    # Нужен признак автозапчасти или известный бренд/сайт
    has_part_word = bool(_PART_WORDS.search(text))
    has_parts_site = bool(
        re.search(r"exist\.|autodoc\.|emex\.|autopiter\.|partreview", href, re.I)
    )
    auto_score = 0
    if has_part_word:
        auto_score += 2
    if brand:
        auto_score += 1
    if has_parts_site:
        auto_score += 1
    if auto_score < 2:
        return None

    # Заголовок не должен быть голым номером
    if compact_article(clean_title) == q_c:
        return None

    return PartCandidate(
        brand=brand,
        article=article,
        title=clean_title[:180],
        category="",
        source="web",
        extra={"href": href, "auto_score": auto_score},
    )


def merge_candidates(
    groups: list[list[PartCandidate]],
    max_candidates: int,
    prefer_query: str = "",
) -> list[PartCandidate]:
    merged: list[PartCandidate] = []
    seen: set[str] = set()
    for group in groups:
        for c in group:
            k = c.key()
            if k in seen:
                continue
            soft = f"{c.brand}|{compact_article(c.article)}|{c.title[:40]}".upper()
            if soft in seen:
                continue
            seen.add(k)
            seen.add(soft)
            merged.append(c)

    prefer = soft_norm_article(prefer_query)
    prefer_c = compact_article(prefer_query)

    def rank(c: PartCandidate) -> tuple:
        art_soft = soft_norm_article(c.article)
        art_c = compact_article(c.article)
        if prefer and art_soft == prefer:
            exact = 0
        elif prefer_c and art_c == prefer_c:
            exact = 1
        elif prefer_c and prefer_c in art_c:
            exact = 2
        else:
            exact = 3
        src_order = {"excel": 0, "web": 1, "fapi": 2}.get(c.source, 9)
        auto = -int((c.extra or {}).get("auto_score") or 0)
        return (exact, src_order, auto, c.brand.upper(), art_soft)

    merged.sort(key=rank)
    return merged[: int(max_candidates)]


def search_parts(variants: list[str], cfg: dict[str, Any]) -> list[PartCandidate]:
    """Полный поиск: Excel-кандидаты + веб + FAPI. Без кэша."""
    from excel_search import search_excel_candidates

    query = variants[0] if variants else ""
    excel_cands = search_excel_candidates(query, cfg)

    fapi: list[PartCandidate] = []
    seen: set[str] = set()
    for v in variants:
        for c in search_fapi(v, cfg):
            if c.key() not in seen:
                seen.add(c.key())
                fapi.append(c)

    web: list[PartCandidate] = []
    if query and cfg.get("enable_web_enrichment", True):
        web = search_web_enrichment(query, cfg)

    # Если веб нашёл бренд+префикс (VKBA361337) — дозапрос в FAPI
    extra_fapi: list[PartCandidate] = []
    for w in web[:5]:
        art = compact_article(w.article)
        if art and art != compact_article(query) and len(art) <= 24:
            for c in search_fapi(art, cfg):
                if c.key() not in seen:
                    seen.add(c.key())
                    extra_fapi.append(c)

    return merge_candidates(
        [excel_cands, web, fapi, extra_fapi],
        int(cfg.get("max_candidates") or 15),
        prefer_query=query,
    )
