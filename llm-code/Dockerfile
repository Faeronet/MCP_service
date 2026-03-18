FROM python:3.12-slim

WORKDIR /app
ENV PYTHONDONTWRITEBYTECODE=1
ENV PYTHONUNBUFFERED=1

RUN apt-get update && apt-get install -y --no-install-recommends curl && rm -rf /var/lib/apt/lists/*

COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

# Use CUDA-enabled PyTorch so this service can run on GPU.
# If CUDA is not available at runtime, torch will fall back to CPU or error depending on driver setup.
RUN pip install --no-cache-dir torch==2.4.0 --index-url https://download.pytorch.org/whl/cu124

COPY main.py .

EXPOSE 8005
CMD ["uvicorn", "main:app", "--host", "0.0.0.0", "--port", "8005"]

