"""
MCP Write: ingestion tool. chunk/embed/rerank quality/build_links/upsert.
Deterministic chunk_id, edge_id; upsert-only to Qdrant.
"""
import io
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
from minio.error import S3Error

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

RERANK_BASE = (os.getenv("RERANK_BINDING_HOST") or os.getenv("RERANK_API_URL") or "http://rerank:8787/api/v1").strip().rstrip("/")
RERANK_MODEL = (os.getenv("RERANK_MODEL") or "BAAI/bge-reranker-v2-m3").strip()

LLM_MODEL = (os.getenv("LLM_MODEL") or "Qwen/Qwen3-0.6B").strip()
USE_LLM_CHUNKING = os.getenv("USE_LLM_CHUNKING", "").lower() in ("1", "true", "yes")
USE_LLM_RERANK_QUERY = os.getenv("USE_LLM_RERANK_QUERY", "").lower() in ("1", "true", "yes")

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


def chunk_id_to_point_id(chunk_id: str) -> int:
    """Стабильный целочисленный id точки для Qdrant (UUID/int требуются API)."""
    return int(hashlib.sha256(chunk_id.encode()).hexdigest()[:15], 16) & 0x7FFFFFFFFFFFFFFF


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


def _call_rerank(query: str, documents: list[tuple[int, str]]) -> list[tuple[int, float]]:
    """Вызов реранкера: query + список (index, text). Возвращает [(index, score), ...] по убыванию score."""
    if not RERANK_BASE or not documents:
        return []
    url = f"{RERANK_BASE}/rerank" if not RERANK_BASE.endswith("/rerank") else RERANK_BASE
    body = {
        "query": (query or "")[:2000],
        "documents": [{"id": idx, "text": (text or "")[:4000]} for idx, text in documents],
        "model": RERANK_MODEL,
    }
    try:
        with httpx.Client(timeout=120.0) as client:
            r = client.post(url, json=body)
        if r.status_code != 200:
            log.warning("rerank ingest: HTTP %s %s", r.status_code, r.text[:300])
            return []
        data = r.json()
        out = data.get("data") or []
        # data: [{id: index, similarity: score}, ...] уже отсортировано по similarity desc
        return [(int(item["id"]), float(item.get("similarity", 0))) for item in out if "id" in item]
    except Exception as e:
        log.warning("rerank ingest failed: %s", e)
        return []


def _call_llm(messages: list[dict[str, str]], max_tokens: int = 2048) -> str:
    """Вызов LLM (vLLM chat/completions). Возвращает content первого ответа или пустую строку при ошибке."""
    if not VLLM_BASE or not LLM_MODEL:
        return ""
    url = f"{VLLM_BASE}/chat/completions"
    body = {"model": LLM_MODEL, "messages": messages, "max_tokens": max_tokens}
    try:
        with httpx.Client(timeout=120.0) as client:
            r = client.post(url, json=body)
        if r.status_code != 200:
            log.warning("llm request: HTTP %s %s", r.status_code, r.text[:200])
            return ""
        data = r.json()
        choices = data.get("choices") or []
        if not choices:
            return ""
        return (choices[0].get("message") or {}).get("content") or ""
    except Exception as e:
        log.warning("llm request failed: %s", e)
        return ""


def _chunk_with_llm(raw: str) -> list[str]:
    """Чанкинг через LLM: один запрос с просьбой разбить текст на логические чанки (параграфы через двойной перенос)."""
    if not raw or not raw.strip():
        return []
    prompt = (
        "Разбей следующий текст на логические фрагменты (чанки). "
        "Выводи только сами фрагменты, разделяй их двойным переносом строки (две пустые строки между чанками). "
        "Не добавляй нумерацию и заголовки.\n\nТекст:\n\n"
    )
    text = (raw or "")[:30000]
    content = _call_llm([{"role": "user", "content": prompt + text}], max_tokens=4096)
    if not content or not content.strip():
        return []
    chunks = [p.strip() for p in content.split("\n\n") if p.strip()]
    return chunks[:50]


