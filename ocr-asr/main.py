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
OCR_MODEL = (os.getenv("OCR_MODEL") or "PaddlePaddle/PaddleOCR-VL-1.5").strip()
OCR_LANGUAGES = (os.getenv("OCR_LANGUAGES") or "ru,en").strip()
ASR_MODEL = (os.getenv("ASR_MODEL") or "openai/whisper-large-v3").strip()
ASR_LANGUAGE = (os.getenv("ASR_LANGUAGE") or "ru").strip() or None
HF_TOKEN = (os.getenv("HF_TOKEN") or os.getenv("HUGGINGFACE_TOKEN") or "").strip()
if HF_TOKEN:
    os.environ["HUGGINGFACE_HUB_TOKEN"] = HF_TOKEN


def _device():
    """cuda на GPU с индексом 1 (NVIDIA_VISIBLE_DEVICES=1), иначе cpu."""
    if not os.getenv("NVIDIA_VISIBLE_DEVICES"):
        return "cpu"
    try:
        import torch
        return "cuda" if torch.cuda.is_available() else "cpu"
    except Exception:
        return "cpu"

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
            if "paddleocr" in OCR_MODEL.lower() or "paddlepaddle" in OCR_MODEL.lower():
                from transformers import AutoProcessor, AutoModelForImageTextToText
                import torch
                token = _hf_token_or_none()
                device = _device()
                dtype = torch.bfloat16 if device == "cuda" else torch.float32
                processor = AutoProcessor.from_pretrained(OCR_MODEL, token=token)
                model = AutoModelForImageTextToText.from_pretrained(
                    OCR_MODEL, token=token, torch_dtype=dtype
                ).to(device).eval()
                _ocr_reader = ("paddleocr", (processor, model, device))
                log.info("OCR reader loaded (PaddleOCR-VL) on %s: %s", device, OCR_MODEL)
            elif "trocr" in OCR_MODEL.lower() or "microsoft" in OCR_MODEL.lower():
                from transformers import TrOCRProcessor, VisionEncoderDecoderModel
                import torch
                token = _hf_token_or_none()
                processor = TrOCRProcessor.from_pretrained(OCR_MODEL, token=token)
                model = VisionEncoderDecoderModel.from_pretrained(OCR_MODEL, token=token)
                device = _device()
                model = model.to(device)
                _ocr_reader = ("trocr", (processor, model, device))
                log.info("OCR reader loaded (TroCR) on %s: %s", device, OCR_MODEL)
            else:
                import easyocr
                langs = [x.strip() for x in (OCR_LANGUAGES or OCR_MODEL or "ru,en").split(",") if x.strip()]
                if not langs:
                    langs = ["ru", "en"]
                use_gpu = _device() == "cuda"
                _ocr_reader = ("easyocr", easyocr.Reader(langs, gpu=use_gpu))
                log.info("OCR reader loaded (easyocr) gpu=%s: %s", use_gpu, langs)
        except Exception as e:
            log.warning("OCR init failed: %s", e)
    return _ocr_reader


def _asr_model_id():
    """Идентификатор модели для faster-whisper (CTranslate2 с HF). openai/whisper-* → Systran/faster-whisper-*."""
    m = (ASR_MODEL or "large-v3").strip()
    if m.startswith("openai/whisper-"):
        # faster-whisper качает CTranslate2 с HF; openai/whisper-* — формат transformers
        name = m.replace("openai/whisper-", "").strip()
        return f"Systran/faster-whisper-{name}" if name else "Systran/faster-whisper-large-v3"
    return m


def get_asr():
    global _asr_model
    if _asr_model is None:
        from faster_whisper import WhisperModel
        device = _device()
        compute_type = "float16" if device == "cuda" else "int8"
        model_id = _asr_model_id()
        # Загрузка с Hugging Face (HUGGINGFACE_HUB_TOKEN в окружении для gated/private)
        try:
            _asr_model = WhisperModel(model_id, device=device, compute_type=compute_type)
            log.info("ASR model loaded on %s (language=%s): %s", device, ASR_LANGUAGE, model_id)
        except Exception as e1:
            log.warning("ASR load %s failed: %s", model_id, e1)
            # Fallback: короткое имя (large-v3, base) — faster-whisper резолвит в HF
            short = "large-v3" if "large-v3" in (model_id or "") else (model_id.split("/")[-1] if "/" in (model_id or "") else model_id)
            if short != model_id:
                try:
                    _asr_model = WhisperModel(short, device=device, compute_type=compute_type)
                    log.info("ASR model loaded (fallback %s) on %s", short, device)
                except Exception as e2:
                    log.exception("ASR init failed (fallback %s): %s", short, e2)
            else:
                log.exception("ASR init failed: %s", e1)
    return _asr_model


@app.post("/ocr")
async def ocr(file: UploadFile = File(...)) -> dict:
    """Изображение → текст (PaddleOCR-VL, TroCR или easyocr)."""
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
        if backend == "paddleocr":
            processor, model, device = ocr_engine
            import torch
            messages = [
                {
                    "role": "user",
                    "content": [
                        {"type": "image", "image": img},
                        {"type": "text", "text": "OCR:"},
                    ],
                }
            ]
            inputs = processor.apply_chat_template(
                messages,
                add_generation_prompt=True,
                tokenize=True,
                return_dict=True,
                return_tensors="pt",
            )
            if hasattr(inputs, "to"):
                inputs = inputs.to(device)
            else:
                inputs = {k: v.to(device) if hasattr(v, "to") else v for k, v in inputs.items()}
            generated_ids = model.generate(**inputs, max_new_tokens=1024)
            input_len = inputs["input_ids"].shape[-1]
            generated_ids_trimmed = generated_ids[0][input_len:]
            text = processor.batch_decode(
                generated_ids_trimmed.unsqueeze(0),
                skip_special_tokens=True,
                clean_up_tokenization_spaces=False,
            )[0].strip()
        elif backend == "trocr":
            processor, model, device = ocr_engine
            import torch
            pixel_values = processor(images=img, return_tensors="pt").pixel_values.to(device)
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
            segments, _ = model.transcribe(path, language=ASR_LANGUAGE)
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
