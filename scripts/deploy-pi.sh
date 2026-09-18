#!/usr/bin/env bash
# Deploy the current branch's pushed state to the Test-Pi and restart the service.
# Deploys only what is already committed and pushed to GitHub - never local
# uncommitted changes. Commit and push from the DEV-LXC first.
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."

host="${1:-${MUPIBOX_PI_HOST:-mupibox-pi}}"
branch="$(git rev-parse --abbrev-ref HEAD)"

if [ -n "$(git status --porcelain)" ]; then
  echo "Warning: uncommitted local changes exist and will NOT be deployed." >&2
fi

ssh "$host" "set -euo pipefail; cd /opt/mupibox-ng && \
  git fetch origin && \
  git merge --ff-only 'origin/$branch' && \
  ./scripts/install-service.sh && \
  systemctl restart mupibox-ng && \
  sleep 1 && \
  systemctl is-active mupibox-ng"

echo "Deployed branch '$branch' to $host."
