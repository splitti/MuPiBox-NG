#!/usr/bin/env python3
"""MCP stdio server exposing local Ollama/Qwen as a read-only analysis worker.

Goal: keep large file/diff/log content out of Claude's (Anthropic) context by
letting this server read repository files and run `git diff` itself, on the
DEV-LXC, and only return Qwen's compact findings. See CLAUDE.md, section
"lokale KI".

No secrets required. Configuration via environment variables:
  LOCAL_AI_URL         Ollama base URL (default: http://192.168.3.45:11434)
  LOCAL_AI_MODEL       Ollama model name (default: qwen3:8b)
  LOCAL_AI_NUM_CTX     Context window in tokens (default: 16384 - the RTX 5050 8GB
                       cannot hold all qwen3:8b layers plus a 24576 KV cache in VRAM,
                       forcing partial CPU offload; 16384 keeps it GPU-resident)
  LOCAL_AI_NUM_PREDICT Max output tokens (default: 1200 - keeps replies compact)
  LOCAL_AI_TIMEOUT     Request timeout in seconds (default: 600)
  LOCAL_AI_KEEP_ALIVE  How long Ollama keeps the model loaded in VRAM between
                       calls (default: 30m - avoids reload cost across tasks)
  LOCAL_AI_REPO_ROOT   Repository root local_ai may read from (default: /opt/mupibox-ng)

Security model (read-only worker, see CLAUDE.md):
  - File/diff access is hard-restricted to LOCAL_AI_REPO_ROOT (symlink- and
    traversal-safe via Path.resolve() + relative_to() containment check).
  - Filenames/paths matching common secret patterns (.env, .ssh/, id_rsa*,
    *.pem, *.key, *credential*, *secret*, .netrc, ...) are refused outright.
  - This server never writes files, never runs arbitrary shell commands, and
    never executes anything Qwen returns. `git diff` is the only subprocess,
    with a fixed, non-shell argv.
"""

import fnmatch
import json
import os
import re
import subprocess
import urllib.error
import urllib.request
from pathlib import Path

from mcp.server.fastmcp import FastMCP

OLLAMA_URL = os.environ.get("LOCAL_AI_URL", "http://192.168.3.45:11434").rstrip("/")
MODEL = os.environ.get("LOCAL_AI_MODEL", "qwen3:8b")
NUM_CTX = int(os.environ.get("LOCAL_AI_NUM_CTX", "16384"))
NUM_PREDICT = int(os.environ.get("LOCAL_AI_NUM_PREDICT", "1200"))
TIMEOUT = int(os.environ.get("LOCAL_AI_TIMEOUT", "600"))
KEEP_ALIVE = os.environ.get("LOCAL_AI_KEEP_ALIVE", "30m")
REPO_ROOT = Path(os.environ.get("LOCAL_AI_REPO_ROOT", "/opt/mupibox-ng")).resolve()

MAX_CONTEXT_CHARS = 60000  # total prompt-context guard sent to Ollama
MAX_FILE_CHARS = 20000  # per-file guard for review_files
MAX_FILES = 12  # max number of files per review_files call
GIT_DIFF_TIMEOUT = 30  # seconds, local git subprocess

SECRET_DIR_NAMES = {".ssh", ".gnupg", ".aws"}
SECRET_NAME_PATTERNS = (
    "*.pem",
    "*.key",
    "*.p12",
    "*.pfx",
    "id_rsa*",
    "id_ed25519*",
    "id_ecdsa*",
    "*credential*",
    "*secret*",
    ".env",
    ".env.*",
    ".netrc",
    "*.crt",
)

COMPACT_INSTRUCTION = (
    "Antworte ausschliesslich im Format 'Finding | Datei/Zeile | Severity | "
    "Empfehlung', eine Zeile je Fund, Severity in {low, medium, high}. Wenn "
    "keine relevanten Probleme gefunden wurden, antworte nur mit einem "
    "kurzen Bestaetigungssatz ohne Tabelle. Keine langen Erklaerungen."
)

