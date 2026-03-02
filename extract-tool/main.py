"""
Сервис извлечения текста: OCR, ASR, текст, PDF, архивы (tar.gz, tar.xz, zip).
POST /extract — универсальный вход; POST /ocr, POST /asr — обратная совместимость.
"""
from __future__ import annotations

import asyncio
import base64
import io
import logging
import os
import tarfile
import zipfile
from contextlib import asynccontextmanager

from fastapi import FastAPI, File, UploadFile, HTTPException

log = logging.getLogger("extract-tool")
logging.basicConfig(level=logging.INFO, format="%(levelname)s: %(message)s")

PORT = int(os.getenv("PORT", "8004"))
# По умолчанию PaddleOCR-VL-1.5 с Hugging Face (HF_TOKEN для gated). При ошибке загрузки — fallback EasyOCR.
OCR_MODEL = (os.getenv("OCR_MODEL") or "PaddlePaddle/PaddleOCR-VL-1.5").strip()
OCR_LANGUAGES = (os.getenv("OCR_LANGUAGES") or "ru,en").strip()
ASR_MODEL = (os.getenv("ASR_MODEL") or "openai/whisper-large-v3").strip()
ASR_LANGUAGE = (os.getenv("ASR_LANGUAGE") or "ru").strip() or None
HF_TOKEN = (os.getenv("HF_TOKEN") or os.getenv("HUGGINGFACE_TOKEN") or "").strip()
if HF_TOKEN:
    os.environ["HUGGINGFACE_HUB_TOKEN"] = HF_TOKEN


def _device():
    if not os.getenv("NVIDIA_VISIBLE_DEVICES"):
        return "cpu"
    try:
        import torch
        return "cuda" if torch.cuda.is_available() else "cpu"
    except Exception:
        return "cpu"

_ocr_reader = None
_asr_model = None


def _preload_models() -> None:
    """Синхронная предзагрузка OCR и ASR в фоне (вызывается из пула потоков)."""
    log.info("Preloading OCR model...")
    try:
        get_ocr()
        log.info("OCR model preloaded")
    except Exception as e:
        log.warning("OCR preload failed: %s", e)
    log.info("Preloading ASR model...")
    try:
        get_asr()
        log.info("ASR model preloaded")
    except Exception as e:
        log.warning("ASR preload failed: %s", e)


@asynccontextmanager
async def lifespan(app: FastAPI):
    """При старте приложения предзагружаем модели в пуле потоков."""
    log.info("Startup: preloading OCR and ASR models in background...")
    await asyncio.to_thread(_preload_models)
    log.info("Startup: models ready")
    yield
    # shutdown: при необходимости можно выгрузить модели
    log.info("Shutdown: extract-tool")


app = FastAPI(title="extract-tool", lifespan=lifespan)


def _hf_token_or_none():
    return HF_TOKEN if HF_TOKEN else None


def _init_easyocr():
    """Инициализация EasyOCR (стабильно ставится, поддерживает ru,en)."""
    import easyocr
    langs = [x.strip() for x in (OCR_LANGUAGES or OCR_MODEL or "ru,en").split(",") if x.strip()]
    if not langs:
        langs = ["ru", "en"]
    use_gpu = _device() == "cuda"
    return ("easyocr", easyocr.Reader(langs, gpu=use_gpu))


def get_ocr():
    global _ocr_reader
    if _ocr_reader is None:
        # Сначала пробуем тяжёлые модели по OCR_MODEL, при ошибке — fallback на EasyOCR
        if OCR_MODEL and ("paddleocr" in OCR_MODEL.lower() or "paddlepaddle" in OCR_MODEL.lower()):
            try:
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
            except Exception as e:
                log.warning("PaddleOCR-VL init failed, using EasyOCR: %s", e)
                _ocr_reader = _init_easyocr()
                log.info("OCR reader loaded (easyocr fallback): %s", OCR_LANGUAGES or "ru,en")
        elif OCR_MODEL and ("trocr" in OCR_MODEL.lower() or "microsoft" in OCR_MODEL.lower()):
            try:
                from transformers import TrOCRProcessor, VisionEncoderDecoderModel
                import torch
                token = _hf_token_or_none()
                processor = TrOCRProcessor.from_pretrained(OCR_MODEL, token=token)
                model = VisionEncoderDecoderModel.from_pretrained(OCR_MODEL, token=token)
                device = _device()
                model = model.to(device)
                _ocr_reader = ("trocr", (processor, model, device))
                log.info("OCR reader loaded (TroCR) on %s: %s", device, OCR_MODEL)
            except Exception as e:
                log.warning("TroCR init failed, using EasyOCR: %s", e)
                _ocr_reader = _init_easyocr()
                log.info("OCR reader loaded (easyocr fallback): %s", OCR_LANGUAGES or "ru,en")
        else:
            try:
                _ocr_reader = _init_easyocr()
                log.info("OCR reader loaded (easyocr): %s", OCR_LANGUAGES or OCR_MODEL or "ru,en")
            except Exception as e:
                log.exception("OCR init failed: %s", e)
    return _ocr_reader


