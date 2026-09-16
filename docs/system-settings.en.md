# System, hardware and admin settings

> [Deutsch](system-settings.md) · English

## Goal and persistence

The admin web UI at `/admin` is the central configuration surface for a box. Dynamic settings are stored in SQLite; bootstrap JSON contains only values needed before opening the database. Hardware access is isolated behind adapters and must be fully simulatable in the development LXC.

Settings are grouped, validated and applied without restart where practical. Changes requiring restart are clearly marked. Secrets are never exposed through Git or public status APIs.

## Audio and playback

Configurable options include:

- startup sound on/off and a bundled or uploaded audio file,
- shutdown sound on/off and a bundled or uploaded audio file,
- startup volume,
- maximum volume as a hard limit for touch, buttons, MQTT and providers,
- audio output device,
- optional mono/stereo and adapter-specific audio options.

The shutdown sound plays before controlled poweroff with a fixed timeout so an unavailable or broken audio device can never block shutdown.

Setup should detect the existing DietPi/ALSA audio configuration and use it as the initial selection. An explicitly selected device is persisted. If it is unavailable at startup, the box falls back in a controlled way to a valid device or a clearly reported error state; it must not silently play through the wrong output.

## Display and startup

Configurable options include:

- splash screen on/off,
- bundled image or a custom uploaded image,
- display brightness,
- turn display off after an independently configured idle period,
- wake display on touch, button, RFID or newly started playback.

`Display idle off` and `box idle shutdown` are independent timers. Turning the display off must not stop active playback. Brightness is handled through a display adapter because backlight interfaces vary by display.

## Admin access

The admin UI initially has no password during development. An admin password can be created, changed or removed. Outside explicitly marked development mode, the UI prominently warns when admin access is unprotected.

Passwords are stored only as suitable password hashes. Once enabled, admin sessions, logout, login-attempt rate limiting and protection for write requests are required. The child/player UI remains separate from admin authentication.

## Status dashboard

Where real adapters provide the data, the dashboard displays:

- battery percentage, charging state, voltage and warnings,
- Wi-Fi SSID, signal strength, IP address and internet state,
- audio output and player status,
- display state and brightness,
- CPU temperature, fan level, uptime and free storage,
- version, backend/hardware profile and recent errors,
- MQTT and provider state.

Unknown values are shown as unknown, never as `0%` or a misleading healthy state.

## MuPiHat and battery profiles

Battery profiles are data stored in SQLite. Voltages are stored as integer millivolts. The admin UI can create, duplicate, edit, validate and select profiles.

Bundled initial profiles:

| Profile | 100% | 75% | 50% | 25% | 0% | Warning | Shutdown |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| Ansmann 2S1P | 8100 | 7800 | 7400 | 7000 | 6700 | 7000 | 6800 |
| ENERpower 2S2P 10,000mAh | 8000 | 7700 | 7300 | 6900 | 6000 | 6500 | 6150 |
| USB-C mode (no battery) | 1 | 1 | 1 | 1 | 1 | 0 | 0 |
| Custom | 8100 | 7800 | 7400 | 7000 | 6700 | 7000 | 6800 |

Battery profiles require `v_100 > v_75 > v_50 > v_25 > v_0`. Percentage is interpolated between points and smoothed so measurement noise does not cause a jumping display. Warning and shutdown thresholds use hysteresis and a minimum duration so a brief voltage dip cannot immediately power off the box. USB-C mode disables battery warnings and voltage-based shutdown.

The exact MuPiHat revision, measurement source, charge detection and safe power-cut sequence must be verified on physical hardware.

## CPU fan

The optional fan uses a hardware adapter. Initial configuration:

- enabled/disabled,
- GPIO, previous default `12`,
- 25% at `45 °C`,
- 50% at `55 °C`,
- 75% at `65 °C`,
- 100% at `75 °C`.

Thresholds and GPIO are editable in admin. Temperatures must validate as strictly increasing. Configurable hysteresis prevents rapid level changes. A safe state is used if temperature measurement fails.

## Power and operational LED

Hardware functions are modeled as selectable profiles. The historical OnOff SHIM profile initially contains:

- `poweroffPin = 4`,
- `triggerPin = 17`,
- `cutPin = 27`,
- `ledPin = 13`,
- `ledBrightnessMax = 100`,
- `ledBrightnessMin = 10`.

These are legacy defaults and must not be treated as verified MuPiHat pins. A MuPiHat profile is added after verifying the available revision.

The operational LED may use `ledBrightnessMax` while active and `ledBrightnessMin` while the idle display is off. It is turned off through the safe hardware shutdown sequence. Min/max validate to 0–100. The LXC adapter reports simulated state only.

## MQTT and Home Assistant

MQTT remains optional. Configuration includes:

- enabled/disabled,
- broker, port and TLS options,
- base topic such as `MuPiBox/Boxname`,
- client ID,
- username and secret password,
- active/idle refresh intervals,
- timeout and debug mode,
- Home Assistant discovery on/off and discovery prefix, default `homeassistant`.

Status should be event-driven where possible; refresh intervals serve as heartbeat or cover slow-changing values. Commands are validated and cannot bypass maximum volume or safe shutdown rules. Home Assistant Discovery uses stable unique IDs.

## Wi-Fi setup and captive portals

A long press on the network icon can open Wi-Fi setup. It provides:

1. network scan with signal strength and security type,
2. network selection,
3. password entry where required,
4. connection state and understandable errors,
5. forgetting or switching saved networks.

Implementation uses a network adapter over the management layer actually present on DietPi; UI and backend do not execute arbitrary shell commands. Permissions are limited to required Wi-Fi operations.

For hotel/guest networks, a captive-portal flow is planned: after connecting, the box checks internet connectivity, detects a possible redirect and can open a time-limited browser view for accepting terms. Captive portals vary widely, so this is a best-effort feature with clear cancel/back navigation, not a guaranteed fully automatic login.

## Themes and custom look and feel

Bundled themes should include at least:

- modern dark streaming look,
- arcade/8-bit look,
- classic minimal theme.

A theme consists of controlled design tokens for colors, fonts, spacing, radii, background, accent, artwork style and optional custom logos/backgrounds. Admin provides an 800×480 preview, activation and import/export of custom themes. Arbitrary CSS is not injected unchecked initially; advanced customization may later be exposed as a deliberately protected expert feature.

## Local media

One or more local media roots can be configured in admin. A root can be bound to any navigation node, for example as category, artist/series folder, album or playlist.

The scanner stays outside SQLite, storing only index/metadata and never modifying media files. It supports rescans, stable media IDs, artwork/metadata and clear errors for unavailable paths. File access is restricted to explicitly allowed media roots.

## Admin structure

Planned sections:

- Overview,
- Content and navigation,
- Playback profiles,
- Local media,
- Spotify and other providers,
- Audio,
- Display and themes,
- TTS,
- Network,
- MuPiHat, battery, fan and LED,
- Power and timers,
- MQTT/Home Assistant,
- Security,
- Backup, update and diagnostics.
