"""
MCP Write: ingestion tool. chunk/embed/rerank quality/build_links/upsert.
Deterministic chunk_id, edge_id; upsert-only to Qdrant.
"""
import logging
import os
import hashlib
import json
from contextlib import asynccontextmanager
from typing import Any, Optional

log = logging.getLogger("mcp-write")

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
VLLM_BASE = (os.getenv("VLLM_OPENAI_BASE") or "http://vllm:8000/v1").strip().rstrip("/")
EMBEDDING_BINDING_HOST = (os.getenv("EMBEDDING_BINDING_HOST") or "").strip().rstrip("/")
EMBED_API_URL = (os.getenv("EMBED_API_URL") or "").strip().rstrip("/")
EMBED_BASE = EMBEDDING_BINDING_HOST or EMBED_API_URL or VLLM_BASE
EMBEDDING_BINDING_API_KEY = (os.getenv("EMBEDDING_BINDING_API_KEY") or "").strip()
EMBEDDING_MODEL = (os.getenv("EMBEDDING_MODEL") or "BAAI/bge-m3").strip()
EMBEDDING_DIM = int(os.getenv("EMBEDDING_DIMENSION", os.getenv("EMBEDDING_DIM", "1024")))  # bge-m3 = 1024

COLLECTION = "chunks"
VECTOR_SIZE = int(os.getenv("VECTOR_SIZE", os.getenv("EMBEDDING_DIMENSION", "1024")))

qdrant: Optional[QdrantClient] = None
minio_client: Optional[Minio] = None


def ensure_collection() -> None:
    """Создать коллекцию chunks, если её нет (после удаления пользователем)."""
    if qdrant is None:
        return
    try:
        qdrant.create_collection(
            COLLECTION,
            vectors_config=VectorParams(size=VECTOR_SIZE, distance=Distance.COSINE),
        )
        log.info("created collection %s with vector_size=%s", COLLECTION, VECTOR_SIZE)
    except UnexpectedResponse as e:
        if e.status_code != 409:
            raise


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
    try:
        if not minio_client.bucket_exists(MINIO_BUCKET):
            minio_client.make_bucket(MINIO_BUCKET)
    except Exception as e:
        import logging
        logging.getLogger("uvicorn.error").warning("minio bucket check: %s", e)
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


def _embed_headers() -> dict:
    if EMBEDDING_BINDING_API_KEY:
        return {"Authorization": f"Bearer {EMBEDDING_BINDING_API_KEY}"}
    return {}


def _get_embed_model_id() -> str:
    """Имя модели с сервера (GET /v1/models), чтобы избежать 404 из-за несовпадения имени."""
    try:
        with httpx.Client(timeout=10.0) as client:
            r = client.get(f"{EMBED_BASE}/models", headers=_embed_headers())
            if r.status_code != 200:
                return EMBEDDING_MODEL
            data = r.json()
            models = data.get("data") or []
            if models and isinstance(models[0], dict) and models[0].get("id"):
                return models[0]["id"]
    except Exception:
        pass
    return EMBEDDING_MODEL


def _embed_via_vllm(texts: list[str]) -> list[list[float]]:
    """Вызов /v1/embeddings (EMBEDDING_BINDING_HOST / EMBED_API_URL / VLLM_OPENAI_BASE)."""
    if not texts or not EMBEDDING_MODEL:
        return []
    url = f"{EMBED_BASE}/embeddings"
    model_id = _get_embed_model_id()
    payload = {
        "model": model_id,
        "input": texts[0] if len(texts) == 1 else texts,
        "encoding_format": "float",
    }
    try:
        with httpx.Client(timeout=60.0) as client:
            r = client.post(url, json=payload, headers=_embed_headers())
            if r.status_code != 200:
                return []
            data = r.json()
            out = [item["embedding"] for item in data.get("data", [])]
            return out if out and isinstance(out[0][0], float) else [[float(x) for x in v] for v in out]
    except Exception:
        return []


def embed_text(text: str) -> list[float]:
    """Вектор эмбеддинга через vLLM; при ошибке — нулевой вектор той же размерности."""
    vecs = _embed_via_vllm([text])
    if vecs:
        return vecs[0]
    return [0.0] * VECTOR_SIZE


@app.get("/healthz")
def health():
    return {"status": "ok"}


def _ingest_error(code: int, detail: str, **kwargs: Any) -> None:
    log.warning("ingest_document error %s: %s %s", code, detail, kwargs)
    raise HTTPException(status_code=code, detail=detail)

