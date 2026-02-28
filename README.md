# Telegram AI Assistant — Monorepo

Система: Telegram AI assistant + ingestion pipeline + Qdrant-only retrieval + MCP read/write + Admin Web UI + Admin Backend + observability. Рассчитана на ~500 одновременных пользователей (concurrent chats/requests).

## Стек

- **Инфраструктура:** MinIO, Postgres, RabbitMQ, Redis, Qdrant, Loki, Promtail, Grafana, vLLM (OpenAI-compatible).
- **Сервисы:** admin-backend (Go), admin-web-ui (React/TS), bot-service (Go), mcp-read (Go), mcp-write (Python), ingestion-worker (Python), attachment-worker (Python), log-indexer (Go).

## Быстрый старт

```bash
cp .env.example .env
# Заполните TELEGRAM_BOT_TOKEN и при необходимости JWT_SECRET, ADMIN_PASSWORD

# Запуск всех сервисов (без GPU и без Grafana)
docker compose up -d
# С миграциями (один раз после первого поднятия Postgres)
export POSTGRES_DSN="postgres://postgres:postgres@localhost:5432/assistant?sslmode=disable"
for f in migrations/*.up.sql; do psql "$POSTGRES_DSN" -f "$f"; done

# С профилем observability (Grafana)
docker compose up -d

# vLLM по умолчанию не запускается (профиль vllm). Варианты:
# 1) В Docker (если GPU в контейнере работает): docker compose --profile vllm up -d
# 2) На хосте (обход Error 803): см. раздел «Вариант: vLLM на хосте» ниже; в .env задайте VLLM_OPENAI_BASE=http://host.docker.internal:8000/v1
docker compose up -d
```
**Важно:** без работающего vLLM (в контейнере с `--profile vllm` или на хосте с указанным `VLLM_OPENAI_BASE`) bot-service и mcp-read/mcp-write не смогут вызывать LLM. При 803 в Docker запустите vLLM на хосте и укажите URL в `.env`.

Одна команда поднятия всей инфраструктуры и приложений:

```bash
docker compose up -d && sleep 15 && for f in migrations/*.up.sql; do psql "${POSTGRES_DSN:-postgres://postgres:postgres@localhost:5432/assistant?sslmode=disable}" -f "$f" 2>/dev/null || true; done
```

### Если Docker Hub недоступен (таймаут auth.docker.io / 403)

Ошибка связана с сетью или DNS. Что сделать:

1. **DNS в Docker** — в `/etc/docker/daemon.json` добавьте и перезапустите Docker (`sudo systemctl restart docker`):
   ```json
   { "dns": ["8.8.8.8", "8.8.4.4"] }
   ```
2. **Проверка доступа** — в терминале: `curl -sI https://auth.docker.io` или `ping registry-1.docker.io`.
3. **Прокси/VPN** — если доступ в интернет только через прокси или VPN, настройте их для демона Docker.
4. **Зеркало** — если в сети есть корпоративное зеркало Docker Hub, укажите его в `daemon.json` в `registry-mirrors`.

Все сервисы, кроме vLLM, запускаются по умолчанию. **GPU в Docker не требуется** — драйвер `nvidia` не используется, пока не запускаете vLLM с профилем `vllm`. Остальные сервисы перезапускаются при падении (`restart: unless-stopped`).

### Запуск на ПК с игровой видеокартой (RTX 3080 Ti и др.)

По умолчанию **ни один сервис не запрашивает GPU** — команда `docker compose up -d` не требует установленного NVIDIA Container Toolkit. Админка работает без мониторинга GPU (в интерфейсе будет N/A).

**Чтобы запустить vLLM в Docker на одной видеокарте (например 3080 Ti):**

1. Установите драйвер NVIDIA на хосте и проверьте: `nvidia-smi`.
2. Установите **NVIDIA Container Toolkit** (Ubuntu/Debian):
   ```bash
   curl -fsSL https://nvidia.github.io/libnvidia-container/gpgkey | sudo gpg --dearmor -o /usr/share/keyrings/nvidia-container-toolkit-keyring.gpg
   curl -s -L https://nvidia.github.io/libnvidia-container/stable/deb/nvidia-container-toolkit.list | \
     sed 's#deb https://#deb [signed-by=/usr/share/keyrings/nvidia-container-toolkit-keyring.gpg] https://#g' | \
     sudo tee /etc/apt/sources.list.d/nvidia-container-toolkit.list
   sudo apt-get update && sudo apt-get install -y nvidia-container-toolkit
   sudo nvidia-ctk runtime configure --runtime=docker
   sudo systemctl restart docker
   ```
