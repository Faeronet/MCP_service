# Telegram AI Assistant — Monorepo 

Система: Telegram AI assistant + ingestion pipeline + Qdrant-only retrieval + MCP read/write + Admin Web UI + Admin Backend + observability. Рассчитана на ~500 одновременных пользователей (concurrent chats/requests).
## Стек

- **Инфраструктура:** MinIO, Postgres, RabbitMQ, Redis, Qdrant, Loki, Promtail, Grafana, vLLM (OpenAI-compatible).
- **Сервисы:** admin-backend (Go), admin-web-ui (React/TS), **tg-bot** (Telegram, Go), **mcp-proxy** (чат + LLM + напоминания, HTTP), mcp-read (Go), mcp-write (Python), ingestion-worker (Python), attachment-worker (Python), log-indexer (Go). Ранее в репозитории ошибочно добавляли `bot-service` как монолит — он удалён; используйте только **tg-bot → mcp-proxy → mcp-read**.

## Быстрый старт

Репозиторий разбит на **зоны** (`db-zone`, `file-orchestrator`, `log-zone`, …). У каждой зоны свой каталог, свой `docker-compose.yml` и свой **`.env`** — общего корневого compose и общего `.env` нет.

На одной машине:

```bash
# Минимум: TELEGRAM_BOT_TOKEN, SCHEDULER_INTERNAL_SECRET (одинаковый в chat-orchestrator и notification), пароли в db-zone при смене дефолтов
./stack-up.sh
```

С GPU для vLLM и реранкера в зоне `ai-zone`:

```bash
MCP_WITH_VLLM=1 ./stack-up.sh
```

Остановка (обратный порядок):

```bash
./stack-up.sh down
```

Разнесение по ВМ: в `.env` каждой зоны замените `host.docker.internal` на приватные IP/DNS других зон (`POSTGRES_HOST`, `LOKI_HOST`, `MCP_PROXY_HOST`, …).

Миграции Postgres запускает контейнер **`migrate`** в `db-zone` (через `./stack-up.sh` или вручную из каталога `db-zone`: `docker compose run --rm migrate`). С хоста при доступной БД: `./scripts/migrate.sh` (SQL в `db-zone/migrations/`).

### Образы с кодом для CI / прод без сборки на сервере

У сервисов с вашим кодом в compose указаны **`image`** и **`build`**: приложение и связанные файлы попадают в образ при сборке (включая шаблоны **promtail**, **loki.yaml**, provisioning **Grafana**, каталог **migrations** в образе **`mcp-db-migrate`**). На билд-сервере соберите и отправьте в registry, на проде достаточно pull тех же тегов и `.env`/compose.

- **`MCP_IMAGE_REGISTRY`** — префикс без завершающего `/` (например `ghcr.io/myorg`); если не задан, образы собираются локально с именами вида `mcp-tg-bot:latest`.
- **`MCP_IMAGE_TAG`** — тег версии (например `v1.2.3`); по умолчанию `latest`.

Пример после `docker login`: `cd tg-bot && docker compose build && docker compose push`.

**mcp-proxy:** тексты промптов по умолчанию встроены в бинарник (`go:embed`). Отдельное монтирование `./mcp-proxy/prompts` из compose убрано; для отладки можно использовать свой compose override и переменную **`MCP_PROMPTS_DIR`**.

**zone-agent** по-прежнему монтирует каталог зоны в `/zone` для работы с compose на хосте; при деплое «только образами» его часто отключают или заменяют внешним оркестратором.

**Важно:** без эмбеддингов (**vllm-embed** в `ai-zone` с профилем `vllm` или внешний `EMBED_API_URL` в `mcp-files/.env` и др.) **mcp-write** не построит векторы. Без чат-модели не ответит LLM — см. `chat-orchestrator/.env`, `ai-zone/.env`; vLLM на хосте: `VLLM_OPENAI_BASE=http://<IP_хоста>:8000/v1` в **`chat-orchestrator/.env`**.

