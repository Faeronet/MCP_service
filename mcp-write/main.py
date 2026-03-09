"""
MCP Write: ingestion tool. chunk/embed/rerank quality/build_links/upsert.
Deterministic chunk_id, edge_id; upsert-only to Qdrant.

Система A (INGESTION_SYSTEM=A): чанки по параграфам/LLM, полный text в payload, name = первое слово.
Система B (INGESTION_SYSTEM=B): по меткам — chunks, obitanie, znak_zodiaka, specificnost, postgres.
"""
from contextlib import asynccontextmanager
from fastapi import FastAPI

from modules import config
from modules import state
from modules import handlers
from modules import qdrant_ops
from qdrant_client import QdrantClient
from qdrant_client.http.models import VectorParams, Distance
from qdrant_client.http.exceptions import UnexpectedResponse
from minio import Minio


@asynccontextmanager
async def lifespan(app: FastAPI):
    state.qdrant = QdrantClient(url=config.QDRANT_URL)
    state.minio_client = Minio(
        config.MINIO_ENDPOINT,
        access_key=config.MINIO_ACCESS,
        secret_key=config.MINIO_SECRET,
        secure=False,
    )
    try:
        if not state.minio_client.bucket_exists(config.MINIO_BUCKET):
            state.minio_client.make_bucket(config.MINIO_BUCKET)
    except Exception as e:
        import logging
        logging.getLogger("uvicorn.error").warning("minio bucket check: %s", e)
    try:
        state.qdrant.create_collection(
            config.COLLECTION,
            vectors_config=VectorParams(size=config.VECTOR_SIZE, distance=Distance.COSINE),
        )
    except UnexpectedResponse as e:
        if e.status_code != 409:
            raise
    if config.INGESTION_SYSTEM == "B":
        for coll in (
            config.COLLECTION_OBITANIE,
            config.COLLECTION_ZNAK_ZODIAKA,
            config.COLLECTION_SPECIFICNOST,
            config.COLLECTION_KACHESTVA_ENERGII,
            config.COLLECTION_ISKAZHENIYA,
            config.COLLECTION_OTHER,
        ):
            try:
                state.qdrant.create_collection(
                    coll,
                    vectors_config=VectorParams(size=config.VECTOR_SIZE, distance=Distance.COSINE),
                )
            except UnexpectedResponse as e:
                if e.status_code != 409:
                    raise
    qdrant_ops.ensure_all_payload_indexes()
    yield
    state.qdrant.close()


app = FastAPI(title="MCP Write", lifespan=lifespan)

app.get("/healthz")(handlers.health)
app.post("/mcp/ingest_document")(handlers.ingest_document)
app.get("/mcp/doc_ids")(handlers.list_doc_ids_in_qdrant)
app.delete("/doc/{doc_id}")(handlers.delete_document)
