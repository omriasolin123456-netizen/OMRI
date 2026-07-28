"""Поиск по локальным Excel-прайсам в папке price.

Поддерживаемые форматы прайсов:

1) Производитель | Товар | Каталожный № | ...
2) Номер | Наименование товара | Мадель | Бренд | ...
3) Номенклатура: «Название //OE…// БРЕНД [код]»
4) Поставщик (скрин): Артикул | Наименование | Модель | Торговая марка | Оригинальный номер
5) Код производителя | Номер OE | Наименование | Марка

Поиск кода из поля «Имя» — по ВСЕЙ строке Excel (все колонки).
Кэш: logs/excel_index_cache.pkl
"""
from __future__ import annotations

import logging
import pickle
import re
import time
from concurrent.futures import ThreadPoolExecutor, as_completed
from dataclasses import dataclass
from pathlib import Path
from typing import Any, Callable

import pandas as pd

logger = logging.getLogger("microinvest_assistant")

ProgressFn = Callable[[int, str], None]

OE_SPLIT_RE = re.compile(r"\s*//\s*", re.UNICODE)
INDEX_VERSION = 4
TRAILING_CODE_RE = re.compile(
    r"^(?P<brand>.+?)\s+(?P<code>[A-Za-zА-Яа-я0-9][A-Za-zА-Яа-я0-9\-/]{3,})$",
    re.UNICODE,
)
# Хвостовой бренд в длинном наименовании: «… WXQP», «… Superzing»
TRAILING_BRAND_RE = re.compile(
    r"^(?P<head>.+?)\s+(?P<brand>[A-Za-z][A-Za-z0-9\-_/]{1,24})$",
)


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


def split_brand_and_code(tail: str) -> tuple[str, str]:
    """«WXQP 320029» → («WXQP», «320029»); «ENGLIIAN» → («ENGLIIAN», «»)."""
    text = (tail or "").strip()
    if not text:
        return "", ""
    m = TRAILING_CODE_RE.match(text)
    if not m:
        return text, ""
    brand = m.group("brand").strip()
    code = m.group("code").strip()
    # Код должен быть «похож на штрихкод/артикул»: много цифр или длинный артикул
    compact = re.sub(r"[^A-Za-zА-Яа-я0-9]", "", code)
    digit_n = sum(ch.isdigit() for ch in compact)
    if digit_n >= 4 or (len(compact) >= 5 and digit_n >= 3):
        return brand, code
    return text, ""


def parse_slash_nomenclature(raw: str) -> tuple[str, str, str, str]:
    """Разбор номенклатуры → (name_model, brand, model_hint, trailing_code).

    Примеры:
      «Радиатор … //OE191121253К// WXQP 320029»
        → name, WXQP, '', 320029
      «Амортизатор … //OE4A9513031B//ENGLIIAN»
        → name, ENGLIIAN, '', ''
    """
    text = str(raw or "").strip()
    if not text:
        return "", "", "", ""

    parts = [p.strip() for p in OE_SPLIT_RE.split(text) if p.strip()]
    if len(parts) >= 3 and parts[1].upper().startswith("OE"):
        name_model = parts[0]
        brand, code = split_brand_and_code(parts[-1])
        return name_model, brand, "", code
    if len(parts) == 2:
        brand, code = split_brand_and_code(parts[1])
        return parts[0], brand, "", code
    # Без // — возможно «Название БРЕНД 320029» в конце
    brand, code = split_brand_and_code(text)
    if code:
        # brand здесь на самом деле «всё до кода» — оставим как name, бренд пустой
        return brand, "", "", code
    return text, "", "", ""


def split_trailing_brand(name: str) -> tuple[str, str]:
    """«Амортизатор зад Audi A4 … WXQP» → (head, WXQP)."""
    text = (name or "").strip()
    if not text:
        return "", ""
    m = TRAILING_BRAND_RE.match(text)
    if not m:
        return text, ""
    brand = m.group("brand").strip()
    head = m.group("head").strip()
    # Не откусывать обычные русские слова
    if re.search(r"[А-Яа-яЁё]", brand):
        return text, ""
    if len(brand) < 2:
        return text, ""
    return head, brand


