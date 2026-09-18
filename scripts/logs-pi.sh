#!/usr/bin/env bash
# Tail or dump recent mupibox-ng service logs from the Test-Pi.
# Usage: scripts/logs-pi.sh [host] [-- journalctl-args...]
set -euo pipefail

host="${1:-${MUPIBOX_PI_HOST:-mupibox-test}}"
shift || true
[ "${1:-}" = "--" ] && shift || true

if [ "$#" -eq 0 ]; then
  ssh "$host" "journalctl -u mupibox-ng -n 200 --no-pager"
else
  ssh "$host" "journalctl -u mupibox-ng $*"
fi
