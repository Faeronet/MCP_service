"""Pydantic-модели запросов."""
from typing import Optional
from pydantic import BaseModel


class IngestDocumentRequest(BaseModel):
    file_uri: str
    doc_id: str
    version_id: str
    file_hash: str
    metadata: Optional[dict] = None