@app.post("/mcp/ingest_document")
def ingest_document(req: IngestDocumentRequest) -> dict[str, Any]:
    """Ingest document: chunk, embed, build links, upsert to Qdrant. Idempotent via deterministic IDs."""
    if qdrant is None or minio_client is None:
        _ingest_error(503, "service not ready")

    # Parse file_uri: minio://bucket/key
    if not req.file_uri.startswith("minio://"):
        _ingest_error(400, "file_uri must be minio://bucket/key", file_uri=req.file_uri)
    parts = req.file_uri[8:].split("/", 1)
    bucket = parts[0]
    key = parts[1] if len(parts) > 1 else ""
    if not key:
        _ingest_error(400, "file_uri key is empty", file_uri=req.file_uri)

    try:
        obj = minio_client.get_object(bucket, key)
        data = obj.read()
        obj.close()
    except Exception as e:
        _ingest_error(400, f"minio get failed: {e!s}", bucket=bucket, key=key)

    try:
        raw = data.decode(errors="replace")
    except Exception as e:
        _ingest_error(400, f"decode failed: {e!s}")

    # Simple chunking: by paragraph
    chunks_text = [p.strip() for p in raw.split("\n\n") if p.strip()][:50]
    if not chunks_text:
        return {"status": "ok", "chunks_upserted": 0, "doc_id": req.doc_id, "version_id": req.version_id}

    # Сначала считаем chunk_id для всех чанков (нужно для связей prev/next)
    chunk_ids = [
        deterministic_chunk_id(req.doc_id, req.version_id, f"sec_{i}", text)
        for i, text in enumerate(chunks_text)
    ]
    points = []
    for i, text in enumerate(chunks_text):
        chunk_id = chunk_ids[i]
        vector = embed_text(text)
        if len(vector) != VECTOR_SIZE:
            vector = (vector + [0.0] * VECTOR_SIZE)[:VECTOR_SIZE]
        payload = {
            "chunk_id": chunk_id,
            "doc_id": req.doc_id,
            "version_id": req.version_id,
            "section_path": f"sec_{i}",
            "text": text,
            **(req.metadata or {}),
        }
        # Связи с соседними чанками (prev/next)
        if i > 0:
            payload["prev_chunk_id"] = chunk_ids[i - 1]
        if i < len(chunk_ids) - 1:
            payload["next_chunk_id"] = chunk_ids[i + 1]
        points.append(
            PointStruct(
                id=chunk_id,
                vector=vector,
                payload=payload,
            )
        )

    ensure_collection()
    try:
        qdrant.upsert(COLLECTION, points=points)
    except UnexpectedResponse as e:
        if e.status_code == 404:
            ensure_collection()
            try:
                qdrant.upsert(COLLECTION, points=points)
            except Exception as retry_err:
                log.warning("qdrant upsert retry after 404 failed: %s", retry_err)
                raise HTTPException(status_code=502, detail=f"qdrant upsert failed: {retry_err!s}") from retry_err
            return {"status": "ok", "chunks_upserted": len(points), "doc_id": req.doc_id, "version_id": req.version_id}
        err = f"{e!s}"
        if "dimension" in err.lower() or "vector" in err.lower() or "size" in err.lower():
            _ingest_error(
                400,
                f"qdrant upsert failed (vector dimension mismatch): {err}. "
                f"Delete the collection so it is recreated with VECTOR_SIZE={VECTOR_SIZE}: "
                "curl -X DELETE http://<qdrant>:6333/collections/chunks",
                doc_id=req.doc_id,
            )
        log.warning("qdrant upsert failed: %s", err)
        raise HTTPException(status_code=502, detail=f"qdrant upsert failed: {err}")
    except Exception as e:
        err = f"{e!s}"
        log.warning("qdrant upsert failed: %s", err)
        raise HTTPException(status_code=502, detail=f"qdrant upsert failed: {err}")
    return {"status": "ok", "chunks_upserted": len(points), "doc_id": req.doc_id, "version_id": req.version_id}


@app.get("/mcp/doc_ids")
def list_doc_ids_in_qdrant() -> dict[str, Any]:
    """Список doc_id, для которых есть хотя бы один чанк в Qdrant (для фильтра списка документов в админке)."""
    if qdrant is None:
        raise HTTPException(status_code=503, detail="service not ready")
    ensure_collection()
    seen: set[str] = set()
    offset = None
    try:
        while True:
            points, next_offset = qdrant.scroll(
                collection_name=COLLECTION,
                limit=500,
                offset=offset,
                with_payload=True,
                with_vectors=False,
            )
            for pt in points:
                if pt.payload and "doc_id" in pt.payload:
                    seen.add(str(pt.payload["doc_id"]))
            if next_offset is None:
                break
            offset = next_offset
    except UnexpectedResponse as e:
        if e.status_code == 404:
            return {"doc_ids": []}
        raise
    return {"doc_ids": list(seen)}


@app.delete("/doc/{doc_id}")
def delete_document(doc_id: str) -> dict[str, Any]:
    """Delete all chunks for the given doc_id from Qdrant (for admin doc removal)."""
    if qdrant is None:
        raise HTTPException(status_code=503, detail="service not ready")
    try:
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
    except UnexpectedResponse as e:
        if e.status_code == 404:
            return {"status": "ok", "doc_id": doc_id}
        raise
    return {"status": "ok", "doc_id": doc_id}
