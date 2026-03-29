"""Парсинг дат из ключа «Физическое» → dd.mm"""

from __future__ import annotations

import re

_RU_MONTH_GEN: dict[str, int] = {
    "января": 1,
    "февраля": 2,
    "марта": 3,
    "апреля": 4,
    "мая": 5,
    "июня": 6,
    "июля": 7,
    "августа": 8,
    "сентября": 9,
    "октября": 10,
    "ноября": 11,
    "декабря": 12,
}


def parse_fizicheskie_daty_ddmm(val: str) -> list[str]:
    if not val or not val.strip():
        return []
    text = val.lower().strip()
    seen: set[tuple[int, int]] = set()
    out: list[str] = []
    for m in re.finditer(r"(\d{1,2})\s+([а-яё]+)", text):
        d_raw, mon_word = m.group(1), m.group(2)
        mon = _RU_MONTH_GEN.get(mon_word)
        if mon is None:
            continue
        try:
            day = int(d_raw)
        except ValueError:
            continue
        if day < 1 or day > 31:
            continue
        key = (mon, day)
        if key in seen:
            continue
        seen.add(key)
        out.append(f"{day:02d}.{mon:02d}")
    out.sort(key=lambda s: (int(s[3:5]), int(s[0:2])))
    return out
