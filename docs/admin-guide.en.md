# Admin area and best practices

The admin area groups settings by responsibility. Each security- or system-relevant option states its recommendation, impact and possible interruption next to the control.

## Sections

- **Box:** language, theme, audio, display, TTS and the admin password. A password of at least ten characters is recommended.
- **Content:** categories, local directories, resume lists and future online providers. The player reloads changes without a restart.
- **Network:** select exactly one Wi-Fi adapter by stable MAC address, optionally disable onboard Wi-Fi at boot, scan Wi-Fi, pair Bluetooth, apply DietPi DHCP/static IPv4 and enable Samba for `/srv/mupibox` only when needed.
- **Providers:** Spotify and Amazon Music account configuration. Storing credentials does not enable a playback adapter. Amazon Music has no general public playback API.
- **Hardware:** MuPiHAT, battery profiles and input-current limit. Legacy profiles are included and `Custom` is editable. Hardware access follows as a dedicated service.
- **Smart Home:** native Home Assistant integration through the MuPiBox API, without an additional MQTT broker.
- **System:** measured boot time, slowest systemd units, swap, network wait, CPU profile, initial turbo and confirmed restart/shutdown actions.
- **Maintenance:** touchscreen restart, backup/restore and release switching with an automatic safety backup and rollback.

## System profiles

| Option | Recommendation | Impact |
|---|---|---|
| Swap | Usually disable for the dedicated player | Fewer SD-card writes; less reserve under real memory pressure. |
| Network wait | Disable | Faster offline boot; online providers load later. |
| CPU profile | Balanced | Good balance between responsiveness, temperature and energy use. |
| Initial turbo | 20 seconds on Raspberry Pi/DietPi | Highest CPU clock only during early boot; `0` disables it and changes require a reboot. |
| Performance | Only for measured UI/audio bottlenecks | Faster response but more heat and energy use. |

The `1–60` second range and `20` second recommendation follow [DietPi's ARM Initial Turbo configuration](https://github.com/MichaIng/DietPi/blob/master/dietpi/dietpi-config).

DietPi swap is changed through the official [`dietpi-set_swapfile`](https://github.com/MichaIng/DietPi/blob/master/dietpi/func/dietpi-set_swapfile) helper. Static addresses use [`dietpi-network apply`](https://github.com/MichaIng/DietPi/blob/master/dietpi/dietpi-network), removing DHCP from that interface's ifupdown stanza without globally removing a DHCP client that another interface may need.

Privileged changes run only through the local `mupibox-system-agent`. It exposes no network port and accepts a fixed set of actions over a Unix socket.
