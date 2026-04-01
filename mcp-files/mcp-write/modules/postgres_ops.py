"""Запись в Postgres: document_context, angel_names, angel_physical_dates; удаление по doc_id."""
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


def save_angel_physical_dates_postgres(chunk_id: str, doc_id: str, name: str, dates_ddmm: list[str]) -> None:
    try:
        import psycopg2
        conn = psycopg2.connect(config.POSTGRES_DSN)
        try:
            with conn.cursor() as cur:
                if not dates_ddmm:
                    cur.execute("DELETE FROM core.angel_physical_dates WHERE chunk_id = %s", (chunk_id,))
                else:
                    cur.execute(
                        """
                        INSERT INTO core.angel_physical_dates (chunk_id, doc_id, name, dates_ddmm)
                        VALUES (%s, %s, %s, %s)
                        ON CONFLICT (chunk_id) DO UPDATE SET
                            doc_id = EXCLUDED.doc_id,
                            name = EXCLUDED.name,
                            dates_ddmm = EXCLUDED.dates_ddmm
                        """,
                        (chunk_id, doc_id, name.strip(), dates_ddmm),
                    )
                    cur.execute(
                        "DELETE FROM core.angel_physical_date_entries WHERE chunk_id = %s",
                        (chunk_id,),
                    )
                    for ddmm in dates_ddmm:
                        cur.execute(
                            """
                            INSERT INTO core.angel_physical_date_entries (ddmm, chunk_id)
                            VALUES (%s, %s)
                            ON CONFLICT (ddmm, chunk_id) DO NOTHING
                            """,
                            (ddmm, chunk_id),
                        )
            conn.commit()
        finally:
            conn.close()
    except Exception as e:
        log.warning("angel_physical_dates insert failed: %s", e)


def delete_document_from_postgres(doc_id: str) -> None:
    try:
        import psycopg2
        conn = psycopg2.connect(config.POSTGRES_DSN)
        try:
            with conn.cursor() as cur:
                cur.execute("DELETE FROM core.document_context WHERE doc_id = %s", (doc_id,))
                cur.execute("DELETE FROM core.angel_names WHERE doc_id = %s", (doc_id,))
                cur.execute("DELETE FROM core.angel_physical_dates WHERE doc_id = %s", (doc_id,))
            conn.commit()
        finally:
            conn.close()
    except Exception as e:
        log.warning("delete document from postgres failed: %s", e)
