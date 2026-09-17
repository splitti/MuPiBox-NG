# Wi-Fi and Bluetooth

> [Deutsch](connectivity.md) · **English**

## Wi-Fi

Holding the Wi-Fi indicator for about one second opens Wi-Fi management in the native touch UI. It lists discovered SSIDs with child-friendly signal bars, security type and the current connection. After selecting a network, the password can be entered using the native Qt on-screen keyboard.

The Go service uses the network manager already present on the system:

1. NetworkManager through `nmcli`, when available,
2. otherwise DietPi/`wpa_supplicant` through `wpa_cli`.

The installer deliberately does not replace or convert DietPi's active network setup. It only installs `wpasupplicant` and adds the service user to the existing `netdev` group. Wi-Fi passwords are never stored in SQLite, MuPiBox configuration or MuPiBox logs. Persistence is handled only by the active network manager.

API:

- `GET /api/connectivity/wifi` – scan networks,
- `POST /api/connectivity/wifi/connect` – connect the selected network.

The admin web UI exposes the same scan/connect flow as a fallback.

The backend service deliberately does not isolate `PrivateTmp`: `wpa_cli` creates its local reply socket below `/tmp`, and `wpa_supplicant`, which runs outside the unit, must be able to reach that path. The other systemd hardening options remain active.

A separate, restricted root service named `mupibox-system-agent` performs actual Wi-Fi adapter up/down operations. It has no network port and accepts only validated commands through a Unix socket restricted to the `mupibox` group. A USB adapter can therefore be enabled and connected before the onboard adapter is disabled after an explicit connection-loss warning.

When multiple Wi-Fi adapters are present, each adapter can be enabled or excluded for MuPiBox and one can be marked preferred. Automatic discovery uses the preferred ready adapter and falls back to another enabled adapter with an active link or `wpa_supplicant` control socket. Hardware that is present but unmanaged remains visible and is not scanned implicitly.

### Hotel Wi-Fi / captive portals

This first stage connects open and WPA/WPA2-Personal networks. A hotel network may be connected afterwards, but its login page is not opened on the box yet.

The next stage will detect captive-portal redirects and open a temporary, tightly restricted browser surface. Cookies will be kept for that session only. Because the target Raspberry Pi has 1 GB RAM, the hardware test will decide whether Qt WebEngine is viable or a lightweight separate browser process is preferable. Captive portals remain best-effort because forms, vouchers, SMS login and certificate failures differ between providers.

## Bluetooth

Bluetooth is off by default and stored as a global SQLite setting. It can be enabled or disabled in the admin UI. When enabled, the UI supports:

- device discovery,
- pairing and trusting,
- connect and disconnect,
- removing saved devices.

The adapter uses BlueZ through `bluetoothctl`. The installer adds `bluez`, `rfkill` and the `bluetooth` group when present.

“Just Works” pairing is supported in this first stage. Devices requiring a PIN/passkey dialog or confirmation on both sides will later get a dedicated agent dialog on the touchscreen.

## Media buttons

Play/pause, next and previous from Bluetooth devices should emit the same commands as touch, buttons and RFID. A BlueZ MediaPlayer event adapter is planned for this. The discovery/pairing stage establishes the boundary, but does not consume AVRCP events yet. Mapping will be tested with real headphones/speakers before device-specific assumptions are committed.

## Security

An admin password can be set and changed in the Security section. Only a salted PBKDF2-SHA-256 hash is stored. Once enabled, server-side sessions, logout and failed-login rate limiting protect admin and remotely accessed connectivity endpoints. The local touchscreen may still configure Wi-Fi through loopback so that the box cannot lock itself out.
