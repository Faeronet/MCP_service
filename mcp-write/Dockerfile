# При таймауте к registry-1.docker.io соберите с зеркалом:
#   docker build --build-arg PYTHON_BASE=ваш-зеркало/python:3.12-slim -f mcp-write/Dockerfile .
ARG PYTHON_BASE=python:3.12-slim
FROM ${PYTHON_BASE}
WORKDIR /app
RUN pip install --no-cache-dir uv
COPY mcp-write/requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt
COPY mcp-write/ ./mcp-write/
WORKDIR /app/mcp-write
EXPOSE 8001
CMD ["uvicorn", "main:app", "--host", "0.0.0.0", "--port", "8001"]
