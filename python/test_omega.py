"""Проверка быстрого поиска Omega Auto Parts API.

Использование:
  python python/test_omega.py
  python python/test_omega.py 361337

Перед полноценным поиском укажите в config.ini:
  [omega]
  login = ваш_email_или_телефон
  api_key = ключ_со_страницы_/shop/api
"""
from __future__ import annotations

import json
import sys
import time
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))

from config_loader import load_config
from omega_search import probe_omega, search_omega


def main() -> int:
    cfg = load_config()
    number = sys.argv[1] if len(sys.argv) > 1 else cfg.get("omega_test_number") or "120281"

    print("=== Omega API probe ===")
    probe = probe_omega(cfg)
    print(json.dumps(probe, ensure_ascii=False, indent=2))

    if not probe.get("has_credentials"):
        print()
        print("Нет login/api_key в config.ini → поиск товаров через Omega пока недоступен.")
        print("1) Зарегистрируйтесь как опт на https://omega-auto-parts.kz")
        print("2) Откройте https://omega-auto-parts.kz/shop/api и включите API")
        print("3) Пропишите login и api_key в config.ini секция [omega]")
        print()
        print("Проверка без ключа: API отвечает ~0.6с и требует авторизацию — это нормально.")
        return 0

    print()
    print(f"=== search_omega({number!r}) ===")
    t0 = time.time()
    rows = search_omega(number, cfg)
    dt = time.time() - t0
    print(f"time: {dt:.2f}s, found: {len(rows)}")
    for i, c in enumerate(rows, 1):
        print(f"{i}. {c.display}")
    return 0 if rows else 1


if __name__ == "__main__":
    raise SystemExit(main())
