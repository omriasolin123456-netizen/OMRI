"""Поиск по локальным Excel-прайсам в папке price.

Важно: артикул / номер каталога в файлах НЕ сопоставляются с полем «Имя»
(исходным штрихкодом). Сопоставление только по найденному наименованию и бренду.

Итоговое имя для Microinvest: «Наименование товара Модель Бренд».
"""
from __future__ import annotations

import logging
import re
from dataclasses import dataclass
from pathlib import Path
from typing import Any

import pandas as pd

logger = logging.getLogger("microinvest_assistant")


@dataclass
class ExcelMatch:
    name: str
    brand: str
    model: str
    source_file: str
    score: float

    @property
    def formatted_name(self) -> str:
        """Порядок: Наименование товара Модель Бренд."""
        parts = [p.strip() for p in (self.name, self.model, self.brand) if p and str(p).strip()]
        return " ".join(parts)


def _norm(s: Any) -> str:
    if s is None or (isinstance(s, float) and pd.isna(s)):
        return ""
    text = str(s).strip().lower()
    text = text.replace("ё", "е")
    text = re.sub(r"\s+", " ", text)
    return text


def _tokens(s: str) -> set[str]:
    return {t for t in re.split(r"[^a-zA-Zа-яА-Я0-9]+", _norm(s)) if len(t) >= 2}


def _find_column(columns: list[str], aliases: list[str]) -> str | None:
    norm_cols = {_norm(c): c for c in columns}
    for alias in aliases:
        a = _norm(alias)
        if a in norm_cols:
            return norm_cols[a]
    # частичное совпадение
    for alias in aliases:
        a = _norm(alias)
        for nc, orig in norm_cols.items():
            if a and (a in nc or nc in a):
                return orig
    return None


def _score_row(
    row_name: str,
    row_brand: str,
    want_title: str,
    want_brand: str,
) -> float:
    """Оценка совпадения по названию и бренду (не по артикулу)."""
    tn = _tokens(want_title)
    rn = _tokens(row_name)
    if not rn or not tn:
        name_score = 0.0
    else:
        inter = len(tn & rn)
        name_score = inter / max(len(tn), 1)
        # бонус за вхождение строки целиком
        if _norm(want_title) and _norm(want_title) in _norm(row_name):
            name_score = max(name_score, 0.9)
        if _norm(row_name) and _norm(row_name) in _norm(want_title):
            name_score = max(name_score, 0.85)

    brand_score = 0.0
    wb = _norm(want_brand)
    rb = _norm(row_brand)
    if wb and rb:
        if wb == rb or wb in rb or rb in wb:
            brand_score = 1.0
        elif _tokens(wb) & _tokens(rb):
            brand_score = 0.7

    # Если бренд известен с обеих сторон и не совпал — сильно штрафуем
    if wb and rb and brand_score == 0:
        return name_score * 0.35

    if wb and brand_score:
        return 0.65 * name_score + 0.35 * brand_score
    return name_score


def load_price_frames(price_dir: Path) -> list[tuple[str, pd.DataFrame, dict[str, str | None]]]:
    files = sorted(price_dir.glob("*.xlsx")) + sorted(price_dir.glob("*.xls"))
    loaded: list[tuple[str, pd.DataFrame, dict[str, str | None]]] = []
    for fp in files:
        if fp.name.startswith("~$"):
            continue
        try:
            # Читаем все листы
            sheets = pd.read_excel(fp, sheet_name=None, dtype=str, engine="openpyxl")
        except Exception as exc:
            logger.error("Не удалось открыть %s: %s", fp, exc)
            continue
        for sheet_name, df in sheets.items():
            if df is None or df.empty:
                continue
            df = df.copy()
            df.columns = [str(c).strip() for c in df.columns]
            loaded.append((f"{fp.name}::{sheet_name}", df, {}))
    return loaded


def search_excel(
    title: str,
    brand: str,
    cfg: dict[str, Any],
) -> ExcelMatch | None:
    """Ищет лучшее совпадение в Excel по наименованию и бренду."""
    price_dir: Path = cfg["price_dir"]
    min_score = float(cfg.get("min_match_score") or 0.55)
    col_name_aliases = cfg.get("col_name") or ["Наименование товара", "Наименование"]
    col_brand_aliases = cfg.get("col_brand") or ["Бренд", "Производитель"]
    col_model_aliases = cfg.get("col_model") or ["МОДЕЛЬ АВТОМОБИЛЯ", "Модель"]

    best: ExcelMatch | None = None
    frames = load_price_frames(price_dir)
    if not frames:
        logger.warning("В папке %s нет Excel-файлов", price_dir)
        return None

    for source, df, _ in frames:
        name_col = _find_column(list(df.columns), col_name_aliases)
        brand_col = _find_column(list(df.columns), col_brand_aliases)
        model_col = _find_column(list(df.columns), col_model_aliases)
        if not name_col:
            logger.warning("В %s не найдена колонка наименования", source)
            continue

        for _, row in df.iterrows():
            row_name = "" if pd.isna(row.get(name_col)) else str(row.get(name_col))
            row_brand = ""
            if brand_col:
                row_brand = "" if pd.isna(row.get(brand_col)) else str(row.get(brand_col))
            row_model = ""
            if model_col:
                row_model = "" if pd.isna(row.get(model_col)) else str(row.get(model_col))

            if not row_name.strip():
                continue

            score = _score_row(row_name, row_brand, title, brand)
            if score < min_score:
                continue
            if best is None or score > best.score:
                best = ExcelMatch(
                    name=row_name.strip(),
                    brand=(row_brand or brand or "").strip(),
                    model=row_model.strip(),
                    source_file=source,
                    score=score,
                )

    if best:
        logger.info(
            "Excel: совпадение score=%.2f файл=%s → %s",
            best.score,
            best.source_file,
            best.formatted_name,
        )
    else:
        logger.info("Excel: совпадений не найдено для «%s» / «%s»", title, brand)
    return best