def _summarize_with_llm(raw: str) -> str:
    """Краткое содержание текста в одно предложение для использования как query реранка."""
    if not raw or not raw.strip():
        return ""
    text = (raw or "")[:8000]
    content = _call_llm([
        {"role": "user", "content": f"Одним предложением опиши основное содержание текста (для поиска по смыслу):\n\n{text}"}
    ], max_tokens=150)
    return (content or "").strip()[:500]


def embed_text(text: str) -> list[float]:
    """Вектор эмбеддинга через vLLM; при ошибке — нулевой вектор той же размерности."""
    vecs = _embed_via_vllm([text])
    if vecs:
        return vecs[0]
    return [0.0] * VECTOR_SIZE


@app.get("/healthz")
def health():
    return {"status": "ok"}


def _extract_text_from_file(data: bytes, key: str) -> str:
    """Извлекает текст из файла: PDF через pypdf, остальное — decode."""
    is_pdf = key.lower().endswith(".pdf") or (len(data) >= 4 and data[:4] == b"%PDF")
    if is_pdf:
        try:
            from pypdf import PdfReader
            reader = PdfReader(io.BytesIO(data))
            parts = []
            for page in reader.pages:
                t = page.extract_text()
                if t:
                    parts.append(t)
            return "\n\n".join(parts) if parts else ""
        except Exception as e:
            log.warning("PDF extract failed: %s", e)
            return ""
    try:
        return data.decode(errors="replace")
    except Exception:
        return ""


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
    except S3Error as e:
        if getattr(e, "code", None) == "NoSuchKey":
            _ingest_error(404, "file_not_found", bucket=bucket, key=key)
        _ingest_error(400, f"minio get failed: {e!s}", bucket=bucket, key=key)
    except Exception as e:
        _ingest_error(400, f"minio get failed: {e!s}", bucket=bucket, key=key)

    raw = _extract_text_from_file(data, key)
    if not raw or not raw.strip():
        return {"status": "ok", "chunks_upserted": 0, "doc_id": req.doc_id, "version_id": req.version_id}

    # Первое слово извлечённого текста — в ключ name у всех чанков этого документа
    first_word = ""
    parts = (raw or "").strip().split()
    if parts:
        first_word = parts[0]

    # Чанкинг: по желанию через LLM, иначе по параграфам
    if USE_LLM_CHUNKING and VLLM_BASE and LLM_MODEL:
        chunks_text = _chunk_with_llm(raw)
        if not chunks_text:
            chunks_text = [p.strip() for p in raw.split("\n\n") if p.strip()][:50]
            log.info("ingest_document: LLM chunking returned empty, using simple split (%d chunks)", len(chunks_text))
        else:
            log.info("ingest_document: LLM chunking returned %d chunks", len(chunks_text))
    else:
        chunks_text = [p.strip() for p in raw.split("\n\n") if p.strip()][:50]
    if not chunks_text:
        return {"status": "ok", "chunks_upserted": 0, "doc_id": req.doc_id, "version_id": req.version_id}

    # Сначала считаем chunk_id для всех чанков (нужно для связей prev/next)
    chunk_ids = [
        deterministic_chunk_id(req.doc_id, req.version_id, f"sec_{i}", text)
        for i, text in enumerate(chunks_text)
    ]

    # Вызов реранкера при инжесте: строим связи чанков (links) и пишем их в Qdrant
    rerank_order: list[tuple[int, float]] = []
    if RERANK_BASE and len(chunks_text) > 0:
        if USE_LLM_RERANK_QUERY and VLLM_BASE and LLM_MODEL:
            query = _summarize_with_llm(raw)
            if not query:
                query = chunks_text[0][:2000] if chunks_text else ""
        else:
            query = chunks_text[0][:2000] if chunks_text else ""
        docs_for_rerank = [(i, chunks_text[i]) for i in range(len(chunks_text))]
        rerank_order = _call_rerank(query, docs_for_rerank)
        if rerank_order:
            log.info("ingest_document: rerank returned %d items (links) for doc_id=%s", len(rerank_order), req.doc_id)
    # По rerank_order: позиция в реранке и связи (links) — список {chunk_id, score} для записи в Qdrant
    rerank_position_by_index: dict[int, int] = {}
    for pos, (idx, _) in enumerate(rerank_order):
        rerank_position_by_index[idx] = pos
    links_by_index: dict[int, list[dict[str, Any]]] = {}
    for i in range(len(chunks_text)):
        # Топ-5 связей по релевантности (chunk_id + score) для записи в payload
        links_by_index[i] = [
            {"chunk_id": chunk_ids[idx], "score": round(score, 4)}
            for (idx, score) in rerank_order if idx != i
        ][:5]

    # Собираем точки как сырые dict для прямого HTTP upsert (payload без искажений клиентом)
    points_payload: list[dict[str, Any]] = []
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
            "name": first_word,
        }
        if i > 0:
            payload["prev_chunk_id"] = chunk_ids[i - 1]
        if i < len(chunk_ids) - 1:
            payload["next_chunk_id"] = chunk_ids[i + 1]
        if i in rerank_position_by_index:
            payload["rerank_position"] = rerank_position_by_index[i]
        if links_by_index.get(i):
            payload["links"] = links_by_index[i]
            payload["related_chunk_ids"] = [ln["chunk_id"] for ln in links_by_index[i]]
        if req.metadata:
            skip = ("prev_chunk_id", "next_chunk_id", "chunk_id", "doc_id", "version_id", "section_path", "text", "name", "rerank_position", "related_chunk_ids", "links")
            for k, v in req.metadata.items():
                if k not in skip:
                    payload[k] = v
        points_payload.append({
            "id": chunk_id_to_point_id(chunk_id),
            "vector": vector,
            "payload": payload,
        })

    with_prev = sum(1 for i in range(len(points_payload)) if i > 0)
    with_next = sum(1 for i in range(len(points_payload)) if i < len(points_payload) - 1)
    first_keys = list(points_payload[0]["payload"].keys()) if points_payload else []
    log.info(
        "ingest_document: upserting %d points (prev=%d next=%d) doc_id=%s first_payload_keys=%s",
        len(points_payload), with_prev, with_next, req.doc_id, first_keys,
    )

    ensure_collection()
    # Upsert через HTTP API, чтобы payload уходил в Qdrant как есть
    url = f"{QDRANT_URL.strip('/')}/collections/{COLLECTION}/points"
    body = {"points": points_payload}
    try:
        with httpx.Client(timeout=60.0) as client:
            r = client.put(url, json=body)
        if r.status_code == 404:
            ensure_collection()
            with httpx.Client(timeout=60.0) as client:
                r = client.put(url, json=body)
        if r.status_code >= 400:
            err = r.text
            log.warning("qdrant upsert HTTP %s: %s", r.status_code, err[:500])
            if "dimension" in err.lower() or "vector" in err.lower():
                _ingest_error(400, f"qdrant dimension mismatch: {err[:200]}. Delete collection chunks and retry.", doc_id=req.doc_id)
            raise HTTPException(status_code=502, detail=f"qdrant upsert failed: {err[:200]}")
    except HTTPException:
        raise
    except Exception as e:
        log.warning("qdrant upsert failed: %s", e)
        raise HTTPException(status_code=502, detail=f"qdrant upsert failed: {e!s}") from e

    # Проверка: читаем обратно один-два чанка и логируем ключи payload (чтобы убедиться, что prev/next попали в Qdrant)
    try:
        points_read, _ = qdrant.scroll(
            collection_name=COLLECTION,
            scroll_filter=Filter(must=[FieldCondition(key="doc_id", match=MatchValue(value=req.doc_id))]),
            limit=2,
            with_payload=True,
            with_vectors=False,
        )
        for idx, pt in enumerate(points_read):
            keys = list(pt.payload.keys()) if pt.payload else []
            has_prev = "prev_chunk_id" in keys
            has_next = "next_chunk_id" in keys
            log.info(
                "ingest_document: after upsert point[%d] payload keys=%s has_prev=%s has_next=%s",
                idx, keys, has_prev, has_next,
            )
    except Exception as e:
        log.warning("ingest_document: verify scroll failed: %s", e)

    return {"status": "ok", "chunks_upserted": len(points_payload), "doc_id": req.doc_id, "version_id": req.version_id}


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
            ensure_collection()
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
