"""
Ingestion worker: consume ingestion_jobs from RabbitMQ, call mcp-write ingest_document.
"""
import os
import json
import logging
import time
import uuid

import httpx
import pika
import psycopg
from psycopg.rows import dict_row

logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(name)s %(message)s")
log = logging.getLogger("ingestion-worker")

POSTGRES_DSN = os.getenv("POSTGRES_DSN", "postgres://postgres:postgres@postgres:5432/assistant?sslmode=disable")
RABBITMQ_URL = os.getenv("RABBITMQ_URL", "amqp://guest:guest@rabbitmq:5672/")
MCP_WRITE_URL = os.getenv("MCP_WRITE_URL", "http://mcp-write:8001")
WORKER_CONCURRENCY = int(os.getenv("WORKER_CONCURRENCY", "4"))


def main():
    params = pika.URLParameters(RABBITMQ_URL)
    conn = None
    for attempt in range(30):
        try:
            conn = pika.BlockingConnection(params)
            break
        except Exception as e:
            log.warning("rabbitmq connect attempt %s: %s", attempt + 1, e)
            time.sleep(2)
    if conn is None:
        raise RuntimeError("rabbitmq connect failed after retries")
    ch = conn.channel()
    ch.queue_declare("ingestion_jobs", durable=True)
    ch.basic_qos(prefetch_count=1)

    def on_message(channel, method, properties, body):
        payload = None
        try:
            payload = json.loads(body)
            job_id = payload.get("job_id")
            doc_id = payload.get("doc_id")
            version_id = payload.get("version_id")
            file_hash = payload.get("file_hash")
            file_uri = payload.get("file_uri")
            request_id = payload.get("request_id", "")

            with psycopg.connect(POSTGRES_DSN, row_factory=dict_row) as pg:
                cur = pg.cursor()
                cur.execute(
                    "UPDATE core.jobs SET status = 'running', updated_at = NOW() WHERE id = %s",
                    (uuid.UUID(job_id),),
                )
                pg.commit()
                cur.execute(
                    "INSERT INTO core.job_steps (job_id, step_name, status) VALUES (%s, 'ingest', 'running')",
                    (uuid.UUID(job_id),),
                )
                pg.commit()

            with httpx.Client(timeout=120.0) as client:
                r = client.post(
                    f"{MCP_WRITE_URL}/mcp/ingest_document",
                    json={
                        "file_uri": file_uri,
                        "doc_id": doc_id,
                        "version_id": version_id,
                        "file_hash": file_hash,
                        "metadata": {},
                    },
                )
                r.raise_for_status()
                result = r.json()

            with psycopg.connect(POSTGRES_DSN, row_factory=dict_row) as pg:
                cur = pg.cursor()
                cur.execute(
                    "UPDATE core.job_steps SET status = 'done', detail = %s WHERE job_id = %s AND step_name = 'ingest'",
                    (json.dumps(result), uuid.UUID(job_id)),
                )
                cur.execute(
                    "UPDATE core.jobs SET status = 'done', updated_at = NOW() WHERE id = %s",
                    (uuid.UUID(job_id),),
                )
                pg.commit()

            channel.basic_ack(delivery_tag=method.delivery_tag)
        except Exception as e:
            log.exception("job failed")
            err_msg = str(e)
            if payload and payload.get("job_id"):
                try:
                    doc_id = None
                    if payload.get("doc_id"):
                        try:
                            doc_id = uuid.UUID(payload["doc_id"])
                        except (ValueError, TypeError):
                            pass
                    with psycopg.connect(POSTGRES_DSN) as pg:
                        cur = pg.cursor()
                        jid = uuid.UUID(payload["job_id"])
                        cur.execute(
                            "UPDATE core.job_steps SET status = 'failed', detail = %s::jsonb WHERE job_id = %s AND step_name = 'ingest'",
                            (json.dumps({"error": err_msg}), jid),
                        )
                        cur.execute(
                            "UPDATE core.jobs SET status = 'failed', updated_at = NOW() WHERE id = %s",
                            (jid,),
                        )
                        pg.commit()
                        # Удалить неудачный документ из списка: job_steps (CASCADE при удалении job), job, versions, doc
                        if doc_id:
                            cur.execute("DELETE FROM core.job_steps WHERE job_id = %s", (jid,))
                            cur.execute("DELETE FROM core.jobs WHERE id = %s", (jid,))
                            cur.execute("DELETE FROM core.versions WHERE doc_id = %s", (doc_id,))
                            cur.execute("DELETE FROM core.docs WHERE id = %s", (doc_id,))
                            pg.commit()
                            log.info("removed failed doc %s from list", doc_id)
                except Exception as cleanup_err:
                    log.warning("cleanup after failed job: %s", cleanup_err)
            channel.basic_nack(delivery_tag=method.delivery_tag, requeue=False)

    ch.basic_consume("ingestion_jobs", on_message_callback=on_message)
    log.info("ingestion-worker consuming ingestion_jobs")
    ch.start_consuming()


if __name__ == "__main__":
    while True:
        try:
            main()
        except Exception as e:
            log.error("consumer error: %s", e)
            time.sleep(5)
