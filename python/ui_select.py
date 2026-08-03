"""Окно выбора варианта товара (tkinter) + ручной ввод.

Широкое окно + таблица: полное наименование видно целиком.
Превью итогового «Имя» в порядке:
  Название + характеристики + Модель + Бренд
"""
from __future__ import annotations

from typing import Sequence

from web_search import PartCandidate


def _preview_name(c: PartCandidate) -> str:
    """Как будет выглядеть поле «Имя» после OK."""
    from name_format import build_final_name

    return build_final_name(c.title, c.model, c.brand) or c.title or ""


def _source_label(c: PartCandidate) -> str:
    if c.source == "excel":
        return "ВАШ ПРАЙС"
    if c.source == "fapi":
        return "интернет"
    if c.source == "manual":
        return "ручной"
    return c.source or ""


def select_candidate(candidates: Sequence[PartCandidate], query: str) -> PartCandidate | None:
    """
    Показывает список вариантов и поля ручного ввода.
    Возвращает выбранный/введённый кандидат или None.
    """
    try:
        import tkinter as tk
        from tkinter import ttk, messagebox
    except ImportError:
        return candidates[0] if candidates else None

    root = tk.Tk()
    root.title("Выбор товара — Microinvest Assistant")
    root.attributes("-topmost", True)
    root.resizable(True, True)

    sw = root.winfo_screenwidth()
    sh = root.winfo_screenheight()
    width = min(max(1100, int(sw * 0.85)), sw - 40)
    height = min(max(640, int(sh * 0.75)), sh - 60)
    root.geometry(f"{width}x{height}+{(sw - width) // 2}+{max(20, (sh - height) // 5)}")

    selected: dict[str, PartCandidate | None] = {"value": None}

    frame = ttk.Frame(root, padding=12)
    frame.pack(fill=tk.BOTH, expand=True)

    ttk.Label(
        frame,
        text=(
            f"Запрос: «{query}» — найдено: {len(candidates)}\n"
            "Сначала ваши Excel; интернет — только если в прайсе пусто.\n"
            "Итоговое имя: Название запчасти + характеристики (4ц, объём, размер…) + Модель + Бренд."
        ),
        justify=tk.LEFT,
    ).pack(anchor=tk.W, pady=(0, 8))

    # Таблица с колонками — длинные названия видны (горизонтальный скролл)
    table_frame = ttk.Frame(frame)
    table_frame.pack(fill=tk.BOTH, expand=True)

    cols = ("preview", "model", "brand", "catalog", "src")
    tree = ttk.Treeview(
        table_frame,
        columns=cols,
        show="headings",
        selectmode="browse",
        height=16,
    )
    tree.heading("preview", text="Имя (как будет в Microinvest)")
    tree.heading("model", text="Модель")
    tree.heading("brand", text="Бренд")
    tree.heading("catalog", text="Каталожный №")
    tree.heading("src", text="Источник")

    tree.column("preview", width=int(width * 0.52), minwidth=320, stretch=True)
    tree.column("model", width=180, minwidth=100, stretch=False)
    tree.column("brand", width=110, minwidth=70, stretch=False)
    tree.column("catalog", width=130, minwidth=80, stretch=False)
    tree.column("src", width=100, minwidth=70, stretch=False)

    yscroll = ttk.Scrollbar(table_frame, orient=tk.VERTICAL, command=tree.yview)
    xscroll = ttk.Scrollbar(table_frame, orient=tk.HORIZONTAL, command=tree.xview)
    tree.configure(yscrollcommand=yscroll.set, xscrollcommand=xscroll.set)

    tree.grid(row=0, column=0, sticky="nsew")
    yscroll.grid(row=0, column=1, sticky="ns")
    xscroll.grid(row=1, column=0, sticky="ew")
    table_frame.rowconfigure(0, weight=1)
    table_frame.columnconfigure(0, weight=1)

    previews: list[str] = []
    if candidates:
        for c in candidates:
            prev = _preview_name(c)
            previews.append(prev)
            tree.insert(
                "",
                tk.END,
                values=(
                    prev,
                    c.model or "",
                    c.brand or "",
                    c.catalog_number or "",
                    _source_label(c),
                ),
            )
        first = tree.get_children()
        if first:
            tree.selection_set(first[0])
            tree.focus(first[0])
            tree.see(first[0])
    else:
        tree.insert("", tk.END, values=("(ничего не найдено)", "", "", "", ""))

    # Полная строка выбранного — чтобы точно всё прочитать
    detail = ttk.LabelFrame(frame, text="Полное наименование выбранной строки", padding=8)
    detail.pack(fill=tk.X, pady=(10, 0))
    detail_var = tk.StringVar(value=previews[0] if previews else "")
    detail_entry = ttk.Entry(detail, textvariable=detail_var, font=("Segoe UI", 11))
    detail_entry.pack(fill=tk.X)

    def on_select(_event=None):
        sel = tree.selection()
        if not sel or not candidates:
            return
        idx = tree.index(sel[0])
        if 0 <= idx < len(previews):
            detail_var.set(previews[idx])
            # Подставить в ручной ввод для правки
            c = candidates[idx]
            name_var.set(c.title or "")
            model_var.set(c.model or "")
            brand_var.set(c.brand or "")

    tree.bind("<<TreeviewSelect>>", on_select)

    # Ручной ввод
    manual = ttk.LabelFrame(frame, text="Ручной ввод / правка (Наименование → Модель → Бренд)", padding=8)
    manual.pack(fill=tk.X, pady=(10, 0))

    ttk.Label(manual, text="Наименование (+ характеристики):").grid(row=0, column=0, sticky="w")
    name_var = tk.StringVar()
    name_entry = ttk.Entry(manual, textvariable=name_var)
    name_entry.grid(row=0, column=1, columnspan=3, sticky="we", padx=4, pady=2)

    ttk.Label(manual, text="Модель авто:").grid(row=1, column=0, sticky="w")
    model_var = tk.StringVar()
    ttk.Entry(manual, textvariable=model_var, width=36).grid(row=1, column=1, sticky="we", padx=4, pady=2)

    ttk.Label(manual, text="Бренд:").grid(row=1, column=2, sticky="w", padx=(12, 0))
    brand_var = tk.StringVar()
    ttk.Entry(manual, textvariable=brand_var, width=22).grid(row=1, column=3, sticky="we", padx=4, pady=2)

    manual.columnconfigure(1, weight=1)
    manual.columnconfigure(3, weight=1)

    if candidates:
        on_select()

    btns = ttk.Frame(frame)
    btns.pack(fill=tk.X, pady=(10, 0))

    def confirm_list(_event=None):
        if not candidates:
            return "break"
        sel = tree.selection()
        if not sel:
            # если синим не видно — берём первую строку
            kids = tree.get_children()
            if not kids:
                return "break"
            tree.selection_set(kids[0])
            sel = tree.selection()
        idx = tree.index(sel[0])
        if idx < 0 or idx >= len(candidates):
            return "break"
        selected["value"] = candidates[idx]
        root.destroy()
        return "break"

    def confirm_manual(_event=None):
        name = name_var.get().strip()
        if not name:
            messagebox.showwarning("Ручной ввод", "Укажите наименование товара.")
            return "break"
        model = model_var.get().strip()
        brand = brand_var.get().strip()
        catalog = ""
        sel = tree.selection()
        if sel and candidates:
            idx = tree.index(sel[0])
            if 0 <= idx < len(candidates):
                catalog = candidates[idx].catalog_number or ""
        selected["value"] = PartCandidate(
            brand=brand,
            article=query,
            title=name,
            model=model,
            source="manual",
            catalog_number=catalog,
        )
        root.destroy()
        return "break"

    def cancel(_event=None):
        selected["value"] = None
        root.destroy()
        return "break"

    # Enter / цифровая Enter всегда подтверждают выделенную (или первую) строку
    btn_ok = ttk.Button(btns, text="OK (Enter)", command=confirm_list)
    btn_ok.pack(side=tk.RIGHT, padx=(6, 0))
    ttk.Button(btns, text="Использовать ручной ввод", command=confirm_manual).pack(side=tk.RIGHT, padx=(6, 0))
    ttk.Button(btns, text="Отмена (Esc)", command=cancel).pack(side=tk.RIGHT)

    def on_return(event=None):
        # Не даём Enter «проглотиться» Treeview/кнопками и закрыть окно без выбора
        return confirm_list(event)

    root.bind("<Escape>", cancel)
    for w in (root, tree, detail_entry, name_entry, btn_ok):
        w.bind("<Return>", on_return)
        w.bind("<KP_Enter>", on_return)
    tree.bind("<Double-Button-1>", confirm_list)
    root.protocol("WM_DELETE_WINDOW", cancel)
    # Глобально на время диалога — иначе Enter в Treeview раньше просто закрывал окно
    root.bind_all("<Return>", on_return)
    root.bind_all("<KP_Enter>", on_return)

    if candidates:
        tree.focus_set()
    else:
        name_entry.focus_set()

    try:
        root.mainloop()
    finally:
        try:
            root.unbind_all("<Return>")
            root.unbind_all("<KP_Enter>")
        except tk.TclError:
            pass
    return selected["value"]


