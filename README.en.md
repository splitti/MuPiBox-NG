# MuPiBox-NG

> [Deutsch](README.md) · **English**

A modular music player for **DietPi ARM64, Raspberry Pi 3 or newer**. Touch, browser, buttons and RFID should all control the same playback state and queue.

**Development status: 0.1.0-dev.** Not production-ready; a first experimental DietPi installer is available for hardware testing. The unchanged Go prototype from 2026-02-10 remains under `legacy/prototype/`. Full Git history is preserved. Development branch: `rebuild/go-foundation`.

## What works now

- Go HTTP service with embedded web UI.
- Small touch UI designed primarily for 800 × 480 landscape.
- Data-driven categories and content rows through `/api/home`.
- Categories, media sources and localized names are stored in SQLite and managed through the admin web UI.
- Global TTS development setting: enabled/disabled, language and provider; `browser-dev` is test fallback only.
- Global power development setting with `idle_shutdown_minutes`; real shutdown is not implemented yet.
- Local folder library with subdirectories, natural ordering, whole-folder playback and artwork.
- Shared queue for all clients with play/pause, previous/next, seek and volume.
- Replaceable audio backend: `mpv` for box audio, `simulated` for tests without audio hardware.
- Simulated button and RFID mappings. No real hardware drivers yet.
- SQLite database with automatic migrations for box settings, navigation and playback progress.
- First real admin web UI at `/admin/` for global settings plus categories/rows.
- Optional admin password protection plus backup/restore for SQLite data and media, and guarded GitHub release switching with rollback.
- Persistent resume for local audio including single very long files; shared progress contract for future providers such as Spotify, with live radio excluded.

The target UI is not a hard-coded music page. Categories such as Audiobooks, Music, Radio or Podcasts are data. The future admin UI will manage this model together with global box settings.

**Persistence decision:** dynamic admin data lives in SQLite. JSON contains only the listen address, SQLite path, local media root and audio backend.

## Development in the LXC

Requirements: Go 1.24+, Git. Verified: Debian 13 / Go 1.24.4, project path `/opt/mupibox-ng`.

```sh
cd /opt/mupibox-ng
mkdir -p music

go test ./...
rm -f bin/mupibox && \
go build -buildvcs=false -o bin/mupibox ./cmd/mupibox && \
./bin/mupibox -config deploy/config.dev.json
```

Open on the home network at `http://<LXC-IP>:8090`.

`-buildvcs=false` avoids the known local Go/Git VCS-stamping failure when a root build is performed in a checkout owned by another user. `rm` plus `&&` ensures an old binary is never started after a failed build.

## Dynamic home screen

For an empty SQLite database, the service creates these initial categories:

- Audiobooks / Hörbücher
- Music / Musik
- Radio
- Podcasts

A category has a stable ID and localized labels. It may contain any number of rows. Each row also has localized labels and a provider. `local-library` already produces artwork tiles from the local music library.

TTS is **not configured per category**. The box has global `enabled`, `language` and `provider` values. When TTS is enabled, tapping a category may speak the category name in the selected box language. Development provider `browser-dev` uses browser speech synthesis; production architecture will use a dedicated local/external TTS adapter.

## Admin and SQLite

The admin surface is available at `/admin/`. Its first version already manages global basics plus categories, ordering, translations and row/provider assignments. Further planned areas include:

- categories including order, visibility and translations,
- content rows and provider assignments,
- global TTS settings and provider setup,
- maximum volume,
- idle shutdown and later schedules,
- later RFID/buttons, provider accounts and further device settings.

These values live in the SQLite file configured by `database_path`; the service target is `/var/lib/mupibox-ng/mupibox.db`. JSON remains bootstrap/deployment only. An admin password can be set in the security section; it protects admin and externally accessed connectivity APIs with a server-side session.

## Real local playback

Install `mpv` through Debian/DietPi. The LXC additionally needs an audio device for audible output; otherwise test on a Pi. Simulation is not an audio test.

```sh
sudo apt-get install mpv
cp deploy/config.example.json config.local.json
# Set backend to "mpv" in config.local.json.
./bin/mupibox -config config.local.json
```

Supported extensions: MP3, FLAC, OGG, OPUS, WAV, M4A, AAC. Actual codec support depends on mpv. Artwork names: `cover.jpg`, `cover.png`, `folder.jpg`, `folder.png`.

## API

| Route | Purpose |
| --- | --- |
| `GET /api/home` | categories, rows, localized labels and normalized media items |
| `GET /api/library` | local folders, tracks, stable IDs and artwork URLs |
| `GET /api/status` | shared queue and adapter status |
| `GET /api/info` | version, backend and global TTS/power development values |
| `GET /api/health` | HTTP service health |
| `POST /api/command` | playback command |
| `POST /api/input` | simulated button/RFID event in development mode |
| `GET/PUT /api/admin/settings` | persistent global box settings |
| `GET/PUT /api/admin/navigation` | persistent categories and rows |
| `GET /api/admin/auth`, `POST /api/admin/login` | password protection and admin session |
| `PUT /api/admin/password`, `POST /api/admin/logout` | set/change password and end the session |
| `GET /api/connectivity/wifi/adapters` | Wi-Fi adapters, state and active selection |
| `PUT /api/connectivity/wifi/preferences` | select exactly one Wi-Fi adapter by stable MAC and optionally disable onboard Wi-Fi |
| `GET/PUT /api/admin/samba` | manage the `/srv/mupibox` Samba share |
| `GET /api/admin/backup`, `POST /api/admin/restore` | back up or restore configuration and optional media |
| `GET /api/admin/releases`, `POST /api/admin/releases/switch` | list, install or roll back releases |
| `GET /api/admin/system` | boot time, slow units, swap, network backend and CPU profile |

Admin and remotely accessed connectivity APIs are protected once an admin password is set. The development box must still remain on a trusted home network and must not be port-forwarded directly to the internet.

## Tests and limits

`go test ./...` covers library/path boundaries, queue/EOF, volume, concurrent control, API, data-driven home model, global box info, RFID/button simulation and a Linux ARM64 cross-build.

Still open: production local TTS, actual idle shutdown, web radio, RSS podcasts, Spotify/Amazon playback, the native Home Assistant integration, video/YouTube, real RFID/GPIO/MuPiHAT access and complete native player controls. Parts of their admin configuration are already persisted.

## Documentation

- [Requirements (German)](docs/requirements.md) · [Requirements](docs/requirements.en.md)
- [Architecture (German)](docs/architecture.md) · [Architecture](docs/architecture.en.md)
- [Persistence (German)](docs/persistence.md) · [Persistence](docs/persistence.en.md)
- [Navigation/content (German)](docs/navigation-model.md) · [Navigation/content](docs/navigation-model.en.md)
- [Player/playback profiles (German)](docs/player-model.md) · [Player/playback profiles](docs/player-model.en.md)
- [System/hardware settings (German)](docs/system-settings.md) · [System/hardware settings](docs/system-settings.en.md)
- [Backup/restore/updates (German)](docs/backup-update.md) · [Backup/restore/updates](docs/backup-update.en.md)
- [Admin area/best practices (German)](docs/admin-guide.md) · [Admin area/best practices](docs/admin-guide.en.md)
- [Spotify/cache (German)](docs/spotify.md) · [Spotify/cache](docs/spotify.en.md)
- [Repository audit (German)](docs/repository-audit.md) · [Repository audit](docs/repository-audit.en.md)
- [DietPi/device test (German)](docs/device-install-dietpi.md)
- [Validation (German)](docs/validation.md) · [Validation](docs/validation.en.md)
