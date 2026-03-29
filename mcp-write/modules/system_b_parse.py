"""Система B: парсинг документа по меткам. Значение = текст от метки до следующей метки (или до конца).
Для proyavlenie, gospodstvo и fizicheskoe — значение только до первой точки (как в исходных чанках):
метки «Физическое:» и «Физическая:» (и синонимы из config) — один ключ fizicheskoe, вырезание до «.»."""
from . import config

# Ключи, у которых значение обрезается по первой точке (не тянем до следующей метки)
KEYS_UNTIL_FIRST_PERIOD = frozenset({
    "proyavlenie",
    "gospodstvo",
    "emocionalnoe",
    "intellektualnye",
    "astralnyi_duh",
    "fizicheskoe",
})

# Ключи, которые не включаются в полный контекст при сохранении в Postgres (core.document_context)
KEYS_EXCLUDED_FROM_FULL_CONTEXT = frozenset(
    {"emocionalnoe", "intellektualnye", "astralnyi_duh", "fizicheskoe"}
)


def _truncate_at_first_period(text: str) -> tuple[str, int]:
    """Возвращает (значение до первой точки, число символов с начала). Точка после многоточия не считается концом."""
    text = text.strip()
    if not text:
        return "", 0
    i = 0
    while i < len(text):
        if text[i] == ".":
            if i + 1 < len(text) and text[i + 1] == ".":
                i += 2
                continue
            val = text[:i].strip()
            return val, i + 1
        i += 1
    return text.strip(), len(text)


def _all_label_positions(raw: str) -> list[tuple[int, str, str]]:
    """Возвращает список (позиция_в_тексте, метка, key_name), отсортированный по позиции."""
    entries: list[tuple[int, str, str]] = []
    for key_name, label_or_labels in config.SYSTEM_B_LABELS[1:]:
        if label_or_labels is None:
            continue
        labels = [label_or_labels] if isinstance(label_or_labels, str) else label_or_labels
        for label in labels:
            if not label:
                continue
            idx = 0
            while True:
                pos = raw.find(label, idx)
                if pos == -1:
                    break
                entries.append((pos, label, key_name))
                idx = pos + 1
    # Сначала позиция в тексте, затем более длинная метка (чтобы «Физическая дата:» была раньше «Физическая:»).
    entries.sort(key=lambda x: (x[0], -len(x[1])))
    # На одной позиции для одного key_name несколько вариантов меток (префиксы) давали бы next_start == pos
    # и пустой сегмент raw[label_end:next_start]. Оставляем одну запись — самую длинную метку.
    seen_pos_key: set[tuple[int, str]] = set()
    deduped: list[tuple[int, str, str]] = []
    for pos, label, key_name in entries:
        pk = (pos, key_name)
        if pk in seen_pos_key:
            continue
        seen_pos_key.add(pk)
        deduped.append((pos, label, key_name))
    return deduped


def _segment_after_label(raw: str, label_end: int, next_label_start: int | None) -> str:
    """Текст от label_end до next_label_start. Убираем концевые точки/пробелы."""
    end = next_label_start if next_label_start is not None else len(raw)
    after = raw[label_end:end].strip()
    return after.rstrip(".").strip() or after.strip()


def _segment_end_for_rest(raw: str, label_end: int, next_start: int | None, key_name: str) -> int:
    """Конец сегмента в raw для get_rest_context: для KEYS_UNTIL_FIRST_PERIOD — до первой точки, иначе до next_start."""
    end = next_start if next_start is not None else len(raw)
    if key_name not in KEYS_UNTIL_FIRST_PERIOD:
        return end
    # Ищем первую точку (не в составе ..) в сыром куске raw[label_end:end]
    segment = raw[label_end:end]
    i = 0
    while i < len(segment):
        if segment[i] == ".":
            if i + 1 < len(segment) and segment[i + 1] == ".":
                i += 2
                continue
            return label_end + i + 1
        i += 1
    return end


