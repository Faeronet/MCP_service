# macOS: весь проект + vLLM в Linux VM

Цель: запустить весь стек проекта на Mac, а **vLLM (та же модель, что сейчас)** — в Linux VM на этом же Mac.

## 1) Что получится

- Docker Compose зоны проекта работают на macOS.
- LLM endpoint (`/v1/chat/completions`) дает vLLM из Linux VM.
- `chat-orchestrator` и `mcp-files` ходят в VM по `http://<VM_IP>:8000/v1`.

## 2) Поднять Linux VM (пример с Multipass)

```bash
brew install --cask multipass
multipass launch 24.04 --name mcp-vllm --cpus 8 --memory 16G --disk 120G
multipass shell mcp-vllm
```

Внутри VM:

```bash
sudo apt update
sudo apt install -y python3-venv python3-pip build-essential
python3 -m venv ~/.venv-vllm
source ~/.venv-vllm/bin/activate
pip install -U pip
pip install vllm
```

## 3) Запустить vLLM в VM

Минимальный smoke-test (легкая модель):

```bash
source ~/.venv-vllm/bin/activate
vllm serve Qwen/Qwen3-0.6B --host 0.0.0.0 --port 8000 --max-model-len 8192
```

Для вашей текущей модели:

```bash
source ~/.venv-vllm/bin/activate
vllm serve Qwen/Qwen3-14B-AWQ --host 0.0.0.0 --port 8000 --max-model-len 8192
```

Проверка из macOS:

```bash
curl "http://<VM_IP>:8000/v1/models"
```

## 4) Настроить проект на Mac

### `chat-orchestrator/.env`

```env
VLLM_OPENAI_BASE=http://<VM_IP>:8000/v1
LLM_MODEL=Qwen/Qwen3-14B-AWQ
LLM_DISABLE_CHAT_TEMPLATE_KWARGS=0
```

### `mcp-files/.env`

Если эмбеддинги тоже идут через vLLM endpoint:

```env
EMBED_API_URL=http://<VM_IP>:8000/v1
```

## 5) Старт проекта на Mac

Запуск без docker-vLLM профиля в `ai-zone`:

```bash
./stack-up.sh
```

> Не ставьте `MCP_WITH_VLLM=1` — иначе проект попробует поднять docker-vLLM внутри `ai-zone`.

## 6) Быстрая диагностика

- Проверить, что `mcp-proxy` видит LLM:

```bash
curl -s "http://127.0.0.1:8083/healthz"
```

- Если в логах `mcp-proxy` есть `llm 4xx/5xx`:
  - проверьте `VLLM_OPENAI_BASE`,
  - проверьте имя модели `LLM_MODEL`,
  - проверьте доступность `http://<VM_IP>:8000/v1/models`.

## 7) Производительность

- На CPU в VM скорость будет заметно ниже GPU.
- Для тестового контура лучше начинать с `Qwen/Qwen3-0.6B`.
- `Qwen/Qwen3-14B-AWQ` на CPU возможен, но медленный и требовательный к RAM.

