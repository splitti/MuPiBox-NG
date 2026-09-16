#!/bin/bash
set -euo pipefail

# =========================================================
# MuPiBox Installer (DietPi / Debian Bookworm)
# =========================================================

# -----------------------------
# CONFIGURATION
# -----------------------------
APP_NAME="mupibox-ng"
INSTALL_DIR="/opt/mupibox-ng"
SERVICE_NAME="mupibox"
GO_PKG="golang"
PORT="8080"

LOG_DIR="/var/log/mupibox"
LOG_FILE="$LOG_DIR/install.log"

REPO_URL="https://github.com/splitti/MuPiBox-NG.git"

# -----------------------------
# COLORS
# -----------------------------
COLOR_RESET="\e[0m"
COLOR_BLUE="\e[1;34m"
COLOR_GREEN="\e[1;32m"
COLOR_RED="\e[1;31m"
COLOR_YELLOW="\e[1;33m"

# -----------------------------
# LOGGING SETUP
# -----------------------------
mkdir -p "$LOG_DIR"
touch "$LOG_FILE"
chmod 750 "$LOG_DIR"
chmod 640 "$LOG_FILE"

exec > >(tee -a "$LOG_FILE") 2>&1

log() {
  echo -e "$(date '+%Y-%m-%d %H:%M:%S')  $1"
}

section() {
  echo
  echo -e "${COLOR_BLUE}=== $1 ===${COLOR_RESET}"
}

success() {
  echo -e "${COLOR_GREEN}✔ $1${COLOR_RESET}"
}

warning() {
  echo -e "${COLOR_YELLOW}⚠ $1${COLOR_RESET}"
}

error() {
  echo -e "${COLOR_RED}✖ $1${COLOR_RESET}"
  echo
  echo "Installation aborted."
  echo "See log file for details:"
  echo "  $LOG_FILE"
  exit 1
}

trap 'error "Unexpected error at line $LINENO"' ERR

# -----------------------------
# START
# -----------------------------
section "MuPiBox Installer started"
log "Log file: $LOG_FILE"

export DEBIAN_FRONTEND=noninteractive

# -----------------------------
# SYSTEM UPDATE
# -----------------------------
section "Updating system"
apt update -y
apt upgrade -y
success "System updated"

# -----------------------------
# PACKAGE INSTALL
# -----------------------------
section "Installing required packages"
apt install -y git ca-certificates curl $GO_PKG
success "Required packages installed"

# -----------------------------
# GO CHECK
# -----------------------------
section "Checking Go installation"
if ! command -v go >/dev/null 2>&1; then
  error "Go is not available after installation"
fi
go version
success "Go is available"

# -----------------------------
# APPLICATION SETUP
# -----------------------------
section "Installing or updating MuPiBox"

if [ -d "$INSTALL_DIR/.git" ]; then
  log "Existing installation found, updating repository"
  cd "$INSTALL_DIR"
  git pull
else
  log "Cloning repository to $INSTALL_DIR"
  git clone "$REPO_URL" "$INSTALL_DIR"
  cd "$INSTALL_DIR"
fi
success "Repository ready"

section "Resolving Go dependencies"
go mod tidy
success "Dependencies resolved"

section "Building MuPiBox binary"
go build -o "$APP_NAME" ./cmd/mupibox
success "Binary built: $INSTALL_DIR/$APP_NAME"

# -----------------------------
# SYSTEMD SERVICE
# -----------------------------
section "Creating systemd service"

cat >/etc/systemd/system/$SERVICE_NAME.service <<EOF
[Unit]
Description=MuPiBox-NextGen
After=network.target

[Service]
Type=simple
WorkingDirectory=$INSTALL_DIR
ExecStart=$INSTALL_DIR/$APP_NAME
Restart=always
RestartSec=2
User=root
Environment=PORT=$PORT

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
success "systemd service created"

# -----------------------------
# FINISH
# -----------------------------
section "Installation completed successfully"

echo
echo -e "${COLOR_GREEN}MuPiBox-NextGen has been installed successfully.${COLOR_RESET}"
echo
echo "Next steps:"
echo "  systemctl start $SERVICE_NAME"
echo "  systemctl enable $SERVICE_NAME"
echo
echo "Web UI:"
echo "  http://<IP>:$PORT"
echo
echo "Installation log:"
echo "  $LOG_FILE"
echo
