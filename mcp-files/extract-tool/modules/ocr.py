"""OCR через OpenRouter vision (OCR_MODEL). Локальные модели не используются."""
from __future__ import annotations

import logging

from . import config
from . import openrouter_client

log = logging.getLogger("extract-tool.ocr")


def ocr_available() -> bool:
    return config.ocr_enabled()


def get_ocr():
    if not config.ocr_enabled():
        return None
    return ("openrouter", config.OCR_MODEL)


def run_ocr_on_image(data: bytes) -> str:
    if not config.ocr_enabled():
        log.info("OCR skipped: OCR_MODEL or OPENROUTER_API_KEY not set")
        return ""
    mime = "image/jpeg"
    if len(data) >= 8 and data[:8] == b"\x89PNG\r\n\x1a\n":
        mime = "image/png"
    elif len(data) >= 3 and data[:3] == b"GIF":
        mime = "image/gif"
    elif len(data) >= 4 and data[:4] == b"RIFF":
        mime = "image/webp"
    log.info("Running OCR via OpenRouter model=%s", config.OCR_MODEL)
    return openrouter_client.ocr_image(data, mime=mime)
