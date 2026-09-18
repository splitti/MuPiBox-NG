#!/usr/bin/env bash
# Show service status and health of the mupibox-ng instance on the Test-Pi.
set -euo pipefail

host="${1:-${MUPIBOX_PI_HOST:-mupibox-pi}}"

ssh "$host" "systemctl status mupibox-ng --no-pager || true; \
  echo; curl -fsS http://127.0.0.1:8090/api/health || echo 'health check failed'"
