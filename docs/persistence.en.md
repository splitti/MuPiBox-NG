# Persistence

> [Deutsch](persistence.md) · English

## Decision

Dynamic MuPiBox data will be stored in SQLite. JSON remains limited to a small bootstrap/deployment configuration.

Reasons for SQLite:

- atomic changes and transactions,
- robust concurrent reads/writes inside the Go service,
- easy sorting and filtering for admin UI and player,
- explicit schema versioning and migrations,
- a single local file without an additional database server,
- straightforward backup/restore,
- very small amounts of navigation and settings data.

A large runtime-rewritten JSON configuration is explicitly not the long-term admin persistence format.

## File layout

Planned service paths:

- `/etc/mupibox-ng/config.json` – bootstrap/deployment,
- `/var/lib/mupibox-ng/mupibox.db` – SQLite database,
- `/srv/mupibox/music` – local media,
- future secrets under restrictive permissions outside Git and public APIs.

The bootstrap configuration currently contains exactly four startup values: listen address, database path, local media root and audio backend. Volume, TTS, power, navigation, media assignments and admin language are loaded exclusively from SQLite. For an empty database, the service inserts defined initial records directly into SQLite; the JSON file is not a second configuration source.

## Planned data areas

### Global box settings

- UI/box language,
- maximum volume,
- TTS enabled/disabled,
- TTS language,
- TTS provider and non-secret provider options,
- idle shutdown timeout in minutes,
- future schedules and other device options.

### Navigation and content

Navigation will not be permanently modeled as a rigid `category -> row` hierarchy. Instead it is a freely configurable tree. A navigation node may be a pure container or directly bind to a content source.

At minimum, persist:

- stable node ID,
- optional parent ID,
- sort position,
- visible/enabled state,
- localized names,
- selection behaviour (`inline`, `drilldown`, later optionally `play`),
- presentation mode,
- optional provider binding,
- provider, source type and provider reference,
- requested content mode (`auto`, `children`, `albums`, `tracks`, `items`),
- optional manually curated items.

This allows `Die drei ??? Kids`, for example, to live under `Audiobooks` and open an album view when tapped, or to be a top-level category that directly exposes Spotify albums or tracks.

Details: [Dynamic navigation and content model](navigation-model.en.md).

Localized text is not modeled as fixed `de`/`en` columns, so more languages can be added without schema changes.

### Playback progress

Progress is keyed by a stable provider-neutral media ID. Position and duration use 64-bit millisecond values so even a single very long file is safe. Additional fields include update time, completed state and optional context/queue ID plus track index. Writes occur at a bounded interval and on all relevant state transitions.

Expected core object: `playback_progress`. A progress record must never depend only on a filename or ephemeral queue position.

### Future data

- RFID/button mappings,
- podcast subscriptions and progress,
- audiobook progress,
- provider/account metadata,
- player/queue restoration,
- admin/device settings,
- battery profiles and fan/LED/display settings,
- provider accounts and expiring provider cache.

Secrets such as API keys or refresh tokens are never exposed unencrypted through public APIs. Exact secret storage is defined separately.

## Schema outline

Concrete tables are introduced through migrations. Expected core objects:

- `schema_migrations`
- `settings`
- `navigation_nodes`
- `navigation_node_labels`
- `content_bindings`
- optional manual items/mappings
- `playback_progress`
- `battery_profiles`
- `provider_accounts` and `provider_cache`

A `content_bindings` record may provider-neutrally reference, for example, a Spotify artist, playlist, album, local folder, RSS feed, or radio station.

IDs remain stable and independent from display names. Ordering is stored explicitly rather than inferred from names or creation time.

The current `categories`/`content_rows` objects in the development prototype are explicitly an intermediate representation, not the final persistence schema.

## Migrations

Every schema change receives a unique monotonic migration. On startup, the database version is checked and pending migrations are applied in defined order.

Migration-critical releases must consider backup and rollback. Rolling back a binary must never silently damage a newer incompatible database.

## SQLite configuration

The application uses exactly one local database file. Foreign keys are enabled. Writes should remain short and transactional. WAL may be enabled after measurement on target hardware, but is not required by the model.

The MVP currently uses `github.com/mattn/go-sqlite3`. Functional database operation requires CGO, so the service is built natively on the target with CGO enabled. This choice remains provisional until Pi 3 validation.

The final Go driver is selected only after Pi 3/ARM64 testing of:

- `CGO_ENABLED=0`/cross-build compatibility or an acceptable CGO build path,
- binary size and memory usage,
- startup and migration time,
- reliability on DietPi,
- license and maintenance state.

The store layer should use `database/sql` or a narrow internal interface so the concrete driver does not leak into UI, player, or provider logic.

## Backup

The database contains user configuration and must survive updates. Planned later:

- consistent local backup,
- import/restore through admin,
- optional export of a human-readable configuration for diagnostics/migration,
- no media copies inside the configuration database.