3. Проверьте, что контейнер видит GPU:
   ```bash
   docker run --rm --gpus all nvidia/cuda:12.0-base-ubuntu22.04 nvidia-smi
   ```
4. Запустите стек с vLLM (одна GPU, подходит для 3080 Ti):
   ```bash
   docker compose --profile vllm up -d
   ```

Если после шага 2 при `docker compose --profile vllm up -d` появляется **«could not select device driver "nvidia" with capabilities: [[gpu]]»** — значит рантайм не подхватился: снова выполните `sudo nvidia-ctk runtime configure --runtime=docker` и `sudo systemctl restart docker`, затем повторите запуск.

**Мониторинг GPU в админке:** по умолчанию админка не запрашивает GPU. Чтобы в веб-интерфейсе отображались загрузка и VRAM видеокарты, после установки toolkit поднимайте стек с override:  
`docker compose -f docker-compose.yml -f docker-compose.gpu.yml up -d` (и при необходимости добавьте `--profile vllm` для vLLM).

#### Две видеокарты: LLM на первой, эмбеддинг/реранк/OCR/ASR на второй

При **двух GPU** (например две RTX 3080 Ti) можно разнести нагрузку:

- **GPU 0** — vLLM (LLM).
- **GPU 1** — mcp-read (эмбеддинг, реранк), mcp-write (эмбеддинг, реранк), attachment-worker (Whisper/ASR, PaddleOCR).

Запуск с двумя файлами и профилем vllm:

```bash
docker compose -f docker-compose.yml -f docker-compose.2gpu.yml --profile vllm up -d
```

Используется override **docker-compose.2gpu.yml**: он выставляет контейнерам доступ к обеим картам и переменные `NVIDIA_VISIBLE_DEVICES` / `CUDA_VISIBLE_DEVICES`, чтобы vLLM видел только GPU 0, а остальные сервисы — только GPU 1. Для работы эмбеддинга/реранка/OCR/ASR на GPU образы mcp-write и attachment-worker должны собираться с поддержкой CUDA (сейчас attachment-worker на python:3.12-slim без GPU; при необходимости замените базовый образ на nvidia/cuda и установите PyTorch с CUDA).

## Устранение неполадок

### vLLM: Error 803 (unsupported display driver / cuda driver combination)

Ошибка означает несовместимость **драйвера на хосте** и **CUDA в контейнере**. Сначала проверьте, что контейнеры видят GPU:

```bash
docker run --rm --gpus all nvidia/cuda:12.0-base-ubuntu22.04 nvidia-smi
```

Если здесь тоже 803 или нет GPU — настройте NVIDIA Container Toolkit и рантайм:

```bash
sudo nvidia-ctk runtime configure --runtime=docker
sudo systemctl restart docker
```

После этого снова `docker compose up -d vllm`. Если ошибка остаётся — обновите драйвер на хосте до версии ≥ 525 (для CUDA 12.x):

1. Текущая версия драйвера:
   ```bash
   nvidia-smi
   ```
   Вверху: **Driver Version: XXX.XX**. Если меньше 525 — обновите.

2. **Ubuntu/Debian:**
   ```bash
   sudo apt update
   sudo apt install -y nvidia-driver-535
   # или новее: nvidia-driver-545, nvidia-driver-550
   sudo reboot
   ```

3. После перезагрузки снова выполните `nvidia-smi` и проверьте версию, затем:
   ```bash
   docker compose --profile vllm up -d
   ```

