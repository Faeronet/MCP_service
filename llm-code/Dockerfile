FROM python:3.12-slim

WORKDIR /app
ENV PYTHONDONTWRITEBYTECODE=1
ENV PYTHONUNBUFFERED=1

RUN apt-get update && apt-get install -y --no-install-recommends curl && rm -rf /var/lib/apt/lists/*

COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

# RTX 50-series / Blackwell uses compute capability sm_120.
# The default stable PyTorch wheels may not contain sm_120 kernels, causing:
# "CUDA error: no kernel image is available for execution on the device".
# Use a newer nightly CUDA wheel that includes sm_120.
RUN pip install --no-cache-dir --pre --upgrade \
  torch torchvision torchaudio \
  --index-url https://download.pytorch.org/whl/nightly/cu128

COPY main.py .

EXPOSE 8005
CMD ["uvicorn", "main:app", "--host", "0.0.0.0", "--port", "8005"]

