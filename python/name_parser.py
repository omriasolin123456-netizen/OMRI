"""Разбор сложных наименований запчастей для поиска NTIN (F7).

Примеры:
  Ремень грм 2108-99-211012,2113-15,ОКА-1111, 1118 Калина 8кл(клапанов) 111зуб (зубов) ТАДЕМ
  Ремень генератора 944 АвтоМагнат
  ступица передняя 2110 SINYEEE
  ремень грм 2110-12, 2170 16кл с ГУР ручейковый(1115мм) ДЕКСТРА DEXTRA

Вытаскивает: тип запчасти, модели/коды авто, характеристики, бренд.
Строит пачку коротких запросов для параллельного поиска в НКТ.
"""
from __future__ import annotations

import re
from dataclasses import dataclass, field


# Слова, которые НЕ бренд (характеристики / общие)
SPEC_WORDS = {
    "кл",
    "клапан",
    "клапанов",
    "клапана",
    "зуб",
    "зубов",
    "зуба",
    "мм",
    "см",
    "с",
    "гур",
    "гур",
    "ручейковый",
    "ручейков",
    "клиновой",
    "зубчатый",
    "усиленный",
    "передняя",
    "задняя",
    "передний",
    "задний",
    "левая",
    "правая",
    "лев",
    "прав",
    "в сборе",
    "голая",
}

CAR_NAME_WORDS = {
    "калина",
    "приора",
    "гранта",
    "ларгус",
    "веста",
    "нива",
    "шеви",
    "ока",
    "классика",
    "самара",
    "десятка",
    "газель",
    "соболь",
    "уаз",
    "патриот",
    "хантер",
    "next",
    "бизнес",
}

PART_MULTI = [
    "ремень грм",
    "ремень генератора",
    "ремень кондиционера",
    "ремень приводной",
    "цепь грм",
    "ступица передняя",
    "ступица задняя",
    "диск тормозной",
    "колодки тормозные",
    "насос водяной",
    "насос топливный",
    "фильтр масляный",
    "фильтр воздушный",
    "фильтр салона",
    "фильтр топливный",
    "амортизатор передний",
    "амортизатор задний",
    "опора амортизатора",
    "шаровая опора",
    "рулевой наконечник",
    "тяга рулевая",
    "подшипник ступицы",
    "помпа водяная",
]

# Нормализация типа
PART_NORMALIZE = {
    "ремень грм": "Ремень ГРМ",
    "ремень генератора": "Ремень генератора",
    "ступица передняя": "Ступица передняя",
    "ступица задняя": "Ступица задняя",
}


@dataclass
class ParsedPartName:
    raw: str
    part: str = ""
    models: list[str] = field(default_factory=list)
    specs: list[str] = field(default_factory=list)
    brands: list[str] = field(default_factory=list)
    extras: list[str] = field(default_factory=list)

    @property
    def primary_brand(self) -> str:
        return self.brands[0] if self.brands else ""


