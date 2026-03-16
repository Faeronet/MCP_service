"""Операции с Qdrant: создание коллекций, upsert, scroll, delete, индексация payload."""
import logging
import httpx
from qdrant_client import QdrantClient
from qdrant_client.models import Filter, FieldCondition, FilterSelector, MatchValue
from qdrant_client.http.models import PointIdsList, PointStruct, VectorParams, Distance
from qdrant_client.http.exceptions import UnexpectedResponse
from fastapi import HTTPException

from . import config
from . import state

log = logging.getLogger("mcp-write")

# Схема полнотекстового индекса (multilingual для лемматизации: яблоко ↔ яблоки)
_FULLTEXT_INDEX_SCHEMA = {
    "type": "text",
    "tokenizer": "multilingual",
    "min_token_len": 1,
    "max_token_len": 50,
    "lowercase": True,
}


def ensure_payload_index_text(collection_name: str, field_name: str) -> None:
    """Создаёт payload-индекс типа text (multilingual) для поля. Идемпотентно (409 = уже есть)."""
    if state.qdrant is None:
        return
    url = f"{config.QDRANT_URL.strip('/')}/collections/{collection_name}/index"
    body = {"field_name": field_name, "field_schema": _FULLTEXT_INDEX_SCHEMA}
    try:
        with httpx.Client(timeout=30.0) as client:
            r = client.put(url, json=body)
        if r.status_code in (200, 201, 409):
            if r.status_code != 409:
                log.info("payload index created: collection=%s field=%s", collection_name, field_name)
            return
        log.warning("payload index %s.%s: HTTP %s %s", collection_name, field_name, r.status_code, r.text[:200])
    except Exception as e:
        log.warning("payload index %s.%s failed: %s", collection_name, field_name, e)


def ensure_all_payload_indexes() -> None:
    """Создаёт полнотекстовые индексы для всех полей payload, по которым нужен поиск (System B)."""
    # chunks: content + отдельные поля main-чанка (name, situacii_problemy, proyavlenie, gospodstvo)
    ensure_payload_index_text(config.COLLECTION, "content")
    ensure_payload_index_text(config.COLLECTION, "name")
    ensure_payload_index_text(config.COLLECTION, "situacii_problemy")
    ensure_payload_index_text(config.COLLECTION, "proyavlenie")
    ensure_payload_index_text(config.COLLECTION, "gospodstvo")
    if config.INGESTION_SYSTEM == "B":
        # obitanie: obitanie + names_text (список имён одной строкой для поиска)
        ensure_payload_index_text(config.COLLECTION_OBITANIE, "obitanie")
        ensure_payload_index_text(config.COLLECTION_OBITANIE, "names_text")
        # znak_zodiaka: znak_zodiaka + names_text
        ensure_payload_index_text(config.COLLECTION_ZNAK_ZODIAKA, "znak_zodiaka")
        ensure_payload_index_text(config.COLLECTION_ZNAK_ZODIAKA, "names_text")
        # specificnost, kachestva_energii, iskazheniya_energii: основное поле + name
        ensure_payload_index_text(config.COLLECTION_SPECIFICNOST, "specificnost")
        ensure_payload_index_text(config.COLLECTION_SPECIFICNOST, "name")
        ensure_payload_index_text(config.COLLECTION_KACHESTVA_ENERGII, "kachestva_energii")
        ensure_payload_index_text(config.COLLECTION_KACHESTVA_ENERGII, "name")
        ensure_payload_index_text(config.COLLECTION_ISKAZHENIYA, "iskazheniya_energii")
        ensure_payload_index_text(config.COLLECTION_ISKAZHENIYA, "name")
        # emocionalnoe, intellektualnye, astralnyi_duh: основное поле + name
        ensure_payload_index_text(config.COLLECTION_EMOCIONALNOE, "emocionalnoe")
        ensure_payload_index_text(config.COLLECTION_EMOCIONALNOE, "name")
        ensure_payload_index_text(config.COLLECTION_INTELLEKTUALNYE, "intellektualnye")
        ensure_payload_index_text(config.COLLECTION_INTELLEKTUALNYE, "name")
        ensure_payload_index_text(config.COLLECTION_ASTRALNYI_DUH, "astralnyi_duh")
        ensure_payload_index_text(config.COLLECTION_ASTRALNYI_DUH, "name")
        # other: context + name
        ensure_payload_index_text(config.COLLECTION_OTHER, "context")
        ensure_payload_index_text(config.COLLECTION_OTHER, "name")


def ensure_collection(name: str | None = None) -> None:
    if name is None:
        name = config.COLLECTION
    if state.qdrant is None:
        return
    try:
        state.qdrant.create_collection(
            name,
            vectors_config=VectorParams(size=config.VECTOR_SIZE, distance=Distance.COSINE),
        )
        log.info("created collection %s with vector_size=%s", name, config.VECTOR_SIZE)
    except UnexpectedResponse as e:
        if e.status_code != 409:
            raise


def qdrant_retrieve_point(collection: str, point_id: int) -> dict | None:
    url = f"{config.QDRANT_URL.strip('/')}/collections/{collection}/points/{point_id}"
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


def qdrant_upsert(collection: str, point_id: int, vector: list[float], payload: dict) -> None:
    ensure_collection(collection)
    url = f"{config.QDRANT_URL.strip('/')}/collections/{collection}/points"
    body = {"points": [{"id": point_id, "vector": vector, "payload": payload}]}
    with httpx.Client(timeout=60.0) as client:
        r = client.put(url, json=body)
    if r.status_code == 404:
        ensure_collection(collection)
        with httpx.Client(timeout=60.0) as client:
            r = client.put(url, json=body)
    if r.status_code >= 400:
        raise HTTPException(status_code=502, detail=f"qdrant upsert {collection}: {r.text[:200]}")


def remove_doc_id_from_grouped_collection(collection: str, doc_id: str) -> None:
    if state.qdrant is None:
        return
    try:
        offset = None
        while True:
            points, offset = state.qdrant.scroll(
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
                    state.qdrant.delete(collection_name=collection, points_selector=PointIdsList(points=[pt.id]))
                else:
                    state.qdrant.upsert(
                        collection_name=collection,
                        points=[PointStruct(id=pt.id, vector=pt.vector or [], payload=new_payload)],
                    )
            if offset is None:
                break
    except Exception as e:
        log.warning("remove doc_id from %s failed: %s", collection, e)
