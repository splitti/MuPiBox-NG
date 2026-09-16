#!/bin/sh
# Configure the Waveshare 5-inch 800x480 DSI panel used by MuPiBox-NG.
#
# On DietPi, dietpi-display is the correct tool for display mode/rotation.
# The Waveshare panel itself still needs its Raspberry Pi device-tree overlay,
# because dietpi-display does not identify/configure third-party DSI panel drivers.
set -eu

[ "$(id -u)" -eq 0 ] || { echo "Run as root: sudo sh $0" >&2; exit 1; }

if [ -f /boot/firmware/config.txt ]; then
    CONFIG=/boot/firmware/config.txt
elif [ -f /boot/config.txt ]; then
    CONFIG=/boot/config.txt
else
    echo "No Raspberry Pi config.txt found under /boot/firmware or /boot." >&2
    exit 1
fi

PANEL_OVERLAY='dtoverlay=vc4-kms-dsi-7inch'
KMS_OVERLAY='dtoverlay=vc4-kms-v3d'

# Do not stack a different DSI panel overlay on top of an existing one.
if grep -Eq '^[[:space:]]*dtoverlay=vc4-kms-dsi-' "$CONFIG" && \
   ! grep -Fqx "$PANEL_OVERLAY" "$CONFIG"; then
    echo "A different vc4-kms-dsi-* panel overlay is already configured in $CONFIG." >&2
    echo "Please remove/resolve it manually before installing the MuPiBox Waveshare profile." >&2
    grep -E '^[[:space:]]*dtoverlay=vc4-kms-dsi-' "$CONFIG" >&2 || true
    exit 1
fi

BACKUP="${CONFIG}.mupibox-before-display"
if [ ! -e "$BACKUP" ]; then
    cp -a "$CONFIG" "$BACKUP"
fi

add_line() {
    line=$1
    if ! grep -Fqx "$line" "$CONFIG"; then
        printf '%s\n' "$line" >> "$CONFIG"
    fi
}

if ! grep -Fqx '# MuPiBox-NG: Waveshare 5-inch 800x480 DSI' "$CONFIG"; then
    printf '\n%s\n' '# MuPiBox-NG: Waveshare 5-inch 800x480 DSI' >> "$CONFIG"
fi

# Qt Quick/EGLFS needs the modern KMS/DRM graphics stack. DietPi v10.5+
# can toggle this via dietpi-config; for an unattended MuPiBox install we
# ensure the same KMS overlay directly and idempotently here.
add_line "$KMS_OVERLAY"

# Waveshare's documented overlay for the 800x480 5-inch DSI LCD / LCD (B).
# Default is DSI1. For Pi 5/CM boards on DSI0, change this line to:
# dtoverlay=vc4-kms-dsi-7inch,dsi0
add_line "$PANEL_OVERLAY"

printf '%s\n' \
    "Configured the Waveshare 5-inch 800x480 DSI panel in $CONFIG." \
    "Backup: $BACKUP" \
    "A reboot is required." \
    "On DietPi, use 'sudo dietpi-display' after reboot for mode/rotation if needed;" \
    "do not add legacy framebuffer or display_rotate settings."
