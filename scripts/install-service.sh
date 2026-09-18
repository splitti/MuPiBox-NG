#!/bin/sh
# Optional installation on Debian/DietPi; does not install packages or start the service.
set -eu
[ "$(id -u)" -eq 0 ] || { echo 'Run as root.' >&2; exit 1; }
cd "$(dirname "$0")/.."
command -v go >/dev/null
command -v mpv >/dev/null
[ -d /run/systemd/system ] || { echo 'systemd is required.' >&2; exit 1; }
go build -trimpath -ldflags "-X main.version=$(cat VERSION)" -o bin/mupibox ./cmd/mupibox
go build -trimpath -o bin/mupibox-system-agent ./cmd/mupibox-system-agent
if ! getent passwd mupibox >/dev/null; then
 useradd --system --home-dir /var/lib/mupibox-ng --shell /usr/sbin/nologin mupibox
fi
install -d -m 0755 /usr/local/lib/mupibox-ng /srv/mupibox/music
install -d -m 0750 -o root -g mupibox /etc/mupibox-ng
install -m 0755 bin/mupibox /usr/local/lib/mupibox-ng/mupibox
install -m 0755 bin/mupibox-system-agent /usr/local/lib/mupibox-ng/mupibox-system-agent
if [ ! -e /etc/mupibox-ng/config.json ]; then
 install -m 0640 -o root -g mupibox deploy/config.example.json /etc/mupibox-ng/config.json
fi
install -m 0644 deploy/mupibox-ng.service /etc/systemd/system/mupibox-ng.service
if [ -e deploy/mupibox-system-agent.service ]; then
 install -m 0644 deploy/mupibox-system-agent.service /etc/systemd/system/mupibox-system-agent.service
fi
systemctl daemon-reload
if systemctl is-enabled --quiet mupibox-system-agent.service 2>/dev/null; then
 systemctl restart mupibox-system-agent.service
fi
printf '%s\n' 'Installed. Add music to /srv/mupibox/music, check config, then:' 'systemctl enable --now mupibox-ng'
