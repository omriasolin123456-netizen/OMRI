"""Маленькое окно живого прогресса 0–100% (поверх всех окон)."""
from __future__ import annotations

from typing import Callable


class ProgressUI:
    """Компактный прогресс-бар. Без tkinter печатает в консоль."""

    def __init__(self, title: str = "Поиск запчасти — Microinvest") -> None:
        self._root = None
        self._pct_var = None
        self._text_var = None
        self._pct_label = None
        self._closed = False
        try:
            import tkinter as tk
            from tkinter import ttk

            root = tk.Tk()
            root.title(title)
            root.attributes("-topmost", True)
            root.resizable(False, False)
            w, h = 380, 118
            sw = root.winfo_screenwidth()
            sh = root.winfo_screenheight()
            root.geometry(f"{w}x{h}+{(sw - w) // 2}+{max(40, sh // 5)}")
            # Не даём закрыть крестиком во время поиска (иначе AHK ждёт зря)
            root.protocol("WM_DELETE_WINDOW", lambda: None)

            frame = ttk.Frame(root, padding=14)
            frame.pack(fill=tk.BOTH, expand=True)

            self._text_var = tk.StringVar(value="Подготовка…")
            ttk.Label(frame, textvariable=self._text_var, wraplength=340).pack(anchor=tk.W)

            self._pct_var = tk.IntVar(value=0)
            bar = ttk.Progressbar(
                frame,
                orient=tk.HORIZONTAL,
                length=340,
                mode="determinate",
                maximum=100,
                variable=self._pct_var,
            )
            bar.pack(fill=tk.X, pady=(10, 4))

            self._pct_label = ttk.Label(frame, text="0% из 100")
            self._pct_label.pack(anchor=tk.E)

            self._root = root
            root.update_idletasks()
            root.update()
        except Exception:
            self._root = None

    @property
    def available(self) -> bool:
        return self._root is not None and not self._closed

    def set(self, pct: int | float, text: str | None = None) -> None:
        """Обновить процент (0–100) и подпись."""
        if self._closed:
            return
        value = max(0, min(100, int(pct)))
        if self._root is not None and self._pct_var is not None:
            try:
                self._pct_var.set(value)
                if text is not None and self._text_var is not None:
                    self._text_var.set(text)
                if self._pct_label is not None:
                    self._pct_label.config(text=f"{value}% из 100")
                self._root.update_idletasks()
                self._root.update()
            except Exception:
                pass
        else:
            msg = text or ""
            print(f"[{value:3d}%] {msg}", flush=True)

    def close(self) -> None:
        if self._closed:
            return
        self._closed = True
        if self._root is not None:
            try:
                self._root.destroy()
            except Exception:
                pass
            self._root = None


ProgressCallback = Callable[[int, str], None]


def make_callback(ui: ProgressUI | None) -> ProgressCallback:
    if ui is None:
        return lambda _pct, _text: None
    return lambda pct, text: ui.set(pct, text)
