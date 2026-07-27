"""Поиск по локальным Excel-прайсам в папке price.

Поддерживаемые форматы прайсов (по реальным базам):

1) Производитель | Товар | Каталожный № | ...
   → имя = «Товар Производитель» (модель уже внутри «Товар»)

2) Номер | Наименование товара | Мадель | Бренд | ...
   → имя = «Наименование товара Мадель Бренд»
   (опечатка «Мадель» поддержана)

3) «Ценовая группа/ Номенклатура/...»:
   «Название Модель //OEНомер// БРЕНД»
   → имя = «Название Модель БРЕНД»

Каталожный № / Номер / OE НЕ используются для сопоставления
с полем «Имя» Microinvest.
"""
from __future__ import annotations

import logging
import re
from dataclasses import dataclass
from pathlib import Path
from typing import Any

import pandas as pd

logger = logging.getLogger("microinvest_assistant")

OE_SPLIT_RE = re.compile(r"\s*//\s*", re.UNICODE)


@dataclass
class ExcelMatch:
    name: str
    brand: str
    model: str
    source_file: str
    score: float
    format_id: str = ""

    @property
    def formatted_name(self) -> str:
        """Порядок: Наименование товара Модель Бренд."""
        parts = [p.strip() for p in (self.name, self.model, self.brand) if p and str(p).strip()]
        # Убрать дубли, если модель/бренд уже есть в name
        cleaned: list[str] = []
        acc = ""
        for p in parts:
            if acc and _norm(p) in _norm(acc):
                continue
            cleaned.append(p)
            acc = " ".join(cleaned)
        return " ".join(cleaned)


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
    tn = _tokens(want_title)
    rn = _tokens(row_name)
    if not rn or not tn:
        name_score = 0.0
    else:
        inter = len(tn & rn)
        name_score = inter / max(len(tn), 1)
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

    if wb and rb and brand_score == 0:
        return name_score * 0.35

    if wb and brand_score:
        return 0.65 * name_score + 0.35 * brand_score
    return name_score


def parse_slash_nomenclature(raw: str) -> tuple[str, str, str]:
    """Разбор «Название Модель //OE...// БРЕНД» → (name_model, brand, model_hint)."""
    text = str(raw or "").strip()
    if not text:
        return "", "", ""

    parts = [p.strip() for p in OE_SPLIT_RE.split(text) if p.strip()]
    if len(parts) >= 3 and parts[1].upper().startswith("OE"):
        name_model = parts[0]
        brand = parts[-1]
        return name_model, brand, ""
    if len(parts) == 2:
        # «Название //БРЕНД» без OE
        return parts[0], parts[1], ""
    return text, "", ""


def detect_format(columns: list[str]) -> str:
    """Определяет формат прайса по заголовкам."""
    joined = " | ".join(_norm(c) for c in columns)

    has_tovar = _find_column(columns, ["Товар"]) is not None
    has_proizv = _find_column(columns, ["Производитель"]) is not None
    has_name = _find_column(columns, ["Наименование товара", "Наименование"]) is not None
    has_model = _find_column(columns, ["Мадель", "Модель", "МОДЕЛЬ АВТОМОБИЛЯ"]) is not None
    has_brand = _find_column(columns, ["Бренд", "Brand"]) is not None
    has_nomen = _find_column(
        columns,
        [
            "Номенклатура",
            "Ценовая группа",
            "Характеристика номенклатуры",
            "Ценовая группа/ Номенклатура/ Характеристика номенклатуры",
        ],
    ) is not None

    if has_nomen and not has_name:
        return "slash_nomenclature"
    if has_name and (has_model or has_brand):
        return "name_model_brand"
    if has_tovar and has_proizv:
        return "tovar_proizvoditel"
    if "номенклатур" in joined or "ценовая группа" in joined:
        return "slash_nomenclature"
    if has_tovar:
        return "tovar_proizvoditel"
    if has_name:
        return "name_model_brand"
    return "unknown"


