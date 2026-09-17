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

# Keep serial diagnostics, but remove every virtual-terminal kernel console.
# fbcon=map:9 prevents early kernel messages from being painted onto fb0 before
# the MuPiBox splash has claimed tty1. DRM/EGLFS remain available to the UI.
filtered=""
for token in $line; do
    case "$token" in
        console=tty[0-9]*) continue ;;
    esac
    filtered+="${filtered:+ }$token"
done
line="$filtered"

for option in quiet splash loglevel=0 systemd.show_status=false vt.global_cursor_default=0 logo.nologo fbcon=map:9; do
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
visible console:      disabled (serial console is preserved)
tty1 getty:           masked

A reboot is required. SSH remains available for recovery.
To restore the previous boot command line:
  cp '$BACKUP' '$CMDLINE'
  systemctl unmask getty@tty1.service
EOF
