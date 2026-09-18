# Binding project requirements

> English. German version: [requirements.md](requirements.md)

Status 2026-09-16, based on splitti's handover and ongoing project decisions. Requirements do not imply that these features are already implemented. The current implementation status is documented in README and CHANGELOG.

## Product and platform

MuPiBox-NG is a music player for children/families with touch, buttons, RFID and a home-network browser UI. Input methods must work individually or in any combination; all control the same player, status and queue. Optional hardware must never be required for the application to start.

Target: DietPi, ARM64/64 bit, Raspberry Pi 3 minimum. Pi 2 is no longer a target. Development and automated tests run in a Debian 13 LXC without Pi hardware. Test hardware includes a Pi 3, probably a Pi 4, and a MuPiHAT. Audio, GPIO, power and display integration must later be verified on real hardware.

## Hardware

Prefer MuPiHAT, while keeping other hardware behind adapters. Required capabilities include battery/charging, controlled power on/off and shutdown, and mono/stereo audio. Before implementation, verify the exact board revision, pinout, drivers, shutdown sequence and available battery/charging telemetry. Never display a battery percentage unless real telemetry exists.

Primary display: Waveshare Raspberry Pi 5inch Capacitive 5-Points Touch Display, 800 × 480, DSI, landscape. Other resolutions remain supported. The 5-inch display is the primary design reference; desktop and phone layouts are secondary.

## UI and navigation

The UI should broadly follow a streaming-style layout similar to Netflix without copying it directly. The goal is a dark, clearly structured interface for very small touch displays, with limited text and large touch targets.

A slim persistent status bar sits at the top, similar to Android. It should later show Wi-Fi status/signal strength, battery/charging state and other relevant system information. A pull-down expanded status area is a later enhancement.

The main area consists of freely configurable categories such as Audiobooks, Music, Radio and Podcasts. Categories are not hard-coded frontend pages. They are delivered by the backend and displayed vertically. Administration must be able to create, delete, rename, translate, sort, enable/disable categories and attach arbitrary content rows.

Each category can contain freely configurable content rows. Rows may show artists, albums, playlists, audiobooks, radio stations or podcasts with artwork. Rows should normally scroll horizontally while categories scroll vertically. The layout must remain touch-friendly at 800 × 480 and must not depend on hover states.

Navigation should remain shallow. Important playback controls must require only a few touches. Active playback should use a compact Now Playing area with artwork, title, progress and large controls.

## Box-wide TTS configuration

TTS is **not configured per category**. It is configured once for each box. The admin UI must allow TTS to be enabled/disabled, select a language and choose/configure a TTS provider.

When TTS is enabled, tapping a category may speak its localized category name. The spoken language follows the box-wide TTS language. Categories therefore store localized display names only; they do not own their own speech engine or language configuration.

TTS must be provider-independent. Planned options include:

- at least one local/offline TTS solution for standalone operation,
- optional free or free-tier external TTS providers where practical,
- documented setup instructions for providers requiring API keys or credentials,
- interchangeable providers without changes to the player/category UI.

Credentials for external TTS providers must never be stored in Git or exposed in public API responses. Browser TTS is development fallback only.

## Administration and dynamic configuration

The UI should be data-driven. The future admin UI is the central management surface for categories, rows, global box settings and content providers. Category and row changes must not require editing HTML, CSS or JavaScript.

The admin UI should at minimum manage:

- categories: create/delete, order, visibility, localized names,
- content rows: create/delete, order, visibility, provider/source,
- manual and dynamic row contents,
- global TTS: on/off, language, provider and provider configuration,
- power/timers: idle shutdown and later schedules,
- maximum volume and other box settings,
- later RFID/button mappings, provider accounts and further device settings.

Details: [System, hardware and admin settings](system-settings.en.md)

This includes startup/shutdown sounds, startup and maximum volume, audio output, splash screen, separate display idle, brightness, admin password, themes, MuPiHat/battery profiles, fan, operational LED, native Home Assistant integration, box status and Wi-Fi setup including a best-effort captive-portal flow.

## Power and timers

