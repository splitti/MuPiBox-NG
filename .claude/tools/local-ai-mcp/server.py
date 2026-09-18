#!/usr/bin/env python3
"""Minimal MCP stdio server exposing local Ollama/Qwen as a worker tool.

No secrets required. Configuration via environment variables:
  LOCAL_AI_URL         Ollama base URL (default: http://192.168.3.45:11434)
  LOCAL_AI_MODEL       Ollama model name (default: qwen3:8b)
  LOCAL_AI_NUM_CTX     Context window in tokens (default: 16384 - the RTX 5050 8GB
                       cannot hold all qwen3:8b layers plus a 24576 KV cache in VRAM,
                       forcing partial CPU offload; 16384 keeps it GPU-resident)
  LOCAL_AI_NUM_PREDICT Max output tokens (default: 1200 - keeps replies compact)
  LOCAL_AI_TIMEOUT     Request timeout in seconds (default: 600)
"""

import os
import urllib.request
import urllib.error
import json

from mcp.server.fastmcp import FastMCP

OLLAMA_URL = os.environ.get("LOCAL_AI_URL", "http://192.168.3.45:11434").rstrip("/")
MODEL = os.environ.get("LOCAL_AI_MODEL", "qwen3:8b")
NUM_CTX = int(os.environ.get("LOCAL_AI_NUM_CTX", "16384"))
NUM_PREDICT = int(os.environ.get("LOCAL_AI_NUM_PREDICT", "1200"))
TIMEOUT = int(os.environ.get("LOCAL_AI_TIMEOUT", "600"))
MAX_CONTEXT_CHARS = 60000  # guard against accidentally dumping huge files/logs

mcp = FastMCP("local-ai")


@mcp.tool()
def local_ai(prompt: str, context: str = "") -> str:
    """Delegate an analysis/review/summarization task to the local Ollama model (qwen3:8b).

    Use this for cost-saving pre-analysis: file/diff/log review, error triage, test
    suggestions, finding relevant code locations, summarizing verbose command output,
    or simple implementation drafts. Do NOT send secrets, credentials, or private keys
    in `prompt` or `context`. Keep `context` targeted (specific file/diff/log excerpt),
    not an entire repository dump.

    Args:
        prompt: The instruction/question for the local model.
        context: Optional supporting text (code, diff, log excerpt) to analyze.

    Returns:
        The model's response text (thinking tags stripped), or a clear error message.
    """
    if context:
        if len(context) > MAX_CONTEXT_CHARS:
            context = context[:MAX_CONTEXT_CHARS] + "\n...[gekürzt]..."
        full_prompt = f"{prompt}\n\n---\nKontext:\n{context}"
    else:
        full_prompt = prompt

    payload = {
        "model": MODEL,
        "prompt": full_prompt,
        "stream": False,
        "think": False,
        "options": {"num_ctx": NUM_CTX, "num_predict": NUM_PREDICT},
    }
    req = urllib.request.Request(
        f"{OLLAMA_URL}/api/generate",
        data=json.dumps(payload).encode("utf-8"),
        headers={"Content-Type": "application/json"},
        method="POST",
    )
    try:
        with urllib.request.urlopen(req, timeout=TIMEOUT) as resp:
            body = json.loads(resp.read().decode("utf-8"))
    except urllib.error.URLError as exc:
        return f"local_ai Fehler: Ollama unter {OLLAMA_URL} nicht erreichbar ({exc})."
    except TimeoutError:
        return f"local_ai Fehler: Zeitüberschreitung nach {TIMEOUT}s bei Modell {MODEL}."
    except Exception as exc:  # noqa: BLE001 - surface any unexpected failure to caller
        return f"local_ai Fehler: {exc}"

    text = body.get("response", "")
    # qwen3 emits <think>...</think> reasoning blocks; strip them for compact results.
    while "<think>" in text and "</think>" in text:
        start = text.index("<think>")
        end = text.index("</think>") + len("</think>")
        text = text[:start] + text[end:]
    return text.strip() or "local_ai: leere Antwort vom Modell."


if __name__ == "__main__":
    mcp.run()