def _normalize_header_row(df: pd.DataFrame) -> pd.DataFrame:
    """Если первая строка — заголовок прайса, а колонки Unnamed — ищем строку заголовков."""
    cols = [str(c) for c in df.columns]
    unnamed = sum(1 for c in cols if c.lower().startswith("unnamed") or c.isdigit())
    if unnamed < max(2, len(cols) // 2):
        return df

    # Ищем строку, похожую на заголовки
    keywords = (
        "товар",
        "наименование",
        "бренд",
        "производитель",
        "модель",
        "мадель",
        "номенклатур",
        "номер",
        "цена",
    )
    for i in range(min(8, len(df))):
        row_vals = [str(v).strip() for v in df.iloc[i].tolist() if not pd.isna(v)]
        score = sum(1 for v in row_vals if any(k in v.lower() for k in keywords))
        if score >= 2:
            new_df = df.iloc[i + 1 :].copy()
            new_df.columns = [
                str(v).strip() if not pd.isna(v) else f"col_{idx}"
                for idx, v in enumerate(df.iloc[i].tolist())
            ]
            new_df.reset_index(drop=True, inplace=True)
            return new_df
    return df


def load_price_frames(price_dir: Path) -> list[tuple[str, pd.DataFrame, str]]:
    files = sorted(price_dir.glob("*.xlsx")) + sorted(price_dir.glob("*.xls"))
    loaded: list[tuple[str, pd.DataFrame, str]] = []
    for fp in files:
        if fp.name.startswith("~$"):
            continue
        try:
            if fp.suffix.lower() == ".xls":
                sheets = pd.read_excel(fp, sheet_name=None, dtype=str, engine="xlrd")
            else:
                sheets = pd.read_excel(fp, sheet_name=None, dtype=str, engine="openpyxl")
        except Exception as exc:
            # Fallback без указания engine
            try:
                sheets = pd.read_excel(fp, sheet_name=None, dtype=str)
            except Exception as exc2:
                logger.error("Не удалось открыть %s: %s / %s", fp, exc, exc2)
                continue
        for sheet_name, df in sheets.items():
            if df is None or df.empty:
                continue
            df = df.copy()
            df.columns = [str(c).strip() for c in df.columns]
            df = _normalize_header_row(df)
            df.columns = [str(c).strip() for c in df.columns]
            fmt = detect_format(list(df.columns))
            logger.info("Excel загружен: %s::%s формат=%s строк=%s", fp.name, sheet_name, fmt, len(df))
            loaded.append((f"{fp.name}::{sheet_name}", df, fmt))
    return loaded


def _row_from_format(
    row: pd.Series,
    columns: list[str],
    fmt: str,
    cfg: dict[str, Any],
) -> tuple[str, str, str] | None:
    """Возвращает (name, model, brand) для строки."""
    col_name = _find_column(
        columns,
        cfg.get("col_name")
        or [
            "Наименование товара",
            "Наименование",
            "Товар",
            "Номенклатура",
            "Ценовая группа",
        ],
    )
    col_brand = _find_column(
        columns,
        cfg.get("col_brand") or ["Бренд", "Производитель", "Фирма", "Brand"],
    )
    col_model = _find_column(
        columns,
        cfg.get("col_model")
        or ["Мадель", "Модель", "МОДЕЛЬ АВТОМОБИЛЯ", "Модель автомобиля", "Model"],
    )

    if fmt == "slash_nomenclature":
        # Берём самую «длинную» текстовую колонку / номенклатуру
        raw_col = col_name or _find_column(
            columns,
            [
                "Ценовая группа/ Номенклатура/ Характеристика номенклатуры",
                "Номенклатура",
                "Характеристика номенклатуры",
            ],
        )
        if not raw_col:
            # первая колонка
            raw_col = columns[0] if columns else None
        if not raw_col:
            return None
        raw = "" if pd.isna(row.get(raw_col)) else str(row.get(raw_col))
        name_model, brand, _ = parse_slash_nomenclature(raw)
        if not name_model:
            return None
        return name_model.strip(), "", brand.strip()

    if fmt == "tovar_proizvoditel":
        t_col = _find_column(columns, ["Товар"]) or col_name
        b_col = _find_column(columns, ["Производитель"]) or col_brand
        if not t_col:
            return None
        name = "" if pd.isna(row.get(t_col)) else str(row.get(t_col)).strip()
        brand = ""
        if b_col:
            brand = "" if pd.isna(row.get(b_col)) else str(row.get(b_col)).strip()
        if not name:
            return None
        # Модель уже внутри «Товар» — отдельное поле не дублируем
        return name, "", brand

    # name_model_brand и unknown
    if not col_name:
        return None
    name = "" if pd.isna(row.get(col_name)) else str(row.get(col_name)).strip()
    if not name:
        return None
    # Если в имени уже //OE// — разобрать
    if "//" in name:
        name_model, brand2, _ = parse_slash_nomenclature(name)
        brand = brand2
        model = ""
        if col_brand and not brand:
            brand = "" if pd.isna(row.get(col_brand)) else str(row.get(col_brand)).strip()
        return name_model, model, brand

    brand = ""
    if col_brand:
        brand = "" if pd.isna(row.get(col_brand)) else str(row.get(col_brand)).strip()
    model = ""
    if col_model:
        model = "" if pd.isna(row.get(col_model)) else str(row.get(col_model)).strip()
    return name, model, brand


def search_excel_candidates(query: str, cfg: dict[str, Any]) -> list:
    """Кандидаты из прайсов, если запрос встречается в названии/модели/бренде (не в каталоге)."""
    from web_search import PartCandidate

    q = (query or "").strip()
    if len(q) < 3:
        return []

    price_dir: Path = cfg["price_dir"]
    frames = load_price_frames(price_dir)
    out: list = []
    seen: set[str] = set()
    qn = _norm(q)
    qc = re.sub(r"[^a-zA-Zа-яА-Я0-9]", "", qn)

    for source, df, fmt in frames:
        columns = list(df.columns)
        for _, row in df.iterrows():
            parsed = _row_from_format(row, columns, fmt, cfg)
            if not parsed:
                continue
            row_name, row_model, row_brand = parsed
            blob = " ".join(p for p in (row_name, row_model, row_brand) if p)
            bn = _norm(blob)
            bc = re.sub(r"[^a-zA-Zа-яА-Я0-9]", "", bn)
            if qn not in bn and qc not in bc:
                continue
            key = f"{row_name}|{row_brand}|{row_model}".upper()
            if key in seen:
                continue
            seen.add(key)
            out.append(
                PartCandidate(
                    brand=row_brand,
                    article=q,
                    title=row_name,
                    model=row_model,
                    source="excel",
                    extra={"file": source},
                )
            )
            if len(out) >= 10:
                return out
    logger.info("Excel-кандидаты по запросу «%s»: %s", q, len(out))
    return out


def search_excel(
    title: str,
    brand: str,
    cfg: dict[str, Any],
    query: str | None = None,
) -> ExcelMatch | None:
    """Ищет лучшее совпадение в Excel по наименованию и бренду."""
    price_dir: Path = cfg["price_dir"]
    min_score = float(cfg.get("min_match_score") or 0.55)

    best: ExcelMatch | None = None
    frames = load_price_frames(price_dir)
    if not frames:
        logger.warning("В папке %s нет Excel-файлов", price_dir)
        return None

    for source, df, fmt in frames:
        columns = list(df.columns)
        for _, row in df.iterrows():
            parsed = _row_from_format(row, columns, fmt, cfg)
            if not parsed:
                continue
            row_name, row_model, row_brand = parsed
            if not row_name.strip():
                continue

            # Для скоринга склеиваем name+model (модель может быть отдельно)
            score_name = " ".join(p for p in (row_name, row_model) if p)
            score = _score_row(score_name, row_brand, title, brand)

            # Не поднимаем score только из-за совпадения исходного штрихкода в тексте —
            # это подменяло выбранный товар чужой строкой Excel.

            if score < min_score:
                continue
            if best is None or score > best.score:
                best = ExcelMatch(
                    name=row_name.strip(),
                    brand=(row_brand or brand or "").strip(),
                    model=row_model.strip(),
                    source_file=source,
                    score=score,
                    format_id=fmt,
                )

    if best:
        logger.info(
            "Excel: совпадение score=%.2f fmt=%s файл=%s → %s",
            best.score,
            best.format_id,
            best.source_file,
            best.formatted_name,
        )
    else:
        logger.info("Excel: совпадений не найдено для «%s» / «%s»", title, brand)
    return best
