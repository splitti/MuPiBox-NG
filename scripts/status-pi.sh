#!/usr/bin/env bash
# Compact status overview of the MuPiBox-NG services and health on the Test-Pi.
set -euo pipefail

host="${1:-${MUPIBOX_PI_HOST:-mupibox-test}}"

ssh "$host" 'for u in mupibox-ng mupibox-ui mupibox-system-agent; do
  printf "%-24s %s\n" "$u" "$(systemctl is-active "$u" 2>&1)"
done
echo
curl -fsS -m 5 http://127.0.0.1:8090/api/health || echo "health check failed"
echo'