def detect_format(columns: list[str]) -> str:
    """Определяет формат прайса по заголовкам."""
    joined = " | ".join(_norm(c) for c in columns)

    has_artikul = _find_column(columns, ["Артикул"]) is not None
    has_trade_mark = _find_column(columns, ["Торговая марка"]) is not None
    has_orig = _find_column(columns, ["Оригинальный номер"]) is not None
    has_kod_proizv = _find_column(columns, ["Код производителя"]) is not None
    has_nomer_oe = _find_column(columns, ["Номер OE", "Номер ОЕ", "OE номер"]) is not None
    has_tovar = _find_column(columns, ["Товар"]) is not None
    has_proizv = _find_column(columns, ["Производитель"]) is not None
    has_name = _find_column(columns, ["Наименование товара", "Наименование"]) is not None
    has_model = _find_column(columns, ["Мадель", "Модель", "МОДЕЛЬ АВТОМОБИЛЯ"]) is not None
    has_brand = _find_column(columns, ["Бренд", "Brand", "Торговая марка"]) is not None
    has_nomen = _find_column(
        columns,
        [
            "Номенклатура",
            "Ценовая группа",
            "Характеристика номенклатуры",
            "Ценовая группа/ Номенклатура/ Характеристика номенклатуры",
        ],
    ) is not None

    # Формат со скриншота 1: Артикул + Торговая марка (+ Модель / Оригинальный номер)
    if has_artikul and (has_trade_mark or has_orig) and has_name:
        return "supplier_artikul"
    # Формат со скриншота 2: Код производителя + Номер OE + Наименование
    if has_kod_proizv and has_name:
        return "kod_proizvoditelya"
    if has_nomer_oe and has_kod_proizv:
        return "kod_proizvoditelya"
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
        "артикул",
        "торговая марка",
        "оригинальный",
        "код производителя",
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


def _list_price_files(price_dir: Path) -> list[Path]:
    files = sorted(price_dir.glob("*.xlsx")) + sorted(price_dir.glob("*.xls"))
    return [fp for fp in files if not fp.name.startswith("~$")]


def _file_signature(price_dir: Path) -> list[tuple[str, float, int]]:
    """Подпись файлов прайса: имя, mtime, size — для инвалидации кэша."""
    sig: list[tuple[str, float, int]] = []
    for fp in _list_price_files(price_dir):
        try:
            st = fp.stat()
            sig.append((fp.name, st.st_mtime, st.st_size))
        except OSError:
            continue
    return sig


def _load_one_file(fp: Path) -> list[tuple[str, pd.DataFrame, str]]:
    try:
        if fp.suffix.lower() == ".xls":
            sheets = pd.read_excel(fp, sheet_name=None, dtype=str, engine="xlrd")
        else:
            sheets = pd.read_excel(fp, sheet_name=None, dtype=str, engine="openpyxl")
    except Exception as exc:
        try:
            sheets = pd.read_excel(fp, sheet_name=None, dtype=str)
        except Exception as exc2:
            logger.error("Не удалось открыть %s: %s / %s", fp, exc, exc2)
            return []
    loaded: list[tuple[str, pd.DataFrame, str]] = []
    for sheet_name, df in sheets.items():
        if df is None or df.empty:
            continue
        df = df.copy()
        df.columns = [str(c).strip() for c in df.columns]
        df = _normalize_header_row(df)
        df.columns = [str(c).strip() for c in df.columns]
        fmt = detect_format(list(df.columns))
        logger.info(
            "Excel загружен: %s::%s формат=%s строк=%s",
            fp.name,
            sheet_name,
            fmt,
            len(df),
        )
        loaded.append((f"{fp.name}::{sheet_name}", df, fmt))
    return loaded


