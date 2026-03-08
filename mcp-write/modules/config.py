"""Конфигурация из env и константы коллекций."""
import os

POSTGRES_DSN = os.getenv("POSTGRES_DSN", "postgres://postgres:postgres@postgres:5432/assistant")
QDRANT_URL = os.getenv("QDRANT_URL", "http://qdrant:6333")
MINIO_ENDPOINT = os.getenv("MINIO_ENDPOINT", "minio:9000")
MINIO_ACCESS = os.getenv("MINIO_ACCESS_KEY", "minioadmin")
MINIO_SECRET = os.getenv("MINIO_SECRET_KEY", "minioadmin")
MINIO_BUCKET = os.getenv("MINIO_BUCKET", "documents")
VLLM_BASE = (os.getenv("VLLM_OPENAI_BASE") or "http://vllm:8000/v1").strip().rstrip("/")
EMBEDDING_BINDING_HOST = (os.getenv("EMBEDDING_BINDING_HOST") or "").strip().rstrip("/")
EMBED_API_URL = (os.getenv("EMBED_API_URL") or "").strip().rstrip("/")
EMBED_BASE = EMBEDDING_BINDING_HOST or EMBED_API_URL or VLLM_BASE
EMBEDDING_BINDING_API_KEY = (os.getenv("EMBEDDING_BINDING_API_KEY") or "").strip()
EMBEDDING_MODEL = (os.getenv("EMBEDDING_MODEL") or "BAAI/bge-m3").strip()
EMBEDDING_DIM = int(os.getenv("EMBEDDING_DIMENSION", os.getenv("EMBEDDING_DIM", "1024")))

COLLECTION = "chunks"
COLLECTION_OBITANIE = "obitanie"
COLLECTION_ZNAK_ZODIAKA = "znak_zodiaka"
COLLECTION_SPECIFICNOST = "specificnost"
COLLECTION_KACHESTVA_ENERGII = "kachestva_energii"
COLLECTION_ISKAZHENIYA = "iskazheniya_energii"
COLLECTION_OTHER = "other"
VECTOR_SIZE = int(os.getenv("VECTOR_SIZE", os.getenv("EMBEDDING_DIMENSION", "1024")))

RERANK_BASE = (os.getenv("RERANK_BINDING_HOST") or os.getenv("RERANK_API_URL") or "http://rerank:8787/api/v1").strip().rstrip("/")
RERANK_MODEL = (os.getenv("RERANK_MODEL") or "BAAI/bge-reranker-v2-m3").strip()

LLM_MODEL = (os.getenv("LLM_MODEL") or "Qwen/Qwen3-0.6B").strip()
USE_LLM_CHUNKING = os.getenv("USE_LLM_CHUNKING", "").lower() in ("1", "true", "yes")
USE_LLM_RERANK_QUERY = os.getenv("USE_LLM_RERANK_QUERY", "").lower() in ("1", "true", "yes")

# По умолчанию B: контекст распределяется по коллекциям (chunks, obitanie, znak_zodiaka, specificnost и др.). A = всё в одну коллекцию chunks.
_ingestion_sys = (os.getenv("INGESTION_SYSTEM") or "B").strip().upper()
INGESTION_SYSTEM: str = "A" if _ingestion_sys == "A" else "B"

SYSTEM_B_LABELS: list[tuple[str, str | list[str] | None]] = [
    ("name", None),
    ("obitanie", "Обитание:"),
    # Спецификация и Специфичность — одно и то же, пишем всегда как specificnost
    ("specificnost", ["Специфичность:", "Спецификация:"]),
    ("znak_zodiaka", "Знак зодиака:"),
    ("kachestva_energii", "Качества энергии ангела:"),
    ("iskazheniya_energii", "Искажения (недостаток энергии ангела):"),
    ("situacii_problemy", "Ситуации и общие проблемы(влияние АНГЕЛА):"),
    ("proyavlenie", "Проявление:"),
    ("gospodstvo", "Господство:"),
    ("adept", "Адепт:"),
    ("pomogaet", "Помогает:"),
]
