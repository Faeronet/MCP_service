"""Система A: чанки по параграфам/LLM, реранк, upsert в chunks."""
import logging
from typing import Any
import httpx
from qdrant_client.models import Filter, FieldCondition, MatchValue
from fastapi import HTTPException

from . import config
from . import state
from .models import IngestDocumentRequest
from . import ids
from . import embed
from . import rerank
from . import llm
from . import qdrant_ops
from . import extract

log = logging.getLogger("mcp-write")


def ingest_document_system_a(req: IngestDocumentRequest, raw: str) -> dict[str, Any]:
    if state.qdrant is None or state.minio_client is None:
        log.warning("ingest_document error 503: service not ready")
        raise HTTPException(status_code=503, detail="service not ready")

    first_word = ""
    parts = (raw or "").strip().split()
    if parts:
        first_word = parts[0]

    if config.USE_LLM_CHUNKING and config.VLLM_BASE and config.LLM_MODEL:
        chunks_text = llm.chunk_with_llm(raw)
        if not chunks_text:
            chunks_text = [p.strip() for p in raw.split("\n\n") if p.strip()][:50]
            log.info("ingest_document: LLM chunking returned empty, using simple split (%d chunks)", len(chunks_text))
        else:
            log.info("ingest_document: LLM chunking returned %d chunks", len(chunks_text))
    else:
        chunks_text = [p.strip() for p in raw.split("\n\n") if p.strip()][:50]
    if not chunks_text:
        return {"status": "ok", "chunks_upserted": 0, "doc_id": req.doc_id, "version_id": req.version_id}

    chunk_ids = [
        ids.deterministic_chunk_id(req.doc_id, req.version_id, f"sec_{i}", text)
        for i, text in enumerate(chunks_text)
    ]

    rerank_order: list[tuple[int, float]] = []
    if config.RERANK_BASE and chunks_text:
        if config.USE_LLM_RERANK_QUERY and config.VLLM_BASE and config.LLM_MODEL:
            query = llm.summarize_with_llm(raw)
            if not query:
                query = chunks_text[0][:2000] if chunks_text else ""
        else:
            query = chunks_text[0][:2000] if chunks_text else ""
        docs_for_rerank = [(i, chunks_text[i]) for i in range(len(chunks_text))]
        rerank_order = rerank.call_rerank(query, docs_for_rerank)
        if rerank_order:
            log.info("ingest_document: rerank returned %d items (links) for doc_id=%s", len(rerank_order), req.doc_id)

    rerank_position_by_index: dict[int, int] = {}
    for pos, (idx, _) in enumerate(rerank_order):
        rerank_position_by_index[idx] = pos
    links_by_index: dict[int, list[dict[str, Any]]] = {}
    for i in range(len(chunks_text)):
        links_by_index[i] = [
            {"chunk_id": chunk_ids[idx], "score": round(score, 4)}
            for (idx, score) in rerank_order if idx != i
        ][:5]

    points_payload: list[dict[str, Any]] = []
    for i, text in enumerate(chunks_text):
        chunk_id = chunk_ids[i]
        vector = embed.embed_text(text)
        if len(vector) != config.VECTOR_SIZE:
            vector = (vector + [0.0] * config.VECTOR_SIZE)[:config.VECTOR_SIZE]
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
        points_payload.append({"id": ids.chunk_id_to_point_id(chunk_id), "vector": vector, "payload": payload})

    log.info(
        "ingest_document: upserting %d points doc_id=%s",
        len(points_payload), req.doc_id,
    )

    qdrant_ops.ensure_collection()
    url = f"{config.QDRANT_URL.strip('/')}/collections/{config.COLLECTION}/points"
    body = {"points": points_payload}
    try:
        with httpx.Client(timeout=60.0) as client:
            r = client.put(url, json=body)
        if r.status_code == 404:
            qdrant_ops.ensure_collection()
            with httpx.Client(timeout=60.0) as client:
                r = client.put(url, json=body)
        if r.status_code >= 400:
            err = r.text
            log.warning("qdrant upsert HTTP %s: %s", r.status_code, err[:500])
            if "dimension" in err.lower() or "vector" in err.lower():
                raise HTTPException(status_code=400, detail=f"qdrant dimension mismatch: {err[:200]}. Delete collection chunks and retry.")
            raise HTTPException(status_code=502, detail=f"qdrant upsert failed: {err[:200]}")
    except HTTPException:
        raise
    except Exception as e:
        log.warning("qdrant upsert failed: %s", e)
        raise HTTPException(status_code=502, detail=f"qdrant upsert failed: {e!s}") from e

    try:
        points_read, _ = state.qdrant.scroll(
            collection_name=config.COLLECTION,
            scroll_filter=Filter(must=[FieldCondition(key="doc_id", match=MatchValue(value=req.doc_id))]),
            limit=2,
            with_payload=True,
            with_vectors=False,
        )
        for idx, pt in enumerate(points_read):
            keys = list(pt.payload.keys()) if pt.payload else []
            log.info("ingest_document: after upsert point[%d] payload keys=%s", idx, keys)
    except Exception as e:
        log.warning("ingest_document: verify scroll failed: %s", e)

    return {"status": "ok", "chunks_upserted": len(points_payload), "doc_id": req.doc_id, "version_id": req.version_id}