def load_price_frames(
    price_dir: Path,
    progress: ProgressFn | None = None,
    pct_from: int = 10,
    pct_to: int = 40,
) -> list[tuple[str, pd.DataFrame, str]]:
    files = _list_price_files(price_dir)
    if not files:
        return []

    loaded: list[tuple[str, pd.DataFrame, str]] = []
    total = len(files)
    # Параллельная загрузка файлов — быстрее на нескольких прайсах
    workers = min(4, total)
    done = 0
    with ThreadPoolExecutor(max_workers=workers) as pool:
        futures = {pool.submit(_load_one_file, fp): fp for fp in files}
        for fut in as_completed(futures):
            loaded.extend(fut.result())
            done += 1
            if progress:
                pct = pct_from + int((pct_to - pct_from) * done / max(total, 1))
                progress(pct, f"Чтение Excel: {done}/{total}…")
    return loaded



def _cell_str(value: Any) -> str:
    if value is None or (isinstance(value, float) and pd.isna(value)):
        return ""
    text = str(value).strip()
    if not text or text.lower() == "nan":
        return ""
    return text


def _row_all_text(row: pd.Series, columns: list[str]) -> str:
    """Все непустые ячейки строки — штрихкод находится в любой колонке."""
    parts: list[str] = []
    for c in columns:
        s = _cell_str(row.get(c))
        if s:
            parts.append(s)
    return " ".join(parts)


@dataclass
class IndexRow:
    name: str
    model: str
    brand: str
    source: str
    blob_norm: str
    blob_compact: str


def _cache_path(cfg: dict[str, Any]) -> Path:
    return Path(cfg["log_dir"]) / "excel_index_cache.pkl"


def _build_index_rows(
    frames: list[tuple[str, pd.DataFrame, str]],
    cfg: dict[str, Any],
    progress: ProgressFn | None = None,
    pct_from: int = 40,
    pct_to: int = 55,
) -> list[IndexRow]:
    """Индекс: ищем штрихкод/код по ВСЕМ ячейкам строки прайса."""
    rows: list[IndexRow] = []
    total = max(len(frames), 1)
    for i, (source, df, fmt) in enumerate(frames):
        columns = list(df.columns)
        for _, row in df.iterrows():
            parsed = _row_from_format(row, columns, fmt, cfg)
            if parsed:
                row_name, row_model, row_brand = parsed
            else:
                row_name = ""
                row_model = ""
                row_brand = ""
                for c in columns:
                    s = _cell_str(row.get(c))
                    if s and not s.replace(".", "", 1).isdigit():
                        row_name = s
                        break
                if not row_name:
                    continue

            blob = _row_all_text(row, columns)
            if not blob.strip():
                continue
            bn = _norm(blob)
            bc = re.sub(r"[^a-zA-Zа-яА-Я0-9]", "", bn)
            if not bn:
                continue
            rows.append(
                IndexRow(
                    name=row_name,
                    model=row_model,
                    brand=row_brand,
                    source=source,
                    blob_norm=bn,
                    blob_compact=bc,
                )
            )
        if progress:
            pct = pct_from + int((pct_to - pct_from) * (i + 1) / total)
            progress(pct, f"Индекс прайса: {i + 1}/{total}…")
    return rows


