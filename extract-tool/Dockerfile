# extract-tool: OCR, ASR, текст, PDF, архивы. CPU-образ (без тяжёлого CUDA-образа).
# Для GPU замените базовый образ на pytorch/pytorch:2.4.0-cuda12.1-cudnn9-runtime и пересоберите.
FROM python:3.11-slim
RUN apt-get update && apt-get install -y --no-install-recommends ffmpeg libgl1 libglib2.0-0 && rm -rf /var/lib/apt/lists/*
WORKDIR /app
ENV PYTHONDONTWRITEBYTECODE=1
ENV PYTHONUNBUFFERED=1
COPY requirements.txt .
# Сначала ставим зависимости (могут подтянуть несовместимый torch/torchvision).
RUN pip install --no-cache-dir -r requirements.txt
# Затем фиксируем совместимую пару torch+torchvision из официального индекса (перезаписывает версии из requirements).
RUN pip install --no-cache-dir --force-reinstall torch==2.4.0 torchvision==0.19.0 --index-url https://download.pytorch.org/whl/cpu
COPY main.py .
EXPOSE 8004
CMD ["uvicorn", "main:app", "--host", "0.0.0.0", "--port", "8004"]
