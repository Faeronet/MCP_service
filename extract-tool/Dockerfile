# extract-tool: OCR, ASR на GPU с индексом 1 (NVIDIA_VISIBLE_DEVICES=1 в docker-compose).
FROM pytorch/pytorch:2.4.0-cuda12.1-cudnn9-runtime
RUN apt-get update && apt-get install -y --no-install-recommends ffmpeg libgl1 libglib2.0-0 && rm -rf /var/lib/apt/lists/*
WORKDIR /app
ENV PYTHONDONTWRITEBYTECODE=1
ENV PYTHONUNBUFFERED=1
COPY requirements.txt .
# Зависимости; затем фиксируем torch+torchvision под CUDA (чтобы pip не подменил на CPU).
RUN pip install --no-cache-dir -r requirements.txt \
  && pip install --no-cache-dir torch==2.4.0 torchvision==0.19.0 --index-url https://download.pytorch.org/whl/cu121
COPY main.py .
COPY modules/ ./modules/
EXPOSE 8004
CMD ["uvicorn", "main:app", "--host", "0.0.0.0", "--port", "8004"]
