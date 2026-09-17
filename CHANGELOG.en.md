# Changelog

> [Deutsch](CHANGELOG.md) · **English**

## 0.1.0-dev – 2026-09-16

- Fixed Wi-Fi discovery under systemd: the backend no longer isolates `PrivateTmp`, allowing `wpa_supplicant` to reach the `wpa_cli` reply socket.
- Implemented admin password protection using PBKDF2-SHA-256, server-side sessions, logout and failed-login rate limiting.
- Multiple Wi-Fi adapters are shown with state and driver; automatic discovery uses only the preferred ready adapter while enablement and priority are persisted in SQLite.
- Quiet boot now separates the visible framebuffer from the kernel console and the installer adds Raspberry Pi Bluetooth firmware when available.
- Added an early framebuffer splash service and separated the visible display from the kernel console.
- Native Qt Quick UI aligned with the web frontend (navigation, media cards and player bar).
- Reversible quiet-boot setup hides kernel/DietPi console output and shows the splash earlier.
- Restarted development on `rebuild/go-foundation` while preserving the complete prototype under `legacy/prototype`.
- Documented requirements, repository audit, architecture and LXC reconciliation.
- Added a Go service with embedded web UI and local folder collections.
- Added shared player/queue control, volume limit, mpv adapter and clearly marked simulation.
- Added optional simulated buttons/RFID, a systemd template and focused tests including an ARM64 cross-build.
- Standardized the active NextGen development HTTP port on `8090`.
- Reworked the home screen around a data-driven category/row model exposed through `/api/home`.
- Added localized labels for categories and rows; the development configuration contains Audiobooks, Music, Radio and Podcasts in German/English.
- Corrected the TTS model: TTS is now a global box setting with enabled state, language and provider; categories no longer own TTS configuration.
- Added `browser-dev` as a clearly limited TTS development fallback; production TTS is intended to use interchangeable local/external providers.
- Prepared global power configuration with `idle_shutdown_minutes`; controlled shutdown itself is not implemented yet.
- Selected SQLite as the target persistence layer for admin data, categories/rows, global settings and later state; JSON remains bootstrap/intermediate configuration.
- Added German/English persistence and migration architecture documentation.
- Expanded the target admin model to include categories, rows, translations, providers, global TTS/power settings and further box options.
- Switched project documentation to paired German/English files.
- Extended player policy with 10-second skips for spoken-word content and persistent resume even for a single many-hour file.
- Documented target configuration for startup/shutdown sounds, startup/maximum volume, audio output, splash screen, display idle/brightness, admin password and themes.
- Specified MuPiHat/battery profiles including custom profiles, fan levels, legacy OnOff pins, operational LED and MQTT/Home Assistant as hardware/system adapters.
- Planned Wi-Fi scan/connect through a long press on the network icon plus a best-effort captive-portal flow.
- Specified local media roots and a Spotify provider with separate catalogue/playback plus compliant temporary metadata/artwork caching.
- Implemented SQLite store with migration 1 for settings, navigation nodes/labels and provider-neutral playback progress.
- Added first admin UI at `/admin/` with persistent box settings and visual category/row editing.
- Player stores local progress periodically and on pause/seek/change/shutdown, resuming even a single very long file.
- Progress contract covers local media, Spotify/podcasts and future video while explicitly excluding live radio.
- Updated the ARM64 build test to use `-buildvcs=false`, preventing Git VCS stamping from failing with differing LXC file ownership.
- Reduced bootstrap JSON to listen address, SQLite path, local media root and audio backend; all dynamic settings and content come from SQLite.
- Added SQLite migration 2 with source type and source reference fields for media entries.
- Made the admin language independent from box/TTS language; German and English are loaded from language files.
- Categories and media entries now expose one “Name” field for the selected content language while preserving existing translations.
- Media can be configured directly below categories using Local media, Spotify, Amazon Music, Stream or Podcast sources. Providers without playback implementations are stored but not presented as playable.
- Removed the settings button and simulation dialog; only the web player opens administration after holding the clock for two seconds while the device clock is display-only.
- Added a native Qt Quick test UI using EGLFS/KMS, a dedicated systemd service, DietPi installer, bootstrap script and display diagnostics.
- Added generic display installation for the official Raspberry Pi DSI display; the Waveshare overlay is only enabled explicitly with `--waveshare-5-dsi`.
- Reconstructed the complete new MuPiBox logo as a checksum-verified 800×480 startup screen and reused it as the default cover.
- Media from different sources is mixed within each player category; when network connectivity is unavailable, only offline-capable local media remains visible.
- Added the configurable “Continue / Resume list” source with a custom name, 1–100 items and direct playback from the saved queue item and position.
- The large 5-inch profile now opens a dedicated child-friendly playback layer with artwork, 10-second skips and large controls after media selection.
- Added a Commodore-inspired 8-bit theme for the Qt, web and admin UIs; DietPi installs the Terminus font for it.
- Defined the MuPiHat integration as a local Python hardware agent with Go-owned safety logic and documented the existing battery/input-current profiles in both languages.
- Added Wi-Fi scanning and connection through `wpa_cli`/`nmcli`; holding the Wi-Fi indicator for about one second opens discovery, selection, password entry and the native Qt on-screen keyboard.
- Added Bluetooth as a global SQLite setting; BlueZ devices can be discovered, paired, connected, disconnected and removed from the admin UI.
- Documented hotel Wi-Fi/captive portals and Bluetooth media buttons as the next hardware-dependent stage.
- Still missing: real MuPiHat/RFID/GPIO hardware adapters, Spotify/Amazon Music/radio/podcast integration, production TTS, actual idle shutdown and rollback.
