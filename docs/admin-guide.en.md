# Admin area and best practices

The admin area groups settings by responsibility. Each security- or system-relevant option states its recommendation, impact and possible interruption next to the control.

## Sections

- **Box:** language, theme, audio, display, TTS and the admin password. A password of at least ten characters is recommended.
- **Content:** categories, local directories, resume lists and future online providers. The player reloads changes without a restart.
- **Network:** enable, prioritize and scan Wi-Fi adapters; pair Bluetooth devices; store DHCP or static IPv4 values. DHCP is the safe default. DietPi applies static configuration only after its detected network backend can be changed without risking SSH access.
- **Providers:** Spotify and Amazon Music account configuration. Storing credentials does not enable a playback adapter. Amazon Music has no general public playback API.
- **Hardware:** MuPiHAT, battery profiles and input-current limit. Legacy profiles are included and `Custom` is editable. Hardware access follows as a dedicated service.
- **Smart Home:** MQTT and Home Assistant Discovery. Use a dedicated broker user restricted to the box topic. The publisher follows after status and control topics are finalized.
- **System:** measured boot time, slowest systemd units, swap, network wait and CPU profile.
- **Maintenance:** touchscreen restart, backup/restore and release switching with an automatic safety backup and rollback.

## System profiles

| Option | Recommendation | Impact |
|---|---|---|
| Swap | Usually disable for the dedicated player | Fewer SD-card writes; less reserve under real memory pressure. |
| Network wait | Disable | Faster offline boot; online providers load later. |
| CPU profile | Balanced | Good balance between responsiveness, temperature and energy use. |
| Performance | Only for measured UI/audio bottlenecks | Faster response but more heat and energy use. |

Privileged changes run only through the local `mupibox-system-agent`. It exposes no network port and accepts a fixed set of actions over a Unix socket.
