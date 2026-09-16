#!/usr/bin/env bash
# Bootstrap MuPiBox-NG from GitHub on a fresh DietPi/Debian Raspberry Pi.
set -euo pipefail

if [[ ${EUID:-$(id -u)} -ne 0 ]]; then
    echo "Run as root, for example: sudo bash $0" >&2
    exit 1
fi

REPO_URL="${MUPIBOX_REPO_URL:-https://github.com/splitti/MuPiBox-NG.git}"
BRANCH="${MUPIBOX_BRANCH:-rebuild/go-foundation}"
TARGET="${MUPIBOX_TARGET:-/opt/mupibox-ng}"

export DEBIAN_FRONTEND=noninteractive
apt-get update
apt-get install -y ca-certificates git

if [[ -e "$TARGET" ]]; then
    echo "$TARGET already exists. Refusing to overwrite an existing installation." >&2
    echo "Remove it manually for a clean reinstall, or run $TARGET/scripts/install-dietpi.sh directly." >&2
    exit 1
fi

git clone --branch "$BRANCH" --single-branch "$REPO_URL" "$TARGET"
cd "$TARGET"
exec bash scripts/install-dietpi.sh "$@"