def get_excel_index(
    cfg: dict[str, Any],
    progress: ProgressFn | None = None,
) -> list[IndexRow]:
    """Загружает или строит быстрый индекс прайсов."""
    price_dir: Path = cfg["price_dir"]
    cache = _cache_path(cfg)
    sig = _file_signature(price_dir)

    if cache.exists():
        try:
            with cache.open("rb") as fh:
                payload = pickle.load(fh)
            if (
                payload.get("version") == INDEX_VERSION
                and payload.get("sig") == sig
                and isinstance(payload.get("rows"), list)
            ):
                if progress:
                    progress(50, "Excel-кэш готов (мгновенный поиск)…")
                logger.info("Excel-индекс из кэша: %s строк", len(payload["rows"]))
                return payload["rows"]
        except Exception as exc:
            logger.warning("Кэш Excel повреждён, пересобираем: %s", exc)

    if progress:
        progress(12, "Первый проход: читаю ваши Excel…")
    t0 = time.perf_counter()
    frames = load_price_frames(price_dir, progress=progress, pct_from=12, pct_to=42)
    rows = _build_index_rows(frames, cfg, progress=progress, pct_from=42, pct_to=55)
    try:
        cache.parent.mkdir(parents=True, exist_ok=True)
        with cache.open("wb") as fh:
            pickle.dump(
                {"version": INDEX_VERSION, "sig": sig, "rows": rows},
                fh,
                protocol=pickle.HIGHEST_PROTOCOL,
            )
    except Exception as exc:
        logger.warning("Не удалось сохранить кэш Excel: %s", exc)

    logger.info(
        "Excel-индекс построен: %s строк за %.2fс",
        len(rows),
        time.perf_counter() - t0,
    )
    if progress:
        progress(55, f"Excel готов: {len(rows)} строк")
    return rows


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
        cfg.get("col_brand")
        or ["Торговая марка", "Бренд", "Производитель", "Фирма", "Brand"],
    )
    col_model = _find_column(
        columns,
        cfg.get("col_model")
        or ["Мадель", "Модель", "МОДЕЛЬ АВТОМОБИЛЯ", "Модель автомобиля", "Model"],
    )

    # --- Формат 4: Артикул | Наименование | Модель | Торговая марка ---
    if fmt == "supplier_artikul":
        name_col = col_name or _find_column(columns, ["Наименование"])
        if not name_col:
            return None
        name = _cell_str(row.get(name_col))
        if not name:
            return None
        sub_col = _find_column(columns, ["Подгруппа"])
        subgroup = _cell_str(row.get(sub_col)) if sub_col else ""
        # Если наименование короткое («Адаптер B1») — добавим подгруппу
        if subgroup and _norm(subgroup) not in _norm(name):
            if len(name.split()) <= 3:
                name = f"{subgroup} {name}".strip()
        model = _cell_str(row.get(col_model)) if col_model else ""
        brand_col = col_brand or _find_column(columns, ["Торговая марка"])
        brand = _cell_str(row.get(brand_col)) if brand_col else ""
        # Убрать хвост OE из бренда вида «SRR OE»
        if brand.upper().endswith(" OE"):
            brand = brand[:-3].strip()
        return name, model, brand

    # --- Формат 5: Код производителя | Номер OE | Наименование | Марка ---
    if fmt == "kod_proizvoditelya":
        name_col = col_name or _find_column(columns, ["Наименование"])
        if not name_col:
            return None
        raw_name = _cell_str(row.get(name_col))
        if not raw_name:
            return None
        # Марка здесь — марка авто (Audi/VW), используем как модель, если нет отдельной
        marka_col = _find_column(columns, ["Марка"])
        marka = _cell_str(row.get(marka_col)) if marka_col else ""
        name, brand = split_trailing_brand(raw_name)
        model = marka
        # Если бренд не отделился, а в имени есть известный хвост — model=marka, brand=""
        if brand and _norm(brand) in _norm(marka):
            # «Audi» как бренд запчасти маловероятно — вернём в имя
            name = raw_name
            brand = ""
        return name, model, brand

    if fmt == "slash_nomenclature":
        raw_col = col_name or _find_column(
            columns,
            [
                "Ценовая группа/ Номенклатура/ Характеристика номенклатуры",
                "Номенклатура",
                "Характеристика номенклатуры",
            ],
        )
        if not raw_col:
            raw_col = columns[0] if columns else None
        if not raw_col:
            return None
        raw = _cell_str(row.get(raw_col))
        name_model, brand, _, _code = parse_slash_nomenclature(raw)
        if not name_model:
            return None
        return name_model.strip(), "", brand.strip()

    if fmt == "tovar_proizvoditel":
        t_col = _find_column(columns, ["Товар"]) or col_name
        b_col = _find_column(columns, ["Производитель"]) or col_brand
        if not t_col:
            return None
        name = _cell_str(row.get(t_col))
        brand = _cell_str(row.get(b_col)) if b_col else ""
        if not name:
            return None
        return name, "", brand

    if not col_name:
        return None
    name = _cell_str(row.get(col_name))
    if not name:
        return None
    if "//" in name:
        name_model, brand2, _, _code = parse_slash_nomenclature(name)
        brand = brand2
        model = ""
        if col_brand and not brand:
            brand = _cell_str(row.get(col_brand))
        return name_model, model, brand

    brand = _cell_str(row.get(col_brand)) if col_brand else ""
    model = _cell_str(row.get(col_model)) if col_model else ""
    return name, model, brand


