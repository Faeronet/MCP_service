"""Система B: инжест по меткам — chunks, obitanie, znak_zodiaka, specificnost, postgres."""
import logging
from typing import Any
from fastapi import HTTPException

from . import config
from . import state
from .models import IngestDocumentRequest
from . import ids
from . import embed
from . import qdrant_ops
from . import postgres_ops
from . import system_b_parse

log = logging.getLogger("mcp-write")


def ingest_document_system_b(req: IngestDocumentRequest, raw: str) -> dict[str, Any]:
    if state.qdrant is None or state.minio_client is None:
        log.warning("ingest_document error 503: service not ready")
        raise HTTPException(status_code=503, detail="service not ready")
    keys = system_b_parse.parse_system_b_keys(raw)
    if not keys:
        log.info("ingest_document system_b: no keys extracted for doc_id=%s", req.doc_id)
        return {"status": "ok", "chunks_upserted": 0, "doc_id": req.doc_id, "version_id": req.version_id}
    name = keys.get("name", "")
    main_payload_keys = {"name", "situacii_problemy", "proyavlenie", "gospodstvo"}
    main_keys = {k: v for k, v in keys.items() if k in main_payload_keys and v}
    if not main_keys:
        log.info("ingest_document system_b: no main keys for doc_id=%s", req.doc_id)
        return {"status": "ok", "chunks_upserted": 0, "doc_id": req.doc_id, "version_id": req.version_id}
    main_normalized = " ".join(f"{k}={v}" for k, v in sorted(main_keys.items()))
    main_chunk_id = ids.deterministic_chunk_id(req.doc_id, req.version_id, "sec_0", main_normalized)
    main_vector = embed.embed_text(main_normalized)
    if len(main_vector) != config.VECTOR_SIZE:
        main_vector = (main_vector + [0.0] * config.VECTOR_SIZE)[:config.VECTOR_SIZE]
    main_payload: dict[str, Any] = {
        "chunk_id": main_chunk_id,
        "doc_id": req.doc_id,
        "version_id": req.version_id,
        "section_path": "sec_0",
        **main_keys,
    }
    qdrant_ops.ensure_collection(config.COLLECTION)
    qdrant_ops.qdrant_upsert(config.COLLECTION, ids.chunk_id_to_point_id(main_chunk_id), main_vector, main_payload)
    chunks_count = 1

    obitanie_val = keys.get("obitanie", "").strip()
    if obitanie_val:
        qdrant_ops.ensure_collection(config.COLLECTION_OBITANIE)
        obitanie_chunk_id = ids.deterministic_chunk_id("obitanie", obitanie_val, "group", obitanie_val)
        point_id = ids.chunk_id_to_point_id(obitanie_chunk_id)
        existing = qdrant_ops.qdrant_retrieve_point(config.COLLECTION_OBITANIE, point_id)
        if existing:
            names = list(existing.get("names") or [])
            doc_ids = list(existing.get("doc_ids") or [])
            chunk_ids_list = list(existing.get("chunk_ids") or [])
            if name and name not in names:
                names.append(name)
                doc_ids.append(req.doc_id)
                chunk_ids_list.append(main_chunk_id)
            payload_obitanie = {"chunk_id": obitanie_chunk_id, "obitanie": obitanie_val, "names": names, "doc_ids": doc_ids, "chunk_ids": chunk_ids_list}
        else:
            payload_obitanie = {"chunk_id": obitanie_chunk_id, "obitanie": obitanie_val, "names": [name] if name else [], "doc_ids": [req.doc_id], "chunk_ids": [main_chunk_id] if name else []}
        text_for_vec = obitanie_val + " " + " ".join(payload_obitanie["names"])
        vec_obitanie = embed.embed_text(text_for_vec)
        if len(vec_obitanie) != config.VECTOR_SIZE:
            vec_obitanie = (vec_obitanie + [0.0] * config.VECTOR_SIZE)[:config.VECTOR_SIZE]
        qdrant_ops.qdrant_upsert(config.COLLECTION_OBITANIE, point_id, vec_obitanie, payload_obitanie)
        chunks_count += 1

    znak_val = keys.get("znak_zodiaka", "").strip()
    if znak_val:
        qdrant_ops.ensure_collection(config.COLLECTION_ZNAK_ZODIAKA)
        znak_chunk_id = ids.deterministic_chunk_id("znak_zodiaka", znak_val, "group", znak_val)
        point_id = ids.chunk_id_to_point_id(znak_chunk_id)
        existing = qdrant_ops.qdrant_retrieve_point(config.COLLECTION_ZNAK_ZODIAKA, point_id)
        if existing:
            names = list(existing.get("names") or [])
            doc_ids = list(existing.get("doc_ids") or [])
            chunk_ids_list = list(existing.get("chunk_ids") or [])
            if name and name not in names:
                names.append(name)
                doc_ids.append(req.doc_id)
                chunk_ids_list.append(main_chunk_id)
            payload_znak = {"chunk_id": znak_chunk_id, "znak_zodiaka": znak_val, "names": names, "doc_ids": doc_ids, "chunk_ids": chunk_ids_list}
        else:
            payload_znak = {"chunk_id": znak_chunk_id, "znak_zodiaka": znak_val, "names": [name] if name else [], "doc_ids": [req.doc_id], "chunk_ids": [main_chunk_id] if name else []}
        text_for_vec = znak_val + " " + " ".join(payload_znak["names"])
        vec_znak = embed.embed_text(text_for_vec)
        if len(vec_znak) != config.VECTOR_SIZE:
            vec_znak = (vec_znak + [0.0] * config.VECTOR_SIZE)[:config.VECTOR_SIZE]
        qdrant_ops.qdrant_upsert(config.COLLECTION_ZNAK_ZODIAKA, point_id, vec_znak, payload_znak)
        chunks_count += 1

    specificnost_val = keys.get("specificnost", "").strip()
    if specificnost_val:
        qdrant_ops.ensure_collection(config.COLLECTION_SPECIFICNOST)
        point_id = ids.chunk_id_to_point_id(main_chunk_id)
        payload_spec = {"chunk_id": main_chunk_id, "doc_id": req.doc_id, "name": name, "specificnost": specificnost_val}
        vec_spec = embed.embed_text(name + " " + specificnost_val)
        if len(vec_spec) != config.VECTOR_SIZE:
            vec_spec = (vec_spec + [0.0] * config.VECTOR_SIZE)[:config.VECTOR_SIZE]
        qdrant_ops.qdrant_upsert(config.COLLECTION_SPECIFICNOST, point_id, vec_spec, payload_spec)
        chunks_count += 1

    kachestva_val = keys.get("kachestva_energii", "").strip()
    if kachestva_val:
        qdrant_ops.ensure_collection(config.COLLECTION_KACHESTVA_ENERGII)
        point_id = ids.chunk_id_to_point_id(main_chunk_id)
        payload_kach = {"chunk_id": main_chunk_id, "doc_id": req.doc_id, "name": name, "kachestva_energii": kachestva_val}
        vec_kach = embed.embed_text(name + " " + kachestva_val)
        if len(vec_kach) != config.VECTOR_SIZE:
            vec_kach = (vec_kach + [0.0] * config.VECTOR_SIZE)[:config.VECTOR_SIZE]
        qdrant_ops.qdrant_upsert(config.COLLECTION_KACHESTVA_ENERGII, point_id, vec_kach, payload_kach)
        chunks_count += 1

    iskazheniya_val = keys.get("iskazheniya_energii", "").strip()
    if iskazheniya_val:
        qdrant_ops.ensure_collection(config.COLLECTION_ISKAZHENIYA)
        point_id = ids.chunk_id_to_point_id(main_chunk_id)
        payload_isk = {"chunk_id": main_chunk_id, "doc_id": req.doc_id, "name": name, "iskazheniya_energii": iskazheniya_val}
        vec_isk = embed.embed_text(name + " " + iskazheniya_val)
        if len(vec_isk) != config.VECTOR_SIZE:
            vec_isk = (vec_isk + [0.0] * config.VECTOR_SIZE)[:config.VECTOR_SIZE]
        qdrant_ops.qdrant_upsert(config.COLLECTION_ISKAZHENIYA, point_id, vec_isk, payload_isk)
        chunks_count += 1

    rest_context = system_b_parse.get_rest_context(raw, keys)
    qdrant_ops.ensure_collection(config.COLLECTION_OTHER)
    point_id = ids.chunk_id_to_point_id(main_chunk_id)
    payload_other: dict[str, Any] = {"chunk_id": main_chunk_id, "doc_id": req.doc_id, "name": name, "context": rest_context}
    vec_other = embed.embed_text(rest_context) if rest_context.strip() else embed.embed_text(" ")
    if len(vec_other) != config.VECTOR_SIZE:
        vec_other = (vec_other + [0.0] * config.VECTOR_SIZE)[:config.VECTOR_SIZE]
    qdrant_ops.qdrant_upsert(config.COLLECTION_OTHER, point_id, vec_other, payload_other)
    chunks_count += 1

    postgres_ops.save_document_context_postgres(main_chunk_id, req.doc_id, raw)
    postgres_ops.save_angel_name_postgres(main_chunk_id, req.doc_id, name)

    log.info(
        "ingest_document system_b: doc_id=%s main_chunk_id=%s obitanie=%s znak=%s spec=%s kach=%s isk=%s other=1",
        req.doc_id, main_chunk_id, bool(obitanie_val), bool(znak_val), bool(specificnost_val), bool(kachestva_val), bool(iskazheniya_val),
    )
    return {"status": "ok", "chunks_upserted": chunks_count, "doc_id": req.doc_id, "version_id": req.version_id}
