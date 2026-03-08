"""Операции с Qdrant: создание коллекций, upsert, scroll, delete."""
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
