"""Очистка и нормализация поискового запроса из поля «Имя»."""
from __future__ import annotations

import re
from dataclasses import dataclass


NOISE_WORDS = {
    "автозапчасть",
    "автозапчасти",
    "запчасть",
    "запчасти",
    "oem",
    "oe",
    "barcode",
    "штрихкод",
    "штрих-код",
    "manufacturer",
    "производитель",
    "артикул",
    "номер",
    "detail",
    "part",
    "parts",
}


@dataclass
class ParsedQuery:
    raw: str
    cleaned: str
    variants: list[str]


def clean_query(raw: str) -> ParsedQuery:
    text = (raw or "").strip()
    # Убрать типичный мусор сканера / копипаста
    text = text.replace("\u00a0", " ")
    text = re.sub(r"[\t\r\n]+", " ", text)

    # Выделить «ядро» артикула: буквы/цифры, допускаем - / .
    tokens = re.findall(r"[A-Za-zА-Яа-я0-9][A-Za-zА-Яа-я0-9\-/\.]*", text)
    kept: list[str] = []
    for tok in tokens:
        low = tok.lower().replace(".", "")
        if low in NOISE_WORDS:
            continue
        kept.append(tok)

    cleaned = " ".join(kept).strip() or text.strip()
    # Нормализованный артикул без пробелов и спецсимволов (для каталогов)
    compact = re.sub(r"[^A-Za-z0-9]", "", cleaned).upper()

    variants: list[str] = []
    for v in (cleaned, compact, cleaned.replace(" ", ""), cleaned.replace("-", "")):
        v = v.strip()
        if v and v not in variants:
            variants.append(v)

    # Доп. поисковые формулировки (как в ТЗ) — для веб-обогащения
    # Сами по себе не меняют cleaned, только помогают поиску.
    return ParsedQuery(raw=raw.strip(), cleaned=cleaned, variants=variants)


def looks_like_part_number(cleaned: str) -> bool:
    if not cleaned:
        return False
    compact = re.sub(r"[^A-Za-z0-9]", "", cleaned)
    if len(compact) < 3:
        return False
    # Есть цифры — типичный артикул / OEM / внутренний код
    return any(ch.isdigit() for ch in compact)
