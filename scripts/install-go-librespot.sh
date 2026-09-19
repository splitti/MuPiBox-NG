#!/usr/bin/env bash
# Installs go-librespot (github.com/devgianlu/go-librespot, GPL-3.0) as a
# separate, systemd-managed process -- MuPiBox-NG never implements the
# Spotify protocol itself, only drives go-librespot's local REST/WebSocket
# API (see internal/providers/spotify and docs/spotify.md). Idempotent: safe
# to re-run, re-running with a newer GO_LIBRESPOT_VERSION upgrades in place.
set -euo pipefail

if [[ ${EUID:-$(id -u)} -ne 0 ]]; then
    echo "Run as root: sudo bash scripts/install-go-librespot.sh" >&2
    exit 1
fi

REPO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
VERSION="${GO_LIBRESPOT_VERSION:-v0.10.0}"
BIN_DIR="${MUPIBOX_BIN_DIR:-/usr/local/lib/mupibox-ng}"
TARGET="$BIN_DIR/go-librespot"

# ARM64-only per CLAUDE.md: this project's product target is Pi 3/4/5,
# 64-bit only. Reject other architectures clearly rather than silently
# trying (and failing in some confusing way) with a mismatched binary.
deb_arch="$(dpkg --print-architecture)"
if [[ "$deb_arch" != "arm64" ]]; then
    echo "go-librespot is only installed for arm64 (MuPiBox-NG's supported target); this host reports '$deb_arch'." >&2
    exit 1
fi

installed_version=""
if [[ -x "$TARGET" ]]; then
    installed_version="$("$TARGET" --version 2>&1 | head -n1 || true)"
fi
if [[ "$installed_version" == *"$VERSION"* ]]; then
    echo "go-librespot $VERSION already installed at $TARGET"
else
    asset="go-librespot_linux_arm64.tar.gz"
    url="https://github.com/devgianlu/go-librespot/releases/download/${VERSION}/${asset}"
    tmp="$(mktemp -d)"
    trap 'rm -rf "$tmp"' EXIT
    echo "Downloading go-librespot $VERSION for arm64..."
    curl -fL --retry 3 "$url" -o "$tmp/$asset"
    tar -C "$tmp" -xzf "$tmp/$asset"
    found="$(find "$tmp" -maxdepth 2 -type f -name 'go-librespot' | head -n1)"
    if [[ -z "$found" ]]; then
        echo "go-librespot binary not found in the downloaded archive." >&2
        exit 1
    fi
    install -d -m 0755 "$BIN_DIR"
    install -m 0755 "$found" "$TARGET"
    echo "Installed go-librespot $VERSION to $TARGET"
fi

install -d -m 0755 /etc/systemd/system
install -m 0644 "$REPO_DIR/deploy/mupibox-spotify.service" /etc/systemd/system/mupibox-spotify.service
systemctl daemon-reload

# Unlike mupibox-system-agent (privileged, opt-in), go-librespot is the
# actual Phase 3A deliverable: enabling and starting it immediately is the
# point of running this installer, not an optional extra. Restarting on a
# re-run picks up a binary/config upgrade; a brief reconnect is an explicit,
# rare admin action here, not something that happens silently.
systemctl enable --now mupibox-spotify.service
systemctl restart mupibox-spotify.service

echo "mupibox-spotify.service installed, enabled and started (it starts after mupibox-ng.service)."
