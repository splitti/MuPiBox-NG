# Spotify integration and caching

> [Deutsch](spotify.md) · English

## Goal

Spotify is integrated as a dedicated provider. Each MuPiBox connects a separate Spotify Premium Family account. Admin should search for artists, albums and playlists and bind results to navigation nodes without manually entering IDs.

## Clear separation

The Spotify Web API provides catalogue, search, library, playlist and player metadata. It is not itself the box's audio decoder. Catalogue access and audio output are therefore separate:

- `SpotifyCatalogProvider`: OAuth, search, artists, albums, playlists, tracks and metadata,
- `SpotifyPlaybackAdapter`: start/pause/seek/queue/device and actual audio output,
- normalized MuPiBox media model between provider, player and UI.

The playback technology is selected only after an end-to-end Pi 3/DietPi test. The official Web Playback SDK requires Spotify Premium and a browser environment; `librespot` remains a possible but unofficial candidate. Architecture must not hard-wire either path.

## Phase 3A -- Spotify Connect (implemented, verified on real hardware)

Playback runs through [`go-librespot`](https://github.com/devgianlu/go-librespot) (GPL-3.0,
`devgianlu/go-librespot`) as its own process managed by `mupibox-spotify.service`
(`scripts/install-go-librespot.sh`, arm64 release binary) -- MuPiBox-NG never implements the
Spotify protocol itself. The user connects their Spotify Premium account via **Zeroconf/Spotify
Connect** (device selection directly in the official Spotify app) -- this needs **no OAuth, no
client secret and no redirect URI**; those only matter for the optional, not-yet-built Web API
part (library/playlists/search, Phase 3B).

`internal/providers/spotify` regenerates go-librespot's `config.yml` on every start from
`settings.Audio.Device` (`ALSADeviceFromMPV`, the same ALSA/dmix path as mpv and TTS -- no
hardcoded MuPiHAT device) and talks to its local REST API/WebSocket events (`127.0.0.1:3678`,
never reachable over the network) through a Go client. `internal/server` mirrors status/control
under `GET /api/spotify/status` and `POST /api/spotify/command` (pause/resume/next/previous/seek/
volume) for the touch/web UI.

**Audio arbitration:** local playback, Spotify and TTS are mutually exclusive even though the
shared ALSA dmix device technically allows concurrent streams -- the control layer deliberately
decides which source may be active. Starting local playback pauses an active Spotify session;
Spotify becoming active from outside (the Spotify app) pauses local playback; TTS continues to
also pause an active Spotify session, with no auto-resume. Telling "we triggered this ourselves"
apart from "the Spotify app triggered this" uses go-librespot's own `play_origin` field, not a
custom mechanism.

Credentials from Zeroconf pairing are persisted by go-librespot itself in `state.json` under
`/var/lib/mupibox-ng/spotify/` (`persist_credentials: true`, so a box restart does not require
re-pairing); directory `0700`, file `0600` (`UMask=0077` in `mupibox-spotify.service` enforces
this independently of go-librespot's own write permissions). MuPiBox-NG itself never reads or
logs this file.

Verified on real Pi 4/MuPiHAT V3.1 hardware: the device appears in the Spotify app, Zeroconf
pairing, audible playback over MuPiHAT/dmix, control from both the Spotify app and directly on
the native touchscreen (play/pause/next/previous/volume), status/track display, restart with
automatic reconnection, all three audio-arbitration directions. Not yet built: Web API access to
library/playlists/search with OAuth+PKCE (Phase 3B).

## Admin setup

Planned flow:

1. validate Spotify app/OAuth configuration,
2. connect the account for this box,
3. display permissions and token state,
4. add a source,
5. search for artist, album or playlist,
6. select a result,
7. choose content mode such as albums or tracks,
8. choose navigation position and playback profile,
9. save and preview at 800×480.

Client Secret, refresh token and other credentials are never delivered to the player UI or written to logs or Git.

## Cache and rate limits

Caching only improves performance and avoids unnecessary API calls. It is not offline music storage.

Only temporarily required metadata and cover art may be cached according to current Spotify rules. Spotify audio is not stored as an ordinary local cache. Each entry records provider, account, resource type, resource key, fetched time and expiry.

Planned behavior:

- short TTLs for search and changing player state,
- longer but finite TTLs for album/artist metadata and artwork,
- playlist `snapshot_id` to detect changes efficiently,
- coalescing/deduplicating concurrent identical requests,
- pagination and demand-driven loading,
- stale-while-revalidate for UI where permitted,
- cache size and age limits,
- delete account-related cache data when disconnecting an account.

HTTP 429 honors `Retry-After`. Retries use backoff, never tight loops. Expired tokens are refreshed server-side.

## Playback progress

Spotify content stores the last position just like local files. The key contains at least provider `spotify`, box/account context and a stable Spotify media ID. Position and duration are stored in milliseconds. Playback resumes unless the item has been marked completed. Live radio remains excluded.

## Offline behavior

Spotify is not promised as a full offline source without internet. During an outage the UI may briefly display permitted cached metadata while clearly reporting that playback is unavailable. Local media remains independent and usable.

Any future Spotify offline feature may only be implemented if compatible with then-current Spotify rules and a supported playback technology.

## Persistence

Expected concepts:

- `provider_accounts`,
- protected token/secret storage,
- `content_bindings` with Spotify ID/URI and type,
- `provider_cache` with expiry,
- optional playlist snapshots,
- provider-neutral stable media IDs for progress and navigation.

Official references:

- https://developer.spotify.com/documentation/web-api/
- https://developer.spotify.com/documentation/web-api/concepts/api-calls
- https://developer.spotify.com/documentation/web-api/concepts/scopes
- https://developer.spotify.com/terms