def select_ntin(candidates, query: str):
    """Выбор NTIN из нескольких карточек НКТ."""
    if not candidates:
        return None
    if len(candidates) == 1:
        return candidates[0]

    try:
        import tkinter as tk
        from tkinter import ttk
    except ImportError:
        return candidates[0]

    root = tk.Tk()
    root.title("Выбор NTIN — Microinvest Assistant")
    root.attributes("-topmost", True)
    root.resizable(True, True)
    sw = root.winfo_screenwidth()
    sh = root.winfo_screenheight()
    w = min(960, sw - 40)
    h = min(520, sh - 80)
    root.geometry(f"{w}x{h}+{(sw - w) // 2}+{(sh - h) // 3}")

    picked = {"value": None}
    frame = ttk.Frame(root, padding=12)
    frame.pack(fill=tk.BOTH, expand=True)
    ttk.Label(
        frame,
        text=(
            f"Найдено несколько NTIN по запросу «{query}»:\n"
            "Выберите карточку товара из Национального каталога."
        ),
        justify=tk.LEFT,
    ).pack(anchor=tk.W, pady=(0, 8))

    cols = ("ntin", "name")
    tree = ttk.Treeview(frame, columns=cols, show="headings", selectmode="browse", height=14)
    tree.heading("ntin", text="NTIN")
    tree.heading("name", text="Название в НКТ")
    tree.column("ntin", width=160, stretch=False)
    tree.column("name", width=700, stretch=True)
    yscroll = ttk.Scrollbar(frame, orient=tk.VERTICAL, command=tree.yview)
    tree.configure(yscrollcommand=yscroll.set)
    tree.pack(side=tk.LEFT, fill=tk.BOTH, expand=True)
    yscroll.pack(side=tk.RIGHT, fill=tk.Y)

    for c in candidates:
        tree.insert("", tk.END, values=(c.ntin, c.name))
    kids = tree.get_children()
    if kids:
        tree.selection_set(kids[0])
        tree.focus(kids[0])

    btns = ttk.Frame(root, padding=12)
    btns.pack(fill=tk.X)

    def ok(_e=None):
        sel = tree.selection()
        if not sel:
            kids = tree.get_children()
            if not kids:
                return "break"
            tree.selection_set(kids[0])
            sel = tree.selection()
        idx = tree.index(sel[0])
        picked["value"] = candidates[idx]
        root.destroy()
        return "break"

    def skip(_e=None):
        picked["value"] = None
        root.destroy()
        return "break"

    ttk.Button(btns, text="OK (Enter)", command=ok).pack(side=tk.RIGHT, padx=(6, 0))
    ttk.Button(btns, text="Пропустить NTIN (Esc)", command=skip).pack(side=tk.RIGHT)
    tree.bind("<Double-Button-1>", ok)
    root.bind("<Escape>", skip)
    root.protocol("WM_DELETE_WINDOW", skip)
    root.bind_all("<Return>", ok)
    root.bind_all("<KP_Enter>", ok)
    tree.focus_set()
    try:
        root.mainloop()
    finally:
        try:
            root.unbind_all("<Return>")
            root.unbind_all("<KP_Enter>")
        except tk.TclError:
            pass
    return picked["value"]
