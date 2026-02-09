package player

import (
	"encoding/json"
	"net/http"
	"strconv"
)

type API struct {
	P Player
}

func NewAPI(p Player) *API {
	return &API{P: p}
}

func (a *API) Register(mux *http.ServeMux) {
	// Status
	mux.HandleFunc("/api/player/status", a.handleStatus)

	// Transport
	mux.HandleFunc("/api/player/play", a.postOnly(a.handlePlay))
	mux.HandleFunc("/api/player/pause", a.postOnly(a.handlePause))
	mux.HandleFunc("/api/player/toggle", a.postOnly(a.handleToggle))

	// Navigation
	mux.HandleFunc("/api/player/next", a.postOnly(a.handleNext))
	mux.HandleFunc("/api/player/prev", a.postOnly(a.handlePrev))
	mux.HandleFunc("/api/player/track", a.postOnly(a.handleTrack))

	// Time control
	mux.HandleFunc("/api/player/seek", a.postOnly(a.handleSeek))
	mux.HandleFunc("/api/player/skip", a.postOnly(a.handleSkip))

	// Volume
	mux.HandleFunc("/api/player/volume", a.postOnly(a.handleVolume))
	mux.HandleFunc("/api/player/mute", a.postOnly(a.handleMute))
	mux.HandleFunc("/api/player/unmute", a.postOnly(a.handleUnmute))
	mux.HandleFunc("/api/player/mute/toggle", a.postOnly(a.handleToggleMute))

	mux.HandleFunc("/api/collections", a.handleCollections)
}

func (a *API) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, a.P.Status())
}

func (a *API) handlePlay(w http.ResponseWriter, r *http.Request) {
	a.P.Play()
	writeOK(w)
}
func (a *API) handlePause(w http.ResponseWriter, r *http.Request) {
	a.P.Pause()
	writeOK(w)
}
func (a *API) handleToggle(w http.ResponseWriter, r *http.Request) {
	a.P.Toggle()
	writeOK(w)
}

func (a *API) handleNext(w http.ResponseWriter, r *http.Request) {
	a.P.Next()
	writeOK(w)
}
func (a *API) handlePrev(w http.ResponseWriter, r *http.Request) {
	a.P.Prev()
	writeOK(w)
}
func (a *API) handleTrack(w http.ResponseWriter, r *http.Request) {
	nr, ok := mustIntQuery(w, r, "nr")
	if !ok {
		return
	}
	a.P.SetTrack(nr) // 1-based
	writeOK(w)
}

func (a *API) handleSeek(w http.ResponseWriter, r *http.Request) {
	pos, ok := mustIntQuery(w, r, "position")
	if !ok {
		return
	}
	a.P.Seek(pos)
	writeOK(w)
}

func (a *API) handleSkip(w http.ResponseWriter, r *http.Request) {
	sec, ok := mustIntQuery(w, r, "seconds")
	if !ok {
		return
	}
	a.P.Skip(sec)
	writeOK(w)
}

func (a *API) handleVolume(w http.ResponseWriter, r *http.Request) {
	level, ok := mustIntQuery(w, r, "level")
	if !ok {
		return
	}
	a.P.SetVolume(level)
	writeOK(w)
}

func (a *API) handleMute(w http.ResponseWriter, r *http.Request) {
	a.P.Mute()
	writeOK(w)
}
func (a *API) handleUnmute(w http.ResponseWriter, r *http.Request) {
	a.P.Unmute()
	writeOK(w)
}
func (a *API) handleToggleMute(w http.ResponseWriter, r *http.Request) {
	a.P.ToggleMute()
	writeOK(w)
}

func (a *API) postOnly(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		next(w, r)
	}
}

func mustIntQuery(w http.ResponseWriter, r *http.Request, key string) (int, bool) {
	val := r.URL.Query().Get(key)
	if val == "" {
		http.Error(w, "missing query param: "+key, http.StatusBadRequest)
		return 0, false
	}
	n, err := strconv.Atoi(val)
	if err != nil {
		http.Error(w, "invalid int for "+key, http.StatusBadRequest)
		return 0, false
	}
	return n, true
}

func writeOK(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func (a *API) handleCollections(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	writeJSON(w, map[string]any{
		"note": "collections mock works",
	})
}
