"""Сборка итогового наименования для поля «Имя» Microinvest."""
from __future__ import annotations

import re


def build_final_name(name: str, model: str, brand: str) -> str:
    """Порядок: Название + характеристики + Модель авто + Бренд.

    Характеристики (4ц, объём, размер, градусы и т.п.) сохраняются.
    Модель и бренд не дублируются, если уже есть в названии.
    """
    text = re.sub(r"\s+", " ", (name or "").strip())
    model_s = re.sub(r"\s+", " ", (model or "").strip())
    brand_s = re.sub(r"\s+", " ", (brand or "").strip())

    if not text and not model_s and not brand_s:
        return ""

    if brand_s:
        if text.lower().endswith(brand_s.lower()):
            text = text[: -len(brand_s)].rstrip(" -–,;")
        text = re.sub(
            rf"(?i)(?:\s+|\b){re.escape(brand_s)}\s*$",
            "",
            text,
        ).strip()

    if model_s:
        text = strip_model_phrase(text, model_s)

    parts: list[str] = []
    for p in (text, model_s, brand_s):
        p = (p or "").strip()
        if not p:
            continue
        joined = " ".join(parts).lower()
        if parts and p.lower() in joined:
            continue
        parts.append(p)
    return " ".join(parts).strip()


def strip_model_phrase(name: str, model: str) -> str:
    """Удаляет фразу модели авто из наименования (варианты слэшей/пробелов)."""
    text = name
    raw = (model or "").strip()
    if not raw:
        return text

    variants = {
        raw,
        raw.replace("/", "/ "),
        raw.replace("/ ", "/"),
        raw.replace("/", " / "),
        raw.replace("/", " "),
        re.sub(r"\s*/\s*", "/", raw),
        re.sub(r"\s*/\s*", " / ", raw),
    }
    for v in variants:
        v = v.strip()
        if len(v) < 2:
            continue
        pat = re.escape(v)
        pat = pat.replace(r"\/", r"\s*/\s*")
        pat = pat.replace(r"\ ", r"\s+")
        text = re.sub(pat, " ", text, flags=re.IGNORECASE)

    return re.sub(r"\s+", " ", text).strip(" -–,;")
