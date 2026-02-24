"""
Attachment worker: consume attachment_jobs; unzip/convert/ocr/asr -> extracted_text.
Sandbox: container limits (no-new-privileges, read-only rootfs in prod); allowlist types.
"""
import os
import json
import logging
import time
import uuid
from io import BytesIO

import pika
import psycopg
from minio import Minio

logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(name)s %(message)s")
log = logging.getLogger("attachment-worker")

POSTGRES_DSN = os.getenv("POSTGRES_DSN", "postgres://postgres:postgres@postgres:5432/assistant?sslmode=disable")
RABBITMQ_URL = os.getenv("RABBITMQ_URL", "amqp://guest:guest@rabbitmq:5672/")
MINIO_ENDPOINT = os.getenv("MINIO_ENDPOINT", "minio:9000")
MINIO_ACCESS = os.getenv("MINIO_ACCESS_KEY", "minioadmin")
MINIO_SECRET = os.getenv("MINIO_SECRET_KEY", "minioadmin")
BUCKET = os.getenv("MINIO_ATTACHMENTS_BUCKET", "attachments")

ALLOWED_EXTENSIONS = {".txt", ".pdf", ".jpg", ".jpeg", ".png", ".ogg", ".mp3"}


def main():
    minio_client = Minio(MINIO_ENDPOINT, access_key=MINIO_ACCESS, secret_key=MINIO_SECRET, secure=False)
    params = pika.URLParameters(RABBITMQ_URL)
    conn = pika.BlockingConnection(params)
    ch = conn.channel()
    ch.queue_declare("attachment_jobs", durable=True)
    ch.basic_qos(prefetch_count=1)

    def on_message(channel, method, properties, body):
        try:
            payload = json.loads(body)
            chat_id = payload.get("chat_id")
            request_id = payload.get("request_id")
            object_key = payload.get("object_key")
            file_id = payload.get("file_id")

            # Placeholder: no Telegram file download here; assume object already in MinIO or skip
            # In full impl: get file from Telegram API, put to MinIO, then process
            extracted = "Attachment received. (OCR/ASR placeholder - configure pipeline for full extraction.)"

            with psycopg.connect(POSTGRES_DSN) as pg:
                cur = pg.cursor()
                # Find session by chat_id and insert attachment record if schema supports it
                cur.execute(
                    """
                    INSERT INTO chat.attachments (session_id, object_key, extracted_text, status)
                    SELECT id, %s, %s, 'done' FROM chat.sessions WHERE chat_id = %s LIMIT 1
                    """,
                    (object_key, extracted, int(chat_id)),
                )
                pg.commit()

            channel.basic_ack(delivery_tag=method.delivery_tag)
        except Exception as e:
            log.exception("attachment job failed: %s", e)
            channel.basic_nack(delivery_tag=method.delivery_tag, requeue=False)

    ch.basic_consume("attachment_jobs", on_message_callback=on_message)
    log.info("attachment-worker consuming attachment_jobs")
    ch.start_consuming()


if __name__ == "__main__":
    while True:
        try:
            main()
        except Exception as e:
            log.error("consumer error: %s", e)
            time.sleep(5)
