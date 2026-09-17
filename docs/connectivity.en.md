# Wi-Fi and Bluetooth

> [Deutsch](connectivity.md) · **English**

## Wi-Fi

Holding the Wi-Fi indicator for 1.2 seconds opens Wi-Fi management in the native touch UI. It lists discovered SSIDs with child-friendly signal bars, security type and the current connection. After selecting a network, the password can be entered using the on-screen keyboard.

The Go service uses the network manager already present on the system:

1. NetworkManager through `nmcli`, when available,
2. otherwise DietPi/`wpa_supplicant` through `wpa_cli`.

The installer deliberately does not replace or convert DietPi's active network setup. It only installs `wpasupplicant` and adds the service user to the existing `netdev` group. Wi-Fi passwords are never stored in SQLite, MuPiBox configuration or MuPiBox logs. Persistence is handled only by the active network manager.

API:

- `GET /api/connectivity/wifi` – scan networks,
- `POST /api/connectivity/wifi/connect` – connect the selected network.

The admin web UI exposes the same scan/connect flow as a fallback.

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

The admin UI has no password in the current development build. Connectivity endpoints must therefore be used only on a trusted home network. They will be covered by the planned admin authentication before a production release.
