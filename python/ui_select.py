"""Окно выбора варианта товара (tkinter)."""
from __future__ import annotations

from typing import Sequence

from web_search import PartCandidate


def select_candidate(candidates: Sequence[PartCandidate], query: str) -> PartCandidate | None:
    """Показывает список вариантов. Возвращает выбранный или None при отмене."""
    if not candidates:
        return None
    if len(candidates) == 1:
        return candidates[0]

    try:
        import tkinter as tk
        from tkinter import ttk
    except ImportError:
        # На серверах без GUI — берём первый вариант (на Windows tkinter есть)
        return candidates[0]

    root = tk.Tk()
    root.title("Выбор товара — Microinvest Assistant")
    root.attributes("-topmost", True)
    root.resizable(True, True)

    width, height = 720, 420
    sw = root.winfo_screenwidth()
    sh = root.winfo_screenheight()
    root.geometry(f"{width}x{height}+{(sw - width) // 2}+{(sh - height) // 3}")

    selected: dict[str, PartCandidate | None] = {"value": None}

    frame = ttk.Frame(root, padding=12)
    frame.pack(fill=tk.BOTH, expand=True)

    ttk.Label(
        frame,
        text=(
            f"Найдено несколько вариантов по запросу «{query}»:\n"
            "Выберите номер и нажмите OK."
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
    )
    lb.pack(side=tk.LEFT, fill=tk.BOTH, expand=True)
    scroll.config(command=lb.yview)

    for i, c in enumerate(candidates, start=1):
        src = f" [{c.source}]" if c.source else ""
        lb.insert(tk.END, f"{i}. {c.display}{src}")

    lb.selection_set(0)
    lb.focus_set()

    btns = ttk.Frame(frame)
    btns.pack(fill=tk.X, pady=(10, 0))

    def confirm(_event=None):
        idxs = lb.curselection()
        if not idxs:
            return
        selected["value"] = candidates[int(idxs[0])]
        root.destroy()

    def cancel(_event=None):
        selected["value"] = None
        root.destroy()

    ttk.Button(btns, text="OK", command=confirm).pack(side=tk.RIGHT, padx=(6, 0))
    ttk.Button(btns, text="Отмена", command=cancel).pack(side=tk.RIGHT)

    root.bind("<Return>", confirm)
    root.bind("<Escape>", cancel)
    lb.bind("<Double-Button-1>", confirm)

    num_frame = ttk.Frame(frame)
    num_frame.pack(fill=tk.X, pady=(8, 0))
    ttk.Label(num_frame, text="Номер варианта:").pack(side=tk.LEFT)
    num_var = tk.StringVar()
    entry = ttk.Entry(num_frame, textvariable=num_var, width=6)
    entry.pack(side=tk.LEFT, padx=6)

    def on_num(_event=None):
        raw = num_var.get().strip()
        if raw.isdigit():
            idx = int(raw) - 1
            if 0 <= idx < len(candidates):
                lb.selection_clear(0, tk.END)
                lb.selection_set(idx)
                lb.see(idx)
                confirm()

    entry.bind("<Return>", on_num)

    root.protocol("WM_DELETE_WINDOW", cancel)
    root.mainloop()
    return selected["value"]
