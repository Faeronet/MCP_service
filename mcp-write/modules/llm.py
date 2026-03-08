"""Вызов LLM (vLLM chat/completions) для чанкинга и саммари."""
import logging
import httpx

from . import config

log = logging.getLogger("mcp-write")


def call_llm(messages: list[dict[str, str]], max_tokens: int = 2048) -> str:
    if not config.VLLM_BASE or not config.LLM_MODEL:
        return ""
    url = f"{config.VLLM_BASE}/chat/completions"
    body = {"model": config.LLM_MODEL, "messages": messages, "max_tokens": max_tokens}
    try:
        with httpx.Client(timeout=120.0) as client:
            r = client.post(url, json=body)
        if r.status_code != 200:
            log.warning("llm request: HTTP %s %s", r.status_code, r.text[:200])
            return ""
        data = r.json()
        choices = data.get("choices") or []
        if not choices:
            return ""
        return (choices[0].get("message") or {}).get("content") or ""
    except Exception as e:
        log.warning("llm request failed: %s", e)
        return ""


def chunk_with_llm(raw: str) -> list[str]:
    if not raw or not raw.strip():
        return []
    prompt = (
        "Разбей следующий текст на логические фрагменты (чанки). "
        "Выводи только сами фрагменты, разделяй их двойным переносом строки (две пустые строки между чанками). "
        "Не добавляй нумерацию и заголовки.\n\nТекст:\n\n"
    )
    text = (raw or "")[:30000]
    content = call_llm([{"role": "user", "content": prompt + text}], max_tokens=4096)
    if not content or not content.strip():
        return []
    chunks = [p.strip() for p in content.split("\n\n") if p.strip()]
    return chunks[:50]


def summarize_with_llm(raw: str) -> str:
    if not raw or not raw.strip():
        return ""
    text = (raw or "")[:8000]
    content = call_llm([
        {"role": "user", "content": f"Одним предложением опиши основное содержание текста (для поиска по смыслу):\n\n{text}"}
    ], max_tokens=150)
    return (content or "").strip()[:500]
