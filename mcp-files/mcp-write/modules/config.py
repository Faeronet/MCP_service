"""Конфигурация из env и константы коллекций."""
import os

POSTGRES_DSN = os.getenv("POSTGRES_DSN", "postgres://postgres:postgres@postgres:5432/assistant")
QDRANT_URL = os.getenv("QDRANT_URL", "http://qdrant:6333")
MINIO_ENDPOINT = os.getenv("MINIO_ENDPOINT", "minio:9000")
MINIO_ACCESS = os.getenv("MINIO_ACCESS_KEY", "minioadmin")
MINIO_SECRET = os.getenv("MINIO_SECRET_KEY", "minioadmin")
MINIO_BUCKET = os.getenv("MINIO_BUCKET", "documents")

OPENROUTER_API_BASE = (
    os.getenv("OPENROUTER_API_BASE")
    or os.getenv("OPENROUTER_BASE_URL")
    or os.getenv("EMBEDDING_BINDING_HOST")
    or os.getenv("EMBED_API_URL")
    or os.getenv("LLM_BINDING_HOST")
    or os.getenv("VLLM_OPENAI_BASE")
    or "https://openrouter.ai/api/v1"
).strip().rstrip("/")

OPENROUTER_API_KEY = (
    os.getenv("OPENROUTER_API_KEY")
    or os.getenv("EMBEDDING_BINDING_API_KEY")
    or os.getenv("LLM_BINDING_API_KEY")
    or ""
).strip()
OPENROUTER_HTTP_REFERER = (os.getenv("OPENROUTER_HTTP_REFERER") or "").strip()
OPENROUTER_APP_TITLE = (os.getenv("OPENROUTER_APP_TITLE") or "MCP Telegram Assistant").strip()

EMBED_BASE = OPENROUTER_API_BASE
EMBEDDING_BINDING_API_KEY = OPENROUTER_API_KEY
EMBEDDING_MODEL = (os.getenv("EMBEDDING_MODEL") or os.getenv("OPENROUTER_EMBED_MODEL") or "").strip()
EMBEDDING_DIM = int(os.getenv("EMBEDDING_DIMENSION", os.getenv("EMBEDDING_DIM", "1024")))


def embedding_enabled() -> bool:
    return bool(EMBEDDING_MODEL)


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

# Физические даты: нейтр./жен. метки и варианты (пробел/NBSP перед «:», регистр) — один ключ fizicheskoe.
_FIZICHESKOE_LABELS: list[str] = [
    "Физическое:",
    "Физическое :",
    "Физическое\u00a0:",
    "Физическая:",
    "Физическая :",
    "Физическая\u00a0:",
    "Физические даты:",
    "Физические даты :",
    "Физические даты\u00a0:",
    "Физическая дата:",
    "Физическая дата :",
    "Физическая дата\u00a0:",
    "физическое:",
    "физическое :",
    "физическое\u00a0:",
    "физическая:",
    "физическая :",
    "физическая\u00a0:",
    "физические даты:",
    "физические даты :",
    "физические даты\u00a0:",
    "физическая дата:",
    "физическая дата :",
    "физическая дата\u00a0:",
    # полноширинное двоеточие (Word / копипаст)
    "Физическое：",
    "Физическое ：",
    "Физическая：",
    "Физическая ：",
    "Физические даты：",
    "Физическая дата：",
]

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
    ("fizicheskoe", _FIZICHESKOE_LABELS),
]