mcp = FastMCP("local-ai")


class LocalAiError(ValueError):
    """Raised for rejected paths/inputs; message is safe to return to the caller."""


def _reject_secret_path(path: Path) -> None:
    parts_lower = {p.lower() for p in path.parts}
    if parts_lower & SECRET_DIR_NAMES:
        raise LocalAiError(f"Zugriff auf Secret-Verzeichnis abgelehnt: {path}")
    name_lower = path.name.lower()
    for pattern in SECRET_NAME_PATTERNS:
        if fnmatch.fnmatch(name_lower, pattern):
            raise LocalAiError(f"Zugriff auf vermutliche Secret-Datei abgelehnt: {path}")


def _resolve_repo_path(rel_path: str, must_exist: bool) -> Path:
    if not rel_path or not isinstance(rel_path, str):
        raise LocalAiError("Ungueltiger (leerer) Pfad.")
    if os.path.isabs(rel_path):
        raise LocalAiError(f"Nur repository-relative Pfade erlaubt: {rel_path!r}")
    candidate = (REPO_ROOT / rel_path).resolve()
    try:
        candidate.relative_to(REPO_ROOT)
    except ValueError as exc:
        raise LocalAiError(
            f"Pfad ausserhalb des Repositories abgelehnt: {rel_path!r}"
        ) from exc
    _reject_secret_path(candidate)
    if must_exist and not candidate.is_file():
        raise LocalAiError(f"Datei nicht gefunden oder kein regulaerer File: {rel_path!r}")
    return candidate


def _truncate(text: str, limit: int) -> str:
    if len(text) > limit:
        return text[:limit] + "\n...[gekuerzt]..."
    return text


