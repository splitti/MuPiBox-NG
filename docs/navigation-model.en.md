# Dynamic navigation and content model

> [Deutsch](navigation-model.md) · English

## Goal

The MuPiBox UI must not assume a fixed hierarchy such as `category -> row -> local folder`.
Navigation is modeled as a freely configurable tree of nodes. A node may be a pure navigation container or may point
directly to a content source.

This enables, among others, the following variants:

### Variant A – classic drill-down

```text
Audiobooks
└── Die drei ??? Kids
    ├── Album/Episode 001
    ├── Album/Episode 002
    └── Album/Episode 003
```

Tapping `Die drei ??? Kids` opens its own view with albums/episodes shown as cover tiles, similar to the old MuPiBox UI.

### Variant B – content directly on the home screen

```text
Die drei ??? Kids
[Album 001] [Album 002] [Album 003] ...
```

The node itself is a top-level category. Its source may, for example, be a Spotify artist. Albums returned by the
provider are rendered directly below the heading.

### Variant C – playlist directly as a category

```text
Favourite songs
[Track 1] [Track 2] [Track 3] ...
```

The category points directly to a Spotify playlist or a manually curated track list. Tracks are shown instead of albums.

## Navigation nodes

Every navigation node has at least:

- stable ID,
- optional parent ID,
- sort position,
- enabled/visible state,
- localized display names,
- presentation mode,
- selection behaviour,
- optional content binding.

The visible label is independent from the technical source. A category may therefore be named `Die drei ??? Kids`
while internally referencing a Spotify artist ID or a local folder.

## Selection behaviour

A node supports at least two core behaviours:

- `drilldown`: tapping opens a new view containing child nodes or resolved content.
- `inline`: content is shown directly below the category heading on the current page.

A future `play` behaviour can additionally start a node immediately without an intermediate view, for example a radio
station or fixed playlist.

## Content binding

A node may optionally contain a provider-neutral content binding. Expected concepts include:

- provider, e.g. `local`, `spotify`, `radio`, `rss`, `manual`,
- source type, e.g. `artist`, `playlist`, `album`, `folder`, `feed`, `station`, `collection`,
- provider reference/ID,
- requested content mode,
- optional provider parameters.

Examples:

```text
Provider: spotify
Type: artist
Reference: <Spotify Artist ID>
Content mode: albums
```

```text
Provider: spotify
Type: playlist
Reference: <Spotify Playlist ID>
Content mode: tracks
```

```text
Provider: local
Type: folder
Reference: /Audiobooks/Die drei Fragezeichen Kids
Content mode: children
```

The web UI contains no Spotify- or filesystem-specific logic. The provider resolves its reference and returns normalized
media objects.

## Content mode

The admin UI should offer at least these modes:

- `auto` – provider selects a sensible default,
- `children` – show child nodes/folders,
- `albums` – show albums/episodes,
- `tracks` – show tracks directly,
- `items` – show generic media objects.

Examples for `auto`:

- Spotify artist -> albums,
- Spotify playlist -> tracks,
- Spotify album -> tracks,
- local folder with subfolders -> subfolders,
- radio station -> directly playable.

The administrator may override the automatic mode where the provider supports the requested mode.

## Presentation

Presentation remains independent of the provider. Planned display modes include, for example:

- `auto`,
- horizontal cover tiles,
- compact list,
- larger cover view on a drill-down page.

On the 800x480 target display the existing principles remain:

- categories scroll vertically,
- media inside a category preferably scroll horizontally,
- large touch targets,
- no hover dependency,
- as few navigation levels as practical.

## Admin interface

When creating or editing a navigation node, the administrator should later be able to choose roughly:

1. name and translations,
2. position/parent category,
3. visible/enabled,
4. show directly (`inline`) or after tapping (`drilldown`),
5. source: manual, local, Spotify, radio, podcast, etc.,
6. source type and concrete reference,
7. requested output: automatic, albums, tracks, child items, etc.,
8. presentation style.

For provider sources the admin UI should offer search and selection rather than requiring technical IDs to be entered
manually. Example: connect Spotify -> search for `Die drei ??? Kids` -> select artist -> choose `show albums`.

## Provider contract

Providers return normalized objects and must not require provider-specific UI components. A normalized object should at
least provide a stable reference, type, title, optional subtitle/cover, and available actions.

The UI can therefore render local folders, Spotify albums, podcast episodes and other sources with the same generic card
and list components.

## Persistence

The navigation model will later be stored in SQLite. The exact table layout will be introduced through migrations.
Expected core objects include:

- `navigation_nodes`,
- `navigation_node_labels`,
- `content_bindings`,
- optional manual mappings/items,
- global presentation settings.

The current `categories`/`content_rows` structure in the development prototype is an intermediate step and may be migrated
to this more general model during the SQLite conversion.

## Dynamic resume list

A category can use `resume-list` as a media source with source type `limit`. Its `source_ref` contains the desired number of items from 1 to 100. This keeps the name, position and combination with other media sources fully configurable.

The list includes only started, unfinished media and sorts it by last playback. Multiple progress records from the same context are collapsed into one tile. Selecting the tile starts the saved track or episode at its saved position. Local resume items remain visible offline; external-provider items require their adapter and content to be available.
