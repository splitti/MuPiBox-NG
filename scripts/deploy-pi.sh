#!/usr/bin/env bash
# Sync the current working tree (including uncommitted changes) to the Test-Pi,
# build natively there (arm64, CGO for go-sqlite3), install, restart the service
# and report health. Committing/pushing to GitHub happens separately, only after
# a successful test on the Pi.
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."

host="${1:-${MUPIBOX_PI_HOST:-mupibox-test}}"

if [ -n "$(git status --porcelain)" ]; then
  echo "Note: deploying working tree with uncommitted changes." >&2
fi

rsync -az --delete \
  --exclude '.git/' \
  --exclude '/bin/' \
  --exclude '/music/' \
  --exclude '/var/' \
  --exclude '/config.local.json' \
  --exclude '*.log' \
  --exclude '*.test' \
  --exclude '.env' \
  --exclude '.env.*' \
  --exclude 'credentials*.json' \
  --exclude '/.claude/tools/' \
  ./ "$host:/opt/mupibox-ng/"

ssh "$host" "set -euo pipefail; cd /opt/mupibox-ng && \
  ./scripts/install-service.sh && \
  systemctl restart mupibox-ng && \
  sleep 1 && \
  systemctl is-active mupibox-ng && \
  curl -fsS http://127.0.0.1:8090/api/health"

echo "Deployed working tree to $host and restarted mupibox-ng."
