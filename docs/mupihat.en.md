# MuPiHat integration

> [Deutsch](mupihat.md) · **English**

## Decision

The first hardware integration keeps the existing Python BQ25792 driver. It already knows the register map and has been exercised with the MuPiHat. The Go service owns configuration, the admin UI, the status API, safety decisions and player coordination.

The old wrapper will not be copied unchanged: it exposes Flask on `0.0.0.0:5000` and writes status directly to `/tmp/mupihat.json`. MuPiBox-NG will use a small local-only hardware agent without a public HTTP interface.

The existing driver is GPLv3. Reuse must preserve its licence, attribution and source-code availability.

## Responsibilities

| Component | Responsibility |
| --- | --- |
| Python hardware agent | I²C access, BQ25792 initialisation, watchdog reset, register reads and approved writes |
| Go service | SQLite settings, validation, admin API, status, warnings and controlled shutdown |
| Qt/web player | Child-friendly battery display without technical values in the normal player |
| Admin UI | Profile selection, custom profile, current limit, raw diagnostics and explicit hardware enablement |

The agent runs as a dedicated systemd service with the minimum I²C/GPIO group access. It communicates locally through a Unix socket below `/run/mupibox-ng/`; no network port is opened. If the agent fails, playback continues and the UI reports an unknown battery instead of keeping stale values.

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

1. Simulated agent and contract tests in the LXC.
2. Read-only agent on the MuPiHat: watchdog, raw values and diagnostics.
3. SQLite profiles and admin UI.
4. Validated write access for the input current limit.
5. Warning and controlled shutdown.
6. MuPiHat GPIO for the operational LED, power button and later fan control.

Steps 4 and 5 require a recorded test on real hardware.
