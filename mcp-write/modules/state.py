"""Глобальное состояние: клиенты Qdrant и MinIO, выставляются в lifespan."""
from typing import Any, Optional

qdrant: Any = None
minio_client: Any = None