def parse_part_name(raw: str) -> ParsedPartName:
    """Разбирает строку имени Microinvest на части."""
    text = re.sub(r"\s+", " ", (raw or "").strip())
    out = ParsedPartName(raw=text)
    if not text:
        return out

    low = text.lower()

    # --- тип запчасти (начало строки) ---
    part = ""
    rest = text
    for phrase in sorted(PART_MULTI, key=len, reverse=True):
        if low.startswith(phrase):
            part = PART_NORMALIZE.get(phrase, phrase.title())
            rest = text[len(phrase) :].strip(" ,;-")
            break
    if not part:
        # 1–3 первых «словесных» токена до цифры/кода
        tokens = text.split()
        taken: list[str] = []
        for t in tokens:
            if re.search(r"\d", t) and not re.fullmatch(r"[A-Za-zА-Яа-яЁё\-]+", t):
                break
            if _looks_like_brand_token(t) and taken:
                break
            taken.append(t)
            if len(taken) >= 3:
                break
        part = " ".join(taken).strip()
        rest = text[len(part) :].strip(" ,;-") if part else text
        part = _normalize_part_label(part)

    out.part = part

    # Раскрыть скобки: 8кл(клапанов) → 8кл клапанов; (1115мм) → 1115мм
    rest_exp = re.sub(r"\(([^)]+)\)", r" \1 ", rest)
    rest_exp = re.sub(r"\s+", " ", rest_exp).strip()

    # --- модели / коды ВАЗ и имена авто ---
    models: list[str] = []
    # ОКА-1111, 2108-99, 2110-12, 211012, 2170, 1118
    for m in re.finditer(
        r"(?i)\b(?:ока[-\s]?)?(\d{3,5}(?:-\d{1,4})?)\b|\b(ока)\b|\b(калина|приора|гранта|ларгус|веста|нива|газель)\b",
        rest_exp,
    ):
        g = next((x for x in m.groups() if x), "")
        if not g:
            continue
        token = g.strip()
        # полный match для ока-1111
        full = m.group(0).strip()
        if "ока" in full.lower() and re.search(r"\d", full):
            token = re.sub(r"\s+", "", full, flags=re.I)
            token = token.replace("Ока", "ОКА").replace("ока", "ОКА")
        if token.lower() in {"ока"} and "ОКА-" not in " ".join(models).upper():
            # голое «ОКА» без номера — пропустим, если рядом есть ОКА-1111
            continue
        if token not in models:
            models.append(token)

    # Также куски через запятую: 2108-99-211012 → 2108-99 и 211012?
    # «2108-99-211012» как один токен — разбить на 2108-99 и 2110/211012
    expanded: list[str] = []
    for m in models:
        if re.fullmatch(r"\d{4}-\d{2}-\d{4,6}", m):
            a, b, c = m.split("-")
            expanded.extend([f"{a}-{b}", c, a])
        elif re.fullmatch(r"\d{4}-\d{1,4}", m):
            expanded.append(m)
            expanded.append(m.split("-")[0])
        else:
            expanded.append(m)
    # unique preserve order
    seen_m: set[str] = set()
    out.models = []
    for m in expanded:
        key = m.lower()
        if key in seen_m:
            continue
        seen_m.add(key)
        out.models.append(m)

    # --- характеристики ---
    specs: list[str] = []
    spec_patterns = [
        (r"(?i)\b(\d+)\s*кл(?:апан\w*)?\b", lambda m: f"{m.group(1)}кл"),
        (r"(?i)\b(\d+)\s*зуб(?:ов|а|ья)?\b", lambda m: f"{m.group(1)} зуб"),
        (r"(?i)\b(\d+[.,]?\d*)\s*мм\b", lambda m: f"{m.group(1).replace(',', '.')}мм"),
        (r"(?i)\b(\d+[.,]?\d*)\s*см\b", lambda m: f"{m.group(1)}см"),
        (r"(?i)\bс\s*гур\b", lambda m: "с ГУР"),
        (r"(?i)\bбез\s*гур\b", lambda m: "без ГУР"),
        (r"(?i)\bручейков\w*\b", lambda m: "ручейковый"),
        (r"(?i)\bклинов\w*\b", lambda m: "клиновой"),
        (r"(?i)\bзубчат\w*\b", lambda m: "зубчатый"),
        (r"(?i)\bусиленн\w*\b", lambda m: "усиленный"),
        (r"(?i)\b\d\s*ц(?:ил\w*)?\b", lambda m: m.group(0).replace(" ", "")),
        (r"(?i)\b(\d{1,2}[.,]\d{1,2})\b", lambda m: m.group(0).replace(",", ".")),  # объём 1.6
    ]
    for pat, fmt in spec_patterns:
        for m in re.finditer(pat, rest_exp):
            val = fmt(m).strip()
            if val and val not in specs:
                specs.append(val)
    out.specs = specs

    # --- бренды: хвост из латиницы / капса / известного вида ---
    brands = _extract_brands(rest_exp, out.models)
    out.brands = brands

    return out


def _normalize_part_label(part: str) -> str:
    low = part.lower().strip()
    if low in PART_NORMALIZE:
        return PART_NORMALIZE[low]
    if "грм" in low:
        return re.sub(r"(?i)грм", "ГРМ", part).strip()
    return part[:1].upper() + part[1:] if part else part


def _looks_like_brand_token(token: str) -> bool:
    t = token.strip(" ,;.")
    if len(t) < 2:
        return False
    low = t.lower()
    if low in SPEC_WORDS or low in CAR_NAME_WORDS:
        return False
    if re.search(r"\d", t):
        return False
    # Latin brand
    if re.fullmatch(r"[A-Za-z][A-Za-z\-_]{1,24}", t):
        return True
    # Cyrillic ALL CAPS brand-like (ТАДЕМ, ДЕКСТРА) length>=3
    if re.fullmatch(r"[А-ЯЁ]{3,20}", t):
        return True
    # Mixed AutoMagnat / АвтоМагнат
    if re.fullmatch(r"[A-Za-zА-Яа-яЁё]{3,24}", t) and (
        t.isupper() or re.search(r"[A-ZА-ЯЁ].*[a-zа-яё]", t) is None and sum(ch.isupper() for ch in t) >= len(t) * 0.6
    ):
        return True
    return False


