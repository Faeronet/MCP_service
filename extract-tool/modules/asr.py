"""Загрузка и запуск ASR (faster-whisper)."""
from __future__ import annotations

import os
import logging
import tempfile

from . import config

log = logging.getLogger("extract-tool.asr")
_asr_model = None


def _asr_model_id() -> str:
    m = (config.ASR_MODEL or "large-v3").strip()
    if m.startswith("openai/whisper-"):
        name = m.replace("openai/whisper-", "").strip()
        return f"Systran/faster-whisper-{name}" if name else "Systran/faster-whisper-large-v3"
    return m


def get_asr():
    global _asr_model
    if _asr_model is None:
        from faster_whisper import WhisperModel
        dev = config.device()
        compute_type = "float16" if dev == "cuda" else "int8"
        model_id = _asr_model_id()
        try:
            _asr_model = WhisperModel(model_id, device=dev, compute_type=compute_type)
            log.info("ASR model loaded on %s (language=%s): %s", dev, config.ASR_LANGUAGE, model_id)
        except Exception as e1:
            log.warning("ASR load %s failed: %s", model_id, e1)
            short = "large-v3" if "large-v3" in (model_id or "") else (model_id.split("/")[-1] if "/" in (model_id or "") else model_id)
            if short != model_id:
                try:
                    _asr_model = WhisperModel(short, device=dev, compute_type=compute_type)
                    log.info("ASR model loaded (fallback %s) on %s", short, dev)
                except Exception as e2:
                    log.exception("ASR init failed (fallback %s): %s", short, e2)
            else:
                log.exception("ASR init failed: %s", e1)
    return _asr_model


def run_asr_on_audio(data: bytes, suffix: str = ".wav") -> str:
    """Аудио (bytes) → текст. Временно пишет в файл для faster-whisper."""
    model = get_asr()
    if model is None:
        return ""
    with tempfile.NamedTemporaryFile(suffix=suffix, delete=False) as f:
        f.write(data)
        path = f.name
    try:
        segments, _ = model.transcribe(path, language=config.ASR_LANGUAGE)
        return " ".join([s.text for s in segments if s.text]).strip()
    finally:
        try:
            os.unlink(path)
        except Exception:
            pass
