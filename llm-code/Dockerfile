FROM python:3.12-slim

WORKDIR /app
ENV PYTHONDONTWRITEBYTECODE=1
ENV PYTHONUNBUFFERED=1

# gptqmodel тянет pypcre и др. — нужен gcc для сборки нативных расширений.
RUN apt-get update && apt-get -y install --no-install-recommends \
    curl \
    build-essential \
    && rm -rf /var/lib/apt/lists/*

# gptqmodel при сборке wheel вызывает setup, которому нужен уже установленный torch.
# Сначала ставим PyTorch, затем остальное из requirements.txt.
# RTX 50-series (sm_120): nightly cu128 обычно содержит нужные CUDA kernels.
RUN pip install --no-cache-dir --pre --upgrade \
    torch torchvision torchaudio \
    --index-url https://download.pytorch.org/whl/nightly/cu128

COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

# AWQ в transformers: без gptqmodel не грузится Qwen3-14B-AWQ.
# sdist gptqmodel в изолированной сборке pip не видит torch — отключаем isolation.
RUN pip install --no-cache-dir --no-build-isolation "gptqmodel>=5.0"

COPY main.py .

EXPOSE 8005
CMD ["uvicorn", "main:app", "--host", "0.0.0.0", "--port", "8005"]