def parse_system_b_keys(raw: str) -> dict[str, str]:
    raw = (raw or "").strip()
    if not raw:
        return {}
    out: dict[str, str] = {}
    parts = raw.split()
    if parts:
        out["name"] = parts[0].strip()

    positions = _all_label_positions(raw)
    for i, (pos, label, key_name) in enumerate(positions):
        label_end = pos + len(label)
        next_start = positions[i + 1][0] if i + 1 < len(positions) else None
        segment = _segment_after_label(raw, label_end, next_start)
        if key_name in KEYS_UNTIL_FIRST_PERIOD and segment:
            val, _ = _truncate_at_first_period(segment)
        else:
            val = segment
        if not val:
            continue
        if key_name == "fizicheskoe":
            prev = (out.get(key_name) or "").strip()
            out[key_name] = (prev + " " + val).strip() if prev else val
        elif key_name not in out or len(val) > len(out.get(key_name, "")):
            out[key_name] = val
    return out


def strip_leading_dots_and_name(s: str, name: str) -> str:
    s = (s or "").strip()
    name = (name or "").strip()
    while s and s[0] in " .\t\n\r":
        s = s[1:].strip()
    if name and (s.startswith(name + " ") or s.startswith(name + " -") or s.startswith(name + ".")):
        s = s[len(name) :].strip()
        while s and s[0] in " .\t\n\r":
            s = s[1:].strip()
    return s


def get_rest_context(raw: str, keys: dict[str, str]) -> str:
    """Остальной контекст: всё, что не входит в распознанные блоки (заголовок name + метки)."""
    raw = (raw or "").strip()
    if not raw:
        return ""
    positions = _all_label_positions(raw)
    # Сегменты = заголовок (имя) + все блоки по меткам
    segments: list[tuple[int, int]] = []
    lead = 0
    while lead < len(raw) and raw[lead] in " \t\n\r":
        lead += 1
    if lead > 0:
        segments.append((0, lead))
    name = (keys.get("name") or "").strip()
    if name and lead < len(raw) and raw[lead:].startswith(name):
        pos = lead + len(name)
        while pos < len(raw) and raw[pos] in " \t\n\r.:-":
            pos += 1
        segments.append((lead, pos))
    for i, (pos, label, key_name) in enumerate(positions):
        label_end = pos + len(label)
        next_start = positions[i + 1][0] if i + 1 < len(positions) else len(raw)
        segment_end = _segment_end_for_rest(raw, label_end, next_start, key_name)
        segments.append((pos, segment_end))
    segments.sort(key=lambda x: x[0])
    merged: list[tuple[int, int]] = []
    for s, e in segments:
        if merged and s <= merged[-1][1]:
            merged[-1] = (merged[-1][0], max(merged[-1][1], e))
        else:
            merged.append((s, e))
    parts = []
    prev = 0
    for s, e in merged:
        if s > prev:
            parts.append(raw[prev:s])
        prev = e
    if prev < len(raw):
        parts.append(raw[prev:])
    rest = " ".join(p.strip() for p in parts if p.strip()).strip()
    rest = strip_leading_dots_and_name(rest, name)
    return rest


def full_context_for_postgres(raw: str, keys: dict[str, str]) -> str:
    """Полный контекст для сохранения в Postgres без секций Эмоциональное, Интеллектуальные, Астральный дух."""
    raw = (raw or "").strip()
    if not raw:
        return raw
    positions = _all_label_positions(raw)
    exclude_ranges: list[tuple[int, int]] = []
    for i, (pos, label, key_name) in enumerate(positions):
        if key_name not in KEYS_EXCLUDED_FROM_FULL_CONTEXT:
            continue
        label_end = pos + len(label)
        next_start = positions[i + 1][0] if i + 1 < len(positions) else len(raw)
        segment_end = _segment_end_for_rest(raw, label_end, next_start, key_name)
        exclude_ranges.append((pos, segment_end))
    if not exclude_ranges:
        return raw
    exclude_ranges.sort(key=lambda x: x[0])
    merged: list[tuple[int, int]] = []
    for s, e in exclude_ranges:
        if merged and s <= merged[-1][1]:
            merged[-1] = (merged[-1][0], max(merged[-1][1], e))
        else:
            merged.append((s, e))
    parts: list[str] = []
    prev = 0
    for s, e in merged:
        if s > prev:
            chunk = raw[prev:s].rstrip()
            if chunk:
                parts.append(chunk)
        prev = e
    if prev < len(raw):
        chunk = raw[prev:].lstrip()
        if chunk:
            parts.append(chunk)
    return "\n\n".join(p for p in parts if p.strip()).strip()
