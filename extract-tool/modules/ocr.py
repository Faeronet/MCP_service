"""Загрузка и запуск OCR (PaddleOCR-VL, TroCR, EasyOCR)."""
from __future__ import annotations

import io
import logging

from . import config

log = logging.getLogger("extract-tool.ocr")
_ocr_reader = None


def _init_easyocr():
    import easyocr
    langs = [x.strip() for x in (config.OCR_LANGUAGES or config.OCR_MODEL or "ru,en").split(",") if x.strip()]
    if not langs:
        langs = ["ru", "en"]
    use_gpu = config.device() == "cuda"
    return ("easyocr", easyocr.Reader(langs, gpu=use_gpu))


def get_ocr():
    global _ocr_reader
    if _ocr_reader is None:
        if config.OCR_MODEL and ("paddleocr" in config.OCR_MODEL.lower() or "paddlepaddle" in config.OCR_MODEL.lower()):
            try:
                from transformers import AutoProcessor, AutoModelForImageTextToText
                import torch
                token = config.hf_token_or_none()
                dev = config.device()
                dtype = torch.bfloat16 if dev == "cuda" else torch.float32
                processor = AutoProcessor.from_pretrained(config.OCR_MODEL, token=token)
                model = AutoModelForImageTextToText.from_pretrained(
                    config.OCR_MODEL, token=token, torch_dtype=dtype
                ).to(dev).eval()
                _ocr_reader = ("paddleocr", (processor, model, dev))
                log.info("OCR reader loaded (PaddleOCR-VL) on %s: %s", dev, config.OCR_MODEL)
            except Exception as e:
                log.warning("PaddleOCR-VL init failed, using EasyOCR: %s", e)
                _ocr_reader = _init_easyocr()
                log.info("OCR reader loaded (easyocr fallback): %s", config.OCR_LANGUAGES or "ru,en")
        elif config.OCR_MODEL and ("trocr" in config.OCR_MODEL.lower() or "microsoft" in config.OCR_MODEL.lower()):
            try:
                from transformers import TrOCRProcessor, VisionEncoderDecoderModel
                import torch
                token = config.hf_token_or_none()
                processor = TrOCRProcessor.from_pretrained(config.OCR_MODEL, token=token)
                model = VisionEncoderDecoderModel.from_pretrained(config.OCR_MODEL, token=token)
                dev = config.device()
                model = model.to(dev)
                _ocr_reader = ("trocr", (processor, model, dev))
                log.info("OCR reader loaded (TroCR) on %s: %s", dev, config.OCR_MODEL)
            except Exception as e:
                log.warning("TroCR init failed, using EasyOCR: %s", e)
                _ocr_reader = _init_easyocr()
                log.info("OCR reader loaded (easyocr fallback): %s", config.OCR_LANGUAGES or "ru,en")
        else:
            try:
                _ocr_reader = _init_easyocr()
                log.info("OCR reader loaded (easyocr): %s", config.OCR_LANGUAGES or config.OCR_MODEL or "ru,en")
            except Exception as e:
                log.exception("OCR init failed: %s", e)
    return _ocr_reader


def run_ocr_on_image(data: bytes) -> str:
    """Изображение (bytes) → текст. Использует текущий get_ocr()."""
    reader = get_ocr()
    if reader is None:
        return ""
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
