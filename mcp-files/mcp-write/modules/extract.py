"""Извлечение текста из файла (PDF / plain)."""
import io
import logging

log = logging.getLogger("mcp-write")


def extract_text_from_file(data: bytes, key: str) -> str:
    is_pdf = key.lower().endswith(".pdf") or (len(data) >= 4 and data[:4] == b"%PDF")
    if is_pdf:
        try:
            from pypdf import PdfReader
            reader = PdfReader(io.BytesIO(data))
            parts = []
            for page in reader.pages:
                t = page.extract_text()
                if t:
                    parts.append(t)
            return "\n\n".join(parts) if parts else ""
        except Exception as e:
            log.warning("PDF extract failed: %s", e)
            return ""
    try:
        return data.decode(errors="replace")
    except Exception:
        return ""
