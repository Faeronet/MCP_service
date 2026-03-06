"""
MCP Write: ingestion tool. chunk/embed/rerank quality/build_links/upsert.
Deterministic chunk_id, edge_id; upsert-only to Qdrant.

Система A (INGESTION_SYSTEM=A или по умолчанию): текущий пайплайн — чанки по параграфам/LLM,
полный text в payload, name = первое слово. Для отката оставить A.

Система B (INGESTION_SYSTEM=B): в основную коллекцию chunks — только name, situacii_problemy, proyavlenie, gospodstvo.
Обитание — в коллекцию obitanie (все ангелы с одним обитанием в одном чанке: chunk_id, doc_id, obitanie, names).
Знак зодиака — в коллекцию znak_zodiaka (группировка по значению). Специфичность — в коллекцию specificnost (свой чанк на ангела).
После записи в Qdrant — полный текст документа в Postgres: core.document_context(chunk_id, context).
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
from qdrant_client.http.models import PointIdsList
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
COLLECTION_OBITANIE = "obitanie"
COLLECTION_ZNAK_ZODIAKA = "znak_zodiaka"
COLLECTION_SPECIFICNOST = "specificnost"
COLLECTION_KACHESTVA_ENERGII = "kachestva_energii"
COLLECTION_ISKAZHENIYA = "iskazheniya_energii"
COLLECTION_OTHER = "other"  # всё остальное (adept, pomogaet и т.д.)
VECTOR_SIZE = int(os.getenv("VECTOR_SIZE", os.getenv("EMBEDDING_DIMENSION", "1024")))

RERANK_BASE = (os.getenv("RERANK_BINDING_HOST") or os.getenv("RERANK_API_URL") or "http://rerank:8787/api/v1").strip().rstrip("/")
RERANK_MODEL = (os.getenv("RERANK_MODEL") or "BAAI/bge-reranker-v2-m3").strip()

LLM_MODEL = (os.getenv("LLM_MODEL") or "Qwen/Qwen3-0.6B").strip()
USE_LLM_CHUNKING = os.getenv("USE_LLM_CHUNKING", "").lower() in ("1", "true", "yes")
USE_LLM_RERANK_QUERY = os.getenv("USE_LLM_RERANK_QUERY", "").lower() in ("1", "true", "yes")

# A = текущий пайплайн (чанки + полный text). B = только ключи по меткам до первой точки.
_ingestion_sys = (os.getenv("INGESTION_SYSTEM") or "A").strip().upper()
INGESTION_SYSTEM: str = "B" if _ingestion_sys == "B" else "A"

# Система B: метки для извлечения ключей (всё после метки до первой точки «.»; не «..» не «;»).
# Если метки нет в документе — ключ не создаём (жёсткая проверка).
SYSTEM_B_LABELS: list[tuple[str, str | None]] = [
    ("name", None),  # ключ 1: первое слово документа
    ("obitanie", "Обитание:"),
    ("specificnost", "Специфичность:"),
    ("znak_zodiaka", "Знак зодиака:"),
    ("kachestva_energii", "Качества энергии ангела:"),
    ("iskazheniya_energii", "Искажения (недостаток либо переизбыток энергии ангела):"),
    ("situacii_problemy", "Ситуации и общие проблемы(влияние АНГЕЛА):"),
    ("proyavlenie", "Проявление:"),
    ("gospodstvo", "Господство:"),
    ("adept", "Адепт:"),
    ("pomogaet", "Помогает:"),
]

qdrant: Optional[QdrantClient] = None
minio_client: Optional[Minio] = None


def ensure_collection(name: str = COLLECTION) -> None:
    """Создать коллекцию, если её нет (после удаления пользователем)."""
    if qdrant is None:
        return
    try:
        qdrant.create_collection(
            name,
            vectors_config=VectorParams(size=VECTOR_SIZE, distance=Distance.COSINE),
        )
        log.info("created collection %s with vector_size=%s", name, VECTOR_SIZE)
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


def _extract_after_label_until_first_period(full_text: str, label: str) -> str | None:
    """Текст после метки до первой одиночной точки (не «..», не «;»). Метка без кавычек в тексте.
    Если метки нет — возвращаем None (жёсткая проверка отсутствия)."""
    if not label or not full_text or label not in full_text:
        return None
    idx = full_text.find(label)
    if idx == -1:
        return None
    after = full_text[idx + len(label) :].lstrip()
    for i, c in enumerate(after):
        if c == ".":
            if i + 1 < len(after) and after[i + 1] == ".":
                continue
            return after[:i].strip()
    return after.strip()


def _parse_system_b_keys(raw: str) -> dict[str, str]:
    """Система B: парсит документ по меткам. Только присутствующие ключи; строго до первой точки."""
    raw = (raw or "").strip()
    if not raw:
        return {}
    out: dict[str, str] = {}
    parts = raw.split()
    if parts:
        out["name"] = parts[0].strip()
    for key_name, label in SYSTEM_B_LABELS[1:]:
        if not label:
            continue
        if label not in raw:
            continue
        val = _extract_after_label_until_first_period(raw, label)
        if val is None or not val.strip():
            continue
        out[key_name] = val.strip()
    return out


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


def _qdrant_retrieve_point(collection: str, point_id: int) -> dict[str, Any] | None:
    """Получить точку по id. Возвращает payload или None."""
    url = f"{QDRANT_URL.strip('/')}/collections/{collection}/points/{point_id}"
    try:
        with httpx.Client(timeout=30.0) as client:
            r = client.get(url)
        if r.status_code != 200:
            return None
        data = r.json()
        result = data.get("result")
        if not result:
            return None
        return result.get("payload") or {}
    except Exception as e:
        log.warning("qdrant retrieve failed: %s", e)
        return None


def _qdrant_upsert(collection: str, point_id: int, vector: list[float], payload: dict[str, Any]) -> None:
    ensure_collection(collection)
    url = f"{QDRANT_URL.strip('/')}/collections/{collection}/points"
    body = {"points": [{"id": point_id, "vector": vector, "payload": payload}]}
    with httpx.Client(timeout=60.0) as client:
        r = client.put(url, json=body)
    if r.status_code == 404:
        ensure_collection(collection)
        with httpx.Client(timeout=60.0) as client:
            r = client.put(url, json=body)
    if r.status_code >= 400:
        raise HTTPException(status_code=502, detail=f"qdrant upsert {collection}: {r.text[:200]}")


def _save_document_context_postgres(chunk_id: str, doc_id: str, context: str) -> None:
    """Записать в core.document_context: chunk_id, doc_id и полный текст документа."""
    try:
        import psycopg2
        conn = psycopg2.connect(POSTGRES_DSN)
        try:
            with conn.cursor() as cur:
                cur.execute(
                    """
                    INSERT INTO core.document_context (chunk_id, doc_id, context)
                    VALUES (%s, %s, %s)
                    ON CONFLICT (chunk_id) DO UPDATE SET doc_id = EXCLUDED.doc_id, context = EXCLUDED.context
                    """,
                    (chunk_id, doc_id, context),
                )
            conn.commit()
        finally:
            conn.close()
    except Exception as e:
        log.warning("document_context insert failed: %s", e)


def _save_angel_name_postgres(chunk_id: str, doc_id: str, name: str) -> None:
    """Записать в core.angel_names: chunk_id, doc_id, name (список имён ангелов)."""
    if not name or not name.strip():
        return
    try:
        import psycopg2
        conn = psycopg2.connect(POSTGRES_DSN)
        try:
            with conn.cursor() as cur:
                cur.execute(
                    """
                    INSERT INTO core.angel_names (chunk_id, doc_id, name)
                    VALUES (%s, %s, %s)
                    ON CONFLICT (chunk_id) DO UPDATE SET doc_id = EXCLUDED.doc_id, name = EXCLUDED.name
                    """,
                    (chunk_id, doc_id, name.strip()),
                )
            conn.commit()
        finally:
            conn.close()
    except Exception as e:
        log.warning("angel_names insert failed: %s", e)


def _delete_document_from_postgres(doc_id: str) -> None:
    """При удалении документа: удалить контекст и имя ангела из Postgres."""
    try:
        import psycopg2
        conn = psycopg2.connect(POSTGRES_DSN)
        try:
            with conn.cursor() as cur:
                cur.execute("DELETE FROM core.document_context WHERE doc_id = %s", (doc_id,))
                cur.execute("DELETE FROM core.angel_names WHERE doc_id = %s", (doc_id,))
            conn.commit()
        finally:
            conn.close()
    except Exception as e:
        log.warning("delete document from postgres failed: %s", e)


def _ingest_document_system_b(req: IngestDocumentRequest, raw: str) -> dict[str, Any]:
    """
    Система B:
    - Основная коллекция chunks: только name, situacii_problemy, proyavlenie, gospodstvo.
    - Обитание: отдельная коллекция obitanie; все ангелы с одним обитанием в одном чанке (chunk_id, doc_id, obitanie, names).
    - Знак зодиака: отдельная коллекция znak_zodiaka; группировка по значению.
    - Специфичность: отдельная коллекция specificnost; у каждого ангела свой чанк (chunk_id, doc_id, name, specificnost).
    - После записи в Qdrant — полный текст документа в Postgres (core.document_context: chunk_id, context).
    """
    if qdrant is None or minio_client is None:
        _ingest_error(503, "service not ready")
    keys = _parse_system_b_keys(raw)
    if not keys:
        log.info("ingest_document system_b: no keys extracted for doc_id=%s", req.doc_id)
        return {"status": "ok", "chunks_upserted": 0, "doc_id": req.doc_id, "version_id": req.version_id}
    name = keys.get("name", "")
    # Основной чанк: только имя + Ситуации и общие проблемы, Проявление, Господство
    main_payload_keys = {"name", "situacii_problemy", "proyavlenie", "gospodstvo"}
    main_keys = {k: v for k, v in keys.items() if k in main_payload_keys and v}
    if not main_keys:
        log.info("ingest_document system_b: no main keys for doc_id=%s", req.doc_id)
        return {"status": "ok", "chunks_upserted": 0, "doc_id": req.doc_id, "version_id": req.version_id}
    main_normalized = " ".join(f"{k}={v}" for k, v in sorted(main_keys.items()))
    main_chunk_id = deterministic_chunk_id(req.doc_id, req.version_id, "sec_0", main_normalized)
    main_vector = embed_text(main_normalized)
    if len(main_vector) != VECTOR_SIZE:
        main_vector = (main_vector + [0.0] * VECTOR_SIZE)[:VECTOR_SIZE]
    main_payload: dict[str, Any] = {
        "chunk_id": main_chunk_id,
        "doc_id": req.doc_id,
        "version_id": req.version_id,
        "section_path": "sec_0",
        **main_keys,
    }
    ensure_collection(COLLECTION)
    _qdrant_upsert(COLLECTION, chunk_id_to_point_id(main_chunk_id), main_vector, main_payload)
    chunks_count = 1

    # Коллекция obitanie: один чанк на значение обитания; в связях — chunk_ids ангелов (main_chunk_id)
    obitanie_val = keys.get("obitanie", "").strip()
    if obitanie_val:
        ensure_collection(COLLECTION_OBITANIE)
        obitanie_chunk_id = deterministic_chunk_id("obitanie", obitanie_val, "group", obitanie_val)
        point_id = chunk_id_to_point_id(obitanie_chunk_id)
        existing = _qdrant_retrieve_point(COLLECTION_OBITANIE, point_id)
        if existing:
            names = list(existing.get("names") or [])
            doc_ids = list(existing.get("doc_ids") or [])
            chunk_ids_list = list(existing.get("chunk_ids") or [])
            if name and name not in names:
                names.append(name)
                doc_ids.append(req.doc_id)
                chunk_ids_list.append(main_chunk_id)
            payload_obitanie = {
                "chunk_id": obitanie_chunk_id,
                "obitanie": obitanie_val,
                "names": names,
                "doc_ids": doc_ids,
                "chunk_ids": chunk_ids_list,
            }
        else:
            payload_obitanie = {
                "chunk_id": obitanie_chunk_id,
                "obitanie": obitanie_val,
                "names": [name] if name else [],
                "doc_ids": [req.doc_id],
                "chunk_ids": [main_chunk_id] if name else [],
            }
        text_for_vec = obitanie_val + " " + " ".join(payload_obitanie["names"])
        vec_obitanie = embed_text(text_for_vec)
        if len(vec_obitanie) != VECTOR_SIZE:
            vec_obitanie = (vec_obitanie + [0.0] * VECTOR_SIZE)[:VECTOR_SIZE]
        _qdrant_upsert(COLLECTION_OBITANIE, point_id, vec_obitanie, payload_obitanie)
        chunks_count += 1

    # Коллекция znak_zodiaka: один чанк на значение знака; в связях — chunk_ids ангелов (main_chunk_id)
    znak_val = keys.get("znak_zodiaka", "").strip()
    if znak_val:
        ensure_collection(COLLECTION_ZNAK_ZODIAKA)
        znak_chunk_id = deterministic_chunk_id("znak_zodiaka", znak_val, "group", znak_val)
        point_id = chunk_id_to_point_id(znak_chunk_id)
        existing = _qdrant_retrieve_point(COLLECTION_ZNAK_ZODIAKA, point_id)
        if existing:
            names = list(existing.get("names") or [])
            doc_ids = list(existing.get("doc_ids") or [])
            chunk_ids_list = list(existing.get("chunk_ids") or [])
            if name and name not in names:
                names.append(name)
                doc_ids.append(req.doc_id)
                chunk_ids_list.append(main_chunk_id)
            payload_znak = {
                "chunk_id": znak_chunk_id,
                "znak_zodiaka": znak_val,
                "names": names,
                "doc_ids": doc_ids,
                "chunk_ids": chunk_ids_list,
            }
        else:
            payload_znak = {
                "chunk_id": znak_chunk_id,
                "znak_zodiaka": znak_val,
                "names": [name] if name else [],
                "doc_ids": [req.doc_id],
                "chunk_ids": [main_chunk_id] if name else [],
            }
        text_for_vec = znak_val + " " + " ".join(payload_znak["names"])
        vec_znak = embed_text(text_for_vec)
        if len(vec_znak) != VECTOR_SIZE:
            vec_znak = (vec_znak + [0.0] * VECTOR_SIZE)[:VECTOR_SIZE]
        _qdrant_upsert(COLLECTION_ZNAK_ZODIAKA, point_id, vec_znak, payload_znak)
        chunks_count += 1

    # Коллекции с одним чанком на ангела: везде один и тот же main_chunk_id (канонический id ангела)
    # specificnost
    specificnost_val = keys.get("specificnost", "").strip()
    if specificnost_val:
        ensure_collection(COLLECTION_SPECIFICNOST)
        point_id = chunk_id_to_point_id(main_chunk_id)
        payload_spec = {
            "chunk_id": main_chunk_id,
            "doc_id": req.doc_id,
            "name": name,
            "specificnost": specificnost_val,
        }
        text_for_vec = name + " " + specificnost_val
        vec_spec = embed_text(text_for_vec)
        if len(vec_spec) != VECTOR_SIZE:
            vec_spec = (vec_spec + [0.0] * VECTOR_SIZE)[:VECTOR_SIZE]
        _qdrant_upsert(COLLECTION_SPECIFICNOST, point_id, vec_spec, payload_spec)
        chunks_count += 1

    # kachestva_energii
    kachestva_val = keys.get("kachestva_energii", "").strip()
    if kachestva_val:
        ensure_collection(COLLECTION_KACHESTVA_ENERGII)
        point_id = chunk_id_to_point_id(main_chunk_id)
        payload_kach = {
            "chunk_id": main_chunk_id,
            "doc_id": req.doc_id,
            "name": name,
            "kachestva_energii": kachestva_val,
        }
        text_for_vec = name + " " + kachestva_val
        vec_kach = embed_text(text_for_vec)
        if len(vec_kach) != VECTOR_SIZE:
            vec_kach = (vec_kach + [0.0] * VECTOR_SIZE)[:VECTOR_SIZE]
        _qdrant_upsert(COLLECTION_KACHESTVA_ENERGII, point_id, vec_kach, payload_kach)
        chunks_count += 1

    # iskazheniya_energii
    iskazheniya_val = keys.get("iskazheniya_energii", "").strip()
    if iskazheniya_val:
        ensure_collection(COLLECTION_ISKAZHENIYA)
        point_id = chunk_id_to_point_id(main_chunk_id)
        payload_isk = {
            "chunk_id": main_chunk_id,
            "doc_id": req.doc_id,
            "name": name,
            "iskazheniya_energii": iskazheniya_val,
        }
        text_for_vec = name + " " + iskazheniya_val
        vec_isk = embed_text(text_for_vec)
        if len(vec_isk) != VECTOR_SIZE:
            vec_isk = (vec_isk + [0.0] * VECTOR_SIZE)[:VECTOR_SIZE]
        _qdrant_upsert(COLLECTION_ISKAZHENIYA, point_id, vec_isk, payload_isk)
        chunks_count += 1

    # other
    keys_used = main_payload_keys | {"obitanie", "znak_zodiaka", "specificnost", "kachestva_energii", "iskazheniya_energii"}
    other_keys = {k: v for k, v in keys.items() if k not in keys_used and v}
    if other_keys:
        ensure_collection(COLLECTION_OTHER)
        other_normalized = " ".join(f"{k}={v}" for k, v in sorted(other_keys.items()))
        point_id = chunk_id_to_point_id(main_chunk_id)
        payload_other: dict[str, Any] = {
            "chunk_id": main_chunk_id,
            "doc_id": req.doc_id,
            "name": name,
            **other_keys,
        }
        vec_other = embed_text(other_normalized)
        if len(vec_other) != VECTOR_SIZE:
            vec_other = (vec_other + [0.0] * VECTOR_SIZE)[:VECTOR_SIZE]
        _qdrant_upsert(COLLECTION_OTHER, point_id, vec_other, payload_other)
        chunks_count += 1

    # Postgres: контекст документа и имя ангела (список имён с chunk_id)
    _save_document_context_postgres(main_chunk_id, req.doc_id, raw)
    _save_angel_name_postgres(main_chunk_id, req.doc_id, name)

    log.info(
        "ingest_document system_b: doc_id=%s main_chunk_id=%s obitanie=%s znak=%s spec=%s kach=%s isk=%s other=%s",
        req.doc_id, main_chunk_id, bool(obitanie_val), bool(znak_val), bool(specificnost_val),
        bool(kachestva_val), bool(iskazheniya_val), bool(other_keys),
    )
    return {"status": "ok", "chunks_upserted": chunks_count, "doc_id": req.doc_id, "version_id": req.version_id}


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

    if INGESTION_SYSTEM == "B":
        return _ingest_document_system_b(req, raw)

    # ---------- Система A ----------
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
            "content": text,
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
            skip = ("prev_chunk_id", "next_chunk_id", "chunk_id", "doc_id", "version_id", "section_path", "content", "name", "rerank_position", "related_chunk_ids", "links")
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


def _remove_doc_id_from_grouped_collection(collection: str, doc_id: str) -> None:
    """В коллекциях obitanie/znak_zodiaka убрать doc_id из списков names, doc_ids, chunk_ids и перезаписать точку."""
    if qdrant is None:
        return
    try:
        offset = None
        while True:
            points, offset = qdrant.scroll(
                collection_name=collection,
                limit=100,
                offset=offset,
                with_payload=True,
                with_vectors=True,
            )
            for pt in points:
                payload = pt.payload or {}
                doc_ids_list = list(payload.get("doc_ids") or [])
                if doc_id not in doc_ids_list:
                    continue
                idx = doc_ids_list.index(doc_id)
                names = list(payload.get("names") or [])
                chunk_ids_list = list(payload.get("chunk_ids") or [])
                if idx < len(names):
                    names.pop(idx)
                doc_ids_list.pop(idx)
                if idx < len(chunk_ids_list):
                    chunk_ids_list.pop(idx)
                new_payload = {**payload, "names": names, "doc_ids": doc_ids_list, "chunk_ids": chunk_ids_list}
                if not names:
                    qdrant.delete(collection_name=collection, points_selector=PointIdsList(points=[pt.id]))
                else:
                    qdrant.upsert(
                        collection_name=collection,
                        points=[PointStruct(id=pt.id, vector=pt.vector or [], payload=new_payload)],
                    )
            if offset is None:
                break
    except Exception as e:
        log.warning("remove doc_id from %s failed: %s", collection, e)


@app.delete("/doc/{doc_id}")
def delete_document(doc_id: str) -> dict[str, Any]:
    """Удалить все чанки документа из Qdrant и Postgres (контекст + имя ангела)."""
    if qdrant is None:
        raise HTTPException(status_code=503, detail="service not ready")
    try:
        for coll in (COLLECTION, COLLECTION_SPECIFICNOST, COLLECTION_KACHESTVA_ENERGII, COLLECTION_ISKAZHENIYA, COLLECTION_OTHER):
            try:
                qdrant.delete(
                    collection_name=coll,
                    points_selector=FilterSelector(
                        filter=Filter(
                            must=[FieldCondition(key="doc_id", match=MatchValue(value=doc_id))],
                        )
                    ),
                )
            except UnexpectedResponse as e:
                if e.status_code != 404:
                    log.warning("delete %s for doc_id=%s: %s", coll, doc_id, e)
        _remove_doc_id_from_grouped_collection(COLLECTION_OBITANIE, doc_id)
        _remove_doc_id_from_grouped_collection(COLLECTION_ZNAK_ZODIAKA, doc_id)
    except UnexpectedResponse as e:
        if e.status_code == 404:
            pass
        else:
            raise
    _delete_document_from_postgres(doc_id)
    return {"status": "ok", "doc_id": doc_id}
