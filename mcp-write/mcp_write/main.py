"""
MCP Write: ingestion tool. chunk/embed/rerank quality/build_links/upsert.
Deterministic chunk_id, edge_id; upsert-only to Qdrant.
"""
import os
import hashlib
import json
from contextlib import asynccontextmanager
from typing import Any, Optional

import httpx
from fastapi import FastAPI, HTTPException
from pydantic import BaseModel
from qdrant_client import QdrantClient
from qdrant_client.models import Filter, FieldCondition, FilterSelector, MatchValue
from qdrant_client.http.models import PointStruct, VectorParams, Distance
from qdrant_client.http.exceptions import UnexpectedResponse
from minio import Minio

# Config from env
POSTGRES_DSN = os.getenv("POSTGRES_DSN", "postgres://postgres:postgres@postgres:5432/assistant")
QDRANT_URL = os.getenv("QDRANT_URL", "http://qdrant:6333")
MINIO_ENDPOINT = os.getenv("MINIO_ENDPOINT", "minio:9000")
MINIO_ACCESS = os.getenv("MINIO_ACCESS_KEY", "minioadmin")
MINIO_SECRET = os.getenv("MINIO_SECRET_KEY", "minioadmin")
MINIO_BUCKET = os.getenv("MINIO_BUCKET", "documents")
VLLM_BASE = os.getenv("VLLM_OPENAI_BASE", "http://vllm:8000/v1")

COLLECTION = "chunks"
VECTOR_SIZE = 384

qdrant: Optional[QdrantClient] = None
minio_client: Optional[Minio] = None


@asynccontextmanager
async def lifespan(app: FastAPI):
    global qdrant, minio_client
    qdrant = QdrantClient(url=QDRANT_URL)
    minio_client = Minio(
        MINIO_ENDPOINT,
        access_key=MINIO_ACCESS,
        secret_key=MINIO_SECRET,
        secure=False,
    )
    # Ensure collection exists (create_collection is idempotent: 409 = already exists)
    try:
        qdrant.create_collection(
            COLLECTION,
            vectors_config=VectorParams(size=VECTOR_SIZE, distance=Distance.COSINE),
        )
    except UnexpectedResponse as e:
        if e.status_code != 409:
            raise
    yield
    qdrant.close()


app = FastAPI(title="MCP Write", lifespan=lifespan)


class IngestDocumentRequest(BaseModel):
    file_uri: str
    doc_id: str
    version_id: str
    file_hash: str
    metadata: Optional[dict] = None


def deterministic_chunk_id(doc_id: str, version_id: str, section_path: str, normalized_text: str) -> str:
    h = hashlib.sha256(f"{doc_id}:{version_id}:{section_path}:{normalized_text}".encode()).hexdigest()
    return h[:32]


def deterministic_edge_id(from_id: str, to_id: str, relation: str) -> str:
    h = hashlib.sha256(f"{from_id}:{to_id}:{relation}".encode()).hexdigest()
    return h[:32]


def dummy_embed(text: str) -> list[float]:
    """Placeholder: return zero vector. Replace with vLLM embeddings."""
    return [0.0] * VECTOR_SIZE


@app.get("/healthz")
def health():
    return {"status": "ok"}


@app.post("/mcp/ingest_document")
def ingest_document(req: IngestDocumentRequest) -> dict[str, Any]:
    """Ingest document: chunk, embed, build links, upsert to Qdrant. Idempotent via deterministic IDs."""
    if qdrant is None or minio_client is None:
        raise HTTPException(status_code=503, detail="service not ready")

    # Parse file_uri: minio://bucket/key
    if not req.file_uri.startswith("minio://"):
        raise HTTPException(status_code=400, detail="file_uri must be minio://bucket/key")
    parts = req.file_uri[8:].split("/", 1)
    bucket = parts[0]
    key = parts[1] if len(parts) > 1 else ""

    # Placeholder: fetch object and split into chunks (in real impl use PyPDF/txt splitter)
    try:
        obj = minio_client.get_object(bucket, key)
        raw = obj.read().decode(errors="replace")
        obj.close()
    except Exception as e:
        raise HTTPException(status_code=400, detail=f"minio get failed: {e}")

    # Simple chunking: by paragraph
    chunks_text = [p.strip() for p in raw.split("\n\n") if p.strip()][:50]
    points = []
    for i, text in enumerate(chunks_text):
        section_path = f"sec_{i}"
        chunk_id = deterministic_chunk_id(req.doc_id, req.version_id, section_path, text)
        vector = dummy_embed(text)
        payload = {
            "chunk_id": chunk_id,
            "doc_id": req.doc_id,
            "version_id": req.version_id,
            "section_path": section_path,
            "text": text,
            **(req.metadata or {}),
        }
        points.append(
            PointStruct(
                id=chunk_id,
                vector=vector,
                payload=payload,
            )
        )

    # Upsert-only
    qdrant.upsert(COLLECTION, points=points)
    return {"status": "ok", "chunks_upserted": len(points), "doc_id": req.doc_id, "version_id": req.version_id}


@app.delete("/doc/{doc_id}")
def delete_document(doc_id: str) -> dict[str, Any]:
    """Delete all chunks for the given doc_id from Qdrant (for admin doc removal)."""
    if qdrant is None:
        raise HTTPException(status_code=503, detail="service not ready")
    qdrant.delete(
        collection_name=COLLECTION,
        points_selector=FilterSelector(
            filter=Filter(
                must=[
                    FieldCondition(key="doc_id", match=MatchValue(value=doc_id)),
                ],
            )
        ),
    )
    return {"status": "ok", "doc_id": doc_id}
