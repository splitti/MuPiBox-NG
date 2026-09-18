#!/usr/bin/env bash
# First-device installer for MuPiBox-NG on DietPi/Debian (Raspberry Pi, ARM64 preferred).
set -euo pipefail

if [[ ${EUID:-$(id -u)} -ne 0 ]]; then
    echo "Run as root: sudo bash scripts/install-dietpi.sh" >&2
    exit 1
fi

REPO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CONFIGURE_WAVESHARE=0
START_SERVICES=1
QUIET_BOOT=1

for arg in "$@"; do
    case "$arg" in
        --waveshare-5-dsi) CONFIGURE_WAVESHARE=1 ;;
        --no-start) START_SERVICES=0 ;;
        --keep-boot-console) QUIET_BOOT=0 ;;
        -h|--help)
            cat <<'EOF'
Usage: sudo bash scripts/install-dietpi.sh [options]

Options:
  --waveshare-5-dsi  Add the device-tree overlay for the Waveshare 5-inch 800x480 DSI LCD / LCD (B).
  --no-start         Install and enable services, but do not start them now.
  --keep-boot-console Keep kernel, DietPi and login messages visible on tty1.

Display handling is generic by default. On a normal Raspberry Pi with the official
Raspberry Pi Touch Display, do not pass a display option: let the firmware/kernel
autodetect the panel and use DietPi's dietpi-display for mode/rotation.
EOF
            exit 0
            ;;
        *) echo "Unknown option: $arg" >&2; exit 2 ;;
    esac
done

if [[ ! -f "$REPO_DIR/go.mod" || ! -f "$REPO_DIR/VERSION" ]]; then
    echo "Run this installer from a MuPiBox-NG Git checkout." >&2
    exit 1
fi

if command -v dietpi-display >/dev/null 2>&1; then
    echo "DietPi detected: display mode/rotation stays under dietpi-display control."
fi

export DEBIAN_FRONTEND=noninteractive
apt-get update
apt-get install -y \
    ca-certificates curl git build-essential pkg-config iproute2 util-linux \
    mpv alsa-utils fbi ffmpeg wpasupplicant bluez rfkill samba \
    qmlscene-qt6 qml6-module-qtquick qml6-module-qtquick-window qml6-module-qtqml \
    qml6-module-qtquick-virtualkeyboard qt6-virtualkeyboard-plugin \
    qt6-qpa-plugins libqt6opengl6 libgl1-mesa-dri libegl1 libgbm1 \
    fonts-dejavu-core fonts-terminus

# Raspberry Pi OS/DietPi provide the board-specific Broadcom Bluetooth patch
# through this package. Keep other Debian targets installable when unavailable.
if apt-cache show pi-bluetooth >/dev/null 2>&1; then
    apt-get install -y pi-bluetooth
fi

version_ge() {
    [[ "$(printf '%s\n%s\n' "$2" "$1" | sort -V | head -n1)" == "$2" ]]
}

install_current_go() {
    local deb_arch go_arch version archive tmp
    deb_arch="$(dpkg --print-architecture)"
    case "$deb_arch" in
        arm64) go_arch=arm64 ;;
        armhf) go_arch=armv6l ;;
        amd64) go_arch=amd64 ;;
        *)
            echo "Unsupported architecture for automatic Go installation: $deb_arch" >&2
            return 1
            ;;
    esac

    version="$(curl -fsSL 'https://go.dev/VERSION?m=text' | head -n1)"
    [[ "$version" == go* ]] || { echo "Could not determine current Go version." >&2; return 1; }
    archive="${version}.linux-${go_arch}.tar.gz"
    tmp="$(mktemp -d)"

    echo "Installing $version from go.dev because the distribution Go is too old."
    curl -fL --retry 3 "https://go.dev/dl/${archive}" -o "$tmp/go.tgz"
    rm -rf /usr/local/go
    tar -C /usr/local -xzf "$tmp/go.tgz"
    rm -rf "$tmp"
    ln -sf /usr/local/go/bin/go /usr/local/bin/go
}

if command -v go >/dev/null 2>&1; then
    GO_VERSION="$(go version | awk '{print $3}' | sed 's/^go//')"
else
    GO_VERSION="0"
fi

if ! version_ge "$GO_VERSION" "1.24.0"; then
    install_current_go
fi

GO_BIN="$(command -v go)"
GO_VERSION="$($GO_BIN version | awk '{print $3}' | sed 's/^go//')"
if ! version_ge "$GO_VERSION" "1.24.0"; then
    echo "Go >= 1.24 is required; found $GO_VERSION." >&2
    exit 1
fi

echo "Building MuPiBox-NG with $($GO_BIN version) ..."
cd "$REPO_DIR"
mkdir -p bin
GOFLAGS=-buildvcs=false CGO_ENABLED=1 "$GO_BIN" test ./...
GOFLAGS=-buildvcs=false CGO_ENABLED=1 "$GO_BIN" build -trimpath \
    -ldflags "-X main.version=$(cat VERSION)" \
    -o bin/mupibox ./cmd/mupibox
GOFLAGS=-buildvcs=false CGO_ENABLED=0 "$GO_BIN" build -trimpath \
    -o bin/mupibox-system-agent ./cmd/mupibox-system-agent

if ! getent passwd mupibox >/dev/null; then
    useradd --system --home-dir /var/lib/mupibox-ng --shell /usr/sbin/nologin mupibox
fi
if ! getent group netdev >/dev/null; then
    groupadd --system netdev
fi

for group in audio video render input tty netdev bluetooth; do
    if getent group "$group" >/dev/null; then
        usermod -a -G "$group" mupibox
    fi
done

