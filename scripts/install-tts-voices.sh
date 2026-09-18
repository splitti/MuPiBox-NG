#!/usr/bin/env bash
# Downloads the Piper voices listed in internal/tts/manifest/voices.json and
# registers them in the MuPiBox-NG database. Safe to re-run: already
# installed voices are skipped, and a single failed download never aborts
# the rest of the run (see docs/tts.md).
set -euo pipefail

if [[ ${EUID:-$(id -u)} -ne 0 ]]; then
    echo "Run as root: sudo bash scripts/install-tts-voices.sh" >&2
    exit 1
fi

REPO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
VOICES_DIR="${MUPIBOX_TTS_VOICES_DIR:-/usr/local/share/mupibox-ng/voices}"
CONFIG="${MUPIBOX_CONFIG:-/etc/mupibox-ng/config.json}"
BIN="${MUPIBOX_BIN:-/usr/local/lib/mupibox-ng/mupibox}"
MANDATORY_ONLY=0

for arg in "$@"; do
    case "$arg" in
        --mandatory-only) MANDATORY_ONLY=1 ;;
        -h|--help)
            cat <<'EOF'
Usage: sudo bash scripts/install-tts-voices.sh [options]

Options:
  --mandatory-only  Only install the default voices for the main languages
                     (de, en-GB, en-US, fr, es, nl, sv, ru), skip the rest.

Environment overrides:
  MUPIBOX_TTS_VOICES_DIR  Where voice files are stored (default: /usr/local/share/mupibox-ng/voices)
  MUPIBOX_CONFIG          mupibox config.json used for -tts-sync-voices (default: /etc/mupibox-ng/config.json)
  MUPIBOX_BIN             mupibox binary to use (default: /usr/local/lib/mupibox-ng/mupibox,
                           falls back to ./bin/mupibox in this checkout)
EOF
            exit 0
            ;;
        *) echo "Unknown option: $arg" >&2; exit 2 ;;
    esac
done

if [[ ! -x "$BIN" ]]; then
    if [[ -x "$REPO_DIR/bin/mupibox" ]]; then
        BIN="$REPO_DIR/bin/mupibox"
    else
        echo "mupibox binary not found at $BIN or $REPO_DIR/bin/mupibox; build it first (see CLAUDE.md)." >&2
        exit 1
    fi
fi

mkdir -p "$VOICES_DIR"

installed=0
skipped=0
failed_mandatory=0

while IFS=$'\t' read -r id language family quality tier model_url config_url license source_url speaker_id; do
    [[ -z "$id" ]] && continue
    if [[ "$MANDATORY_ONLY" -eq 1 && "$tier" != "mandatory" ]]; then
        continue
    fi
    model_path="$VOICES_DIR/$id.onnx"
    config_path="$VOICES_DIR/$id.onnx.json"
    if [[ -s "$model_path" && -s "$config_path" ]]; then
        echo "Already installed: $id"
        continue
    fi

    echo "Installing voice $id ($language, $quality, tier=$tier, license=$license)"
    tmp_model="$(mktemp "$VOICES_DIR/.${id}.onnx.XXXXXX")"
    tmp_config="$(mktemp "$VOICES_DIR/.${id}.onnx.json.XXXXXX")"
    ok=1
    if ! curl -fL --retry 3 "$model_url" -o "$tmp_model"; then
        ok=0
    fi
    if [[ "$ok" -eq 1 ]] && ! curl -fL --retry 3 "$config_url" -o "$tmp_config"; then
        ok=0
    fi

    if [[ "$ok" -eq 1 && -s "$tmp_model" && -s "$tmp_config" ]]; then
        mv "$tmp_model" "$model_path"
        mv "$tmp_config" "$config_path"
        installed=$((installed + 1))
    else
        echo "WARNING: could not download voice $id ($language); skipping, installation continues." >&2
        rm -f "$tmp_model" "$tmp_config"
        skipped=$((skipped + 1))
        if [[ "$tier" == "mandatory" ]]; then
            failed_mandatory=$((failed_mandatory + 1))
        fi
    fi
done < <("$BIN" -tts-print-manifest)

echo "Voices installed: $installed, skipped: $skipped"

if [[ -f "$CONFIG" ]]; then
    "$BIN" -config "$CONFIG" -tts-sync-voices "$VOICES_DIR"
else
    echo "Config $CONFIG not found; skipping database sync (run with MUPIBOX_CONFIG=... once the service is configured)." >&2
fi

if [[ "$failed_mandatory" -gt 0 ]]; then
    echo "WARNING: $failed_mandatory mandatory voice(s) could not be installed. Re-run this script later to retry them; MuPiBox-NG keeps running without them." >&2
fi
exit 0
