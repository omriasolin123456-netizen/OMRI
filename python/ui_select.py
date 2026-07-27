"""Окно выбора варианта товара (tkinter) + ручной ввод."""
from __future__ import annotations

from typing import Sequence

from web_search import PartCandidate


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

    width, height = 820, 560
    sw = root.winfo_screenwidth()
    sh = root.winfo_screenheight()
    root.geometry(f"{width}x{height}+{(sw - width) // 2}+{(sh - height) // 4}")

    selected: dict[str, PartCandidate | None] = {"value": None}

    frame = ttk.Frame(root, padding=12)
    frame.pack(fill=tk.BOTH, expand=True)

    ttk.Label(
        frame,
        text=(
            f"Запрос: «{query}» — найдено: {len(candidates)}\n"
            "Сверху обычно позиции из ВАШЕГО прайса (Excel) — выбирайте их в первую очередь.\n"
            "Если нужного нет — введите вручную: Наименование  Модель  Бренд."
        ),
        justify=tk.LEFT,
    ).pack(anchor=tk.W, pady=(0, 8))

    list_frame = ttk.Frame(frame)
    list_frame.pack(fill=tk.BOTH, expand=True)

    scroll = ttk.Scrollbar(list_frame)
    scroll.pack(side=tk.RIGHT, fill=tk.Y)

    lb = tk.Listbox(
        list_frame,
        font=("Segoe UI", 11),
        yscrollcommand=scroll.set,
        activestyle="dotbox",
        exportselection=False,
        height=12,
    )
    lb.pack(side=tk.LEFT, fill=tk.BOTH, expand=True)
    scroll.config(command=lb.yview)

    if candidates:
    for i, c in enumerate(candidates, start=1):
        if c.source == "excel":
            src = " [ВАШ ПРАЙС]"
        elif c.source == "omega":
            src = " [Omega]"
        elif c.source:
            src = f" [{c.source}]"
        else:
            src = ""
        lb.insert(tk.END, f"{i}. {c.display}{src}")
        lb.selection_set(0)
    else:
        lb.insert(tk.END, "(Автоматический поиск ничего подходящего не нашёл)")

    # Ручной ввод
    manual = ttk.LabelFrame(frame, text="Ручной ввод (если список неверный)", padding=8)
    manual.pack(fill=tk.X, pady=(10, 0))

    ttk.Label(manual, text="Наименование:").grid(row=0, column=0, sticky="w")
    name_var = tk.StringVar()
    name_entry = ttk.Entry(manual, textvariable=name_var, width=70)
    name_entry.grid(row=0, column=1, columnspan=3, sticky="we", padx=4, pady=2)

    ttk.Label(manual, text="Модель авто:").grid(row=1, column=0, sticky="w")
    model_var = tk.StringVar()
    ttk.Entry(manual, textvariable=model_var, width=28).grid(row=1, column=1, sticky="w", padx=4, pady=2)

    ttk.Label(manual, text="Бренд:").grid(row=1, column=2, sticky="w", padx=(12, 0))
    brand_var = tk.StringVar()
    ttk.Entry(manual, textvariable=brand_var, width=20).grid(row=1, column=3, sticky="w", padx=4, pady=2)

    manual.columnconfigure(1, weight=1)

    btns = ttk.Frame(frame)
    btns.pack(fill=tk.X, pady=(10, 0))

    def confirm_list(_event=None):
        if not candidates:
            return
        idxs = lb.curselection()
        if not idxs:
            return
        idx = int(idxs[0])
        if idx < 0 or idx >= len(candidates):
            return
        selected["value"] = candidates[idx]
        root.destroy()

    def confirm_manual(_event=None):
        name = name_var.get().strip()
        if not name:
            messagebox.showwarning("Ручной ввод", "Укажите наименование товара.")
            return
        model = model_var.get().strip()
        brand = brand_var.get().strip()
        selected["value"] = PartCandidate(
            brand=brand,
            article=query,
            title=name,
            model=model,
            source="manual",
        )
        root.destroy()

    def cancel(_event=None):
        selected["value"] = None
        root.destroy()

    ttk.Button(btns, text="OK (из списка)", command=confirm_list).pack(side=tk.RIGHT, padx=(6, 0))
    ttk.Button(btns, text="Использовать ручной ввод", command=confirm_manual).pack(side=tk.RIGHT, padx=(6, 0))
    ttk.Button(btns, text="Отмена", command=cancel).pack(side=tk.RIGHT)

    root.bind("<Escape>", cancel)
    lb.bind("<Double-Button-1>", confirm_list)
    root.protocol("WM_DELETE_WINDOW", cancel)

    if candidates:
        lb.focus_set()
    else:
        name_entry.focus_set()

    root.mainloop()
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
    w, h = 780, 420
    root.geometry(f"{w}x{h}+{(root.winfo_screenwidth()-w)//2}+{(root.winfo_screenheight()-h)//3}")

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

    lb = tk.Listbox(frame, font=("Segoe UI", 11), exportselection=False)
    lb.pack(fill=tk.BOTH, expand=True)
    for i, c in enumerate(candidates, 1):
        lb.insert(tk.END, f"{i}. {c.display}")
    lb.selection_set(0)

    btns = ttk.Frame(frame)
    btns.pack(fill=tk.X, pady=(10, 0))

    def ok(_e=None):
        idxs = lb.curselection()
        if not idxs:
            return
        picked["value"] = candidates[int(idxs[0])]
        root.destroy()

    def skip(_e=None):
        picked["value"] = None
        root.destroy()

    ttk.Button(btns, text="OK", command=ok).pack(side=tk.RIGHT, padx=(6, 0))
    ttk.Button(btns, text="Пропустить NTIN", command=skip).pack(side=tk.RIGHT)
    lb.bind("<Double-Button-1>", ok)
    root.bind("<Return>", ok)
    root.bind("<Escape>", skip)
    root.protocol("WM_DELETE_WINDOW", skip)
    root.mainloop()
    return picked["value"]
