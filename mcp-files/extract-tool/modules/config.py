"""Конфигурация и константы сервиса."""
from __future__ import annotations

import os

PORT = int(os.getenv("PORT", "8004"))

OPENROUTER_API_BASE = (
    os.getenv("OPENROUTER_API_BASE")
    or os.getenv("OPENROUTER_BASE_URL")
    or os.getenv("LLM_BINDING_HOST")
    or os.getenv("VLLM_OPENAI_BASE")
    or "https://openrouter.ai/api/v1"
).strip().rstrip("/")

OPENROUTER_API_KEY = (
    os.getenv("OPENROUTER_API_KEY")
    or os.getenv("LLM_BINDING_API_KEY")
    or ""
).strip()

OPENROUTER_HTTP_REFERER = (os.getenv("OPENROUTER_HTTP_REFERER") or "").strip()
OPENROUTER_APP_TITLE = (os.getenv("OPENROUTER_APP_TITLE") or "MCP Telegram Assistant").strip()

OCR_MODEL = (os.getenv("OCR_MODEL") or os.getenv("OPENROUTER_OCR_MODEL") or "").strip()
OCR_LANGUAGES = (os.getenv("OCR_LANGUAGES") or "ru,en").strip()
ASR_MODEL = (os.getenv("ASR_MODEL") or os.getenv("OPENROUTER_ASR_MODEL") or "").strip()
ASR_LANGUAGE = (os.getenv("ASR_LANGUAGE") or "ru").strip() or None

TEXT_EXT = {".txt", ".text", ".log", ".md", ".csv", ".json", ".xml", ".html", ".htm", ".py", ".js", ".sh", ".yml", ".yaml"}
EXPORT_TO_PDF_EXT = {".docx", ".odt", ".md", ".markdown", ".html", ".htm", ".txt", ".text", ".log"}
IMAGE_EXT = {".jpg", ".jpeg", ".png", ".gif", ".bmp", ".webp", ".tiff", ".tif"}
AUDIO_EXT = {".ogg", ".mp3", ".wav", ".m4a", ".flac", ".opus", ".mpga"}


def ocr_enabled() -> bool:
    return bool(OCR_MODEL and OPENROUTER_API_KEY)


def asr_enabled() -> bool:
    return bool(ASR_MODEL and OPENROUTER_API_KEY)


def openrouter_headers() -> dict[str, str]:
    headers = {"Authorization": f"Bearer {OPENROUTER_API_KEY}"}
    if OPENROUTER_HTTP_REFERER:
        headers["HTTP-Referer"] = OPENROUTER_HTTP_REFERER
    if OPENROUTER_APP_TITLE:
        headers["X-Title"] = OPENROUTER_APP_TITLE
    return headers
