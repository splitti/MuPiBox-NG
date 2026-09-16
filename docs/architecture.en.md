# Architecture – foundation

> English. German version: [architecture.md](architecture.md)

The interfaces are designed around one box with one shared player.

| Package | Responsibility |
| --- | --- |
| `cmd/mupibox` | bootstrap configuration, adapter selection, HTTP lifecycle, SIGTERM/SIGINT |
| `internal/library` | local-folder scan, stable IDs, track ordering, path validation |
| `internal/core` | serialized commands, queue, status, volume limit, track changes |
| `internal/audio` | audio adapter contract; mpv and explicit simulation |
| `internal/server` | JSON API, home/category model, box status, input mappings, cover serving |
| `webui` | embedded HTML/CSS/JS |
| future `internal/store` | SQLite, migrations, settings, categories/rows, persistent state |
| future `internal/tts` | common TTS contract and provider adapters |
| future `internal/power` | idle detection, schedules, controlled shutdown |
| future `internal/providers` | local sources, Spotify, radio, RSS; normalized content and cache |
| future `internal/hardware` | MuPiHat, battery profiles, GPIO, fan and operational LED |
| future `internal/display` | brightness, display idle, splash screen and display adapters |
| future `internal/network` | Wi-Fi state, scan/connect and captive-portal detection |
| future `internal/mqtt` | MQTT state/commands and Home Assistant Discovery |

All control modules call the same controller. It serializes commands and audio access with a mutex; copied status objects do not share mutable queue slices. The backend is currently polled every 500 ms and browsers every 750 ms. Polling is intentionally simple for M1.

mpv is started without a shell or user configuration. A private 0700 temp directory contains the Unix socket. The controller passes only scanned and revalidated file paths. Decoder errors move the player into an error state; EOF advances the queue.

A new mpv process is currently created per track. This simplifies stale-event isolation but is not gapless. Resource use and load time still need measurement on Pi 3. The IPC socket is never exposed to the network.

## UI architecture

The primary target is a 5-inch 800 × 480 landscape touch display. Design starts with the small display and scales outward to larger browsers. Hover must never be required for operation or information.

The UI conceptually has four layers:

1. slim system status bar,
2. freely configurable text-based categories,
3. artwork-oriented content rows within categories,
4. compact Now Playing/player area.

The status bar should expose normalized system information such as Wi-Fi, signal strength, battery/charging state, time and later additional states. The UI must not depend directly on MuPiHAT, NetworkManager or OS-specific details. Hardware/system adapters provide a common status model. Unknown values remain unknown rather than being rendered as 0%.

## Data-driven home screen

Categories such as `Audiobooks`, `Music`, `Radio` or `Podcasts` are not hard-coded frontend pages. The backend exposes a generic model of categories, rows and media objects through `/api/home`. The frontend renders that model without category-specific logic.

The current intermediate step still reads categories from JSON. Once persistence is implemented, the same data comes from SQLite. The future admin UI edits the same model persistently, allowing categories and rows to be created, deleted, sorted, translated, enabled/disabled and connected to providers without frontend changes.

A category has at minimum a stable ID, localized display names, ordering and visibility. A content row likewise has a stable ID, localized label, ordering, visibility and a provider. Providers produce normalized media objects. `local-library` exists today; later providers may include Spotify, radio, RSS podcasts and manually curated items.

A normalized media item may contain stable ID, type, title, subtitle, artwork, provider reference and an executable action. This allows the same horizontal row to represent local folders, artists, Spotify playlists, radio stations, audiobooks or podcasts.

Vertical scrolling moves through categories/rows; horizontal scrolling browses a content group.

## Global box settings

TTS, language, idle timers and other device options are **box-wide settings**, not category properties. The admin UI writes these values to persistent settings storage.

Today `/api/info` already exposes development TTS and power values separately from the home/category model. This intentionally matches the target architecture.

Audio, display, network, MuPiHat/battery, fan, LED, MQTT and themes are also box-wide settings or adapter capabilities. Display idle and box shutdown remain separate state machines. Details: [System, hardware and admin settings](system-settings.en.md).

## TTS

TTS is treated as its own adapter. Global TTS state includes at least:

- `enabled`,
- language/locale,
- provider,
- later voice ID and provider-specific options.

When TTS is enabled, tapping a category may speak the category's localized name. The category itself does not own a language or speech engine.

The TTS contract passes text plus language/locale to a provider. Providers may be local/offline or external. A local engine is important so a box using local media can remain fully usable without internet access. External providers are optional and must store credentials separately and securely.

`browser-dev` is a development provider only. It is not a production TTS solution.

## Progress and resume

Progress is persisted by stable normalized media ID rather than current queue position. Position and duration use 64-bit millisecond values. This safely resumes even a single 23-hour file. Writes occur periodically and on pause, seek, source change and controlled shutdown. Spoken-word content defaults to clearly labeled 10-second skips.

Details: [Dynamic player and playback profile](player-model.en.md).

## Power management

Power management is planned as its own service/adapter rather than direct UI/player logic.

The first use case is `idle_shutdown_minutes`: after the configured period without active playback and without relevant user activity, the box may perform a controlled shutdown. `0` disables the feature.

Later additions may include schedules, quiet hours or fixed shutdown times. Before shutdown, writes must be completed, persistent state must be consistent and audio must be closed cleanly. Development LXC environments must never power off their host system accidentally.

## Persistence and data separation

Dynamic administration data will be stored in SQLite. JSON remains a small bootstrap configuration.

| Type | Target path |
| --- | --- |
| program | `/usr/local/lib/mupibox-ng/mupibox` |
| bootstrap config | `/etc/mupibox-ng/config.json` |
| SQLite database | `/var/lib/mupibox-ng/mupibox.db` |
| secrets/provider credentials | restrictive storage below `/var/lib/mupibox-ng/` or a later secrets abstraction |
| local music | `/srv/mupibox/music` or another configured path |

SQLite stores categories, rows, translations, global box settings, mappings and later progress/state. The schema is versioned and changed through migrations. Updates must preserve settings and media.

Details: [persistence.en.md](persistence.en.md)

## Administration

The admin UI is a separate management surface, likely under `/admin`. It should at minimum manage categories/rows, order/visibility, translations, provider/source assignments, global TTS settings, idle timers, maximum volume and later further device/account settings.

The child/player UI receives only the normalized read model. Admin writes and permissions are secured separately and are not equivalent to today's development API.

## Expansion

- add SQLite store with migrations and a repository/service layer,
- add admin API and admin web UI,
- add TTS adapter with at least one offline provider; external providers are optional,
- implement idle detection and controlled shutdown,
- add media providers behind a common provider contract,
- expose normalized Wi-Fi/battery/charging/system status,
- integrate Spotify with separate auth/catalog/playback adapters, compliant temporary caching and per-box account separation,
- add RSS feed management, episodes and progress,
- move simulated inputs into real GPIO/RFID adapters,
- implement update/rollback with versioned binaries and verified DB migrations.

## Device validation

Boot Pi 3 ARM64/DietPi; identify ALSA/MuPiHAT audio device, verify audio, 800×480 DSI touch and kiosk mode. Specifically measure touch target sizes, vertical/horizontal scrolling, status bar readability, artwork performance, TTS latency, SQLite latency and idle/shutdown behavior. Final GPIO/power control should only be implemented after hardware revision is confirmed.
