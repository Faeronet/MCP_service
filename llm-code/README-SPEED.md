# Скорость llm-code vs vLLM

**Transformers + gptqmodel на одной карте почти всегда медленнее vLLM** (PagedAttention, оптимизированные ядра, батчинг). Если нужна прежняя скорость — по возможности снова поднимайте **vLLM** с AWQ.

## Что уже сделано в `main.py`

1. **AWQ-бэкенд по умолчанию**: при `LLM_CODE_AWQ_BACKEND` не задан или `auto` перебираются **`marlin` → `gemm_triton` → `torch_awq`**. Marlin обычно сильно быстрее `torch_awq`.
2. **Attention**: `LLM_CODE_ATTN_IMPLEMENTATION=auto` (по умолчанию) — если в образе есть **flash-attn**, будет `flash_attention_2`, иначе `sdpa`.
3. **Лимиты**: дефолт `MAX_NEW_TOKENS=512`, жёсткий потолок `LLM_CODE_MAX_NEW_TOKENS_CAP=1024` (0 = отключить потолок).
4. **Лог скорости**: каждый ответ пишет `generate: Xs, new_tokens, tok/s` (выкл: `LLM_CODE_LOG_TIMING=0`).
5. **Опционально**: `LLM_CODE_TORCH_COMPILE=1` — экспериментально, может ускорить или сломать AWQ.

## Переменные

| Переменная | Значение |
|------------|----------|
| `LLM_CODE_AWQ_BACKEND` | пусто / `auto` — перебор быстрых; `torch_awq` — только медленный, но стабильный; `marlin` — только marlin |
| `LLM_CODE_ATTN_IMPLEMENTATION` | `auto`, `sdpa`, `flash_attention_2`, `eager` |
| `MAX_NEW_TOKENS` | дефолт длина ответа |
| `LLM_CODE_MAX_NEW_TOKENS_CAP` | максимум новых токенов за запрос |
| `LLM_CODE_LOG_TIMING` | `0` — не логировать время генерации |

## Flash Attention в Docker

Нужен пакет, собранный под вашу связку **CUDA + torch** (часто из исходников). Если сборка тяжёлая — оставьте `LLM_CODE_ATTN_IMPLEMENTATION=sdpa`.
