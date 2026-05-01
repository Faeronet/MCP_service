# Полная инструкция: запуск на macOS без виртуалки

Этот гайд описывает запуск **всего проекта на одном Mac, без Linux VM**.

Важно:

- в этом режиме LLM работает через **локальный Ollama** (OpenAI-compatible API), а не через docker-vLLM;
- это режим **не 1:1 по runtime/model artifact** относительно `vLLM + Qwen/Qwen3-14B-AWQ`;
- если нужна **строго та же модель и тот же стек**, используйте Linux runtime (см. `docs/mac-vllm-vm.md`).

---

## 0) Что нужно установить

- `git`
- Docker Desktop (с включенным Docker Compose)
- Homebrew
- Ollama

Установка:

```bash
brew install --cask docker
brew install --cask ollama
```

---

## 1) Клонирование

```bash
git clone <URL_РЕПО> MCP_service
cd MCP_service
```

---

## 2) Поднять Ollama и модель

Запуск Ollama:

```bash
ollama serve
```

В другом терминале загрузить модель:

```bash
ollama pull <OLLAMA_MODEL_TAG>
```

Проверка:

```bash
curl http://127.0.0.1:11434/v1/models
```

---

## 3) Настроить `.env` для Mac-only

### `chat-orchestrator/.env`

Поставьте:

```env
VLLM_OPENAI_BASE=http://host.docker.internal:11434/v1
LLM_MODEL=<OLLAMA_MODEL_TAG>
LLM_DISABLE_CHAT_TEMPLATE_KWARGS=1
```

Комментарий:
- `host.docker.internal` нужен, чтобы контейнеры достучались до Ollama на хосте Mac.
- `LLM_DISABLE_CHAT_TEMPLATE_KWARGS=1` отключает vLLM-специфичное поле в payload.

### `mcp-files/.env`

Если хотите, чтобы эмбеддинги тоже шли через тот же OpenAI-compatible endpoint:

```env
EMBED_API_URL=http://host.docker.internal:11434/v1
```

> Для качественного embedding обычно лучше отдельный embedding backend. Для быстрого локального запуска можно оставить так.

### Обязательные секреты

- `tg-bot/.env`: `TELEGRAM_BOT_TOKEN=...`
- `chat-orchestrator/.env` и `notification/.env`: одинаковый `SCHEDULER_INTERNAL_SECRET`

---

## 4) Запуск проекта

```bash
./stack-up.sh
```

Важно: **не** использовать `MCP_WITH_VLLM=1` в этом macOS-only режиме.

---

## 5) Проверка после запуска

```bash
curl http://127.0.0.1:8080/healthz   # admin-backend
curl http://127.0.0.1:8081/healthz   # tg-bot
curl http://127.0.0.1:8083/healthz   # mcp-proxy
curl http://127.0.0.1:8001/healthz   # mcp-write
```

UI:

- Admin UI: `http://127.0.0.1:5173`
- MinIO console: `http://127.0.0.1:9001`

---

## 6) Остановка

```bash
./stack-up.sh down
```

---

## 7) Типовые проблемы

### 7.1 `llm 4xx/5xx` в `mcp-proxy`

Проверьте:

- Ollama поднят: `ollama ps`
- API доступно: `curl http://127.0.0.1:11434/v1/models`
- в `chat-orchestrator/.env`:
  - `VLLM_OPENAI_BASE=http://host.docker.internal:11434/v1`
  - `LLM_MODEL` совпадает с `ollama list`
  - `LLM_DISABLE_CHAT_TEMPLATE_KWARGS=1`

### 7.2 `model not found`

Скачать модель:

```bash
ollama pull <OLLAMA_MODEL_TAG>
```

И выставить это же имя в `LLM_MODEL`.

### 7.3 Медленно отвечает LLM

Для CPU на Mac это ожидаемо.

Что помогает:
- выбрать более легкую модель в Ollama (пример: `qwen2.5:7b-instruct`)
- уменьшить `LLM_MAX_TOKENS` и `LLM_EXTRACT_MAX_TOKENS`

---

## 8) Быстрый чеклист

- [ ] Ollama запущен (`ollama serve`)
- [ ] Модель скачана (`ollama pull ...`)
- [ ] В `chat-orchestrator/.env` настроены `VLLM_OPENAI_BASE`, `LLM_MODEL`, `LLM_DISABLE_CHAT_TEMPLATE_KWARGS=1`
- [ ] Заполнен `TELEGRAM_BOT_TOKEN`
- [ ] Секрет scheduler одинаковый в 2 зонах
- [ ] Выполнен `./stack-up.sh`
- [ ] `/healthz` сервисов отвечает

