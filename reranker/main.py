"""
Reranker service: POST /api/v1/rerank with query + documents[{id, text}].
Returns {data: [{id, similarity}]} sorted by similarity desc.
Uses BAAI/bge-reranker-v2-m3 (or MODEL env).
"""
import os
import logging
from typing import List, Union
from fastapi import FastAPI
from pydantic import BaseModel
import torch
from transformers import AutoModelForSequenceClassification, AutoTokenizer

port = int(os.getenv("PORT", "8787"))
max_length = int(os.getenv("MAX_LENGTH", "512"))
model_name = os.getenv("MODEL", "BAAI/bge-reranker-v2-m3")
device = os.getenv("DEVICE", "cuda" if torch.cuda.is_available() else "cpu")

logging.basicConfig(level=logging.INFO, format="%(levelname)s: %(message)s")
logging.info("port: %d max_length: %d model: %s device: %s", port, max_length, model_name, device)

tokenizer = AutoTokenizer.from_pretrained(model_name)
model = AutoModelForSequenceClassification.from_pretrained(model_name)
model = model.to(device)
model.eval()

app = FastAPI(title="Reranker")


class Document(BaseModel):
    id: Union[int, str]
    text: str


class RequestData(BaseModel):
    query: str
    documents: List[Document]


@app.post("/api/v1/rerank")
async def rerank_documents(request: RequestData):
    if not request.documents:
        return {"data": []}
    pairs = [[request.query, doc.text] for doc in request.documents]
    with torch.no_grad():
        inputs = tokenizer(
            pairs,
            padding=True,
            truncation=True,
            return_tensors="pt",
            max_length=max_length,
        ).to(device)
        scores = model(**inputs, return_dict=True).logits.view(-1).float()
    result = [{"id": doc.id, "similarity": scores[i].item()} for i, doc in enumerate(request.documents)]
    result.sort(key=lambda x: x["similarity"], reverse=True)
    return {"data": result}


@app.get("/healthz")
def health():
    return {"status": "ok"}


if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="0.0.0.0", port=port)
