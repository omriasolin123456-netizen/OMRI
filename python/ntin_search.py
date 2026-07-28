"""Поиск NTIN через API Национального каталога товаров Казахстана.

Основной endpoint (без токена):
  POST https://nationalcatalog.kz/gw/search/api/v1/search

Стратегия (несколько коротких запросов ПАРАЛЛЕЛЬНО — быстро):
  1) штрихкод из поля «Имя» (цифры/код)
  2) Наименование + фирма (бренд)
  3) Наименование + машина по частям: AUDI → AUDI 100 → AUDI 100 A6
  4) полный: Наименование + Модель + Бренд
"""
from __future__ import annotations

import logging
import re
from concurrent.futures import ThreadPoolExecutor, as_completed
from dataclasses import dataclass
from typing import Any

import requests

logger = logging.getLogger("microinvest_assistant")

NTIN_RE = re.compile(r"\b(02\d{11,12}|\d{13,14})\b")
NTIN_LABEL_RE = re.compile(
    r"(?:NTIN|НТИН|ntin_code)\s*[:=]?\s*(\d{13,14})",
    re.I,
)
YEAR_TOKEN_RE = re.compile(r"^\d{2}(-\d{2,4})?$|^\d{4}$")
DEFAULT_GW_SEARCH = "https://nationalcatalog.kz/gw/search/api/v1/search"

# Известные марки авто — чтобы резать «Audi 100 A6 VW Passat…» после первой машины
CAR_MAKES = {
    "audi",
    "vw",
    "volkswagen",
    "bmw",
    "mercedes",
    "mercedes-benz",
    "mb",
    "toyota",
    "honda",
    "nissan",
    "ford",
    "opel",
    "skoda",
    "seat",
    "porsche",
    "volvo",
    "mazda",
    "hyundai",
    "kia",
    "chevrolet",
    "daewoo",
    "renault",
    "peugeot",
    "citroen",
    "citroën",
    "fiat",
    "jeep",
    "lexus",
    "subaru",
    "mitsubishi",
    "suzuki",
    "gaz",
    "uaz",
    "lada",
    "vaz",
    "mers",
}


@dataclass
class NtinCandidate:
    ntin: str
    name: str
    source: str = ""
    query: str = ""
    priority: int = 99
    is_auto: bool = False

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


def expand_car_model(model: str) -> list[str]:
    """«Audi 100 A6 94-98 VW Passat…» → ['AUDI', 'AUDI 100', 'AUDI 100 A6'].

    «Audi/VW» → ['AUDI', 'VW'].
    """
    text = (model or "").strip()
    if not text:
        return []

    # Короткое «Audi/VW» / «Chevrolet/Daewoo»
    if "/" in text and len(text.split()) <= 2:
        parts = [p.strip().upper() for p in text.split("/") if p.strip()]
        return parts[:3]

    raw_tokens = text.replace("/", " ").split()
    tokens: list[str] = []
    for t in raw_tokens:
        t = t.strip()
        if not t:
            continue
        if YEAR_TOKEN_RE.match(t):
            break
        low = t.lower().strip(",;")
        # Вторая марка после уже набранной первой машины — стоп
        if tokens and low in CAR_MAKES and tokens[0].lower() in CAR_MAKES:
            break
        if tokens and low in CAR_MAKES and len(tokens) >= 2:
            break
        tokens.append(t)
        if len(tokens) >= 4:
            break

    out: list[str] = []
    for i in range(1, len(tokens) + 1):
        out.append(" ".join(tokens[:i]).upper())
    return out


def build_ntin_query(name: str, model: str, brand: str) -> str:
    """Полный запрос: Название Модель Бренд."""
    short = short_product_name(name)
    parts = [p.strip() for p in (short, model, brand) if p and str(p).strip()]
    out: list[str] = []
    for p in parts:
        if out and p.lower() in " ".join(out).lower():
            continue
        out.append(p)
    return " ".join(out).strip()


