from __future__ import annotations

import os
import traceback
import uuid
from typing import Any, Dict, List, Literal, Optional

import torch
from fastapi import FastAPI, HTTPException
from pydantic import BaseModel, Field
from transformers import AutoModelForCausalLM, AutoTokenizer


app = FastAPI()

MODEL_NAME = os.getenv("LLM_CODE_MODEL") or os.getenv("LLM_MODEL", "Qwen/Qwen3-0.6B")
PORT = int(os.getenv("PORT", "8005"))

# generation params
MAX_NEW_TOKENS_DEFAULT = int(os.getenv("MAX_NEW_TOKENS", "2048"))
TEMPERATURE = float(os.getenv("TEMPERATURE", "0.0"))
TOP_P = float(os.getenv("TOP_P", "1.0"))


class ChatMessage(BaseModel):
    role: Literal["system", "user", "assistant"] = "user"
    content: str


class ChatCompletionsRequest(BaseModel):
    model: Optional[str] = None
    messages: List[ChatMessage]
    max_tokens: Optional[int] = None
    # mcp-proxy sends this extra field; we ignore it.
    chat_template_kwargs: Optional[Dict[str, Any]] = None


_tokenizer = None
_model = None


def _resolve_hf_hub_snapshot(models_dir: str, model_id: str) -> Optional[str]:
    """
    Hugging Face cache layout (как на скриншоте):
      {models_dir}/hub/models--{org}--{repo}/snapshots/{revision_hash}/
    """
    if "/" not in model_id or model_id.startswith("/"):
        return None
    org, repo = model_id.split("/", 1)
    cache_folder = f"models--{org}--{repo.replace('/', '--')}"
    snapshots_root = os.path.join(models_dir, "hub", cache_folder, "snapshots")
    if not os.path.isdir(snapshots_root):
        return None
    subdirs = [
        os.path.join(snapshots_root, d)
        for d in os.listdir(snapshots_root)
        if os.path.isdir(os.path.join(snapshots_root, d))
    ]
    if not subdirs:
        return None
    subdirs.sort(key=lambda p: os.path.getmtime(p), reverse=True)
    snap = subdirs[0]
    if os.path.isfile(os.path.join(snap, "config.json")):
        return snap
    return snap


def _resolve_model_name(model_name: str) -> str:
    """
    Try to resolve a HF model id to a local directory under /models.
    Поддержка: HF hub cache (models/hub/models--Org--Name/snapshots/...), плоские папки.
    """
    if os.path.isabs(model_name) and os.path.isdir(model_name):
        return model_name

    models_dir = os.getenv("MODELS_DIR", "/models")

    # 1) Стандартный кэш HF: models/hub/models--Qwen--Qwen3-14B-AWQ/snapshots/<hash>
    snap = _resolve_hf_hub_snapshot(models_dir, model_name)
    if snap:
        return snap

    last = model_name.split("/")[-1] if "/" in model_name else model_name
    candidates = [
        os.path.join(models_dir, model_name),  # /models/Qwen/Qwen3-14B-AWQ
        os.path.join(models_dir, last),  # /models/Qwen3-14B-AWQ
        os.path.join(models_dir, model_name.replace("/", "_")),  # /models/Qwen_Qwen3-14B-AWQ
        os.path.join(models_dir, model_name.replace("/", "--")),  # /models/Qwen--Qwen3-14B-AWQ
    ]
    for c in candidates:
        if os.path.isdir(c) and os.path.isfile(os.path.join(c, "config.json")):
            return c
    return model_name


def _is_awq_path(path_or_id: str) -> bool:
    p = path_or_id.lower()
    return "awq" in p


def _load_causal_lm(resolved: str, torch_dtype: torch.dtype, device: torch.device):
    """
    AWQ: transformers ожидает gptqmodel; для квантованных весов лучше device_map='auto' на CUDA.
    Обычные веса: одно устройство + .to(device).
    """
    awq = _is_awq_path(resolved) or _is_awq_path(MODEL_NAME)
    if awq and torch.cuda.is_available():
        return AutoModelForCausalLM.from_pretrained(
            resolved,
            trust_remote_code=True,
            device_map="auto",
            torch_dtype=torch_dtype,
        )
    m = AutoModelForCausalLM.from_pretrained(
        resolved,
        trust_remote_code=True,
        torch_dtype=torch_dtype,
        device_map=None,
    )
    m.to(device)
    return m


