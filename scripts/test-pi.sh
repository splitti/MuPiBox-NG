#!/usr/bin/env bash
# Run the Go test suite plus non-destructive runtime healthchecks natively on
# the Test-Pi (arm64 hardware target). Deploy first (scripts/deploy-pi.sh) so
# the Pi checkout matches the working tree.
set -euo pipefail

host="${1:-${MUPIBOX_PI_HOST:-mupibox-test}}"

echo "== go vet / go test =="
ssh "$host" "set -euo pipefail; cd /opt/mupibox-ng && go vet ./... && go test ./..."

echo
echo "== Service status =="
ssh "$host" 'for u in mupibox-ng mupibox-ui mupibox-system-agent; do
  printf "%-24s %s\n" "$u" "$(systemctl is-active "$u" 2>&1)"
done'

echo
echo "== HTTP health =="
ssh "$host" 'curl -fsS -m 5 http://127.0.0.1:8090/api/health && echo'

echo
echo "== Relevant processes =="
ssh "$host" 'pgrep -fal "mupibox|mpv" || echo "(keine passenden Prozesse gefunden)"'
