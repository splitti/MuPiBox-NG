# MuPiHat integration

> [Deutsch](mupihat.md) · **English**

## Decision

MuPiBox-NG is **Go-first**: the BQ25792 is driven directly from Go over I²C, covering only the
functionality MuPiBox actually needs (see the register list below) -- not a full 1:1 port of the
existing ~250 KB Python library.

The existing Python driver (GPLv3) is kept as:

- the register reference and a template for the initialisation sequence,
- a documented comparison for charge-state and fault behaviour,
- a possible fallback if the native Go path hits a concrete, demonstrated blocker on real
  hardware.

It is therefore **no longer automatically the target architecture**. If such a blocker occurs it
will be documented and a Python fallback proposed -- never a silent switch back to Python. Where
parts of the existing driver are reused (register constants, initialisation logic as a template),
its licence, attribution and source-code availability are preserved per GPLv3.

The old standalone Python wrapper (Flask on `0.0.0.0:5000`, status written directly to
`/tmp/mupihat.json`) is not carried over. Privileged hardware access (I²C/GPIO) goes through the
existing `mupibox-system-agent`, or a clearly justified extension of it -- no separate,
permanently running hardware service just for the MuPiHat. The exact wiring is decided alongside
the actual BQ25792 implementation.

## Responsibilities

| Component | Responsibility |
| --- | --- |
| `mupibox-system-agent` (or an extension) | I²C access, BQ25792 initialisation, watchdog reset, register reads and approved writes |
| Go service (`mupibox-ng`) | SQLite settings, validation, admin API, status, warnings and controlled shutdown |
| Qt/web player | Child-friendly battery display without technical values in the normal player |
| Admin UI | Profile selection, custom profile, current limit, raw diagnostics and explicit hardware enablement |

The privileged access runs under a restricted user with the minimum I²C/GPIO group membership and
communicates locally through a Unix socket below `/run/mupibox-ng/` or the existing
`mupibox-system-agent` socket; no network port is opened. If it fails, playback continues and the
UI reports an unknown battery instead of keeping stale values.

## Status contract

The hardware agent exposes at least:

- timestamp and connection state,
- `vbat_mv`, `vbus_mv`, `ibat_ma` and `ibus_ma`,
- charger IC temperature,
- charger state and phase,
- battery-present state,
- calculated charge from 0 to 100 percent,
- selected battery profile and input current limit,
- warning, shutdown and fault states.

Charge is calculated using piecewise-linear interpolation between `v_0`, `v_25`, `v_50`, `v_75` and `v_100`. This avoids the old five-value jumps. Averaging and hysteresis prevent flicker under load. Warning and shutdown states require several consecutive samples.

## Battery profiles

All voltages are stored in SQLite as integer millivolts.

| Profile | 100% | 75% | 50% | 25% | 0% | Warning | Shutdown |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| Ansmann 2S1P | 8100 | 7800 | 7400 | 7000 | 6700 | 7000 | 6800 |
| ENERpower 2S2P 10,000 mAh | 8000 | 7700 | 7300 | 6900 | 6000 | 6500 | 6150 |
| USB-C mode (no battery) | 1 | 1 | 1 | 1 | 1 | 0 | 0 |
| Custom | 8100 | 7800 | 7400 | 7000 | 6700 | 7000 | 6800 |

`Custom` remains fully editable. The USB-C profile disables battery warnings and battery-triggered shutdown. Profile changes are validated and must satisfy `v_100 >= v_75 >= v_50 >= v_25 >= v_0`.

## Audio

MuPiHAT V3.x uses a MAX98357A (I2S Class-D amplifier, 2x3 W). Its device-tree overlay
(`dtoverlay=max98357a,sdmode-pin=16` + `dtoverlay=i2s-mmap`) is no longer hand-maintained in
`/boot/firmware/config.txt`; it is managed by `mupibox-system-agent` inside a clearly marked
block (`# BEGIN/END MUPIBOX-NG MUPIHAT AUDIO`, `PUT /api/admin/audio/mupihat`), idempotently and
without touching unrelated lines. Enabling it requires a reboot; the admin UI states this clearly
and never reboots automatically.

`GET /api/admin/audio/status` detects, read-only (no root needed), `aplay -l`,
`mpv --audio-device=help`, whether the overlay block is configured, and whether a MAX98357A card
is actually visible. The concrete mpv playback device (`PUT /api/admin/audio/device`) is stored
in SQLite and passed to mpv as `--audio-device` on the next start (`internal/audio/mpv.go`); like
volume/`max_volume`, a change only takes effect after a player restart.

Stereo/mono is a hardware switch (SW3) on V3.x, not a software option -- the admin UI only
reflects the detected state, it does not switch it.

Verified on real Pi 4 / MuPiHAT V3.1 hardware: overlay activation, detection, playback device
selection and audible playback through the real MuPiBox player path (touch/API ->
`internal/audio` -> mpv -> ALSA -> MuPiHAT speaker). If a saved device no longer exists at
runtime, `internal/audio/mpv.go` has surfaced a clear error since Phase 2
(`configured audio device "..." is not available`, visible as `state:"error"` in `/api/status`)
instead of silently falling back to another device; the service stays stable and the admin UI
reachable.

**dmix instead of plughw for MuPiHAT:** `alsa/plughw:CARD=...` is exclusive -- a second mpv
process (e.g. a TTS announcement) cannot open the device while the main player still holds it,
even while merely paused. `GET /api/admin/audio/status` therefore suggests the ALSA
software-mixed `alsa/dmix:CARD=...` device for MuPiHAT cards via `recommended_device` (admin UI:
hint with a "Use" button); `plughw` remains selectable as a direct low-level option, it just isn't
the recommended default anymore. Verified on real hardware: music playback and a TTS announcement
work concurrently over `dmix` (music paused, announcement audible), with no audible quality loss
compared to `plughw`.

## Input current

| Profile | IINDMP |
| --- | ---: |
| safe | 1790 mA |
| medium | 2200 mA |
| high | 2700 mA |

Writes to the charger IC remain disabled at first. They are enabled only after read-only testing on the real Pi/MuPiHat. The admin UI must clearly distinguish a stored value from one that has actually been applied to hardware.

## Safety behaviour

- Below the warning threshold, the UI shows a prominent battery warning.
- Below the shutdown threshold, a controlled shutdown starts after a configurable confirmation period.
- External power, charger state and read failures are considered so a short voltage dip does not trigger shutdown.
- The hardware agent never shuts the computer down itself. The Go service makes that decision so playback progress, shutdown sound and SQLite can finish cleanly.
- Missing or implausible readings never trigger automatic shutdown.

## Rollout

1. Audio (MAX98357A): overlay management via `mupibox-system-agent`, device detection, device
   selection, verified audible on real Pi 4 / MuPiHAT V3.1 hardware.
2. Simulated agent and contract tests in the LXC.
3. Read-only agent on the MuPiHat: watchdog, raw values and diagnostics.
4. SQLite profiles and admin UI.
5. Validated write access for the input current limit.
6. Warning and controlled shutdown.
7. MuPiHat GPIO for the operational LED, power button and later fan control.

Steps 5 and 6 require a recorded test on real hardware.