### Если Docker Hub недоступен (таймаут auth.docker.io / 403)

Ошибка связана с сетью или DNS. Что сделать:

1. **DNS в Docker** — в `/etc/docker/daemon.json` добавьте и перезапустите Docker (`sudo systemctl restart docker`):
   ```json
   { "dns": ["8.8.8.8", "8.8.4.4"] }
   ```
2. **Проверка доступа** — в терминале: `curl -sI https://auth.docker.io` или `ping registry-1.docker.io`.
3. **Прокси/VPN** — если доступ в интернет только через прокси или VPN, настройте их для демона Docker.
4. **Зеркало** — если в сети есть корпоративное зеркало Docker Hub, укажите его в `daemon.json` в `registry-mirrors`.

Все сервисы, кроме vLLM, запускаются по умолчанию. **extract-tool** (OCR/ASR) в штатном образе работает **только на CPU** и не запрашивает NVIDIA. **GPU в Docker** нужен только для профиля `vllm` (чат, эмбеддинги, реранк). Остальные сервисы перезапускаются при падении (`restart: unless-stopped`).

### Запуск на ПК с игровой видеокартой (RTX 3080 Ti и др.)

По умолчанию **ни один сервис не запрашивает GPU** — `./stack-up.sh` без `MCP_WITH_VLLM=1` не поднимает vLLM и не требует NVIDIA Container Toolkit. Админка работает без мониторинга GPU (в интерфейсе будет N/A).

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
   docker run --rm --gpus all nvidia/cuda:12.4.0-runtime-ubuntu22.04 nvidia-smi
   ```
4. Запустите зону **ai-zone** с vLLM (одна GPU, подходит для 3080 Ti):
   ```bash
   MCP_WITH_VLLM=1 ./stack-up.sh
   ```
   или из каталога `ai-zone`: `docker compose --profile vllm up -d`

Если после шага 2 при запуске с GPU появляется **«could not select device driver "nvidia" with capabilities: [[gpu]]»** — значит рантайм не подхватился: снова выполните `sudo nvidia-ctk runtime configure --runtime=docker` и `sudo systemctl restart docker`, затем повторите запуск.

**vLLM в контейнере (зона `ai-zone`):** чтобы чат видел LLM, поднимайте **ai-zone** с профилем `vllm` (`MCP_WITH_VLLM=1 ./stack-up.sh` или `cd ai-zone && docker compose --profile vllm up -d`). Остановка: `./stack-up.sh down` или `cd ai-zone && docker compose --profile vllm down`. Если vLLM и **chat-orchestrator** на одной машине, в **`chat-orchestrator/.env`** можно оставить `VLLM_OPENAI_BASE=http://host.docker.internal:8000/v1` (порт 8000 с ai-zone). Если vLLM не принимает модель `default`, задайте `LLM_MODEL` как в `VLLM_MODEL_NAME`.

**vLLM на хосте (не в Docker):** в **`chat-orchestrator/.env`** задайте `VLLM_OPENAI_BASE=http://host.docker.internal:8000/v1` (или IP хоста), затем поднимите зоны без профиля vllm в `ai-zone` или только promtail/rerank при необходимости.

## Устранение неполадок

### Ошибка: «could not select device driver "nvidia" with capabilities: [[gpu]]»

Ошибка появляется, когда вы запускаете **ai-zone** с GPU (`MCP_WITH_VLLM=1 ./stack-up.sh` или `docker compose --profile vllm` из `ai-zone/`), а **NVIDIA Container Toolkit** не установлен или Docker его не использует.

**Что сделать:**

