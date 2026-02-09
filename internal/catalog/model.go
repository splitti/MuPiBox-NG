package catalog

type Catalog struct {
	Categories []Category `json:"categories"`
}

type Category struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Items []Item `json:"items"`
}

type Item struct {
	ID           string        `json:"id"`
	DisplayName  string        `json:"display_name"`
	Type         string        `json:"type"` // artist, playlist, podcast, album
	Resume       bool          `json:"resume,omitempty"`
	PlayBehavior *PlayBehavior `json:"play_behavior,omitempty"`
	Sources      []Source      `json:"sources"`
}

type PlayBehavior struct {
	Shuffle bool   `json:"shuffle,omitempty"`
	Repeat  bool   `json:"repeat,omitempty"`
	StartAt string `json:"start_at,omitempty"` // resume | beginning
}

type Source struct {
	Type     string `json:"type"` // amazon, spotify, local, rss
	Priority int    `json:"priority"`

	ArtistID    string `json:"artistId,omitempty"`
	ArtistURL   string `json:"artistUrl,omitempty"`
	PlaylistURL string `json:"playlistUrl,omitempty"`

	Path      string `json:"path,omitempty"`
	CoverPath string `json:"cover_path,omitempty"`

	URL string `json:"url,omitempty"`
}
