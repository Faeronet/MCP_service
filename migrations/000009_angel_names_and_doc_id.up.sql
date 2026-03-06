-- doc_id в document_context для удаления по документу.
ALTER TABLE core.document_context ADD COLUMN IF NOT EXISTS doc_id TEXT;
CREATE INDEX IF NOT EXISTS idx_document_context_doc_id ON core.document_context(doc_id);

-- Список имён ангелов: chunk_id (канонический id ангела), doc_id (для удаления), name.
CREATE TABLE IF NOT EXISTS core.angel_names (
    chunk_id TEXT PRIMARY KEY,
    doc_id   TEXT NOT NULL,
    name     TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_angel_names_doc_id ON core.angel_names(doc_id);