def _asr_model_id():
    m = (ASR_MODEL or "large-v3").strip()
    if m.startswith("openai/whisper-"):
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
        try:
            _asr_model = WhisperModel(model_id, device=device, compute_type=compute_type)
            log.info("ASR model loaded on %s (language=%s): %s", device, ASR_LANGUAGE, model_id)
        except Exception as e1:
            log.warning("ASR load %s failed: %s", model_id, e1)
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


TEXT_EXT = {".txt", ".text", ".log", ".md", ".csv", ".json", ".xml", ".html", ".htm", ".py", ".js", ".sh", ".yml", ".yaml"}
# Форматы, которые экспортируются в PDF: извлекаем текст и конвертируем в PDF
EXPORT_TO_PDF_EXT = {".docx", ".odt", ".md", ".markdown", ".html", ".htm", ".txt", ".text", ".log"}
IMAGE_EXT = {".jpg", ".jpeg", ".png", ".gif", ".bmp", ".webp", ".tiff", ".tif"}
AUDIO_EXT = {".ogg", ".mp3", ".wav", ".m4a", ".flac", ".opus", ".mpga"}


def _extract_archive(data: bytes, filename: str) -> list[tuple[str, bytes]] | None:
    fn = (filename or "").lower().strip()
    try:
        if fn.endswith(".zip"):
            out = []
            with zipfile.ZipFile(io.BytesIO(data), "r") as z:
                for name in z.namelist():
                    if z.getinfo(name).is_dir():
                        continue
                    try:
                        out.append((name, z.read(name)))
                    except Exception as e:
                        log.warning("zip read %s: %s", name, e)
            return out
        if fn.endswith(".tar.xz") or fn.endswith(".txz"):
            with tarfile.open(fileobj=io.BytesIO(data), mode="r:*") as t:
                return [(m.name, t.extractfile(m).read()) for m in t.getmembers() if m.isfile()]
        if fn.endswith(".tar.gz") or fn.endswith(".tgz") or fn.endswith(".tar"):
            with tarfile.open(fileobj=io.BytesIO(data), mode="r:*") as t:
                return [(m.name, t.extractfile(m).read()) for m in t.getmembers() if m.isfile()]
    except Exception as e:
        log.warning("archive extract failed: %s", e)
    return None


def _extract_text_from_text_file(data: bytes, filename: str) -> str:
    try:
        return data.decode("utf-8", errors="replace").strip()
    except Exception:
        try:
            return data.decode("cp1251", errors="replace").strip()
        except Exception:
            return ""


def _extract_text_from_pdf(data: bytes) -> str:
    try:
        from pypdf import PdfReader
        reader = PdfReader(io.BytesIO(data))
        parts = []
        for page in reader.pages:
            t = page.extract_text()
            if t:
                parts.append(t)
        return "\n\n".join(parts).strip() if parts else ""
    except Exception as e:
        log.warning("PDF extract failed: %s", e)
        return ""


def _extract_text_from_docx(data: bytes) -> str:
    """Извлечение текста из DOCX."""
    try:
        from docx import Document
        doc = Document(io.BytesIO(data))
        return "\n\n".join(p.text.strip() for p in doc.paragraphs if p.text.strip()).strip()
    except Exception as e:
        log.warning("DOCX extract failed: %s", e)
        return ""


def _extract_text_from_odt(data: bytes) -> str:
    """Извлечение текста из ODT."""
    try:
        from odf.opendocument import load
        from odf import text, teletype
        doc = load(io.BytesIO(data))
        parts = []
        for el in doc.getElementsByType(text.P) + doc.getElementsByType(text.H):
            t = teletype.extractText(el).strip()
            if t:
                parts.append(t)
        return "\n\n".join(parts).strip() if parts else ""
    except Exception as e:
        log.warning("ODT extract failed: %s", e)
        return ""