1. **Установить NVIDIA Container Toolkit** (если ещё не ставили), затем настроить рантайм и перезапустить Docker:
   ```bash
   curl -fsSL https://nvidia.github.io/libnvidia-container/gpgkey | sudo gpg --dearmor -o /usr/share/keyrings/nvidia-container-toolkit-keyring.gpg
   curl -s -L https://nvidia.github.io/libnvidia-container/stable/deb/nvidia-container-toolkit.list | \
     sed 's#deb https://#deb [signed-by=/usr/share/keyrings/nvidia-container-toolkit-keyring.gpg] https://#g' | \
     sudo tee /etc/apt/sources.list.d/nvidia-container-toolkit.list
   sudo apt-get update && sudo apt-get install -y nvidia-container-toolkit
   sudo nvidia-ctk runtime configure --runtime=docker
   sudo systemctl restart docker
   ```
2. **Проверить**, что контейнер видит GPU:
   ```bash
   docker run --rm --gpus all nvidia/cuda:12.4.0-runtime-ubuntu22.04 nvidia-smi
   ```
   Если здесь команда выполняется и показывается таблица с GPU — снова запустите ваш compose с профилем/override. Если ошибка остаётся — перезайдите в консоль после перезапуска Docker и повторите `nvidia-ctk runtime configure` и `systemctl restart docker`.

**Без GPU:** `./stack-up.sh` без `MCP_WITH_VLLM=1` — vLLM в Docker не поднимается, остальные зоны работают. LLM можно запустить на хосте и указать `VLLM_OPENAI_BASE` в **`chat-orchestrator/.env`**.

### vLLM: «model's context length is only 2048» / 400 на `/v1/chat/completions`

По умолчанию **mcp-proxy** считает контекст **8192** токенов (`LLM_CONTEXT_LENGTH`). Если в команде запуска vLLM **нет** `--max-model-len`, сервер часто ограничивает модель **2048** токенами — промпт не помещается, в логах vLLM `VLLMValidationError`, в UI «модель недоступна».

**Что сделать:** в `ai-zone/docker-compose.yml` для сервиса `vllm` задано `--max-model-len` (по умолчанию **8192**, переменная `VLLM_MAX_MODEL_LEN` в **`ai-zone/.env`**). После изменения пересоздайте контейнер:

```bash
cd ai-zone && docker compose --profile vllm up -d --force-recreate vllm
```

Убедитесь, что `VLLM_MAX_MODEL_LEN` ≥ `LLM_CONTEXT_LENGTH` в окружении **mcp-proxy**. При нехватке VRAM уменьните оба до **4096** согласованно.

### vLLM: Error 803 (unsupported display driver / cuda driver combination)

Ошибка означает несовместимость **драйвера на хосте** и **CUDA в контейнере**. Сначала проверьте, что контейнеры видят GPU:

```bash
docker run --rm --gpus all nvidia/cuda:12.4.0-runtime-ubuntu22.04 nvidia-smi
```

Если здесь тоже 803 или нет GPU — настройте NVIDIA Container Toolkit и рантайм:

```bash
sudo nvidia-ctk runtime configure --runtime=docker
sudo systemctl restart docker
```

После этого снова `cd ai-zone && docker compose --profile vllm up -d vllm`. Если ошибка остаётся — обновите драйвер на хосте до версии ≥ 525 (для CUDA 12.4 — актуальный драйвер из пакетов или с сайта NVIDIA):

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
   cd ai-zone && docker compose --profile vllm up -d
   ```

Альтернатива: [драйвер с сайта NVIDIA](https://www.nvidia.com/Download/index.aspx) (выберите видеокарту и ОС).

**CUDA 12.4:** проверка GPU в контейнере и рекомендуемый базовый образ для своих сборок (эмбеддинг, реранк, OCR, ASR) — `nvidia/cuda:12.4.0-runtime-ubuntu22.04`; PyTorch для второй карты: `pip install torch --index-url https://download.pytorch.org/whl/cu124`. Образ vLLM (vllm/vllm-openai) собирается со своим CUDA; при несовместимости драйвера используйте вариант «vLLM на хосте» ниже.

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

