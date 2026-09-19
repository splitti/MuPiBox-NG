package spotify

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func TestFetchStatusParsesPlayingTrack(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/status" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"username":"u","device_id":"d","play_origin":"connect","stopped":false,"paused":false,"buffering":false,"volume":40,"volume_steps":100,"track":{"uri":"spotify:track:1","name":"Song","artist_names":["Artist"],"album_name":"Album","album_cover_url":"https://x/y.jpg","position":1000,"duration":200000}}`))
	}))
	defer srv.Close()
	c := NewClient(srv.URL)
	s, err := c.FetchStatus(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !s.Connected || !s.Playing() || s.Track == nil || s.Track.Name != "Song" {
		t.Fatalf("unexpected status: %#v", s)
	}
	if !s.ExternallyOriginated() {
		t.Fatal("play_origin=connect should count as externally originated")
	}
}

func TestFetchStatusHandlesNoActiveSession(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	c := NewClient(srv.URL)
	s, err := c.FetchStatus(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if s.Connected || s.Playing() || s.ExternallyOriginated() {
		t.Fatalf("expected a disconnected zero status, got %#v", s)
	}
}

func TestPlayerCommandsHitExpectedEndpoints(t *testing.T) {
	var got []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = append(got, r.Method+" "+r.URL.Path)
		body := map[string]any{}
		_ = json.NewDecoder(r.Body).Decode(&body)
	}))
	defer srv.Close()
	c := NewClient(srv.URL)
	ctx := context.Background()
	if err := c.Pause(ctx); err != nil {
		t.Fatal(err)
	}
	if err := c.Resume(ctx); err != nil {
		t.Fatal(err)
	}
	if err := c.Next(ctx); err != nil {
		t.Fatal(err)
	}
	if err := c.Previous(ctx); err != nil {
		t.Fatal(err)
	}
	if err := c.Seek(ctx, 5000); err != nil {
		t.Fatal(err)
	}
	if err := c.SetVolume(ctx, 30); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"POST /player/pause", "POST /player/resume", "POST /player/next",
		"POST /player/prev", "POST /player/seek", "POST /player/volume",
	}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("call %d: got %q want %q", i, got[i], want[i])
		}
	}
}

func TestDoReturnsClearErrorOnNonSuccessStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("the uri is missing or empty"))
	}))
	defer srv.Close()
	c := NewClient(srv.URL)
	err := c.Pause(context.Background())
	if err == nil || !strings.Contains(err.Error(), "the uri is missing or empty") {
		t.Fatalf("expected the upstream error message surfaced, got %v", err)
	}
}

// wsTestServer wires an httptest.Server serving /status (backing
// Manager.refresh's GET /status calls) and /events (a WebSocket the test
// pushes fixture events into), mirroring go-librespot's real API shape
// closely enough to exercise Manager.Run end to end.
func wsTestServer(t *testing.T, statusJSON func() string) (*httptest.Server, func(event string)) {
	t.Helper()
	var connMu sync.Mutex
	var conn *websocket.Conn
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/status":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(statusJSON()))
		case "/events":
			c, err := websocket.Accept(w, r, nil)
			if err != nil {
				return
			}
			connMu.Lock()
			conn = c
			connMu.Unlock()
			<-r.Context().Done()
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	send := func(event string) {
		deadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) {
			connMu.Lock()
			c := conn
			connMu.Unlock()
			if c != nil {
				_ = c.Write(context.Background(), websocket.MessageText, []byte(`{"type":"`+event+`","data":{}}`))
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
		t.Fatal("no websocket client connected in time")
	}
	return srv, send
}

func TestManagerRunUpdatesStatusFromEvents(t *testing.T) {
	playing := false
	statusJSON := func() string {
		if playing {
			return `{"username":"u","device_id":"d","play_origin":"go-librespot","stopped":false,"paused":false,"buffering":false,"volume":10,"volume_steps":100,"track":null}`
		}
		return `{"username":"u","device_id":"d","play_origin":"go-librespot","stopped":false,"paused":true,"buffering":false,"volume":10,"volume_steps":100,"track":null}`
	}
	srv, send := wsTestServer(t, func() string { return statusJSON() })
	defer srv.Close()

	m := NewManager(srv.URL)
	events := make(chan string, 8)
	m.OnEvent(func(eventType string, status Status) { events <- eventType })
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go m.Run(ctx)

	waitFor(t, events, "connected")
	if m.Status().Paused != true {
		t.Fatalf("expected initial paused status, got %#v", m.Status())
	}
	playing = true
	send("playing")
	waitFor(t, events, "playing")
	if !m.Status().Playing() {
		t.Fatalf("expected playing status after event, got %#v", m.Status())
	}
}

func waitFor(t *testing.T, ch chan string, want string) {
	t.Helper()
	deadline := time.After(3 * time.Second)
	for {
		select {
		case got := <-ch:
			if got == want {
				return
			}
		case <-deadline:
			t.Fatalf("timed out waiting for event %q", want)
		}
	}
}
