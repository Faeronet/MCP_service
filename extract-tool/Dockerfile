# extract-tool: OCR, ASR, текст, PDF, архивы. CPU-образ (без тяжёлого CUDA-образа).
# Для GPU замените базовый образ на pytorch/pytorch:2.4.0-cuda12.1-cudnn9-runtime и пересоберите.
FROM python:3.11-slim
RUN apt-get update && apt-get install -y --no-install-recommends ffmpeg libgl1 libglib2.0-0 && rm -rf /var/lib/apt/lists/*
WORKDIR /app
ENV PYTHONDONTWRITEBYTECODE=1
ENV PYTHONUNBUFFERED=1
COPY requirements.txt .
# PyTorch + torchvision одной парой версий (иначе torchvision::nms / extension — несовместимость).
RUN pip install --no-cache-dir torch==2.4.0 torchvision==0.19.0 --index-url https://download.pytorch.org/whl/cpu \
  && pip install --no-cache-dir -r requirements.txt
COPY main.py .
EXPOSE 8004
CMD ["uvicorn", "main:app", "--host", "0.0.0.0", "--port", "8004"]
