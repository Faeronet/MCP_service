"""Запись в Postgres: document_context, angel_names; удаление по doc_id."""
import logging

from . import config

log = logging.getLogger("mcp-write")


def save_document_context_postgres(chunk_id: str, doc_id: str, context: str) -> None:
    try:
        import psycopg2
        conn = psycopg2.connect(config.POSTGRES_DSN)
        try:
            with conn.cursor() as cur:
                cur.execute(
                    """
                    INSERT INTO core.document_context (chunk_id, doc_id, context)
                    VALUES (%s, %s, %s)
                    ON CONFLICT (chunk_id) DO UPDATE SET doc_id = EXCLUDED.doc_id, context = EXCLUDED.context
                    """,
                    (chunk_id, doc_id, context),
                )
            conn.commit()
        finally:
            conn.close()
    except Exception as e:
        log.warning("document_context insert failed: %s", e)


def save_angel_name_postgres(chunk_id: str, doc_id: str, name: str) -> None:
    if not name or not name.strip():
        return
    try:
        import psycopg2
        conn = psycopg2.connect(config.POSTGRES_DSN)
        try:
            with conn.cursor() as cur:
                cur.execute(
                    """
                    INSERT INTO core.angel_names (chunk_id, doc_id, name)
                    VALUES (%s, %s, %s)
                    ON CONFLICT (chunk_id) DO UPDATE SET doc_id = EXCLUDED.doc_id, name = EXCLUDED.name
                    """,
                    (chunk_id, doc_id, name.strip()),
                )
            conn.commit()
        finally:
            conn.close()
    except Exception as e:
        log.warning("angel_names insert failed: %s", e)


def delete_document_from_postgres(doc_id: str) -> None:
    try:
        import psycopg2
        conn = psycopg2.connect(config.POSTGRES_DSN)
        try:
            with conn.cursor() as cur:
                cur.execute("DELETE FROM core.document_context WHERE doc_id = %s", (doc_id,))
                cur.execute("DELETE FROM core.angel_names WHERE doc_id = %s", (doc_id,))
            conn.commit()
        finally:
            conn.close()
    except Exception as e:
        log.warning("delete document from postgres failed: %s", e)