def _extract_brands(rest: str, models: list[str]) -> list[str]:
    tokens = rest.split()
    brands: list[str] = []
    # Идём с конца
    i = len(tokens) - 1
    while i >= 0:
        t = tokens[i].strip(" ,;.")
        if not t:
            i -= 1
            continue
        # пропуск моделей/спеков
        if any(t.lower() == m.lower() for m in models):
            break
        if re.search(r"\d", t) and not _looks_like_brand_token(t):
            break
        if t.lower() in SPEC_WORDS or t.lower() in CAR_NAME_WORDS:
            break
        if _looks_like_brand_token(t) or (
            re.fullmatch(r"[A-Za-zА-Яа-яЁё\-]+", t)
            and t.lower() not in SPEC_WORDS
            and not re.search(r"\d", t)
            and i >= len(tokens) - 3
        ):
            brands.insert(0, t)
            i -= 1
            # максимум 2 токена бренда (ДЕКСТРА DEXTRA)
            if len(brands) >= 2:
                break
            continue
        break

    # Уникальные, плюс варианты без утроенных букв (SINYEEE → SINYEE)
    out: list[str] = []
    seen: set[str] = set()
    for b in brands:
        for variant in _brand_variants(b):
            k = variant.lower()
            if k in seen:
                continue
            seen.add(k)
            out.append(variant)
    return out


def _brand_variants(brand: str) -> list[str]:
    b = brand.strip()
    out = [b]
    # схлопнуть повтор 3+ букв: SINYEEE → SINYEE → SINYE
    collapsed = re.sub(r"(.)\1{2,}", r"\1\1", b)
    if collapsed != b:
        out.append(collapsed)
    collapsed2 = re.sub(r"(.)\1+", r"\1", b)
    if collapsed2 != b and len(collapsed2) >= 3:
        out.append(collapsed2)
    if b.upper() != b:
        out.append(b.upper())
    return out


def build_ntin_queries_from_name(raw_name: str, max_queries: int = 10) -> list[tuple[str, int]]:
    """Запросы для F7: (текст, приоритет). Меньше = важнее."""
    p = parse_part_name(raw_name)
    ordered: list[tuple[str, int]] = []
    seen: set[str] = set()

    def add(q: str, prio: int) -> None:
        q = re.sub(r"\s+", " ", (q or "").strip())
        if len(q) < 3:
            return
        key = q.lower()
        if key in seen:
            return
        seen.add(key)
        ordered.append((q, prio))

    part = p.part
    brand0 = p.primary_brand
    specs = p.specs
    models = p.models
    spec_s = " ".join(specs[:3])
    spec_s2 = " ".join(specs[:2])

    def model_key(m: str) -> tuple:
        low = m.lower()
        if low in CAR_NAME_WORDS or any(w in low for w in CAR_NAME_WORDS):
            return (0, len(m))
        if "ока" in low or any(ch.isalpha() for ch in m):
            return (1, len(m))
        return (2, len(m))

    named_models = sorted(models, key=model_key)

    # Самые точные сначала
    if part and spec_s and brand0:
        add(f"{part} {spec_s} {brand0}", 0)
    if part and brand0:
        add(f"{part} {brand0}", 1)
        for b in p.brands[1:3]:
            add(f"{part} {b}", 1)

    # Имена авто / ОКА / ключевые коды + спеки (без бренда = «похожий»)
    for i, m in enumerate(named_models[:4]):
        if part and spec_s2:
            add(f"{part} {m} {spec_s2}", 2 + i)
        if part and brand0:
            add(f"{part} {m} {brand0}", 3 + i)
        if part:
            add(f"{part} {m}", 4 + i)

    if part and spec_s:
        add(f"{part} {spec_s}", 6)
        for s in specs[:3]:
            add(f"{part} {s}", 7)

    if part:
        add(part, 9)

    cleaned = " ".join(x for x in [part, *named_models[:2], *specs[:2], brand0] if x)
    add(cleaned, 5)

    ordered.sort(key=lambda x: x[1])
    return ordered[:max_queries]
