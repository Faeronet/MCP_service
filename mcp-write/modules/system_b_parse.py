"""Система B: парсинг документа по меткам. Значение = текст от метки до следующей метки (или до конца)."""
from . import config


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
    entries.sort(key=lambda x: x[0])
    return entries


def _segment_after_label(raw: str, label_end: int, next_label_start: int | None) -> str:
    """Текст от label_end до next_label_start. Убираем концевые точки/пробелы."""
    end = next_label_start if next_label_start is not None else len(raw)
    after = raw[label_end:end].strip()
    return after.rstrip(".").strip() or after.strip()


def parse_system_b_keys(raw: str) -> dict[str, str]:
    raw = (raw or "").strip()
    if not raw:
        return {}
    out: dict[str, str] = {}
    parts = raw.split()
    if parts:
        out["name"] = parts[0].strip()

    # Все вхождения меток в порядке появления в документе
    positions = _all_label_positions(raw)
    for i, (pos, label, key_name) in enumerate(positions):
        label_end = pos + len(label)
        next_start = positions[i + 1][0] if i + 1 < len(positions) else None
        val = _segment_after_label(raw, label_end, next_start)
        if val and (key_name not in out or len(val) > len(out.get(key_name, ""))):
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
    for i, (pos, label, _) in enumerate(positions):
        label_end = pos + len(label)
        next_start = positions[i + 1][0] if i + 1 < len(positions) else len(raw)
        segments.append((pos, next_start))
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
