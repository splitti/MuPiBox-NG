#!/bin/sh
# Read-only display/touch diagnostics for MuPiBox-NG.
set -u

echo '== MuPiBox display diagnostics =='
printf 'Kernel: '; uname -srmo 2>/dev/null || true
printf 'Machine: '; uname -m 2>/dev/null || true

if command -v dietpi-display >/dev/null 2>&1; then
    echo 'DietPi: yes (dietpi-display available)'
else
    echo 'DietPi: no/unknown'
fi

echo
echo '== DRM connectors =='
found=0
for status in /sys/class/drm/*/status; do
    [ -e "$status" ] || continue
    found=1
    connector=$(basename "$(dirname "$status")")
    printf '%s: ' "$connector"
    cat "$status" 2>/dev/null || true
    modes=$(dirname "$status")/modes
    if [ -s "$modes" ]; then
        printf '  modes: '
        tr '\n' ' ' < "$modes"
        echo
    fi
done
[ "$found" -eq 1 ] || echo 'No DRM connector status files found.'

echo
echo '== Framebuffer =='
if [ -r /sys/class/graphics/fb0/virtual_size ]; then
    printf 'fb0 virtual_size: '; cat /sys/class/graphics/fb0/virtual_size
else
    echo 'fb0 virtual_size: unavailable (normal on some DRM-only setups)'
fi

echo
echo '== Input devices =='
if [ -r /proc/bus/input/devices ]; then
    grep -E '^(N: Name=|H: Handlers=)' /proc/bus/input/devices || true
else
    echo '/proc/bus/input/devices unavailable'
fi

echo
echo '== Raspberry Pi boot display entries =='
for cfg in /boot/firmware/config.txt /boot/config.txt; do
    [ -r "$cfg" ] || continue
    echo "[$cfg]"
    grep -E '^[[:space:]]*(dtoverlay=.*(kms|dsi)|display_auto_detect=|disable_touchscreen=)' "$cfg" || echo '  no matching display entries'
done

echo
echo 'This script changes nothing.'
