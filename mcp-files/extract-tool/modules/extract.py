"""Универсальное извлечение: один файл или архив → текст (и опционально PDF)."""
from __future__ import annotations

import base64
import logging

from . import config
from . import archives
from . import document
from . import ocr
from . import asr

log = logging.getLogger("extract-tool.extract")


def extract_single_sync(data: bytes, filename: str) -> str | None:
    """Один файл: текст / PDF / DOCX / ODT / OCR / ASR. None = неподдерживаемый тип."""
    ext = archives.file_extension(filename)
    if ext == ".docx":
        return document.extract_text_from_docx(data)
    if ext == ".odt":
        return document.extract_text_from_odt(data)
    if ext in config.TEXT_EXT:
        return document.extract_text_from_text_file(data, filename)
    if ext == ".pdf" or (len(data) >= 4 and data[:4] == b"%PDF"):
        return document.extract_text_from_pdf(data)
    if ext in config.IMAGE_EXT:
        if not config.ocr_enabled():
            log.info("Image skipped (OCR not configured): %s", filename)
            return ""
        try:
            return ocr.run_ocr_on_image(data)
        except Exception as e:
            log.warning("OCR in extract failed: %s", e)
            return ""
    if ext in config.AUDIO_EXT:
        if not config.asr_enabled():
            log.info("Audio skipped (ASR not configured): %s", filename)
            return ""
        try:
            return asr.run_asr_on_audio(data, suffix=ext or ".wav")
        except Exception as e:
            log.warning("ASR in extract failed: %s", e)
            return ""
    return None


def extract_impl(data: bytes, filename: str) -> dict:
    """Синхронная реализация извлечения (архив или один файл). Возвращает {"text": "...", "pdf_base64": "..."?}."""
    ext = archives.file_extension(filename)
    log.info("Extract request: filename=%s size=%d ext=%s", filename, len(data), ext)
    if not data:
        return {"text": ""}
    if ext in {".zip", ".tar", ".tar.gz", ".tar.xz", ".tgz", ".txz"}:
        items = archives.extract_archive(data, filename)
        if items is None:
            raise ValueError("Файл или архив нельзя обработать.")
        parts = []
        for name, content in items:
            item_ext = archives.file_extension(name)
            if item_ext == ".docx":
                parts.append(f"--- {name}\n" + document.extract_text_from_docx(content))
            elif item_ext == ".odt":
                parts.append(f"--- {name}\n" + document.extract_text_from_odt(content))
            elif item_ext in config.TEXT_EXT:
                parts.append(f"--- {name}\n" + document.extract_text_from_text_file(content, name))
            elif item_ext == ".pdf" or (len(content) >= 4 and content[:4] == b"%PDF"):
                parts.append(f"--- {name}\n" + document.extract_text_from_pdf(content))
            elif item_ext in config.IMAGE_EXT or item_ext in config.AUDIO_EXT:
                t = extract_single_sync(content, name)
                if t is not None:
                    parts.append(f"--- {name}\n" + (t or ""))
        return {"text": "\n\n".join(p for p in parts if p.strip())}
    if ext in config.IMAGE_EXT:
        log.info("Processing as image (OCR): %s", filename)
    elif ext in config.AUDIO_EXT:
        log.info("Processing as audio (ASR): %s", filename)
    elif ext in config.TEXT_EXT or ext == ".pdf" or ext in config.EXPORT_TO_PDF_EXT:
        log.info("Processing as text/PDF/export: %s", filename)
    result = extract_single_sync(data, filename)
    if result is not None:
        out = {"text": result}
        if ext in config.EXPORT_TO_PDF_EXT and result.strip():
            pdf_bytes = document.text_to_pdf_bytes(result)
            if pdf_bytes:
                out["pdf_base64"] = base64.b64encode(pdf_bytes).decode("ascii")
        return out
    raise ValueError("Файл или архив нельзя обработать.")