The box requires configurable idle shutdown. Example: if no playback is active and no relevant user activity occurs for the configured period, the box may perform a controlled shutdown. `0` or “Off” disables idle shutdown.

Later power management may also include schedules such as allowed operating windows, quiet hours or planned shutdown. Shutdown must always be controlled and must not corrupt database writes, migrations or player state.

## Playback progress

Spoken-word content defaults to 10-second skips. Playback progress is persisted even for a single many-hour audio file and resumes after pause, restart or shutdown. Details: [Dynamic player and playback profile](player-model.en.md).

## Persistence

SQLite is the preferred persistence layer for data maintained dynamically through the admin UI. This includes categories, rows, global box settings, TTS configuration, idle timers, mappings and later progress/state data.

JSON remains suitable for a small number of static bootstrap/deployment settings such as listen address, database path, development mode or initial installation defaults. The live admin configuration should not be maintained by rewriting a large JSON file.

SQLite must use explicit schema versions and migrations. Settings and user data remain outside the application binary and must survive updates.

Details: [persistence.en.md](persistence.en.md)

## Media sources

Required targets: local music organized by directory, whole folders, configured Spotify albums/artists/playlists and other content, web radio/music streams and podcasts. Sources must remain replaceable and extensible.

Spotify: Premium Family is available, with a separate account/login per box. Authentication, selection and playback must be validated end-to-end. Credentials must never be committed. Catalogue/API, playback and temporary caching remain separate; metadata/artwork may only be cached temporarily and according to current rules, while audio is not an ordinary local cache.

Details: [Spotify integration and caching](spotify.en.md)

Podcasts: RSS subscriptions, episode lists and saved progress.
Amazon Music remains a future target; technical/legal feasibility has not yet been confirmed.

Individual local video clips and YouTube videos are planned as a later phase. They should reuse navigation, artwork/thumbnails, player policy and progress persistence. Exact YouTube integration and legal/API constraints will be decided in that later step.

## Input modules

Supported operating modes include touch only, buttons + RFID without display, and any combination with browser control. Example: RFID starts an album, the display shows artwork, a button skips, and a phone pauses the same shared player.

Buttons: play/pause, previous/next, volume and configurable mappings. RFID: map cards to folders, Spotify content, radio or podcasts; start/resume. Optional pause-on-removal only with readers that reliably report card presence. Simulated buttons/RFID are required for development.

## Software and data

Go backend as a systemd service. Shared player/queue, separate media sources, independently enabled input modules, hardware adapters and responsive web UI. M1 decision: embedded HTML/CSS/JS plus mpv adapter and an explicit simulation backend; no binding decision for Spotify or future providers.

Categories and content mappings are data, not frontend code. The frontend renders categories, rows and media items returned by the backend, allowing local music, Spotify, radio and podcasts to share the same navigation model.

## Repository and versioning

Repository: https://github.com/splitti/MuPiBox-NG
Reference: https://github.com/splitti/MuPiBox
Installer reference: https://raw.githubusercontent.com/splitti/MuPiBox/main/autosetup/autosetup.sh
The legacy stack is a reference, not a requirement.

Preserve history; do not delete old code without review. Use traceable commits/branches, marked releases, visible versions and changelogs; later support updates with rollback while preserving settings and media.

Project and GitHub documentation is maintained in German and English. Both versions must express the same binding decisions.

## Milestones

1. Go service, 800×480 UI, local folder library, local playback and shared status.
2. Data-driven 800×480 navigation with status bar, freely configurable categories and artwork-oriented rows.
3. SQLite persistence and admin UI for categories, rows and global box settings.
4. Global TTS abstraction with a local offline provider plus optional external providers.
5. Idle/timer controls and robust power management.
6. Persistent progress, robust error/restart handling and web radio.
7. RSS podcasts and resume support.
8. Spotify as a fully validated per-box workflow.
9. Hardware adapters/setup, Pi/MuPiHAT testing, kiosk, battery/Wi-Fi status and safe poweroff.
10. Releases, update/rollback and evaluation of additional platforms such as Amazon.

The order of later milestones may change based on technical findings.
