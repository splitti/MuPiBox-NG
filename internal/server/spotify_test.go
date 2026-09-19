package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"mupibox/internal/audio"
	"mupibox/internal/core"
	"mupibox/internal/library"
	"mupibox/internal/providers/spotify"
)

// spotifyFixtureServer serves /status from a caller-controlled JSON string
// and accepts (but never pushes into) /events, just enough for
// spotify.Manager.Run's initial "connected" refresh to seed its status cache
// with whatever statusJSON currently returns.
func spotifyFixtureServer(t *testing.T, statusJSON *string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/status":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(*statusJSON))
		case "/events":
			c, err := websocket.Accept(w, r, nil)
			if err != nil {
				return
			}
			<-r.Context().Done()
			_ = c
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func runningManager(t *testing.T, statusJSON *string) *spotify.Manager {
	t.Helper()
	srv := spotifyFixtureServer(t, statusJSON)
	m := spotify.NewManager(srv.URL)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go m.Run(ctx)
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if m.Status().UpdatedAt.After(time.Time{}) {
			return m
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("spotify manager never picked up the initial status")
	return nil
}

const externallyPlayingStatus = `{"username":"u","device_id":"d","play_origin":"connect","stopped":false,"paused":false,"buffering":false,"volume":10,"volume_steps":100,"track":{"uri":"spotify:track:1","name":"Song","artist_names":["Artist"],"album_name":"Album","album_cover_url":"","position":1000,"duration":200000}}`

func TestSpotifyStatusEndpointReportsDisconnectedWithoutManager(t *testing.T) {
	a := &API{}
	w := ttsRequest(t, a.Handler(), http.MethodGet, "/api/spotify/status", "")
	if w.Code != 200 {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var resp spotifyStatusResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Connected {
		t.Fatalf("expected disconnected without a manager, got %#v", resp)
	}
}

func TestSpotifyStatusEndpointReflectsManagerStatus(t *testing.T) {
	status := externallyPlayingStatus
	m := runningManager(t, &status)
	a := &API{Spotify: m}
	w := ttsRequest(t, a.Handler(), http.MethodGet, "/api/spotify/status", "")
	if w.Code != 200 {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var resp spotifyStatusResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if !resp.Connected || !resp.Playing || resp.Track == nil || resp.Track.Name != "Song" {
		t.Fatalf("unexpected status response: %#v", resp)
	}
}

func TestSpotifyCommandEndpointDispatchesActions(t *testing.T) {
	// Every command is followed by a GET /status to return a fresh result
	// instead of the Manager's asynchronous cache (see
	// TestSpotifyCommandEndpointReturnsFreshStatusNotStaleCache) -- this
	// test only cares which /player/... endpoint each action hits, so it
	// filters those refresh calls out rather than asserting an exact count.
	var calls []string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.Path)
		if r.URL.Path == "/status" {
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	defer upstream.Close()
	a := &API{Spotify: spotify.NewManager(upstream.URL)}
	cases := []struct{ body, wantPath string }{
		{`{"action":"pause"}`, "/player/pause"},
		{`{"action":"resume"}`, "/player/resume"},
		{`{"action":"next"}`, "/player/next"},
		{`{"action":"previous"}`, "/player/prev"},
		{`{"action":"seek","value":5000}`, "/player/seek"},
		{`{"action":"volume","value":30}`, "/player/volume"},
	}
	for _, c := range cases {
		w := ttsRequest(t, a.Handler(), http.MethodPost, "/api/spotify/command", c.body)
		if w.Code != 200 {
			t.Fatalf("action %s: status=%d body=%s", c.body, w.Code, w.Body.String())
		}
	}
	var playerCalls []string
	for _, c := range calls {
		if !strings.Contains(c, "/status") {
			playerCalls = append(playerCalls, c)
		}
	}
	if len(playerCalls) != len(cases) {
		t.Fatalf("expected %d player calls, got %v (all calls: %v)", len(cases), playerCalls, calls)
	}
	for i, c := range cases {
		if !strings.HasSuffix(playerCalls[i], c.wantPath) {
			t.Fatalf("call %d: got %q, want suffix %q", i, playerCalls[i], c.wantPath)
		}
	}
}

// TestSpotifyCommandEndpointReturnsFreshStatusNotStaleCache covers a real
// bug found on hardware: the response used to echo the Manager's cache,
// which only updates asynchronously off the WebSocket stream, so a client
// tapping "resume" briefly saw the pre-command paused state reflected back.
func TestSpotifyCommandEndpointReturnsFreshStatusNotStaleCache(t *testing.T) {
	live := `{"username":"u","device_id":"d","play_origin":"go-librespot","stopped":false,"paused":true,"buffering":false,"volume":10,"volume_steps":100,"track":null}`
	mux := http.NewServeMux()
	mux.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(live))
	})
	mux.HandleFunc("/player/resume", func(w http.ResponseWriter, r *http.Request) {
		live = `{"username":"u","device_id":"d","play_origin":"go-librespot","stopped":false,"paused":false,"buffering":false,"volume":10,"volume_steps":100,"track":null}`
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	a := &API{Spotify: spotify.NewManager(srv.URL)}
	w := ttsRequest(t, a.Handler(), http.MethodPost, "/api/spotify/command", `{"action":"resume"}`)
	if w.Code != 200 {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var resp spotifyStatusResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Paused || !resp.Playing {
		t.Fatalf("expected the fresh post-resume status, got stale/incorrect: %#v", resp)
	}
}

func TestSpotifyCommandEndpointRejectsUnknownAction(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("unexpected upstream call for an unknown action: %s", r.URL.Path)
	}))
	defer upstream.Close()
	a := &API{Spotify: spotify.NewManager(upstream.URL)}
	w := ttsRequest(t, a.Handler(), http.MethodPost, "/api/spotify/command", `{"action":"teleport"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for an unknown action, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSpotifyCommandEndpointUnavailableWithoutManager(t *testing.T) {
	a := &API{}
	w := ttsRequest(t, a.Handler(), http.MethodPost, "/api/spotify/command", `{"action":"pause"}`)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 without a manager, got %d: %s", w.Code, w.Body.String())
	}
}

// TestLocalPlaybackPausesSpotify covers the audio-arbitration requirement
// from the Phase 3A approval: starting local playback must pause an
// actively playing Spotify session first, so the two never play over each
// other even though the shared dmix device would technically allow it.
func TestLocalPlaybackPausesSpotify(t *testing.T) {
	var paused bool
	statusJSON := externallyPlayingStatus
	mux := http.NewServeMux()
	mux.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(statusJSON))
	})
	mux.HandleFunc("/player/pause", func(w http.ResponseWriter, r *http.Request) { paused = true })
	mux.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		<-r.Context().Done()
		_ = c
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	a := &API{Spotify: spotify.NewManager(srv.URL)}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go a.Spotify.Run(ctx)
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && !a.Spotify.Status().Playing() {
		time.Sleep(10 * time.Millisecond)
	}
	if !a.Spotify.Status().Playing() {
		t.Fatal("spotify manager never reported playing")
	}

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "track.wav"), []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	lib, err := library.Scan(dir)
	if err != nil {
		t.Fatal(err)
	}
	player, err := core.New(lib, &audio.Simulated{}, 60)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = player.Close() })
	a.Player = player

	w := ttsRequest(t, a.Handler(), http.MethodPost, "/api/command", `{"action":"folder","folder_id":"`+lib.Folders[0].ID+`"}`)
	if w.Code != 200 {
		t.Fatalf("local folder command failed: %d %s", w.Code, w.Body.String())
	}
	if !paused {
		t.Fatal("expected local playback to pause the actively playing Spotify session")
	}
}

// TestSpotifyArbitrationPausesLocalPlayback covers the reverse direction:
// Spotify becoming active from an external controller (the Spotify app)
// while local media is playing must pause the local player.
func TestSpotifyArbitrationPausesLocalPlayback(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "track.wav"), []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	lib, err := library.Scan(dir)
	if err != nil {
		t.Fatal(err)
	}
	player, err := core.New(lib, &audio.Simulated{}, 60)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = player.Close() })
	if err := player.Execute(core.Command{Action: "folder", FolderID: lib.Folders[0].ID}); err != nil {
		t.Fatal(err)
	}
	if player.Status().State != "playing" {
		t.Fatalf("expected local playback to start, got %q", player.Status().State)
	}

	status := externallyPlayingStatus
	srv := spotifyFixtureServer(t, &status)
	m := spotify.NewManager(srv.URL)
	a := &API{Player: player, Spotify: m}
	// Wire the arbitration handler BEFORE starting Run, so the very first
	// "connected" refresh (which already carries the externally-playing
	// status) reaches it -- registering it after Run has already fired that
	// first event would miss it, since events are not replayed.
	a.WireSpotifyArbitration()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go m.Run(ctx)

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && player.Status().State != "paused" {
		time.Sleep(10 * time.Millisecond)
	}
	if player.Status().State != "paused" {
		t.Fatalf("expected local player to be paused by external Spotify activation, got %q", player.Status().State)
	}
}
