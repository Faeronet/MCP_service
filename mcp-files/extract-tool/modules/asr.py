"""ASR через OpenRouter (ASR_MODEL). Локальный Whisper не используется."""
from __future__ import annotations

import logging

from . import config
from . import openrouter_client

log = logging.getLogger("extract-tool.asr")

_EXT_MIME = {
    ".wav": "audio/wav",
    ".mp3": "audio/mpeg",
    ".ogg": "audio/ogg",
    ".opus": "audio/opus",
    ".m4a": "audio/mp4",
    ".flac": "audio/flac",
    ".mpga": "audio/mpeg",
}


def asr_available() -> bool:
    return config.asr_enabled()


def get_asr():
    if not config.asr_enabled():
        return None
    return ("openrouter", config.ASR_MODEL)


def run_asr_on_audio(data: bytes, suffix: str = ".wav") -> str:
    if not config.asr_enabled():
        log.info("ASR skipped: ASR_MODEL or OPENROUTER_API_KEY not set")
        return ""
    mime = _EXT_MIME.get(suffix.lower(), "audio/wav")
    log.info("Running ASR via OpenRouter model=%s", config.ASR_MODEL)
    return openrouter_client.transcribe_audio(data, mime=mime)
