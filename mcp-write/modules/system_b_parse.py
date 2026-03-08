"""Система B: парсинг документа по меткам (ключи до первой точки)."""
from . import config


def extract_after_label_until_first_period(full_text: str, label: str) -> str | None:
    if not label or not full_text or label not in full_text:
        return None
    idx = full_text.find(label)
    if idx == -1:
        return None
    after = full_text[idx + len(label) :].lstrip()
    for i, c in enumerate(after):
        if c == ".":
            if i + 1 < len(after) and after[i + 1] == ".":
                continue
            return after[:i].strip()
    return after.strip()


def parse_system_b_keys(raw: str) -> dict[str, str]:
    raw = (raw or "").strip()
    if not raw:
        return {}
    out: dict[str, str] = {}
    parts = raw.split()
    if parts:
        out["name"] = parts[0].strip()
    for key_name, label_or_labels in config.SYSTEM_B_LABELS[1:]:
        if label_or_labels is None:
            continue
        labels = [label_or_labels] if isinstance(label_or_labels, str) else label_or_labels
        for label in labels:
            if not label or label not in raw:
                continue
            val = extract_after_label_until_first_period(raw, label)
            if val is not None and val.strip():
                out[key_name] = val.strip()
                break
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
    raw = (raw or "").strip()
    if not raw:
        return ""
    segments: list[tuple[int, int]] = []
    lead = 0
    while lead < len(raw):
        c = raw[lead]
        if c.isalpha() or c.isdigit() or c == ":":
            break
        lead += 1
    if lead > 0:
        segments.append((0, lead))
    name = (keys.get("name") or "").strip()
    if name and lead < len(raw) and raw[lead:].startswith(name):
        pos = lead + len(name)
        while pos < len(raw):
            c = raw[pos]
            if c.isalpha() or c.isdigit() or c == ":":
                break
            pos += 1
        segments.append((lead, pos))
    for key_name, label_or_labels in config.SYSTEM_B_LABELS[1:]:
        if label_or_labels is None:
            continue
        value = keys.get(key_name, "").strip()
        if not value:
            continue
        labels = [label_or_labels] if isinstance(label_or_labels, str) else label_or_labels
        for label in labels:
            if not label or label not in raw:
                continue
            idx = raw.find(label)
            if idx == -1:
                continue
            after = raw[idx + len(label) :].lstrip()
            end_in_after: int | None = None
            for i, c in enumerate(after):
                if c == ".":
                    if i + 1 < len(after) and after[i + 1] == ".":
                        continue
                    end_in_after = i
                    break
            if end_in_after is None:
                segment_end = idx + len(label) + len(after)
            else:
                segment_end = idx + len(label) + len(after[: end_in_after + 1])
            extracted = after[:end_in_after].strip() if end_in_after is not None else after.strip()
            if extracted != value:
                continue
            segments.append((idx, segment_end))
            break
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
    rest = strip_leading_dots_and_name(rest, keys.get("name") or "")
    return rest
