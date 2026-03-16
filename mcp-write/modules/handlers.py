"""HTTP-обработчики: health, ingest_document, doc_ids, delete_document."""
import logging
from typing import Any
from fastapi import HTTPException
from minio.error import S3Error
from qdrant_client.models import Filter, FieldCondition, FilterSelector, MatchValue
from qdrant_client.http.models import PointIdsList
from qdrant_client.http.exceptions import UnexpectedResponse

from . import config
from . import state
from .models import IngestDocumentRequest
from . import extract
from . import ids
from . import ingest_system_a
from . import ingest_system_b
from . import qdrant_ops
from . import postgres_ops

log = logging.getLogger("mcp-write")


def ingest_error(code: int, detail: str, **kwargs: Any) -> None:
    log.warning("ingest_document error %s: %s %s", code, detail, kwargs)
    raise HTTPException(status_code=code, detail=detail)


def health() -> dict[str, str]:
    return {"status": "ok"}


def ingest_document(req: IngestDocumentRequest) -> dict[str, Any]:
    if state.qdrant is None or state.minio_client is None:
        ingest_error(503, "service not ready")

    if not req.file_uri.startswith("minio://"):
        ingest_error(400, "file_uri must be minio://bucket/key", file_uri=req.file_uri)
    parts = req.file_uri[8:].split("/", 1)
    bucket = parts[0]
    key = parts[1] if len(parts) > 1 else ""
    if not key:
        ingest_error(400, "file_uri key is empty", file_uri=req.file_uri)

    try:
        obj = state.minio_client.get_object(bucket, key)
        data = obj.read()
        obj.close()
    except S3Error as e:
        if getattr(e, "code", None) == "NoSuchKey":
            ingest_error(404, "file_not_found", bucket=bucket, key=key)
        ingest_error(400, f"minio get failed: {e!s}", bucket=bucket, key=key)
    except Exception as e:
        ingest_error(400, f"minio get failed: {e!s}", bucket=bucket, key=key)

    raw = extract.extract_text_from_file(data, key)
    if not raw or not raw.strip():
        return {"status": "ok", "chunks_upserted": 0, "doc_id": req.doc_id, "version_id": req.version_id}

    if config.INGESTION_SYSTEM == "B":
        return ingest_system_b.ingest_document_system_b(req, raw)
    return ingest_system_a.ingest_document_system_a(req, raw)


def list_doc_ids_in_qdrant() -> dict[str, Any]:
    if state.qdrant is None:
        raise HTTPException(status_code=503, detail="service not ready")
    qdrant_ops.ensure_collection()
    seen: set[str] = set()
    offset = None
    try:
        while True:
            points, next_offset = state.qdrant.scroll(
                collection_name=config.COLLECTION,
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
            qdrant_ops.ensure_collection()
            return {"doc_ids": []}
        raise
    return {"doc_ids": list(seen)}


def delete_document(doc_id: str) -> dict[str, Any]:
    if state.qdrant is None:
        raise HTTPException(status_code=503, detail="service not ready")
    try:
        for coll in (config.COLLECTION, config.COLLECTION_SPECIFICNOST, config.COLLECTION_KACHESTVA_ENERGII, config.COLLECTION_ISKAZHENIYA, config.COLLECTION_OTHER):
            try:
                state.qdrant.delete(
                    collection_name=coll,
                    points_selector=FilterSelector(
                        filter=Filter(must=[FieldCondition(key="doc_id", match=MatchValue(value=doc_id))]),
                    ),
                )
            except UnexpectedResponse as e:
                if e.status_code != 404:
                    log.warning("delete %s for doc_id=%s: %s", coll, doc_id, e)
        for coll in (config.COLLECTION_EMOCIONALNOE, config.COLLECTION_INTELLEKTUALNYE, config.COLLECTION_ASTRALNYI_DUH):
            try:
                point_id = ids.point_id_for_doc_collection(doc_id, coll)
                state.qdrant.delete(collection_name=coll, points_selector=PointIdsList(points=[point_id]))
            except UnexpectedResponse as e:
                if e.status_code != 404:
                    log.warning("delete %s for doc_id=%s: %s", coll, doc_id, e)
        qdrant_ops.remove_doc_id_from_grouped_collection(config.COLLECTION_OBITANIE, doc_id)
        qdrant_ops.remove_doc_id_from_grouped_collection(config.COLLECTION_ZNAK_ZODIAKA, doc_id)
    except UnexpectedResponse as e:
        if e.status_code == 404:
            pass
        else:
            raise
    postgres_ops.delete_document_from_postgres(doc_id)
    return {"status": "ok", "doc_id": doc_id}