3. **Остановить контейнер vLLM и указать адрес хоста в `chat-orchestrator/.env`:**
   ```bash
   cd ai-zone && docker compose stop vllm
   ```
   В **`chat-orchestrator/.env`** задайте (подставьте IP хоста, с которого контейнеры ходят в vLLM; на той же машине — `host.docker.internal` или `172.17.0.1`):
   ```
   VLLM_OPENAI_BASE=http://host.docker.internal:8000/v1
   ```
   На Linux без `host.docker.internal` используйте IP интерфейса хоста (например `192.168.x.x`) или добавьте в compose нужной зоны `extra_hosts: - "host.docker.internal:host-gateway"`.

4. Остальные сервисы (tg-bot, mcp-write и т.д.) будут обращаться к vLLM по этому URL.

## Эмбеддинг, реранк, OCR, ASR и Qdrant

**Текущее состояние:** в коде заложены переменные окружения и заглушки; полноценные пайплайны нужно дорабатывать.

- **vLLM для чата и эмбеддингов:** в **`ai-zone`** с профилем `vllm` поднимаются **vllm** (чат) и **vllm-embed** (BAAI/bge-m3). По умолчанию оба на **GPU 0**; при **двух** картах задайте в **`ai-zone/.env`** и при необходимости поправьте `device_ids` в **`ai-zone/docker-compose.yml`** (`NVIDIA_VISIBLE_DEVICES_EMBED`, `NVIDIA_VISIBLE_DEVICES_RERANK`).
- **extract-tool:** образ `python` + **torch CPU**; CUDA не используется. Для экспериментов с GPU — свой Dockerfile с torch CUDA и переменная **`EXTRACT_USE_CUDA=1`**.
- **Реранк:** задайте `RERANK_API_URL` на сервис с `POST /rerank` (тело: `query`, `documents`, `model`; ответ: `results[]` с `index` и `relevance_score` или `scores[]`). Реранкер (например FlagEmbedding) — отдельный сервис, не входит в образ vLLM.
- **OCR и ASR:** задайте `OCR_SERVICE_URL` и `ASR_SERVICE_URL` на сервисы с `POST /ocr` и `POST /asr` (multipart file → JSON `{"text": "..."}`). attachment-worker вызывает их для картинок/PDF и аудио. Реализации (PaddleOCR, Whisper) — отдельные контейнеры, не vLLM.
- **Реранк:** mcp-read читает `RERANK_MODEL`, но реранк по ответам Qdrant **не вызывается** — нужно добавить вызов модели реранка после поиска по чанкам.
- **OCR и ASR:** в attachment-worker заданы `WHISPER_MODEL` и `OCR_MODEL`, но пайплайн обработки вложений (документы, голос) может быть заглушкой — нужна реализация распознавания и индексации.
- **Связи между чанками:** в Qdrant хранятся только точки в коллекции **chunks**; отдельной коллекции для рёбер/связей нет. Построение графа связей (links) нужно добавлять в mcp-write при инжесте.

**Удаление документа в админке:** кнопка «Удалить» сначала удаляет запись из БД (документ пропадает из списка), затем по возможности чистит чанки в Qdrant через mcp-write. Если при удалении показывается «документ не найден», проверьте, что удаляете по тому же id, что отображается в таблице (обновите страницу Documents и попробуйте снова).

## MinIO: доступ с другого компьютера

Порты **9000** (S3 API) и **9001** (веб-консоль) проброшены на хост. С другого ПК в той же сети:

- **API:** `http://IP_СЕРВЕРА:9000`
- **Консоль:** `http://IP_СЕРВЕРА:9001` (логин/пароль — `MINIO_ACCESS_KEY` / `MINIO_SECRET_KEY` из **`file-orchestrator/.env`**)

Если консоль при открытии по IP редиректит на localhost, задайте в **`file-orchestrator/.env`**:
```bash
MINIO_SERVER_URL=http://IP_СЕРВЕРА:9001
```
и перезапустите MinIO из каталога **file-orchestrator**: `docker compose up -d minio`.

