"""
Единый сервис OCR + ASR. POST /ocr и POST /asr: multipart file → JSON {"text": "..."}.
"""
import io
import logging
import os
from fastapi import FastAPI, File, UploadFile, HTTPException

log = logging.getLogger("ocr-asr")
logging.basicConfig(level=logging.INFO, format="%(levelname)s: %(message)s")

PORT = int(os.getenv("PORT", "8004"))
OCR_MODEL = (os.getenv("OCR_MODEL") or "microsoft/trocr-base-handwritten").strip()
ASR_MODEL = (os.getenv("ASR_MODEL") or "openai/whisper-large-v3").strip()
HF_TOKEN = (os.getenv("HF_TOKEN") or os.getenv("HUGGINGFACE_TOKEN") or "").strip()
if HF_TOKEN:
    os.environ["HUGGINGFACE_HUB_TOKEN"] = HF_TOKEN

app = FastAPI(title="OCR+ASR")

# Lazy-load models
_ocr_reader = None
_asr_model = None


def _hf_token_or_none():
    return HF_TOKEN if HF_TOKEN else None


def get_ocr():
    global _ocr_reader
    if _ocr_reader is None:
        try:
            if "trocr" in OCR_MODEL.lower() or "microsoft" in OCR_MODEL.lower():
                from transformers import TrOCRProcessor, VisionEncoderDecoderModel
                token = _hf_token_or_none()
                processor = TrOCRProcessor.from_pretrained(OCR_MODEL, token=token)
                model = VisionEncoderDecoderModel.from_pretrained(OCR_MODEL, token=token)
                _ocr_reader = ("trocr", (processor, model))
                log.info("OCR reader loaded (TroCR): %s", OCR_MODEL)
            else:
                import easyocr
                langs = [x.strip() for x in (OCR_MODEL or "en").split(",") if x.strip()]
                if not langs:
                    langs = ["en"]
                _ocr_reader = ("easyocr", easyocr.Reader(langs, gpu=False))
                log.info("OCR reader loaded (easyocr): %s", langs)
        except Exception as e:
            log.warning("OCR init failed: %s", e)
    return _ocr_reader


def get_asr():
    global _asr_model
    if _asr_model is None:
        try:
            from faster_whisper import WhisperModel
            _asr_model = WhisperModel(ASR_MODEL or "base", device="cpu", compute_type="int8")
            log.info("ASR model loaded: %s", ASR_MODEL)
        except Exception as e:
            log.warning("ASR init failed: %s", e)
    return _asr_model


@app.post("/ocr")
async def ocr(file: UploadFile = File(...)) -> dict:
    """Изображение → текст (TroCR или easyocr)."""
    data = await file.read()
    if not data:
        return {"text": ""}
    reader = get_ocr()
    if reader is None:
        raise HTTPException(status_code=503, detail="OCR not available")
    try:
        from PIL import Image
        import numpy as np
        img = Image.open(io.BytesIO(data)).convert("RGB")
        backend, ocr_engine = reader
        if backend == "trocr":
            processor, model = ocr_engine
            pixel_values = processor(images=img, return_tensors="pt").pixel_values
            generated_ids = model.generate(pixel_values)
            text = processor.batch_decode(generated_ids, skip_special_tokens=True)[0].strip()
        else:
            arr = np.array(img)
            results = ocr_engine.readtext(arr)
            text = " ".join([r[1] for r in results if len(r) > 1]).strip()
        return {"text": text or ""}
    except Exception as e:
        log.exception("OCR failed")
        raise HTTPException(status_code=400, detail=f"OCR failed: {e!s}")


@app.post("/asr")
async def asr(file: UploadFile = File(...)) -> dict:
    """Аудио файл → текст (faster-whisper)."""
    data = await file.read()
    if not data:
        return {"text": ""}
    model = get_asr()
    if model is None:
        raise HTTPException(status_code=503, detail="ASR not available")
    try:
        import tempfile
        with tempfile.NamedTemporaryFile(suffix=os.path.splitext(file.filename or "")[1] or ".wav", delete=False) as f:
            f.write(data)
            path = f.name
        try:
            segments, _ = model.transcribe(path, language=None)
            text = " ".join([s.text for s in segments if s.text]).strip()
            return {"text": text or ""}
        finally:
            try:
                os.unlink(path)
            except Exception:
                pass
    except Exception as e:
        log.exception("ASR failed")
        raise HTTPException(status_code=400, detail=f"ASR failed: {e!s}")


@app.get("/healthz")
def health():
    return {"status": "ok"}


if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="0.0.0.0", port=PORT)
