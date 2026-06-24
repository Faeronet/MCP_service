"""Эмбеддинги через OpenRouter (OpenAI-compatible /embeddings)."""
import httpx

from . import config


def _embed_headers() -> dict:
    headers: dict[str, str] = {}
    if config.EMBEDDING_BINDING_API_KEY:
        headers["Authorization"] = f"Bearer {config.EMBEDDING_BINDING_API_KEY}"
    if "openrouter.ai" in config.EMBED_BASE.lower():
        headers["X-Title"] = config.OPENROUTER_APP_TITLE
        if config.OPENROUTER_HTTP_REFERER:
            headers["HTTP-Referer"] = config.OPENROUTER_HTTP_REFERER
    return headers


def _get_embed_model_id() -> str:
    if not config.EMBEDDING_MODEL:
        return ""
    if "openrouter.ai" in config.EMBED_BASE.lower():
        return config.EMBEDDING_MODEL
    try:
        with httpx.Client(timeout=10.0) as client:
            r = client.get(f"{config.EMBED_BASE}/models", headers=_embed_headers())
            if r.status_code != 200:
                return config.EMBEDDING_MODEL
            data = r.json()
            models = data.get("data") or []
            if models and isinstance(models[0], dict) and models[0].get("id"):
                return models[0]["id"]
    except Exception:
        pass
    return config.EMBEDDING_MODEL


def _embed_via_api(texts: list[str]) -> list[list[float]]:
    if not texts or not config.embedding_enabled():
        return []
    url = f"{config.EMBED_BASE}/embeddings"
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
    if not config.embedding_enabled():
        return []
    vecs = _embed_via_api([text])
    if vecs:
        return vecs[0]
    return []
