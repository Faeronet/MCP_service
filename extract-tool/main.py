"""
Сервис извлечения текста: OCR, ASR, текст, PDF, архивы (tar.gz, tar.xz, zip).
POST /extract — универсальный вход; POST /ocr, POST /asr — обратная совместимость.
"""
from modules.routes import create_app
from modules.config import PORT

app = create_app()

if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="0.0.0.0", port=PORT)