## Структура репозитория

Корень — только **скрипты и документация**; у каждой зоны свой **`docker-compose.yml`**, **`.env`**, при необходимости **`go.mod`/`go.sum`** (Go) и **`Dockerfile`**.

```
├── stack-up.sh             # подъём зон по очереди (каждая из своего каталога)
├── tg-bot/                 # .env, docker-compose, Go + pkg
├── chat-orchestrator/      # mcp-proxy, mcp-read
├── notification/           # scheduler
├── angels-web/
├── admin-zone/
├── db-zone/                # postgres, redis, qdrant, migrate; migrations/
├── file-orchestrator/      # minio, rabbitmq
├── workers/
├── mcp-files/              # mcp-write, extract-tool
├── ai-zone/                # vllm, promtail, rerank; models/ (кэш HF)
├── log-zone/               # loki, grafana, log-indexer
└── scripts/
    └── migrate.sh          # хост → db-zone/migrations
```

## Доступ к админ-панели с другого устройства

Откройте в браузере **http://IP_СЕРВЕРА:5173** (подставьте IP машины, где запущен Docker). Логин и пароль по умолчанию: **admin** / **admin**.

- Убедитесь, что порт **5173** открыт в файрволе (например: `sudo ufw allow 5173` или настройки роутера).
- В docker-compose порт задан как `0.0.0.0:5173:80`, поэтому сервис слушает на всех интерфейсах.
- API вызывается относительно того же хоста (запросы идут на тот же IP:5173, nginx проксирует `/api` на backend).

## Настройка Telegram

1. Создайте бота через [@BotFather](https://t.me/BotFather), получите токен.
2. В **`tg-bot/.env`** задайте `TELEGRAM_BOT_TOKEN=<token>`.
3. По умолчанию используется **polling**. Для webhook задайте `TELEGRAM_WEBHOOK_URL` и настройте reverse proxy (в README не расписано, при необходимости добавьте Traefik/Nginx).

## Основные curl

```bash
# Health
curl http://localhost:8080/healthz   # admin-backend
curl http://localhost:8081/healthz   # tg-bot
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

- Секреты в **`.env` внутри каждой зоны**; для продакшена не коммитьте реальные значения (оставьте в репозитории только безопасные дефолты или пустые плейсхолдеры по политике команды).
- mcp-read — только чтение из Qdrant; mcp-write — запись.
- admin-backend вызывает mcp-write; bot — только mcp-read и attachment pipeline.

## Логи и observability

- Все логи — структурированный JSON: `ts`, `level`, `service`, `request_id`, `message`, `fields`.
- `request_id` задаётся на входе (bot/admin/workers) и прокидывается в очередях и HTTP-заголовках.
- Admin UI → вкладка Logs: быстрый поиск по индексу в Postgres (obs.logs_index) + при необходимости сырой лог из Loki (через admin-backend).
- Grafana встроена в Admin UI (iframe / proxy через admin-backend). Datasource: Loki (и при необходимости Prometheus).

## Миграции

Основной путь: контейнер **`migrate`** в **`db-zone`** (см. `./stack-up.sh`). С хоста при доступной БД:

```bash
export POSTGRES_DSN="postgres://postgres:postgres@localhost:5432/assistant?sslmode=disable"
./scripts/migrate.sh
# SQL лежат в db-zone/migrations/*.up.sql
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
for f in db-zone/migrations/00000*.up.sql; do psql "$POSTGRES_DSN" -f "$f"; done
```

Или `./scripts/migrate.sh` (обходит все `.up.sql` в `db-zone/migrations/`).

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
| tg-bot   | 8081  |
| mcp-read      | 8082  |
| mcp-write     | 8001  |
| postgres      | 5432  |
| minio         | 9000, 9001 |
| rabbitmq      | 5672, 15672 |
| qdrant        | 6333  |
| loki          | 3100  |
| grafana       | 3001  |
| vllm          | 8000  |