install -d -m 0755 /usr/local/lib/mupibox-ng
install -d -m 0755 /usr/local/share/mupibox-ng/ui/assets
install -d -m 0775 -o mupibox -g mupibox /srv/mupibox/music
chown -R mupibox:mupibox /srv/mupibox/music
install -d -m 0750 -o root -g mupibox /etc/mupibox-ng
install -d -m 0750 -o mupibox -g mupibox /var/lib/mupibox-ng
install -d -m 0750 -o root -g mupibox /var/lib/mupibox-ng/network

install -m 0755 bin/mupibox /usr/local/lib/mupibox-ng/mupibox
install -m 0755 bin/mupibox-system-agent /usr/local/lib/mupibox-ng/mupibox-system-agent
install -m 0755 scripts/update-release.sh /usr/local/lib/mupibox-ng/update-release.sh
if [[ ! -e /etc/mupibox-ng/config.json ]]; then
    install -m 0640 -o root -g mupibox deploy/config.example.json /etc/mupibox-ng/config.json
fi

install -m 0644 ui/qtquick/Main.qml /usr/local/share/mupibox-ng/ui/Main.qml
install -m 0644 ui/qtquick/assets/PressStart2P-Regular.ttf /usr/local/share/mupibox-ng/ui/assets/PressStart2P-Regular.ttf

shopt -s nullglob
SPLASH_PARTS=(ui/qtquick/assets/mupibox-startscreen.jpg.b64.*)
shopt -u nullglob
if [[ ${#SPLASH_PARTS[@]} -eq 0 ]]; then
    echo "Missing splash asset parts under ui/qtquick/assets/." >&2
    exit 1
fi
cat "${SPLASH_PARTS[@]}" | base64 --decode \
    > /usr/local/share/mupibox-ng/ui/assets/mupibox-startscreen.jpg
chmod 0644 /usr/local/share/mupibox-ng/ui/assets/mupibox-startscreen.jpg

EXPECTED_SPLASH_SHA256='c20480d973d4bee2db6b31b34985d14ee14b8a1a6c3bc9e0854809b49940570e'
ACTUAL_SPLASH_SHA256="$(sha256sum /usr/local/share/mupibox-ng/ui/assets/mupibox-startscreen.jpg | awk '{print $1}')"
if [[ "$ACTUAL_SPLASH_SHA256" != "$EXPECTED_SPLASH_SHA256" ]]; then
    echo "Splash asset checksum mismatch." >&2
    exit 1
fi

install -m 0644 deploy/mupibox-ng.service /etc/systemd/system/mupibox-ng.service
install -m 0644 deploy/mupibox-system-agent.service /etc/systemd/system/mupibox-system-agent.service
install -m 0644 deploy/mupibox-update@.service /etc/systemd/system/mupibox-update@.service
install -m 0644 deploy/mupibox-splash.service /etc/systemd/system/mupibox-splash.service
install -m 0644 deploy/mupibox-ui.service /etc/systemd/system/mupibox-ui.service

# Samba is available for the admin-controlled media share, but consumes no
# boot-time resources until the user explicitly enables the managed share.
if ! grep -q '^# Managed by MuPiBox-NG' /etc/samba/smb.conf 2>/dev/null; then
    systemctl disable --now smbd.service nmbd.service 2>/dev/null || true
fi

if [[ "$QUIET_BOOT" -eq 1 ]]; then
    bash scripts/configure-quiet-boot.sh
fi

if [[ "$CONFIGURE_WAVESHARE" -eq 1 ]]; then
    sh scripts/configure-waveshare-5-dsi.sh
fi

# TTS is an optional subsystem (see docs/tts.md): a failure here must never
# abort the rest of the device install, MuPiBox-NG starts fine without it.
echo "Installing Piper TTS engine..."
if ! bash scripts/install-piper-engine.sh; then
    echo "WARNING: Piper TTS engine installation failed; MuPiBox-NG will start without TTS. Re-run scripts/install-piper-engine.sh later." >&2
fi
echo "Installing default TTS voices..."
if ! bash scripts/install-tts-voices.sh; then
    echo "WARNING: some TTS voices could not be installed; re-run scripts/install-tts-voices.sh later." >&2
fi

systemctl daemon-reload
systemctl enable mupibox-system-agent.service mupibox-ng.service mupibox-splash.service mupibox-ui.service

if [[ "$START_SERVICES" -eq 1 ]]; then
    # The framebuffer splash starts on the next boot. Restarting it while Qt owns
    # the display can block a live update; the QML UI shows its own startup image.
    systemctl restart mupibox-system-agent.service
    systemctl restart mupibox-ng.service
    systemctl restart mupibox-ui.service || true
fi

cat <<'EOF'

MuPiBox-NG installation finished.

Installed components:
  backend: /usr/local/lib/mupibox-ng/mupibox
  config:  /etc/mupibox-ng/config.json
  music:   /srv/mupibox/music
  native UI: Qt Quick via EGLFS/KMS (no Chromium/browser)
  boot display: early framebuffer splash and quiet appliance boot unless --keep-boot-console was used

Display policy:
  - No display-specific mode is forced by default.
  - Official Raspberry Pi DSI displays on regular Pi boards should autodetect.
  - On DietPi, run 'sudo dietpi-display' for resolution/mode/rotation.
  - Pass --waveshare-5-dsi only for that specific Waveshare panel profile.

Useful checks:
  systemctl status mupibox-ng --no-pager
  systemctl status mupibox-ui --no-pager
  journalctl -u mupibox-ui -b --no-pager -n 100
  curl http://127.0.0.1:8090/api/health || curl http://127.0.0.1:8080/api/health

If a third-party DSI panel overlay was added during this run, reboot now.
EOF
