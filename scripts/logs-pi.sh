#!/usr/bin/env bash
# Fetch recent MuPiBox-NG service logs (mupibox-ng/-ui/-system-agent) from the
# Test-Pi. Usage: scripts/logs-pi.sh [host] [-- journalctl-args...]
#
# Default output is capped at 200 lines to stay reasonably sized. For larger
# or open-ended ranges (e.g. "--since -1h"), pipe the result through the
# local-ai analyze_log tool instead of reading it all directly - see
# CLAUDE.md, section "lokale KI".
set -euo pipefail

host="${1:-${MUPIBOX_PI_HOST:-mupibox-test}}"
shift || true
[ "${1:-}" = "--" ] && shift || true

units="-u mupibox-ng -u mupibox-ui -u mupibox-system-agent"

if [ "$#" -eq 0 ]; then
  ssh "$host" "journalctl $units -n 200 --no-pager"
else
  ssh "$host" "journalctl $units $*"
fi
