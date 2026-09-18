#!/usr/bin/env bash
# Run the Go test suite natively on the Test-Pi (arm64 hardware target).
# Deploy first (scripts/deploy-pi.sh) so the Pi checkout matches the working tree.
set -euo pipefail

host="${1:-${MUPIBOX_PI_HOST:-mupibox-test}}"

ssh "$host" "set -euo pipefail; cd /opt/mupibox-ng && go vet ./... && go test ./..."
