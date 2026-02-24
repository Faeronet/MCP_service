FROM python:3.12-slim
WORKDIR /app
RUN apt-get update && apt-get install -y --no-install-recommends \
    poppler-utils tesseract-ocr unzip \
    && rm -rf /var/lib/apt/lists/*
RUN pip install --no-cache-dir uv
COPY attachment-worker/requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt
COPY attachment-worker/ ./attachment-worker/
ENV PYTHONPATH=/app/attachment-worker
CMD ["python", "-m", "attachment_worker.main"]
