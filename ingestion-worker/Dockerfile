FROM python:3.12-slim
WORKDIR /app
RUN pip install --no-cache-dir uv
COPY ingestion-worker/requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt
COPY ingestion-worker/ ./ingestion-worker/
ENV PYTHONPATH=/app/ingestion-worker
CMD ["python", "-m", "ingestion_worker.main"]
