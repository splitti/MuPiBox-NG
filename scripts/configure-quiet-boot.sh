#!/usr/bin/env bash
# Reserve tty1 for MuPiBox and suppress boot-time console/status text on the display.
# The original kernel command line is kept once for recovery.
set -euo pipefail

if [[ ${EUID:-$(id -u)} -ne 0 ]]; then
    echo "Run as root: sudo bash scripts/configure-quiet-boot.sh" >&2
    exit 1
fi

CMDLINE=""
for candidate in /boot/firmware/cmdline.txt /boot/cmdline.txt; do
    if [[ -f "$candidate" ]]; then
        CMDLINE="$candidate"
        break
    fi
done

if [[ -z "$CMDLINE" ]]; then
    echo "No Raspberry Pi cmdline.txt found; quiet boot was not configured." >&2
    exit 1
fi

BACKUP="${CMDLINE}.mupibox-backup"
if [[ ! -e "$BACKUP" ]]; then
    cp -a "$CMDLINE" "$BACKUP"
fi

read -r line < "$CMDLINE"

# Keep serial diagnostics, but move the visible kernel console away from the
# appliance display. tty1 remains reserved for the framebuffer splash and Qt.
if [[ " $line " == *" console=tty1 "* ]]; then
    line="${line//console=tty1/console=tty3}"
elif [[ " $line " != *" console=tty3 "* ]]; then
    line="$line console=tty3"
fi

for option in quiet splash loglevel=0 systemd.show_status=false vt.global_cursor_default=0 logo.nologo; do
    case " $line " in
        *" $option "*) ;;
        *) line="$line $option" ;;
    esac
done
printf '%s\n' "$line" > "$CMDLINE"

# Prevent DietPi/login output from claiming the appliance display before Qt starts.
systemctl mask getty@tty1.service
systemctl daemon-reload

cat <<EOF
MuPiBox quiet boot configured.

Kernel command line: $CMDLINE
Backup:              $BACKUP
visible console:      tty3
tty1 getty:           masked

A reboot is required. SSH remains available for recovery.
To restore the previous boot command line:
  cp '$BACKUP' '$CMDLINE'
  systemctl unmask getty@tty1.service
EOF
