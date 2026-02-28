"""
Attachment worker: consume attachment_jobs; unzip/convert/ocr/asr -> extracted_text.
OCR_SERVICE_URL / ASR_SERVICE_URL: POST multipart file → JSON {"text": "..."}.
"""
import os
import json
import logging
import time
import uuid
from io import BytesIO

import pika
import psycopg
import requests
from minio import Minio

logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(name)s %(message)s")
log = logging.getLogger("attachment-worker")

POSTGRES_DSN = os.getenv("POSTGRES_DSN", "postgres://postgres:postgres@postgres:5432/assistant?sslmode=disable")
RABBITMQ_URL = os.getenv("RABBITMQ_URL", "amqp://guest:guest@rabbitmq:5672/")
MINIO_ENDPOINT = os.getenv("MINIO_ENDPOINT", "minio:9000")
MINIO_ACCESS = os.getenv("MINIO_ACCESS_KEY", "minioadmin")
MINIO_SECRET = os.getenv("MINIO_SECRET_KEY", "minioadmin")
BUCKET = os.getenv("MINIO_ATTACHMENTS_BUCKET", "attachments")
WHISPER_MODEL = os.getenv("WHISPER_MODEL", "openai/whisper-large-v3")
OCR_MODEL = os.getenv("OCR_MODEL", "PaddlePaddle/PaddleOCR-VL-1.5")
OCR_SERVICE_URL = (os.getenv("OCR_SERVICE_URL") or "").strip()
ASR_SERVICE_URL = (os.getenv("ASR_SERVICE_URL") or "").strip()

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

            extracted = "Attachment received."
            if object_key:
                try:
                    obj = minio_client.get_object(BUCKET, object_key)
                    data = obj.read()
                    obj.close()
                except Exception as e:
                    log.warning("minio get %s: %s", object_key, e)
                    data = b""
                if data:
                    ext = os.path.splitext(object_key)[1].lower()
                    if ext in (".jpg", ".jpeg", ".png", ".pdf") and OCR_SERVICE_URL:
                        r = requests.post(
                            OCR_SERVICE_URL.rstrip("/") + "/ocr",
                            files={"file": (os.path.basename(object_key), data, "application/octet-stream")},
                            timeout=60,
                        )
                        if r.status_code == 200:
                            out = r.json()
                            extracted = out.get("text", extracted)
                    elif ext in (".ogg", ".mp3", ".wav", ".m4a") and ASR_SERVICE_URL:
                        r = requests.post(
                            ASR_SERVICE_URL.rstrip("/") + "/asr",
                            files={"file": (os.path.basename(object_key), data, "audio/ogg")},
                            timeout=120,
                        )
                        if r.status_code == 200:
                            out = r.json()
                            extracted = out.get("text", extracted)
                    elif ext == ".txt":
                        extracted = data.decode(errors="replace")
                else:
                    extracted = "Attachment received. (File empty or not in MinIO.)"
            else:
                extracted = "Attachment received. (OCR/ASR: set OCR_SERVICE_URL and ASR_SERVICE_URL for extraction.)"

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
    log.info(
        "attachment-worker consuming attachment_jobs (WHISPER_MODEL=%s, OCR_MODEL=%s)",
        WHISPER_MODEL,
        OCR_MODEL,
    )
    ch.start_consuming()


if __name__ == "__main__":
    while True:
        try:
            main()
        except Exception as e:
            log.error("consumer error: %s", e)
            time.sleep(5)