def build_ntin_query_variants(
    barcode: str,
    name: str,
    model: str,
    brand: str,
) -> list[tuple[str, int]]:
    """Список (запрос, приоритет). Меньше priority — важнее.

    0  штрихкод из «Имя»
    1  наименование + фирма
    2+ наименование + AUDI / AUDI 100 / AUDI 100 A6
    9  полный запрос
    """
    short = short_product_name(name)
    brand_s = (brand or "").strip()
    model_s = (model or "").strip()
    code = (barcode or "").strip()

    ordered: list[tuple[str, int]] = []
    seen: set[str] = set()

    def add(q: str, prio: int) -> None:
        q = re.sub(r"\s+", " ", (q or "").strip())
        if len(q) < 2:
            return
        key = q.lower()
        if key in seen:
            return
        seen.add(key)
        ordered.append((q, prio))

    # 1) исходный штрихкод / код из поля «Имя»
    if code:
        add(code, 0)
        compact = re.sub(r"[^A-Za-zА-Яа-я0-9]", "", code)
        if compact and compact.lower() != code.lower():
            add(compact, 0)

    # 2) наименование + фирма (бренд запчасти)
    if short and brand_s:
        add(f"{short} {brand_s}", 1)

    # 3) наименование + машина по нарастающей
    for i, car in enumerate(expand_car_model(model_s)):
        if short:
            add(f"{short} {car}", 2 + i)
        else:
            add(car, 2 + i)

    # 4) только наименование (если совсем короткое — всё же полезно)
    if short:
        add(short, 8)

    # 5) полный
    full = build_ntin_query(name, model_s, brand_s)
    if full:
        add(full, 9)

    return ordered


def search_ntin_candidates(
    product_name: str,
    model: str,
    brand: str,
    cfg: dict[str, Any],
    barcode: str = "",
) -> list[NtinCandidate]:
    if not cfg.get("ntin_enabled", True):
        return []

    variants = build_ntin_query_variants(barcode, product_name, model, brand)
    if not variants:
        return []

    # Скорость: короткий таймаут, меньше page size, все запросы параллельно
    timeout = float(cfg.get("ntin_timeout") or 4)
    max_queries = int(cfg.get("ntin_max_queries") or 8)
    variants = variants[:max_queries]

    logger.info(
        "NTIN-запросы (%s шт., parallel): %s",
        len(variants),
        [q for q, _ in variants],
    )

    found: list[NtinCandidate] = []
    seen: set[str] = set()

    def merge(batch: list[NtinCandidate]) -> None:
        for c in batch:
            if c.ntin in seen:
                for existing in found:
                    if existing.ntin == c.ntin:
                        if c.priority < existing.priority:
                            existing.priority = c.priority
                            existing.query = c.query
                        break
                continue
            seen.add(c.ntin)
            found.append(c)

    workers = min(6, len(variants))
    with ThreadPoolExecutor(max_workers=workers) as pool:
        futs = {
            pool.submit(_search_ntin_gateway, q, cfg, timeout, prio): (q, prio)
            for q, prio in variants
        }
        for fut in as_completed(futs):
            q, prio = futs[fut]
            try:
                batch = fut.result()
            except Exception as exc:
                logger.error("NTIN parallel «%s»: %s", q, exc)
                continue
            for c in batch:
                c.query = q
                c.priority = prio
            merge(batch)

    # Если gateway пуст и есть токен — один запасной запрос (полный)
    token = (cfg.get("ntin_api_token") or "").strip()
    if not found and token:
        full = build_ntin_query(product_name, model, brand) or barcode
        if full:
            for c in _search_ntin_api_list(full, token, cfg, timeout):
                c.query = full
                c.priority = 10
                merge([c])

    if not found and cfg.get("ntin_web_search", False):
        full = build_ntin_query(product_name, model, brand) or barcode
        if full:
            for c in _search_ntin_web_list(full, timeout):
                c.query = full
                c.priority = 20
                merge([c])

    found = _filter_ntin_noise(found, product_name, brand, barcode)
    found = _rank_ntin(found, product_name, brand, barcode)
    logger.info("NTIN кандидатов: %s", len(found))
    return found[:12]


def search_ntin(
    product_name: str,
    brand: str,
    cfg: dict[str, Any],
    model: str = "",
    barcode: str = "",
) -> str | None:
    cands = search_ntin_candidates(product_name, model, brand, cfg, barcode=barcode)
    return cands[0].ntin if cands else None


