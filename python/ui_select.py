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
            f"Запрос: «{query}» — найдено вариантов: {len(candidates)}\n"
            "Если в списке нет нужного товара — введите название вручную внизу\n"
            "(формат: Наименование  Модель  Бренд)."
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
            src = f" [{c.source}]" if c.source else ""
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
