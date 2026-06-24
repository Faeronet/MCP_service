"""OpenRouter OpenAI-compatible client for vision OCR and audio ASR."""
from __future__ import annotations

import base64
import logging

import httpx

from . import config

log = logging.getLogger("extract-tool.openrouter")


def _chat_completion(messages: list[dict], model: str, max_tokens: int = 4096) -> str:
    if not config.OPENROUTER_API_KEY or not model:
        return ""
    url = f"{config.OPENROUTER_API_BASE}/chat/completions"
    payload = {
        "model": model,
        "messages": messages,
        "max_tokens": max_tokens,
    }
    try:
        with httpx.Client(timeout=120.0) as client:
            r = client.post(url, json=payload, headers=config.openrouter_headers())
            if r.status_code != 200:
                log.warning("OpenRouter chat %s: %s", r.status_code, r.text[:500])
                return ""
            data = r.json()
            choices = data.get("choices") or []
            if not choices:
                return ""
            msg = choices[0].get("message") or {}
            content = msg.get("content") or ""
            if isinstance(content, list):
                parts = []
                for part in content:
                    if isinstance(part, dict) and part.get("type") == "text":
                        parts.append(part.get("text") or "")
                content = "\n".join(p for p in parts if p)
            return str(content).strip()
    except Exception as e:
        log.warning("OpenRouter request failed: %s", e)
        return ""


def ocr_image(data: bytes, mime: str = "image/jpeg") -> str:
    if not config.ocr_enabled():
        return ""
    b64 = base64.standard_b64encode(data).decode("ascii")
    langs = config.OCR_LANGUAGES or "ru,en"
    prompt = (
        f"Extract all visible text from this image. "
        f"Preserve line breaks where meaningful. Languages: {langs}. "
        f"Return only the extracted text, no commentary."
    )
    messages = [
        {
            "role": "user",
            "content": [
                {"type": "text", "text": prompt},
                {"type": "image_url", "image_url": {"url": f"data:{mime};base64,{b64}"}},
            ],
        }
    ]
    return _chat_completion(messages, config.OCR_MODEL, max_tokens=4096)


def transcribe_audio(data: bytes, mime: str = "audio/wav") -> str:
    if not config.asr_enabled():
        return ""
    b64 = base64.standard_b64encode(data).decode("ascii")
    lang_hint = config.ASR_LANGUAGE or "auto"
    prompt = f"Transcribe this audio accurately. Language hint: {lang_hint}. Return only the transcript."
    messages = [
        {
            "role": "user",
            "content": [
                {"type": "text", "text": prompt},
                {"type": "input_audio", "input_audio": {"data": b64, "format": mime.split("/")[-1] or "wav"}},
            ],
        }
    ]
    text = _chat_completion(messages, config.ASR_MODEL, max_tokens=4096)
    if text:
        return text
    # Fallback: some models accept file URL style
    messages_alt = [
        {
            "role": "user",
            "content": [
                {"type": "text", "text": prompt},
                {"type": "image_url", "image_url": {"url": f"data:{mime};base64,{b64}"}},
            ],
        }
    ]
    return _chat_completion(messages_alt, config.ASR_MODEL, max_tokens=4096)