def _load_model() -> None:
    global _tokenizer, _model
    if _tokenizer is not None and _model is not None:
        return

    # device selection: prefer GPU if present
    device = torch.device("cuda:0" if torch.cuda.is_available() else "cpu")
    torch_dtype = torch.float16 if device.type == "cuda" else torch.float32

    resolved_model_name = _resolve_model_name(MODEL_NAME)

    # Helpful startup log: confirm CUDA visibility from inside the container.
    try:
        cuda_ok = torch.cuda.is_available()
        dev_count = torch.cuda.device_count() if cuda_ok else 0
        msg = f"[llm-code] cuda_available={cuda_ok} device_count={dev_count} resolved_model_name={resolved_model_name}"
        if cuda_ok and dev_count > 0:
            cur = torch.cuda.current_device()
            msg += f" current_device={cur} name={torch.cuda.get_device_name(cur)}"
        print(msg, flush=True)
    except Exception as e:
        print(f"[llm-code] cuda probe failed: {e}", flush=True)

    _tokenizer = AutoTokenizer.from_pretrained(resolved_model_name, trust_remote_code=True)
    if _tokenizer.pad_token_id is None:
        _tokenizer.pad_token_id = _tokenizer.eos_token_id

    try:
        _model = _load_causal_lm(resolved_model_name, torch_dtype, device)
    except Exception as e:
        print(f"[llm-code] model load failed for {resolved_model_name}: {e}", flush=True)
        traceback.print_exc()
        raise

    _model.eval()


def _build_prompt(messages: List[ChatMessage]) -> str:
    # Most chat-capable models (Qwen) support apply_chat_template.
    # Fall back to a simple concatenation if template is not available.
    assert _tokenizer is not None
    raw_messages = [{"role": m.role, "content": m.content} for m in messages]
    if hasattr(_tokenizer, "apply_chat_template"):
        return _tokenizer.apply_chat_template(raw_messages, tokenize=False, add_generation_prompt=True)
    return "\n".join([f"{m['role']}: {m['content']}" for m in raw_messages]) + "\nassistant:"


@app.get("/health")
def health() -> Dict[str, str]:
    return {"status": "ok"}


@app.post("/v1/chat/completions")
def chat_completions(req: ChatCompletionsRequest) -> Dict[str, Any]:
    try:
        _load_model()
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"model load failed: {e}")

    if not req.messages:
        raise HTTPException(status_code=400, detail="messages required")

    max_new = req.max_tokens or MAX_NEW_TOKENS_DEFAULT
    max_new = max(1, int(max_new))

    prompt = _build_prompt(req.messages)

    try:
        inputs = _tokenizer(prompt, return_tensors="pt")
        model_device = next(_model.parameters()).device
        inputs = {k: v.to(model_device) for k, v in inputs.items()}
        input_ids = inputs["input_ids"]

        with torch.no_grad():
            outputs = _model.generate(
                **inputs,
                max_new_tokens=max_new,
                do_sample=(TEMPERATURE > 0),
                temperature=TEMPERATURE if TEMPERATURE > 0 else None,
                top_p=TOP_P if TEMPERATURE > 0 else None,
                pad_token_id=_tokenizer.eos_token_id,
                eos_token_id=_tokenizer.eos_token_id,
            )

        gen_ids = outputs[0][input_ids.shape[-1] :]
        text = _tokenizer.decode(gen_ids, skip_special_tokens=True).strip()
        if text == "":
            text = _tokenizer.decode(outputs[0], skip_special_tokens=True).strip()
    except Exception as e:
        print(f"[llm-code] generate failed: {e}", flush=True)
        traceback.print_exc()
        raise HTTPException(status_code=500, detail=str(e))

    return {
        "id": f"chatcmpl-{uuid.uuid4().hex}",
        "object": "chat.completion",
        "created": 0,
        "model": req.model or MODEL_NAME,
        "choices": [
            {
                "index": 0,
                "message": {"role": "assistant", "content": text},
                "finish_reason": "stop",
            }
        ],
        "usage": {
            "prompt_tokens": int(input_ids.shape[-1]),
            "completion_tokens": int(gen_ids.shape[-1]),
            "total_tokens": int(input_ids.shape[-1] + gen_ids.shape[-1]),
        },
    }

