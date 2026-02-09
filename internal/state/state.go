package state

import (
	"encoding/json"
	"os"
	"sort"
	"sync"
	"time"
)

type ResumeState struct {
	// Kontext (optional aber empfohlen)
	ItemID  string `json:"item_id,omitempty"`  // z.B. artist-id oder playlist-id
	AlbumID string `json:"album_id,omitempty"` // z.B. folge/album-id

	// Track-Resume
	TrackID     string `json:"track_id,omitempty"`
	TrackIndex  int    `json:"track_index,omitempty"` // 0-based
	PositionSec int    `json:"position_sec"`

	// Sortierung "Weiter abspielen"
	UpdatedAt string `json:"updated_at,omitempty"` // RFC3339
}

type Entry struct {
	Key   string
	State ResumeState
}

type Store struct {
	path string
	mu   sync.Mutex
	data map[string]ResumeState
}

func NewStore(path string) (*Store, error) {
	s := &Store{
		path: path,
		data: map[string]ResumeState{},
	}

	raw, err := os.ReadFile(path)
	if err == nil {
		_ = json.Unmarshal(raw, &s.data)
	}

	// wenn Datei nicht existiert: anlegen
	if os.IsNotExist(err) {
		_ = os.WriteFile(path, []byte("{}"), 0644)
	}

	return s, nil
}

func (s *Store) Get(key string) (ResumeState, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.data[key]
	return v, ok
}

func (s *Store) Set(key string, st ResumeState) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	st.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	s.data[key] = st
	return s.persist()
}

func (s *Store) ListRecent(limit int) []Entry {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]Entry, 0, len(s.data))
	for k, v := range s.data {
		// nur sinnvolle Einträge
		if v.PositionSec <= 0 {
			continue
		}
		out = append(out, Entry{Key: k, State: v})
	}

	sort.Slice(out, func(i, j int) bool {
		// fehlende Zeiten nach hinten
		ti := parseTime(out[i].State.UpdatedAt)
		tj := parseTime(out[j].State.UpdatedAt)
		return ti.After(tj)
	})

	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

func parseTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}
	}
	return t
}

func (s *Store) persist() error {
	raw, _ := json.MarshalIndent(s.data, "", "  ")
	return os.WriteFile(s.path, raw, 0644)
}
