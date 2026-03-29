"""Парсинг дат из ключа «Физическое» → список строк dd.mm для core.angel_physical_date_entries.

В Postgres:
- core.angel_physical_dates: chunk_id, doc_id, name, dates_ddmm TEXT[]
- core.angel_physical_date_entries: по одной строке (ddmm, chunk_id) на каждую дату для поиска «кто на dd.mm».

Поддерживаемые форматы в тексте после метки «Физическое:»:
- «15 января», «3 марта» (родительный падеж)
- «15 январь», «3 март» (номинатив)
- «15.01», «15/03», «3-12» (день.месяц или день/месяц; год не обязателен)
"""

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

_RU_MONTH_NOM: dict[str, int] = {
    "январь": 1,
    "февраль": 2,
    "март": 3,
    "апрель": 4,
    "май": 5,
    "июнь": 6,
    "июль": 7,
    "август": 8,
    "сентябрь": 9,
    "октябрь": 10,
    "ноябрь": 11,
    "декабрь": 12,
}


def _add_date(seen: set[tuple[int, int]], out: list[str], day: int, mon: int) -> None:
    if mon < 1 or mon > 12 or day < 1 or day > 31:
        return
    key = (mon, day)
    if key in seen:
        return
    seen.add(key)
    out.append(f"{day:02d}.{mon:02d}")


def parse_fizicheskie_daty_ddmm(val: str) -> list[str]:
    if not val or not val.strip():
        return []
    text = val.lower().strip()
    seen: set[tuple[int, int]] = set()
    out: list[str] = []

    # Числовые dd.mm / dd/mm / dd-mm (не забираем фрагменты вроде 2015.01)
    for m in re.finditer(r"(?<![0-9])(\d{1,2})\s*([./-])\s*(\d{1,2})(?![0-9])", text):
        try:
            day = int(m.group(1))
            mon = int(m.group(3))
        except ValueError:
            continue
        _add_date(seen, out, day, mon)

    # «12 января» / «5 март»
    for m in re.finditer(r"(\d{1,2})\s+([а-яё]+)", text):
        d_raw, mon_word = m.group(1), m.group(2)
        mon = _RU_MONTH_GEN.get(mon_word) or _RU_MONTH_NOM.get(mon_word)
        if mon is None:
            continue
        try:
            day = int(d_raw)
        except ValueError:
            continue
        _add_date(seen, out, day, mon)

    out.sort(key=lambda s: (int(s[3:5]), int(s[0:2])))
    return out
