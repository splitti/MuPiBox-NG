package player

type PlaybackState string

const (
	StateStopped PlaybackState = "stopped"
	StatePlaying PlaybackState = "playing"
	StatePaused  PlaybackState = "paused"
)

type Mode string

const (
	ModeMusic            Mode = "music"
	ModeAudiobookSingle  Mode = "audiobook_single"
	ModeAudiobookChapters Mode = "audiobook_chapters"
)

type PlayerStatus struct {
	State PlaybackState `json:"state"`
	Mode  Mode          `json:"mode"`

	Series string `json:"series"`
	Title  string `json:"title"`

	Track      int `json:"track"`       // 1-based
	TrackCount int `json:"track_count"` // total tracks

	Position int `json:"position"` // seconds within current track
	Duration int `json:"duration"` // seconds of current track

	Cover string `json:"cover"`

	Volume int  `json:"volume"` // 0..100
	Muted  bool `json:"muted"`

	CanSeek      bool `json:"can_seek"`
	CanSkipTrack bool `json:"can_skip_track"`
	CanSkipTime  bool `json:"can_skip_time"`
}
