"""Вызов реранкера при инжесте."""
import logging
import httpx

from . import config

log = logging.getLogger("mcp-write")


def call_rerank(query: str, documents: list[tuple[int, str]]) -> list[tuple[int, float]]:
    if not config.RERANK_BASE or not documents:
        return []
    url = f"{config.RERANK_BASE}/rerank" if not config.RERANK_BASE.endswith("/rerank") else config.RERANK_BASE
    body = {
        "query": (query or "")[:2000],
        "documents": [{"id": idx, "text": (text or "")[:4000]} for idx, text in documents],
        "model": config.RERANK_MODEL,
    }
    try:
        with httpx.Client(timeout=120.0) as client:
            r = client.post(url, json=body)
        if r.status_code != 200:
            log.warning("rerank ingest: HTTP %s %s", r.status_code, r.text[:300])
            return []
        data = r.json()
        out = data.get("data") or []
        return [(int(item["id"]), float(item.get("similarity", 0))) for item in out if "id" in item]
    except Exception as e:
        log.warning("rerank ingest failed: %s", e)
        return []