def _text_to_pdf_bytes(text: str) -> bytes:
    """Конвертация текста в PDF (reportlab). Длинные строки переносятся по 80 символов."""
    if not text or not text.strip():
        return b""
    try:
        from reportlab.pdfgen import canvas
        from reportlab.lib.pagesizes import A4
        from reportlab.lib.units import cm
        buf = io.BytesIO()
        c = canvas.Canvas(buf, pagesize=A4)
        width, height = A4
        margin = 1.5 * cm
        x, y = margin, height - margin
        line_height = 14
        max_chars = 80
        lines = text.replace("\r\n", "\n").replace("\r", "\n").split("\n")
        for line in lines:
            while line:
                chunk = line[:max_chars] if len(line) > max_chars else line
                if len(line) > max_chars:
                    line = line[max_chars:]
                else:
                    line = ""
                if y < margin:
                    c.showPage()
                    y = height - margin
                c.drawString(x, y, chunk)
                y -= line_height
        c.save()
        return buf.getvalue()
    except Exception as e:
        log.warning("text-to-PDF failed: %s", e)
        return b""


def _file_extension(name: str) -> str:
    name = (name or "").lower()
    if name.endswith(".tar.gz") or name.endswith(".tgz"):
        return ".tar.gz"
    if name.endswith(".tar.xz") or name.endswith(".txz"):
        return ".tar.xz"
    return os.path.splitext(name)[1]


def _extract_single_sync(data: bytes, filename: str) -> str | None:
    ext = _file_extension(filename)
    if ext == ".docx":
        return _extract_text_from_docx(data)
    if ext == ".odt":
        return _extract_text_from_odt(data)
    if ext in TEXT_EXT:
        return _extract_text_from_text_file(data, filename)
    if ext == ".pdf" or (len(data) >= 4 and data[:4] == b"%PDF"):
        return _extract_text_from_pdf(data)
    if ext in IMAGE_EXT:
        reader = get_ocr()
        if reader is None:
            log.warning("OCR reader not available, skipping image")
            return ""
        try:
            from PIL import Image
            import numpy as np
            img = Image.open(io.BytesIO(data)).convert("RGB")
            backend, ocr_engine = reader
            log.info("Running OCR backend=%s", backend)
            if backend == "paddleocr":
                processor, model, device = ocr_engine
                import torch
                messages = [{"role": "user", "content": [{"type": "image", "image": img}, {"type": "text", "text": "OCR:"}]}]
                inputs = processor.apply_chat_template(messages, add_generation_prompt=True, tokenize=True, return_dict=True, return_tensors="pt")
                if hasattr(inputs, "to"):
                    inputs = inputs.to(device)
                else:
                    inputs = {k: v.to(device) if hasattr(v, "to") else v for k, v in inputs.items()}
                generated_ids = model.generate(**inputs, max_new_tokens=1024)
                input_len = inputs["input_ids"].shape[-1]
                generated_ids_trimmed = generated_ids[0][input_len:]
                return processor.batch_decode(generated_ids_trimmed.unsqueeze(0), skip_special_tokens=True, clean_up_tokenization_spaces=False)[0].strip()
            elif backend == "trocr":
                processor, model, device = ocr_engine
                import torch
                pixel_values = processor(images=img, return_tensors="pt").pixel_values.to(device)
                generated_ids = model.generate(pixel_values)
                return processor.batch_decode(generated_ids, skip_special_tokens=True)[0].strip()
            else:
                arr = np.array(img)
                results = ocr_engine.readtext(arr)
                return " ".join([r[1] for r in results if len(r) > 1]).strip()
        except Exception as e:
            log.exception("OCR in extract failed: %s", e)
            return ""
    if ext in AUDIO_EXT:
        model = get_asr()
        if model is None:
            return ""
        try:
            import tempfile
            suf = ext or ".wav"
            with tempfile.NamedTemporaryFile(suffix=suf, delete=False) as f:
                f.write(data)
                path = f.name
            try:
                segments, _ = model.transcribe(path, language=ASR_LANGUAGE)
                return " ".join([s.text for s in segments if s.text]).strip()
            finally:
                try:
                    os.unlink(path)
                except Exception:
                    pass
        except Exception as e:
            log.warning("ASR in extract: %s", e)
            return ""
    return None