def _call_ollama(prompt: str) -> str:
    payload = {
        "model": MODEL,
        "prompt": prompt,
        "stream": False,
        "think": False,
        "keep_alive": KEEP_ALIVE,
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
        return f"local_ai Fehler: Zeitueberschreitung nach {TIMEOUT}s bei Modell {MODEL}."
    except Exception as exc:  # noqa: BLE001 - surface any unexpected failure to caller
        return f"local_ai Fehler: {exc}"

    text = body.get("response", "")
    # qwen3 emits <think>...</think> reasoning blocks; strip them for compact results.
    while "<think>" in text and "</think>" in text:
        start = text.index("<think>")
        end = text.index("</think>") + len("</think>")
        text = text[:start] + text[end:]
    return text.strip() or "local_ai: leere Antwort vom Modell."


@mcp.tool()
def local_ai(prompt: str, context: str = "") -> str:
    """Delegate a free-form analysis/review/summarization task to Qwen (qwen3:8b).

    Generic fallback tool. Prefer review_files / review_diff / analyze_log /
    analyze_command_output when the task fits one of those, since they read
    repository content locally instead of requiring the caller to paste it.
    Do NOT send secrets, credentials, or private keys in `prompt` or `context`.

    Args:
        prompt: The instruction/question for the local model.
        context: Optional supporting text (code, diff, log excerpt) to analyze.

    Returns:
        The model's response text (thinking tags stripped), or a clear error message.
    """
    if context:
        context = _truncate(context, MAX_CONTEXT_CHARS)
        full_prompt = f"{prompt}\n\n---\nKontext:\n{context}"
    else:
        full_prompt = prompt
    return _call_ollama(full_prompt)


@mcp.tool()
def review_files(paths: list[str], question: str = "") -> str:
    """Read up to 12 repository files locally and have Qwen review them.

    The caller does NOT need to read the file contents first - this tool
    reads them server-side from the MuPiBox-NG repository
    (LOCAL_AI_REPO_ROOT, default /opt/mupibox-ng) and only sends them to the
    local Ollama model. Only repository-relative paths are accepted; path
    traversal, absolute paths, and known secret file patterns
    (.env, .ssh/, id_rsa*, *.pem, *.key, *credential*, *secret*, ...) are
    rejected before anything is read.

    Args:
        paths: Repository-relative file paths to review (max 12).
        question: Optional focus for the review (e.g. "check error handling").
            Defaults to a general bug/dead-code/inconsistency review.

    Returns:
        Qwen's compact findings ("Finding | Datei/Zeile | Severity |
        Empfehlung"), or a clear error message if a path was rejected.
    """
    if not paths:
        return "local_ai Fehler: keine Dateipfade uebergeben."
    if len(paths) > MAX_FILES:
        return f"local_ai Fehler: maximal {MAX_FILES} Dateien pro Aufruf, {len(paths)} uebergeben."

    blocks = []
    total_len = 0
    for rel_path in paths:
        try:
            resolved = _resolve_repo_path(rel_path, must_exist=True)
            content = resolved.read_text(encoding="utf-8", errors="replace")
        except LocalAiError as exc:
            return f"local_ai Fehler: {exc}"
        except OSError as exc:
            return f"local_ai Fehler beim Lesen von {rel_path!r}: {exc}"
        content = _truncate(content, MAX_FILE_CHARS)
        block = f"=== {rel_path} ===\n{content}"
        total_len += len(block)
        if total_len > MAX_CONTEXT_CHARS:
            blocks.append(f"=== {rel_path} === [uebersprungen, Kontextlimit erreicht]")
            break
        blocks.append(block)

    task = question.strip() or (
        "Pruefe die folgenden Dateien auf offensichtliche Bugs, Dead Code, "
        "Inkonsistenzen und riskante Stellen."
    )
    prompt = f"{task}\n\n{COMPACT_INSTRUCTION}\n\n" + "\n\n".join(blocks)
    return _call_ollama(prompt)


@mcp.tool()
def review_diff(paths: list[str] | None = None, staged: bool = False, question: str = "") -> str:
    """Run a local, scoped `git diff` in the repository and have Qwen review it.

    The MCP server runs `git diff` itself (no shell, fixed argv) so the
    caller does not need to paste the diff. Uses `--function-context` so
    Qwen sees the full surrounding function for each changed hunk, not just
    bare hunks, to reduce out-of-context false positives. This is never a
    full-repository analysis: pass `paths` to scope it further; without
    `paths`, it is still limited to the current working-tree diff (or the
    staged diff if `staged=True`), not repository history.

    Args:
        paths: Optional repository-relative paths to limit the diff to.
        staged: If True, diff staged changes (`git diff --cached`) instead
            of the working tree.
        question: Optional focus for the review. Defaults to a general
            correctness/bug review.

    Returns:
        Qwen's compact findings, or a clear error message.
    """
    cmd = ["git", "-C", str(REPO_ROOT), "diff", "--function-context"]
    if staged:
        cmd.append("--cached")
    if paths:
        try:
            validated = [
                str(_resolve_repo_path(p, must_exist=False).relative_to(REPO_ROOT))
                for p in paths
            ]
        except LocalAiError as exc:
            return f"local_ai Fehler: {exc}"
        cmd.append("--")
        cmd.extend(validated)

    try:
        result = subprocess.run(
            cmd,
            cwd=REPO_ROOT,
            capture_output=True,
            text=True,
            timeout=GIT_DIFF_TIMEOUT,
            check=False,
        )
    except (OSError, subprocess.SubprocessError) as exc:
        return f"local_ai Fehler: git diff konnte nicht ausgefuehrt werden ({exc})."

    if result.returncode != 0:
        return f"local_ai Fehler: git diff schlug fehl: {result.stderr.strip()[:500]}"
    diff_text = result.stdout.strip()
    if not diff_text:
        return "local_ai: kein Diff gefunden (keine Aenderungen im angefragten Bereich)."
    diff_text = _truncate(diff_text, MAX_CONTEXT_CHARS)

    task = question.strip() or (
        "Review dieses Git-Diffs (mit Funktionskontext) auf Bugs, riskante "
        "Aenderungen und Inkonsistenzen."
    )
    prompt = f"{task}\n\n{COMPACT_INSTRUCTION}\n\n{diff_text}"
    return _call_ollama(prompt)


_LOG_MATCH_RE = re.compile(
    r"error|warn|fail|exception|panic|fatal|critical|refused|denied|timeout",
    re.IGNORECASE,
)


def _filter_log(text: str, context_lines: int = 3, max_lines: int = 1500) -> str:
    """Keep lines around error/warning matches; fall back to the tail if none match."""
    lines = text.splitlines()
    if len(lines) <= max_lines:
        candidate_lines = lines
    else:
        match_indices = [i for i, line in enumerate(lines) if _LOG_MATCH_RE.search(line)]
        if not match_indices:
            candidate_lines = lines[-max_lines:]
        else:
            keep = set()
            for idx in match_indices:
                for i in range(max(0, idx - context_lines), min(len(lines), idx + context_lines + 1)):
                    keep.add(i)
            candidate_lines = []
            prev = None
            for i in sorted(keep):
                if prev is not None and i != prev + 1:
                    candidate_lines.append("...")
                candidate_lines.append(lines[i])
                prev = i
            if len(candidate_lines) > max_lines:
                candidate_lines = candidate_lines[-max_lines:]
    return "\n".join(candidate_lines)


@mcp.tool()
def analyze_log(log_text: str, focus: str = "") -> str:
    """Pre-filter a (potentially large) log and have Qwen summarize errors/warnings.

    Intended for journalctl/service logs. Large input is filtered to the
    lines around error/warning/failure matches (with surrounding context so
    Qwen is not misled by out-of-context lines) plus the tail if nothing
    matches, then truncated to a hard size limit before being sent to Qwen.

    Args:
        log_text: Raw log content (do not include secrets/credentials).
        focus: Optional focus, e.g. "audio startup failures" or a time window.

    Returns:
        Qwen's compact findings, or a clear error message.
    """
    if not log_text or not log_text.strip():
        return "local_ai Fehler: kein Logtext uebergeben."
    filtered = _filter_log(log_text)
    filtered = _truncate(filtered, MAX_CONTEXT_CHARS)

    task = (
        f"Analysiere dieses Log mit Fokus auf: {focus.strip()}."
        if focus.strip()
        else "Analysiere dieses Log. Fokus auf Errors, Warnings, Failures und zeitlich "
        "auffaellige Ereignisse."
    )
    prompt = f"{task}\n\n{COMPACT_INSTRUCTION}\n\n{filtered}"
    return _call_ollama(prompt)


@mcp.tool()
def analyze_command_output(output_text: str, tool: str = "") -> str:
    """Have Qwen analyze the output of a diagnostic command and return compact findings.

    For output of systemctl, journalctl, dmesg, network (wpa_cli/nmcli/ip),
    audio (aplay/amixer), display (KMS/DRM), GPIO, or DietPi tools. Unlike
    analyze_log this does not error-filter by default (status/inventory
    output is usually short and fully relevant), only size-truncates.

    Args:
        output_text: The raw command output (do not include secrets/tokens).
        tool: Optional name of the source command, e.g. "systemctl status
            mupibox-ng" or "dietpi-display", for better context.

    Returns:
        Qwen's compact findings, or a clear error message.
    """
    if not output_text or not output_text.strip():
        return "local_ai Fehler: keine Ausgabe uebergeben."
    text = _truncate(output_text.strip(), MAX_CONTEXT_CHARS)

    tool_hint = f" von `{tool.strip()}`" if tool.strip() else ""
    task = (
        f"Analysiere diese Diagnose-Ausgabe{tool_hint} eines DietPi/Raspberry-Pi-Systems "
        "(MuPiBox-NG-Testgeraet). Nenne auffaellige Zustaende, Fehler oder Risiken."
    )
    prompt = f"{task}\n\n{COMPACT_INSTRUCTION}\n\n{text}"
    return _call_ollama(prompt)


if __name__ == "__main__":
    mcp.run()
