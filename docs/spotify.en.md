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
