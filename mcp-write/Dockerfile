FROM python:3.12-slim
WORKDIR /app
RUN pip install --no-cache-dir uv
COPY mcp-write/requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt
COPY mcp-write/ ./mcp-write/
ENV PYTHONPATH=/app/mcp-write
EXPOSE 8001
CMD ["uvicorn", "mcp_write.main:app", "--host", "0.0.0.0", "--port", "8001"]
