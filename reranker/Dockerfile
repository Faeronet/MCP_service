# Reranker (BAAI/bge-reranker-v2-m3) on GPU. PyTorch 2.4+ required by current transformers.
FROM pytorch/pytorch:2.4.0-cuda12.1-cudnn9-runtime AS base
WORKDIR /app
ENV PYTHONDONTWRITEBYTECODE=1
ENV PYTHONUNBUFFERED=1
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt
COPY main.py .
EXPOSE 8787
CMD ["uvicorn", "main:app", "--host", "0.0.0.0", "--port", "8787"]
