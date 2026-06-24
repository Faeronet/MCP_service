"""FastAPI приложение и эндпоинты."""
from __future__ import annotations

import asyncio
import logging

from fastapi import FastAPI, File, UploadFile, HTTPException
from contextlib import asynccontextmanager

from . import config
from . import archives
from . import ocr
from . import asr
from . import extract

log = logging.getLogger("extract-tool")
logging.basicConfig(level=logging.INFO, format="%(levelname)s: %(message)s")


def _log_startup_capabilities() -> None:
    if config.ocr_enabled():
        log.info("OCR enabled: model=%s", config.OCR_MODEL)
    else:
        log.info("OCR disabled (set OCR_MODEL + OPENROUTER_API_KEY to enable)")
    if config.asr_enabled():
        log.info("ASR enabled: model=%s", config.ASR_MODEL)
    else:
        log.info("ASR disabled (set ASR_MODEL + OPENROUTER_API_KEY to enable)")


@asynccontextmanager
async def lifespan(app: FastAPI):
    await asyncio.to_thread(_log_startup_capabilities)
    yield
    log.info("Shutdown: extract-tool")


def create_app() -> FastAPI:
    app = FastAPI(title="extract-tool", lifespan=lifespan)

    @app.post("/extract")
    async def extract_endpoint(file: UploadFile = File(...)) -> dict:
        data = await file.read()
        filename = file.filename or ""
        try:
            return await asyncio.to_thread(extract.extract_impl, data, filename)
        except ValueError as e:
            raise HTTPException(status_code=422, detail=str(e))

    @app.post("/ocr")
    async def ocr_endpoint(file: UploadFile = File(...)) -> dict:
        data = await file.read()
        if not data:
            return {"text": ""}
        if not ocr.ocr_available():
            raise HTTPException(status_code=503, detail="OCR not configured (OCR_MODEL + OPENROUTER_API_KEY)")
        try:
            text = await asyncio.to_thread(ocr.run_ocr_on_image, data)
            return {"text": text or ""}
        except Exception as e:
            log.exception("OCR failed")
            raise HTTPException(status_code=400, detail=f"OCR failed: {e!s}")

    @app.post("/asr")
    async def asr_endpoint(file: UploadFile = File(...)) -> dict:
        data = await file.read()
        if not data:
            return {"text": ""}
        if not asr.asr_available():
            raise HTTPException(status_code=503, detail="ASR not configured (ASR_MODEL + OPENROUTER_API_KEY)")
        filename = file.filename or ""
        try:
            suffix = archives.file_extension(filename) or ".wav"
            text = await asyncio.to_thread(asr.run_asr_on_audio, data, suffix)
            return {"text": text or ""}
        except Exception as e:
            log.exception("ASR failed")
            raise HTTPException(status_code=400, detail=f"ASR failed: {e!s}")

    @app.get("/healthz")
    def health():
        return {
            "status": "ok",
            "ocr": ocr.ocr_available(),
            "asr": asr.asr_available(),
        }

    return app
