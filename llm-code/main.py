from __future__ import annotations

import os
import time
import traceback
import uuid
from contextlib import asynccontextmanager
from typing import Any, Dict, List, Literal, Optional

import torch
from fastapi import FastAPI, HTTPException
from pydantic import BaseModel
from transformers import AutoConfig, AutoModelForCausalLM, AutoTokenizer
from transformers.utils.quantization_config import AwqBackend


_tokenizer = None
_model = None


def _env_flag(name: str, default_true: bool = True) -> bool:
    v = os.getenv(name)
    if v is None:
        return default_true
    return v.strip().lower() not in ("0", "false", "no", "off")


@asynccontextmanager
async def _lifespan(app: FastAPI):
    """Предзагрузка весов при старте процесса (пока Uvicorn не начнёт принимать запросы)."""
    global _tokenizer, _model
    if _env_flag("LLM_CODE_PRELOAD", default_true=True):
        print("[llm-code] LLM_CODE_PRELOAD: loading model at startup...", flush=True)
        try:
            _load_model()
        except Exception:
            print("[llm-code] startup preload failed (process will exit)", flush=True)
            raise
        print("[llm-code] model loaded; accepting traffic", flush=True)
    else:
        print(
            "[llm-code] LLM_CODE_PRELOAD=0: model will load on first /v1/chat/completions",
            flush=True,
        )
    yield


app = FastAPI(lifespan=_lifespan)

MODEL_NAME = os.getenv("LLM_CODE_MODEL") or os.getenv("LLM_MODEL", "Qwen/Qwen3-0.6B")
PORT = int(os.getenv("PORT", "8005"))

# generation params (2048 по умолчанию сильно тянет время; vLLM на том же железе обычно быстрее из‑за ядер/батчинга)
MAX_NEW_TOKENS_DEFAULT = int(os.getenv("MAX_NEW_TOKENS", "1024"))
TEMPERATURE = float(os.getenv("TEMPERATURE", "0.0"))
TOP_P = float(os.getenv("TOP_P", "1.0"))


class ChatMessage(BaseModel):
    role: Literal["system", "user", "assistant"] = "user"
    content: str


class ChatCompletionsRequest(BaseModel):
    model: Optional[str] = None
    messages: List[ChatMessage]
    max_tokens: Optional[int] = None
    chat_template_kwargs: Optional[Dict[str, Any]] = None


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


def _awq_quantization_config_dict(qc: Any, backend: AwqBackend) -> dict[str, Any] | None:
    """
    Force AWQ `backend` (e.g. torch_awq) in the quantization config.

    Must stay a plain dict: some transformers versions call
    `quantization_config.get("quant_method")` (see quantizers/auto.py), so an
    `AwqConfig` instance on `PretrainedConfig` breaks loading.
    """
    if qc is None:
        return None

    if isinstance(qc, dict):
        d = dict(qc)
    elif hasattr(qc, "to_dict"):
        d = dict(qc.to_dict())
    else:
        return None

    d["backend"] = backend.value
    # Enum / object from to_dict() → JSON-friendly strings for HF quantizer checks
    qm = d.get("quant_method")
    if qm is not None and hasattr(qm, "value"):
        d["quant_method"] = qm.value
    elif d.get("quant_method") is None:
        d["quant_method"] = "awq"

    return d


def _awq_backend_try_names() -> list[str] | None:
    """
    Список имён бэкенда по порядку. None = не трогать quantization_config (риск ExLlama).

    По умолчанию (пусто или auto): marlin → gemm_triton → torch_awq (marlin обычно намного быстрее torch_awq).
    Явно: LLM_CODE_AWQ_BACKEND=marlin или torch_awq — только один.
    exllama_v2: без override (как в чекпойнте).
    """
    raw = (os.getenv("LLM_CODE_AWQ_BACKEND") or "").strip().lower()
    if raw in ("exllama_v2", "exllamav2"):
        return None
    if raw in ("", "auto"):
        return ["marlin", "gemm_triton", "torch_awq"]
    return [raw]


def _resolve_attn_implementation() -> str:
    """
    sdpa — без лишних зависимостей.
    auto: flash_attention_2 если установлен flash-attn, иначе sdpa.
    """
    raw = (os.getenv("LLM_CODE_ATTN_IMPLEMENTATION") or "auto").strip().lower()
    if raw in ("", "auto"):
        try:
            import flash_attn  # noqa: F401

            return "flash_attention_2"
        except ImportError:
            return "sdpa"
    return raw


