-- Полный контекст документа по chunk_id (после записи в Qdrant).
-- chunk_id = основной чанк документа в коллекции chunks; context = весь текст документа.
CREATE TABLE IF NOT EXISTS core.document_context (
    chunk_id TEXT PRIMARY KEY,
    context  TEXT NOT NULL
);
