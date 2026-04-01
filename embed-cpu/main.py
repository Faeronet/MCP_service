"""
Эмбеддинги на CPU: OpenAI-совместимый API /v1/embeddings.
Модель BAAI/bge-m3 (или EMBEDDING_MODEL), dimension 1024.
"""
import os
import logging
from typing import List, Union

from fastapi import FastAPI
from pydantic import BaseModel
from sentence_transformers import SentenceTransformer

logger = logging.getLogger(__name__)
logging.basicConfig(level=logging.INFO, format="%(levelname)s: %(message)s")

port = int(os.getenv("PORT", "8000"))
model_name = os.getenv("EMBEDDING_MODEL", "BAAI/bge-m3")

logger.info("Loading embedding model %s on CPU...", model_name)
model = SentenceTransformer(model_name, device="cpu")
logger.info("Model loaded. Dimension: %s", model.get_sentence_embedding_dimension())

app = FastAPI(title="Embeddings (CPU)")


class EmbeddingRequest(BaseModel):
    model: str
    input: Union[str, List[str]]
    encoding_format: str = "float"


@app.post("/v1/embeddings")
async def embeddings(req: EmbeddingRequest):
    texts = [req.input] if isinstance(req.input, str) else req.input
    if not texts:
        return {"data": []}
    vecs = model.encode(texts, convert_to_numpy=True, normalize_embeddings=False)
    if len(vecs.shape) == 1:
        vecs = vecs.reshape(1, -1)
    data = [{"embedding": row.tolist(), "index": i} for i, row in enumerate(vecs)]
    return {"data": data, "model": req.model, "usage": {"total_tokens": 0}}


@app.get("/v1/models")
async def list_models():
    return {
        "data": [
            {
                "id": model_name,
                "object": "model",
            }
        ]
    }


@app.get("/health")
def health():
    return {"status": "ok"}


if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="0.0.0.0", port=port)
