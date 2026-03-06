DROP TABLE IF EXISTS core.angel_names;
DROP INDEX IF EXISTS core.idx_angel_names_doc_id;

ALTER TABLE core.document_context DROP COLUMN IF EXISTS doc_id;
DROP INDEX IF EXISTS core.idx_document_context_doc_id;