def _extract_impl(data: bytes, filename: str) -> dict:
    """Синхронная реализация извлечения (вызывается из пула потоков, чтобы не блокировать event loop)."""
    ext = _file_extension(filename)
    log.info("Extract request: filename=%s size=%d ext=%s", filename, len(data), ext)
    if not data:
        return {"text": ""}
    if ext in {".zip", ".tar", ".tar.gz", ".tar.xz", ".tgz", ".txz"}:
        items = _extract_archive(data, filename)
        if items is None:
            raise ValueError("Файл или архив нельзя обработать.")
        parts = []
        for name, content in items:
            item_ext = _file_extension(name)
            if item_ext == ".docx":
                parts.append(f"--- {name}\n" + _extract_text_from_docx(content))
            elif item_ext == ".odt":
                parts.append(f"--- {name}\n" + _extract_text_from_odt(content))
            elif item_ext in TEXT_EXT:
                parts.append(f"--- {name}\n" + _extract_text_from_text_file(content, name))
            elif item_ext == ".pdf" or (len(content) >= 4 and content[:4] == b"%PDF"):
                parts.append(f"--- {name}\n" + _extract_text_from_pdf(content))
            elif item_ext in IMAGE_EXT or item_ext in AUDIO_EXT:
                t = _extract_single_sync(content, name)
                if t is not None:
                    parts.append(f"--- {name}\n" + (t or ""))
        return {"text": "\n\n".join(p for p in parts if p.strip())}
    if ext in IMAGE_EXT:
        log.info("Processing as image (OCR): %s", filename)
    elif ext in AUDIO_EXT:
        log.info("Processing as audio (ASR): %s", filename)
    elif ext in TEXT_EXT or ext == ".pdf" or ext in EXPORT_TO_PDF_EXT:
        log.info("Processing as text/PDF/export: %s", filename)
    result = _extract_single_sync(data, filename)
    if result is not None:
        out = {"text": result}
        if ext in EXPORT_TO_PDF_EXT and result.strip():
            pdf_bytes = _text_to_pdf_bytes(result)
            if pdf_bytes:
                out["pdf_base64"] = base64.b64encode(pdf_bytes).decode("ascii")
        return out
    raise ValueError("Файл или архив нельзя обработать.")


@app.post("/extract")
async def extract(file: UploadFile = File(...)) -> dict:
    """Универсальное извлечение: архив → файлы; текст/PDF/фото/аудио → текст. 422 = нельзя обработать."""
    data = await file.read()
    filename = file.filename or ""
    try:
        return await asyncio.to_thread(_extract_impl, data, filename)
    except ValueError as e:
        raise HTTPException(status_code=422, detail=str(e))


def _ocr_impl(data: bytes) -> dict:
    """Синхронный OCR (в пуле потоков)."""
    if not data:
        return {"text": ""}
    reader = get_ocr()
    if reader is None:
        raise RuntimeError("OCR not available")
    from PIL import Image
    import numpy as np
    img = Image.open(io.BytesIO(data)).convert("RGB")
    backend, ocr_engine = reader
    if backend == "paddleocr":
        processor, model, device = ocr_engine
        import torch
        messages = [{"role": "user", "content": [{"type": "image", "image": img}, {"type": "text", "text": "OCR:"}]}]
        inputs = processor.apply_chat_template(messages, add_generation_prompt=True, tokenize=True, return_dict=True, return_tensors="pt")
        if hasattr(inputs, "to"):
            inputs = inputs.to(device)
        else:
            inputs = {k: v.to(device) if hasattr(v, "to") else v for k, v in inputs.items()}
        generated_ids = model.generate(**inputs, max_new_tokens=1024)
        input_len = inputs["input_ids"].shape[-1]
        generated_ids_trimmed = generated_ids[0][input_len:]
        text = processor.batch_decode(generated_ids_trimmed.unsqueeze(0), skip_special_tokens=True, clean_up_tokenization_spaces=False)[0].strip()
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


@app.post("/ocr")
async def ocr(file: UploadFile = File(...)) -> dict:
    data = await file.read()
    try:
        return await asyncio.to_thread(_ocr_impl, data)
    except RuntimeError as e:
        raise HTTPException(status_code=503, detail=str(e))
    except Exception as e:
        log.exception("OCR failed")
        raise HTTPException(status_code=400, detail=f"OCR failed: {e!s}")

 
def _asr_impl(data: bytes, filename: str) -> dict:
    """Синхронный ASR (в пуле потоков)."""
    if not data:
        return {"text": ""}
    model = get_asr()
    if model is None:
        raise RuntimeError("ASR not available")
    import tempfile
    suf = os.path.splitext(filename or "")[1] or ".wav"
    with tempfile.NamedTemporaryFile(suffix=suf, delete=False) as f:
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


@app.post("/asr")
async def asr(file: UploadFile = File(...)) -> dict:
    data = await file.read()
    filename = file.filename or ""
    try:
        return await asyncio.to_thread(_asr_impl, data, filename)
    except RuntimeError as e:
        raise HTTPException(status_code=503, detail=str(e))
    except Exception as e:
        log.exception("ASR failed")
        raise HTTPException(status_code=400, detail=f"ASR failed: {e!s}")


@app.get("/healthz")
def health():
    return {"status": "ok"}


if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="0.0.0.0", port=PORT)