def search_excel_candidates(
    query: str,
    cfg: dict[str, Any],
    progress: ProgressFn | None = None,
) -> list:
    """Кандидаты из ВАШИХ прайсов — главный источник нужной запчасти.

    Код из поля «Имя» (штрихкод) ищем в любой ячейке строки прайса:
    номенклатура, Штрихкод, Номер, Артикул, Каталожный № и т.д.
    """
    from web_search import PartCandidate

    q = (query or "").strip()
    if len(q) < 3:
        return []

    if progress:
        progress(10, "Супербыстрый поиск в ваших Excel…")

    index = get_excel_index(cfg, progress=progress)
    scored: list[tuple[tuple, object]] = []
    seen: set[str] = set()
    qn = _norm(q)
    qc = re.sub(r"[^a-zA-Zа-яА-Я0-9]", "", qn)
    limit = max(int(cfg.get("max_candidates") or 12), 15)

    total = max(len(index), 1)
    step = max(total // 20, 1)
    for i, row in enumerate(index):
        if qn not in row.blob_norm and qc not in row.blob_compact:
            continue
        key = f"{row.name}|{row.brand}|{row.model}".upper()
        if key in seen:
            continue
        seen.add(key)
        # Точное совпадение токена/компакта важнее подстроки
        tokens = set(re.split(r"[^a-zA-Zа-яА-Я0-9]+", row.blob_norm))
        exact_token = 0 if (qn in tokens or qc in tokens) else 1
        # Более полное наименование — выше
        rank = (exact_token, -len(row.name or ""), -len(row.brand or ""), row.name)
        scored.append(
            (
                rank,
                PartCandidate(
                    brand=row.brand,
                    article=q,
                    title=row.name,
                    model=row.model,
                    source="excel",
                    extra={"file": row.source, "match": "price"},
                ),
            )
        )
        if progress and i % step == 0:
            pct = 55 + int(15 * i / total)
            progress(min(pct, 70), f"Сканирую прайс… найдено {len(scored)}")

    scored.sort(key=lambda x: x[0])
    out = [c for _, c in scored[:limit]]

    if progress:
        if out:
            progress(72, f"В Excel найдено: {len(out)}")
        else:
            progress(72, "В Excel подходящего нет")

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
    # Для скоринга по title/brand используем индекс (быстрее повторного чтения xlsx)
    try:
        index = get_excel_index(cfg, progress=None)
        for row in index:
            if not (row.name or "").strip():
                continue
            score_name = " ".join(p for p in (row.name, row.model) if p)
            score = _score_row(score_name, row.brand, title, brand)
            if score < min_score:
                continue
            if best is None or score > best.score:
                best = ExcelMatch(
                    name=row.name.strip(),
                    brand=(row.brand or brand or "").strip(),
                    model=(row.model or "").strip(),
                    source_file=row.source,
                    score=score,
                    format_id="",
                )
    except Exception:
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
                score_name = " ".join(p for p in (row_name, row_model) if p)
                score = _score_row(score_name, row_brand, title, brand)
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
