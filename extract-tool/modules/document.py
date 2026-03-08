"""Извлечение текста из PDF, DOCX, ODT, текстовых файлов; конвертация текста в PDF."""
from __future__ import annotations

import io
import logging

from . import config

log = logging.getLogger("extract-tool.document")


def extract_text_from_text_file(data: bytes, filename: str) -> str:
    try:
        return data.decode("utf-8", errors="replace").strip()
    except Exception:
        try:
            return data.decode("cp1251", errors="replace").strip()
        except Exception:
            return ""


def extract_text_from_pdf(data: bytes) -> str:
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


def extract_text_from_docx(data: bytes) -> str:
    try:
        from docx import Document
        doc = Document(io.BytesIO(data))
        return "\n\n".join(p.text.strip() for p in doc.paragraphs if p.text.strip()).strip()
    except Exception as e:
        log.warning("DOCX extract failed: %s", e)
        return ""


def extract_text_from_odt(data: bytes) -> str:
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


def text_to_pdf_bytes(text: str) -> bytes:
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
