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


def _preload_models() -> None:
    log.info("Preloading OCR model...")
    try:
        ocr.get_ocr()
        log.info("OCR model preloaded")
    except Exception as e:
        log.warning("OCR preload failed: %s", e)
    log.info("Preloading ASR model...")
    try:
        asr.get_asr()
        log.info("ASR model preloaded")
    except Exception as e:
        log.warning("ASR preload failed: %s", e)


@asynccontextmanager
async def lifespan(app: FastAPI):
    log.info("Startup: preloading OCR and ASR models in background...")
    await asyncio.to_thread(_preload_models)
    log.info("Startup: models ready")
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
        if ocr.get_ocr() is None:
            raise HTTPException(status_code=503, detail="OCR not available")
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
        if asr.get_asr() is None:
            raise HTTPException(status_code=503, detail="ASR not available")
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
        return {"status": "ok"}

    return app
