#!/usr/bin/env bash
# Installs the Piper TTS engine (OHF-Voice/piper1-gpl, PyPI package
# "piper-tts") into a dedicated venv under /usr/local/lib/mupibox-ng.
#
# rhasspy/piper (the old project, self-contained binary releases) was
# archived 2025-10-06; development moved to OHF-Voice/piper1-gpl, which
# ships only as a Python package invoked as "python3 -m piper", not a
# standalone binary. A venv keeps piper-tts and its dependencies
# (onnxruntime, ...) isolated from the system Python (required on
# Debian/DietPi 12+ due to PEP 668 "externally managed environment") and
# from MuPiBox-NG's own Go toolchain.
#
# This only installs the engine; voices are handled separately by
# scripts/install-tts-voices.sh. Idempotent: safe to re-run to update.
set -euo pipefail

if [[ ${EUID:-$(id -u)} -ne 0 ]]; then
    echo "Run as root: sudo bash scripts/install-piper-engine.sh" >&2
    exit 1
fi

VENV_DIR="${MUPIBOX_PIPER_VENV:-/usr/local/lib/mupibox-ng/piper-venv}"

if ! command -v python3 >/dev/null 2>&1; then
    echo "python3 is required but not found." >&2
    exit 1
fi

PYTHON_VERSION="$(python3 -c 'import sys; print(f"{sys.version_info.major}.{sys.version_info.minor}")')"
if ! python3 -c 'import venv' >/dev/null 2>&1; then
    if apt-cache show "python${PYTHON_VERSION}-venv" >/dev/null 2>&1; then
        DEBIAN_FRONTEND=noninteractive apt-get install -y "python${PYTHON_VERSION}-venv"
    else
        DEBIAN_FRONTEND=noninteractive apt-get install -y python3-venv
    fi
fi

install -d -m 0755 "$(dirname "$VENV_DIR")"
if [[ ! -x "$VENV_DIR/bin/python3" ]]; then
    python3 -m venv "$VENV_DIR"
fi

"$VENV_DIR/bin/pip" install --quiet --upgrade pip
"$VENV_DIR/bin/pip" install --quiet --upgrade piper-tts

VERSION="$("$VENV_DIR/bin/python3" -c "import importlib.metadata as m; print(m.version('piper-tts'))")"
echo "piper-tts $VERSION installed at $VENV_DIR"
echo "Set tts_piper_python to $VENV_DIR/bin/python3 in config.json (this is already the built-in default)."