def _load_causal_lm(resolved: str, torch_dtype: torch.dtype, device: torch.device):
    """
    AWQ: transformers ожидает gptqmodel; для квантованных весов лучше device_map='auto' на CUDA.
    Обычные веса: одно устройство + .to(device).
    """
    awq = _is_awq_path(resolved) or _is_awq_path(MODEL_NAME)
    if awq and torch.cuda.is_available():
        load_kw: dict[str, Any] = {
            "trust_remote_code": True,
            "device_map": "auto",
            "torch_dtype": torch_dtype,
        }
        attn = _resolve_attn_implementation()
        if attn.lower() not in ("", "none", "eager"):
            load_kw["attn_implementation"] = attn
            print(f"[llm-code] attn_implementation={attn!r}", flush=True)

        try_names = _awq_backend_try_names()
        if try_names is None:
            print("[llm-code] AWQ: backend override disabled (checkpoint default)", flush=True)
            return AutoModelForCausalLM.from_pretrained(resolved, **load_kw)

        last_err: BaseException | None = None
        for name in try_names:
            try:
                backend = AwqBackend(name)
            except ValueError:
                print(f"[llm-code] skip unknown AWQ backend name {name!r}", flush=True)
                continue
            config = AutoConfig.from_pretrained(resolved, trust_remote_code=True)
            qc = getattr(config, "quantization_config", None)
            new_qc = _awq_quantization_config_dict(qc, backend)
            if new_qc is None:
                print("[llm-code] no usable quantization_config; cannot set AWQ backend", flush=True)
                last_err = RuntimeError("missing quantization_config")
                continue
            config.quantization_config = new_qc
            attempt_kw = {**load_kw, "config": config}
            print(f"[llm-code] AWQ trying backend={backend.value!r} ...", flush=True)
            try:
                m = AutoModelForCausalLM.from_pretrained(resolved, **attempt_kw)
                print(f"[llm-code] AWQ using backend={backend.value!r}", flush=True)
                return m
            except BaseException as e:
                last_err = e
                print(f"[llm-code] AWQ backend={backend.value!r} failed: {e}", flush=True)
        if last_err is not None:
            raise last_err
        raise RuntimeError("no AWQ backend could be loaded")

    attn = _resolve_attn_implementation()
    load_plain: dict[str, Any] = {
        "trust_remote_code": True,
        "torch_dtype": torch_dtype,
        "device_map": None,
    }
    if attn.lower() not in ("", "none", "eager"):
        load_plain["attn_implementation"] = attn
        print(f"[llm-code] attn_implementation={attn!r}", flush=True)
    m = AutoModelForCausalLM.from_pretrained(resolved, **load_plain)
    m.to(device)
    return m


def _load_model() -> None:
    global _tokenizer, _model
    if _tokenizer is not None and _model is not None:
        return

    # device selection: prefer GPU if present
    device = torch.device("cuda:0" if torch.cuda.is_available() else "cpu")
    torch_dtype = torch.float16 if device.type == "cuda" else torch.float32

    if device.type == "cuda":
        torch.backends.cuda.matmul.allow_tf32 = True
        torch.backends.cudnn.allow_tf32 = True
        torch.backends.cudnn.benchmark = True
        try:
            torch.set_float32_matmul_precision("high")
        except Exception:
            pass
        print(
            "[llm-code] CUDA: TF32 + cudnn.benchmark, matmul precision=high",
            flush=True,
        )

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

    if _env_flag("LLM_CODE_TORCH_COMPILE", default_true=False):
        try:
            _model = torch.compile(_model, mode="reduce-overhead")  # type: ignore[assignment]
            print("[llm-code] torch.compile(mode=reduce-overhead) applied", flush=True)
        except Exception as e:
            print(f"[llm-code] torch.compile skipped: {e}", flush=True)


def _build_prompt(
    messages: List[ChatMessage],
    chat_template_kwargs: Optional[Dict[str, Any]] = None,
) -> str:
    # Qwen3: enable_thinking=True сильно удлиняет ответ и время — по умолчанию выключаем
    # (LLM_CODE_QWEN_THINKING=1 чтобы включить «мышление» в шаблоне).
    assert _tokenizer is not None
    raw_messages = [{"role": m.role, "content": m.content} for m in messages]
    if hasattr(_tokenizer, "apply_chat_template"):
        tmpl_kw: Dict[str, Any] = {}
        if _env_flag("LLM_CODE_QWEN_THINKING", default_true=False):
            tmpl_kw["enable_thinking"] = True
        else:
            tmpl_kw["enable_thinking"] = False
        if chat_template_kwargs:
            tmpl_kw.update(chat_template_kwargs)
        try:
            return _tokenizer.apply_chat_template(
                raw_messages,
                tokenize=False,
                add_generation_prompt=True,
                chat_template_kwargs=tmpl_kw,
            )
        except (TypeError, ValueError):
            return _tokenizer.apply_chat_template(
                raw_messages, tokenize=False, add_generation_prompt=True
            )
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
    if MAX_NEW_TOKENS_CAP > 0 and max_new > MAX_NEW_TOKENS_CAP:
        print(
            f"[llm-code] capping max_new_tokens {max_new} -> {MAX_NEW_TOKENS_CAP} (LLM_CODE_MAX_NEW_TOKENS_CAP)",
            flush=True,
        )
        max_new = MAX_NEW_TOKENS_CAP

    prompt = _build_prompt(req.messages, req.chat_template_kwargs)

    try:
        inputs = _tokenizer(prompt, return_tensors="pt")
        model_device = next(_model.parameters()).device
        inputs = {k: v.to(model_device) for k, v in inputs.items()}
        input_ids = inputs["input_ids"]

        gen_kw: Dict[str, Any] = {
            **inputs,
            "max_new_tokens": max_new,
            "do_sample": TEMPERATURE > 0,
            "use_cache": True,
            "pad_token_id": _tokenizer.eos_token_id,
            "eos_token_id": _tokenizer.eos_token_id,
        }
        if TEMPERATURE > 0:
            gen_kw["temperature"] = TEMPERATURE
            gen_kw["top_p"] = TOP_P

        t0 = time.monotonic()
        with torch.inference_mode():
            outputs = _model.generate(**gen_kw)
        dt = time.monotonic() - t0
        n_out = int(outputs.shape[1] - input_ids.shape[1])
        if _env_flag("LLM_CODE_LOG_TIMING", default_true=True):
            tps = n_out / dt if dt > 0 else 0.0
            print(
                f"[llm-code] generate: {dt:.2f}s, new_tokens≈{n_out}, ~{tps:.2f} tok/s",
                flush=True,
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

