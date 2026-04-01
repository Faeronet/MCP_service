"""Конфигурация и константы сервиса."""
from __future__ import annotations

import os

PORT = int(os.getenv("PORT", "8004"))
OCR_MODEL = (os.getenv("OCR_MODEL") or "PaddlePaddle/PaddleOCR-VL-1.5").strip()
OCR_LANGUAGES = (os.getenv("OCR_LANGUAGES") or "ru,en").strip()
ASR_MODEL = (os.getenv("ASR_MODEL") or "openai/whisper-large-v3").strip()
ASR_LANGUAGE = (os.getenv("ASR_LANGUAGE") or "ru").strip() or None
HF_TOKEN = (os.getenv("HF_TOKEN") or os.getenv("HUGGINGFACE_TOKEN") or "").strip()
if HF_TOKEN:
    os.environ["HUGGINGFACE_HUB_TOKEN"] = HF_TOKEN

TEXT_EXT = {".txt", ".text", ".log", ".md", ".csv", ".json", ".xml", ".html", ".htm", ".py", ".js", ".sh", ".yml", ".yaml"}
EXPORT_TO_PDF_EXT = {".docx", ".odt", ".md", ".markdown", ".html", ".htm", ".txt", ".text", ".log"}
IMAGE_EXT = {".jpg", ".jpeg", ".png", ".gif", ".bmp", ".webp", ".tiff", ".tif"}
AUDIO_EXT = {".ogg", ".mp3", ".wav", ".m4a", ".flac", ".opus", ".mpga"}


def device() -> str:
    """По умолчанию CPU (штатный Docker-образ без CUDA). CUDA только при явном EXTRACT_USE_CUDA=1 и доступном GPU."""
    flag = (os.getenv("EXTRACT_USE_CUDA") or "").strip().lower()
    if flag not in ("1", "true", "yes", "on"):
        return "cpu"
    try:
        import torch
        return "cuda" if torch.cuda.is_available() else "cpu"
    except Exception:
        return "cpu"


def hf_token_or_none():
    return HF_TOKEN if HF_TOKEN else None