Альтернатива: [драйвер с сайта NVIDIA](https://www.nvidia.com/Download/index.aspx) (выберите видеокарту и ОС).

**Совпадение с [ai-tg-bot-for-rag](https://github.com/Faeronet/ai-tg-bot-for-rag):** там GPU-сервисы (stt, hf-router) поднимаются с образом `nvidia/cuda:12.1.1-runtime-ubuntu22.04`, PyTorch из `cu121`, переменными `NVIDIA_VISIBLE_DEVICES=all` и `NVIDIA_DRIVER_CAPABILITIES=compute,utility` и резервированием одной GPU (`count: 1`). Здесь для vLLM включены те же переменные и `count: 1`; если у вас в том репозитории stt/hf-router в Docker работают, а vLLM здесь — нет, причина в разном CUDA внутри образов (vLLM тянет свой PyTorch/CUDA). Тогда остаётся вариант «vLLM на хосте» ниже.

### Вариант: vLLM на хосте (если в Docker стабильно Error 803)

При драйвере 580 и установленном toolkit ошибка 803 в контейнере иногда остаётся из‑за особенностей передачи драйвера. Можно запустить vLLM **на хосте** (без Docker) и подключать к нему остальные сервисы:

1. **Установка на хосте** (Python 3.10–3.12):
   ```bash
   pip install vllm
   # или: uv pip install vllm --torch-backend=auto
   ```

2. **Запуск сервера на порту 8000:**
   ```bash
   vllm serve --model Qwen/Qwen3-0.6B --port 8000
   ```
   Оставьте процесс запущенным (или запустите в screen/tmux).

3. **Остановить контейнер vLLM и указать адрес хоста в `.env`:**
   ```bash
   docker compose stop vllm
   ```
   В `.env` задайте (подставьте IP хоста, с которого контейнеры ходят в vLLM; на той же машине — `host.docker.internal` или `172.17.0.1`):
   ```
   VLLM_OPENAI_BASE=http://host.docker.internal:8000/v1
   ```
   На Linux без `host.docker.internal` используйте IP интерфейса хоста (например `192.168.x.x`) или добавьте в `docker-compose.yml` для сервисов, которым нужен vLLM: `extra_hosts: - "host.docker.internal:host-gateway"`.

4. Остальные сервисы (bot-service, mcp-write и т.д.) будут обращаться к vLLM по этому URL.

## Структура репозитория

```
├── docker-compose.yml
├── .env.example
├── go.mod, go.work
├── pkg/                    # shared Go: config, logging, storage, queue, ratelimit, cache
├── migrations/             # Postgres: core, chat, obs
├── config/                 # loki.yaml, promtail.yml, grafana/provisioning
├── admin-backend/          # Go: upload, docs, jobs, logs, grafana proxy, JWT
├── admin-web-ui/          # React/TS (Vite): Docs, Jobs, Logs, Grafana (iframe)
├── bot-service/           # Go: Telegram polling, worker pool, mcp-read, LLM, rate limit
├── mcp-read/               # Go: build_context (embed → qdrant → rerank → context), cache
├── mcp-write/              # Python: ingest_document (chunk/embed/upsert), deterministic IDs
├── ingestion-worker/      # Python: consumer ingestion_jobs → mcp-write
├── attachment-worker/     # Python: consumer attachment_jobs, sandbox, allowlist
├── log-indexer/           # Go: Loki → obs.logs_index
└── scripts/
    └── migrate.sh
```

## Доступ к админ-панели с другого устройства

Откройте в браузере **http://IP_СЕРВЕРА:5173** (подставьте IP машины, где запущен Docker). Логин и пароль по умолчанию: **admin** / **admin**.

- Убедитесь, что порт **5173** открыт в файрволе (например: `sudo ufw allow 5173` или настройки роутера).
- В docker-compose порт задан как `0.0.0.0:5173:80`, поэтому сервис слушает на всех интерфейсах.
- API вызывается относительно того же хоста (запросы идут на тот же IP:5173, nginx проксирует `/api` на backend).

## Настройка Telegram

1. Создайте бота через [@BotFather](https://t.me/BotFather), получите токен.
2. В `.env` задайте `TELEGRAM_BOT_TOKEN=<token>`.
3. По умолчанию используется **polling**. Для webhook задайте `TELEGRAM_WEBHOOK_URL` и настройте reverse proxy (в README не расписано, при необходимости добавьте Traefik/Nginx).

## Основные curl

```bash
# Health
curl http://localhost:8080/healthz   # admin-backend
curl http://localhost:8081/healthz   # bot-service
curl http://localhost:8082/healthz   # mcp-read
curl http://localhost:8001/healthz  # mcp-write

# Login (admin)
curl -X POST http://localhost:8080/api/login -H "Content-Type: application/json" -d '{"username":"admin","password":"admin"}'

# Upload (нужен JWT в заголовке)
curl -X POST http://localhost:8080/api/upload -H "Authorization: Bearer <JWT>" -F "file=@doc.pdf" -F "name=MyDoc"
```

## Масштабирование до ~500 concurrent

- **BOT_WORKER_CONCURRENCY** — число воркеров обработки апдейтов (например 64–128).
- **MAX_INFLIGHT_LLM** — глобальный лимит одновременных вызовов LLM (например 32–64).
- **PER_CHAT_DEBOUNCE_MS** — дебаунс по chat_id (мс), чтобы схлопывать дубли (например 300–500).
- **MCP_READ_CACHE_TTL** — TTL кэша результата retrieval в секундах (30–120).
- **EMBED_CACHE_TTL** — TTL кэша эмбеддингов запроса (короткий, например 30).
- **MAX_INFLIGHT_RERANK**, **MAX_INFLIGHT_EMBED** — ограничение одновременных запросов к rerank и embed в mcp-read.
- **INGESTION_WORKER_CONCURRENCY**, **ATTACHMENT_WORKER_CONCURRENCY** — параллелизм воркеров (4–8).
- Redis обязателен для кэша при высокой нагрузке; при одном инстансе mcp-read можно начать с in-memory.
- Рекомендуется отдельные пулы для тяжёлых задач (ingestion, attachments), чтобы не блокировать чат.

## Модели (LLM / Embeddings / Rerank)

Инфраструктура готова, конфигурация — через env, без хардкода:

- **VLLM_OPENAI_BASE** — базовый URL OpenAI-compatible API (vLLM).
- **VLLM_MODEL_NAME**, **VLLM_MODEL_PATH** — для сервиса vLLM.
- **EMBEDDING_MODEL**, **RERANK_MODEL** — опционально для mcp-read (если пусто — graceful degradation, пустой context или без rerank).

Если модели не настроены, сервисы стартуют и возвращают понятные ошибки (например MODEL_NOT_CONFIGURED / COLLECTION_NOT_READY).

## Безопасность

- Секреты только в env, не коммитить `.env`; использовать `.env.example` как шаблон.
- mcp-read — только чтение из Qdrant; mcp-write — запись.
- admin-backend вызывает mcp-write; bot — только mcp-read и attachment pipeline.

## Логи и observability

- Все логи — структурированный JSON: `ts`, `level`, `service`, `request_id`, `message`, `fields`.
- `request_id` задаётся на входе (bot/admin/workers) и прокидывается в очередях и HTTP-заголовках.
- Admin UI → вкладка Logs: быстрый поиск по индексу в Postgres (obs.logs_index) + при необходимости сырой лог из Loki (через admin-backend).
- Grafana встроена в Admin UI (iframe / proxy через admin-backend). Datasource: Loki (и при необходимости Prometheus).

## Миграции

```bash
export POSTGRES_DSN="postgres://postgres:postgres@localhost:5432/assistant?sslmode=disable"
./scripts/migrate.sh
# или вручную по порядку:
psql "$POSTGRES_DSN" -f migrations/000001_init_schemas.up.sql
psql "$POSTGRES_DSN" -f migrations/000002_chat_schema.up.sql
psql "$POSTGRES_DSN" -f migrations/000003_obs_schema.up.sql
psql "$POSTGRES_DSN" -f migrations/000004_add_indexes.up.sql
psql "$POSTGRES_DSN" -f migrations/000005_partition_logs_index.up.sql
```

## Postgres indexing

Индексы добавлены в миграциях **000004** (индексы + full-text) и **000005** (партиционирование логов). Миграции рассчитаны на применение на пустой или свежей БД при старте контейнеров (обычный `CREATE INDEX`). Для продакшена с уже существующими данными новые индексы лучше накатывать вручную через `CREATE INDEX CONCURRENTLY` вне транзакции.

### Основные индексы

| Схема / таблица | Индексы |
|------------------|--------|
| **core.uploads** | `file_hash` UNIQUE (в DDL), `created_at` |
| **core.docs** | `created_at` |
| **core.versions** | `doc_id`, `created_at` |
| **core.jobs** | `status`, `type`, `created_at`, `doc_id`, `version_id` |
| **core.job_steps** | `job_id`, `step_name`, `status` |
| **chat.sessions** | `(telegram_id, chat_id)` UNIQUE, `last_active`, `telegram_id`, `created_at` |
| **chat.messages** | `session_id`, `created_at`, `(session_id, created_at DESC)`, `role`, GIN(`content_tsv`) |
| **chat.attachments** | `session_id`, `created_at`, `status` |
| **obs.logs_index** | `ts`, `ts DESC`, `service`, `request_id`, `level`, GIN(`message_tsv`) |

### Full-Text Search

- **chat.messages**: колонка `content_tsv` (tsvector, GENERATED), GIN-индекс — поиск по `content`.
- **obs.logs_index**: колонка `message_tsv` (tsvector, GENERATED), GIN-индекс — поиск по `message`. Триггер не нужен: используется GENERATED ALWAYS AS.

Пример поиска по логам (в коде можно добавить параметр `q` и использовать):

```sql
SELECT ts, level, service, request_id, message
FROM obs.logs_index
WHERE message_tsv @@ plainto_tsquery('simple', 'error timeout')
ORDER BY ts DESC LIMIT 100;
```

### Партиционирование и retention логов

- **obs.logs_index** переведена на партиционирование по **RANGE (ts)** с месячными партициями и партицией DEFAULT для старых данных.
- Функции:
  - `obs.create_logs_partition_for_ts(ts_val TIMESTAMPTZ)` — создаёт партицию на месяц для указанной даты, если её ещё нет (вызывать по крону раз в день/месяц).
  - `obs.drop_logs_partitions_older_than(days int DEFAULT 30)` — удаляет партиции старше N дней (рекомендуется вызывать по крону, например раз в сутки).

Рекомендация: настроить cron (или pg_cron), например:

```sql
SELECT obs.create_logs_partition_for_ts(NOW() + interval '1 month');
SELECT obs.drop_logs_partitions_older_than(30);
```

### Как применить миграции

По порядку, от 000001 до 000005:

```bash
export POSTGRES_DSN="postgres://postgres:postgres@localhost:5432/assistant?sslmode=disable"
for f in migrations/00000*.up.sql; do psql "$POSTGRES_DSN" -f "$f"; done
```

Или использовать `./scripts/migrate.sh` (убедитесь, что в нём перечислены все `.up.sql`).

### Как проверить использование индексов (EXPLAIN)

```bash
psql "$POSTGRES_DSN" -c "EXPLAIN (ANALYZE, BUFFERS) SELECT * FROM obs.logs_index WHERE service = 'admin-backend' ORDER BY ts DESC LIMIT 100;"
psql "$POSTGRES_DSN" -c "EXPLAIN (ANALYZE, BUFFERS) SELECT * FROM core.jobs ORDER BY created_at DESC LIMIT 50;"
psql "$POSTGRES_DSN" -c "EXPLAIN (ANALYZE, BUFFERS) SELECT * FROM chat.messages WHERE session_id = '...' ORDER BY created_at DESC LIMIT 50;"
```

В плане должны фигурировать Index Scan / Index Only Scan по соответствующим индексам.

## Идемпотентность ingestion

- **file_hash** (SHA-256) на admin-backend при загрузке; в Postgres `core.uploads` UNIQUE(file_hash).
- **chunk_id** — детерминированный: hash(doc_id + version_id + section_path + normalized_text).
- **edge_id** — детерминированный: hash(from_id + to_id + relation).
- Qdrant: только upsert.

## Порты

| Сервис        | Порт  |
|---------------|-------|
| admin-backend | 8080  |
| admin-web-ui  | 5173  |
| bot-service   | 8081  |
| mcp-read      | 8082  |
| mcp-write     | 8001  |
| postgres      | 5432  |
| minio         | 9000, 9001 |
| rabbitmq      | 5672, 15672 |
| qdrant        | 6333  |
| loki          | 3100  |
| grafana       | 3001  |
| vllm          | 8000  |