def _filter_ntin_noise(
    items: list[NtinCandidate],
    product_name: str,
    brand: str,
    barcode: str,
) -> list[NtinCandidate]:
    """Убирает случайные совпадения по цифрам штрихкода (колбаса, вазы…)."""
    tokens = {
        t.lower()
        for t in re.findall(r"[A-Za-zА-Яа-я]{3,}", f"{product_name} {brand}")
    }
    if not items:
        return items
    if not tokens:
        return items

    strong: list[NtinCandidate] = []
    weak: list[NtinCandidate] = []
    for c in items:
        name_l = (c.name or "").lower()
        hit = sum(1 for t in tokens if t in name_l)
        if c.is_auto or hit >= 1 or c.priority >= 1:
            strong.append(c)
        else:
            # priority 0 = только штрихкод, без пересечения с названием — шум
            weak.append(c)

    return strong if strong else weak


def _rank_ntin(
    items: list[NtinCandidate],
    product_name: str,
    brand: str,
    barcode: str,
) -> list[NtinCandidate]:
    q_tokens = {
        t.lower()
        for t in re.findall(r"[A-Za-zА-Яа-я0-9]{2,}", f"{product_name} {brand}")
    }
    b = (brand or "").strip().lower()
    code = re.sub(r"[^A-Za-zА-Яа-я0-9]", "", (barcode or "")).lower()

    def key(c: NtinCandidate) -> tuple:
        name_l = (c.name or "").lower()
        hits = sum(1 for t in q_tokens if t in name_l)
        brand_hit = 0 if (b and b in name_l) else 1
        code_hit = 0 if (code and code in re.sub(r"[^a-z0-9а-я]", "", name_l)) else 1
        auto_hit = 0 if c.is_auto else 1
        return (auto_hit, c.priority, code_hit, -hits, brand_hit, c.name)

    return sorted(items, key=key)


def _search_ntin_gateway(
    query: str,
    cfg: dict[str, Any],
    timeout: float,
    priority: int = 99,
) -> list[NtinCandidate]:
    """POST /gw/search/api/v1/search — как на nationalcatalog.kz."""
    url = (cfg.get("ntin_gw_search") or DEFAULT_GW_SEARCH).strip()
    size = int(cfg.get("ntin_page_size") or 10)
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
        "User-Agent": "MicroinvestPartsAssistant/1.2",
    }
    try:
        resp = requests.post(url, json=payload, headers=headers, timeout=timeout)
        resp.raise_for_status()
        data = resp.json()
    except Exception as exc:
        logger.error("НКТ gateway «%s»: %s", query, exc)
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
        name = str(
            item.get("nameRu") or item.get("shortNameRu") or item.get("nameKk") or ""
        ).strip()
        article = _attr_value(item, "article")
        brand_attr = _attr_value(item, "brand")
        extra = " ".join(p for p in (brand_attr, article) if p)
        if extra and name and extra.lower() not in name.lower():
            name = f"{name} ({extra})"
        cat_blob = " ".join(
            str(item.get(k) or "")
            for k in (
                "categoryNameRuL1",
                "categoryNameRuL2",
                "categoryNameRuL3",
                "categoryNameRuL4",
            )
        ).lower()
        is_auto = any(
            w in cat_blob
            for w in ("транспорт", "автомоб", "запчаст", "двигател", "радиатор")
        )
        out.append(
            NtinCandidate(
                ntin=ntin,
                name=name or ntin,
                source="nct-gw",
                query=query,
                priority=priority,
                is_auto=is_auto,
            )
        )

    total = (data.get("pageInfo") or {}).get("totalSize")
    logger.info("НКТ «%s»: %s шт. (всего %s)", query, len(out), total)
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
        "User-Agent": "MicroinvestPartsAssistant/1.2",
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
                node.get("name")
                or node.get("nameRu")
                or node.get("product_name")
                or node.get("title")
                or ""
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
    try:
        from ddgs import DDGS  # type: ignore
    except ImportError:
        try:
            from duckduckgo_search import DDGS  # type: ignore
        except ImportError:
            return []

    q = f'site:nct.gov.kz OR site:nationalcatalog.kz "{query}"'
    out: list[NtinCandidate] = []
    seen: set[str] = set()
    try:
        results = list(DDGS().text(q, max_results=8))
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
