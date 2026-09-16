# Dynamic player and playback profile

> [Deutsch](player-model.md) · English

## Goal

MuPiBox does not use the same player UI for every kind of media. Instead, a navigation node, content binding or
media object receives a configurable playback profile. This allows spoken-word content to use a very reduced player,
while a music playlist may expose a scrollable track list, shuffle and repeat.

The web UI must not decide which controls to show based on hard-coded media types such as `spotify`, `audiobook` or
`playlist`. It renders the player policy supplied by the backend.

## Base profiles

### Reduced spoken-word / audiobook mode

Typical 800x480 presentation:

- large cover,
- title/episode and optional chapter/track number,
- progress bar with position and duration,
- large play/pause control,
- large previous/next controls for the allowed navigation behavior,
- volume,
- back navigation.

Track list, shuffle and repeat may be hidden completely. This is the preferred default for linear audio dramas and
audiobooks.

### Music / playlist mode

For music, playlists or other freely selectable track collections, a scrollable queue/track list can replace most of
the large-cover area. The currently playing row is highlighted. Tapping a row starts that track directly.

Depending on policy, the UI may additionally show:

- shuffle,
- repeat,
- repeat-one,
- queue/track list,
- direct track selection,
- a smaller now-playing cover,
- previous/next.

## Player policy

The exact field names will be finalized during implementation. Semantically, a policy should at least express:

- `show_queue` – display queue/track list,
- `allow_track_selection` – allow selecting individual tracks,
- `allow_shuffle` – expose shuffle,
- `allow_repeat` – expose repeat,
- `allow_repeat_one` – allow repeating one track,
- `show_cover` – display cover art,
- `cover_size` or a presentation profile,
- `allow_previous` / `allow_next`,
- `allow_seek` – allow seeking inside the current item,
- `allow_volume` – show volume controls,
- optional `previous_next_behavior` – track change, chapter change or time skip.

A navigation node/content binding may define the policy. Providers may supply sensible defaults. The admin UI can
override the allowed features.

## Admin presets

To avoid configuring many switches for every item, the admin UI should offer practical presets, for example:

- `Audio drama / audiobook` – large cover, no queue, no shuffle/repeat,
- `Music / playlist` – queue, track selection, shuffle and repeat allowed,
- `Album` – queue and track selection, shuffle optional,
- `Radio` – minimal now-playing view, no queue/seek when unsupported by the stream,
- `Custom` – all options editable individually.

Presets are defaults only. Persist either the resulting policy or a stable preset reference with optional overrides.

## Track list and touch

A visible track list must remain easy to operate on the 800x480 touchscreen. Each row shows at least a title and,
where useful, subtitle/artist/duration. The currently playing item is clearly highlighted.

Default interaction:

- tap the track row: play that track,
- optional speaker/TTS button inside the row: read the title aloud,
- no hidden hover-only action,
- no long-press as the sole access path to an important function.

This keeps `tap = play` unambiguous while TTS remains separately available when global box TTS is enabled. On very
small layouts, the TTS button can be omitted when TTS is disabled globally.

## TTS in the queue

TTS remains a global box feature with globally selected language and provider. The player policy only determines
whether TTS actions may be offered for visible tracks.

The spoken string comes from normalized media metadata. The web UI does not contain provider-specific TTS logic;
Spotify, local files and podcasts use the same path.

## Progress and queue

Player state and queue remain server-side shared truth for touchscreen, browser, RFID and buttons. The list view
therefore reflects the same queue that may be changed by external inputs.

Audiobooks/audio dramas should later support persistent progress. The playback profile controls visible/allowed UI
features; it does not define the technical progress-storage mechanism.

## Time skips for spoken-word content

For audio dramas and audiobooks, previous/next defaults to a **10-second** backward/forward time skip. Player policy gains a mode such as `time_skip` plus separate backward and forward values. Other profiles may continue to use track or chapter navigation.

A skip is clamped to `0` and the known media duration. Touch targets clearly show the amount, for example `−10` and `+10`, instead of using symbols that can be confused with track changes.

## Persistent resume

Progress is stored per stable normalized media item, regardless of whether an audiobook contains 22 tracks or **one single 23-hour file**. A long single track must not restart at zero after pause, restart or shutdown.

A progress record contains at least:

- stable media ID including provider/account context,
- position in milliseconds as a 64-bit value,
- known duration in milliseconds,
- last update timestamp,
- completed/not completed state,
- optional queue/track index and context ID for collections.

Progress is written periodically during playback and always on pause, seek, item/source change, controlled shutdown and process exit. Write frequency is bounded to avoid unnecessary SQLite load; after a hard power loss, at most a short defined interval may be lost.

Restarting an unfinished item resumes at the stored position. Near the beginning no pointless resume is offered; near the end, configurable rules may mark an item completed. Deliberately starting from the beginning may reset progress.

Progress is server-side and therefore identical for touch, buttons, RFID, browser and supported providers. Provider-specific progress may be synchronized, but the local MuPiBox database remains the reliable shared abstraction.

## Resume policy by source

Progress is enabled by default for finite media: local audio, Spotify tracks/episodes, podcasts and future video files or videos. Live radio and other non-seekable live streams use `resume_policy = none` and never create a progress record.

Spotify uses the same local progress contract with provider, account and media IDs. Future provider synchronization may complement this, but does not replace MuPiBox's shared state.

## Persistence

Playback profiles will later be stored in SQLite. Expected concepts include:

- `playback_profiles`,
- assignment of a profile to `navigation_nodes` or `content_bindings`,
- optional provider default profiles,
- optional per-node/content overrides.

The resolved profile is delivered to the frontend alongside content so that the player UI remains data-driven.
