"""Конфигурация из env и константы коллекций."""
import os

POSTGRES_DSN = os.getenv("POSTGRES_DSN", "postgres://postgres:postgres@postgres:5432/assistant")
QDRANT_URL = os.getenv("QDRANT_URL", "http://qdrant:6333")
MINIO_ENDPOINT = os.getenv("MINIO_ENDPOINT", "minio:9000")
MINIO_ACCESS = os.getenv("MINIO_ACCESS_KEY", "minioadmin")
MINIO_SECRET = os.getenv("MINIO_SECRET_KEY", "minioadmin")
MINIO_BUCKET = os.getenv("MINIO_BUCKET", "documents")
EMBEDDING_BINDING_HOST = (os.getenv("EMBEDDING_BINDING_HOST") or "").strip().rstrip("/")
EMBED_API_URL = (os.getenv("EMBED_API_URL") or "").strip().rstrip("/")
_vllm_embed_fallback = (
    os.getenv("LLM_BINDING_HOST") or os.getenv("VLLM_OPENAI_BASE") or "http://vllm:8000/v1"
).strip().rstrip("/")
EMBED_BASE = EMBEDDING_BINDING_HOST or EMBED_API_URL or _vllm_embed_fallback
EMBEDDING_BINDING_API_KEY = (os.getenv("EMBEDDING_BINDING_API_KEY") or "").strip()
EMBEDDING_MODEL = (os.getenv("EMBEDDING_MODEL") or "BAAI/bge-m3").strip()
EMBEDDING_DIM = int(os.getenv("EMBEDDING_DIMENSION", os.getenv("EMBEDDING_DIM", "1024")))

COLLECTION = "chunks"
COLLECTION_OBITANIE = "obitanie"
COLLECTION_ZNAK_ZODIAKA = "znak_zodiaka"
COLLECTION_SPECIFICNOST = "specificnost"
COLLECTION_KACHESTVA_ENERGII = "kachestva_energii"
COLLECTION_ISKAZHENIYA = "iskazheniya_energii"
COLLECTION_EMOCIONALNOE = "emocionalnoe"
COLLECTION_INTELLEKTUALNYE = "intellektualnye"
COLLECTION_ASTRALNYI_DUH = "astralnyi_duh"
COLLECTION_OTHER = "other"
VECTOR_SIZE = int(os.getenv("VECTOR_SIZE", os.getenv("EMBEDDING_DIMENSION", "1024")))

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
    ("emocionalnoe", "Эмоциональное:"),
    ("intellektualnye", "Интеллектуальные:"),
    ("astralnyi_duh", ["Астральный дух:", "Астральный дух: "]),
    ("adept", "Адепт:"),
    ("pomogaet", "Помогает:"),
    ("fizicheskoe", "Физическое:"),
]
