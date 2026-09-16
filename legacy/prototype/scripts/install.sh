#!/bin/bash
set -e

echo "MuPiBox Installer started"

# Ensure script runs as root
if [ "$EUID" -ne 0 ]; then
  echo "ERROR: Please run this installer as root (sudo)"
  exit 1
fi

# Ensure apt is available
if ! command -v apt >/dev/null 2>&1; then
  echo "ERROR: apt not found. This installer requires a Debian-based system."
  exit 1
fi

# Ensure git is installed
if ! command -v git >/dev/null 2>&1; then
  echo "Git not found. Installing git..."
  export DEBIAN_FRONTEND=noninteractive
  apt update -y
  apt install -y git ca-certificates
fi

TMP_DIR="/tmp/mupibox-install"
REPO_URL="https://github.com/splitti/MuPiBox-NG.git"

rm -rf "$TMP_DIR"
mkdir -p "$TMP_DIR"

echo "Cloning MuPiBox repository..."
git clone "$REPO_URL" "$TMP_DIR"

echo "Starting MuPiBox installer..."
bash "$TMP_DIR/scripts/install-mupibox.sh"
