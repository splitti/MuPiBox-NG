package spotify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
)

// Track mirrors go-librespot's "track" API schema (see api-spec.yml).
type Track struct {
	URI           string   `json:"uri"`
	Name          string   `json:"name"`
	ArtistNames   []string `json:"artist_names"`
	AlbumName     string   `json:"album_name"`
	AlbumCoverURL string   `json:"album_cover_url"`
	PositionMS    int64    `json:"position"`
	DurationMS    int64    `json:"duration"`
}

// Status mirrors go-librespot's "status" API schema (GET /status).
type Status struct {
	Connected   bool      `json:"-"` // false when GET /status returned 204 (no active session)
	Username    string    `json:"username"`
	DeviceID    string    `json:"device_id"`
	PlayOrigin  string    `json:"play_origin"`
	Stopped     bool      `json:"stopped"`
	Paused      bool      `json:"paused"`
	Buffering   bool      `json:"buffering"`
	Volume      int       `json:"volume"`
	VolumeSteps int       `json:"volume_steps"`
	Track       *Track    `json:"track"`
	UpdatedAt   time.Time `json:"-"`
}

// Playing reports whether audio is actually expected to be coming out of
// the speaker right now (connected, not stopped, not paused).
func (s Status) Playing() bool { return s.Connected && !s.Stopped && !s.Paused }

// ExternallyOriginated reports whether the current playback state was
// started by something other than our own REST calls (i.e. the Spotify app
// or another Connect controller picked this box) -- see api-spec.yml's
// play_origin: "go-librespot" identifies our own API as the origin.
func (s Status) ExternallyOriginated() bool {
	return s.Connected && s.PlayOrigin != "" && s.PlayOrigin != "go-librespot"
}

// Client is a thin wrapper around go-librespot's local REST API
// (127.0.0.1-only, see config.go) -- MuPiBox-NG never speaks the Spotify
// protocol itself, only this local control surface.
type Client struct {
	baseURL string
	http    *http.Client
}

func NewClient(baseURL string) *Client {
	return &Client{baseURL: strings.TrimRight(baseURL, "/"), http: &http.Client{Timeout: 5 * time.Second}}
}

func (c *Client) do(ctx context.Context, method, path string, body any) ([]byte, int, error) {
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, 0, err
		}
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return nil, 0, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}
	if resp.StatusCode >= 300 {
		msg := strings.TrimSpace(string(data))
		if msg == "" {
			msg = resp.Status
		}
		return data, resp.StatusCode, fmt.Errorf("go-librespot: %s %s: %s", method, path, msg)
	}
	return data, resp.StatusCode, nil
}

// FetchStatus calls GET /status. A 204 (no active Spotify Connect session
// yet -- nobody has selected the box) is not an error: it returns a zero
// Status with Connected=false.
func (c *Client) FetchStatus(ctx context.Context) (Status, error) {
	data, code, err := c.do(ctx, http.MethodGet, "/status", nil)
	if err != nil {
		return Status{}, err
	}
	if code == http.StatusNoContent {
		return Status{UpdatedAt: time.Now()}, nil
	}
	var s Status
	if err := json.Unmarshal(data, &s); err != nil {
		return Status{}, fmt.Errorf("go-librespot: decode status: %w", err)
	}
	s.Connected = true
	s.UpdatedAt = time.Now()
	return s, nil
}

func (c *Client) Pause(ctx context.Context) error {
	_, _, err := c.do(ctx, http.MethodPost, "/player/pause", nil)
	return err
}
func (c *Client) Resume(ctx context.Context) error {
	_, _, err := c.do(ctx, http.MethodPost, "/player/resume", nil)
	return err
}
func (c *Client) Next(ctx context.Context) error {
	_, _, err := c.do(ctx, http.MethodPost, "/player/next", nil)
	return err
}
func (c *Client) Previous(ctx context.Context) error {
	_, _, err := c.do(ctx, http.MethodPost, "/player/prev", nil)
	return err
}
func (c *Client) Seek(ctx context.Context, positionMS int64) error {
	_, _, err := c.do(ctx, http.MethodPost, "/player/seek", map[string]any{"position": positionMS})
	return err
}
func (c *Client) SetVolume(ctx context.Context, volume int) error {
	_, _, err := c.do(ctx, http.MethodPost, "/player/volume", map[string]any{"volume": volume})
	return err
}
func (c *Client) SetDeviceName(ctx context.Context, name string) error {
	_, _, err := c.do(ctx, http.MethodPost, "/set_device_name", map[string]any{"device_name": name})
	return err
}

// wsEvent mirrors the {"type": "...", "data": {...}} envelope documented in
// API.md's Websocket section.
type wsEvent struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

// Manager owns the cached Status (kept current via the WebSocket event
// stream) and notifies a caller-supplied handler on every event, so
// internal/server can arbitrate against internal/core.Controller without
// Manager knowing anything about the local player.
type Manager struct {
	client  *Client
	wsURL   string
	mu      sync.RWMutex
	status  Status
	handler func(eventType string, status Status)
}

func NewManager(baseURL string) *Manager {
	wsURL := strings.Replace(strings.TrimRight(baseURL, "/"), "http://", "ws://", 1)
	wsURL = strings.Replace(wsURL, "https://", "wss://", 1)
	return &Manager{client: NewClient(baseURL), wsURL: wsURL + "/events"}
}

func (m *Manager) Client() *Client { return m.client }

// OnEvent registers the arbitration/status-broadcast hook. Safe to call
// concurrently with (or after) Run -- go-librespot may not even be reachable
// yet when the caller wires this up, so Run does not wait for it.
func (m *Manager) OnEvent(handler func(eventType string, status Status)) {
	m.mu.Lock()
	m.handler = handler
	m.mu.Unlock()
}

func (m *Manager) Status() Status {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.status
}

func (m *Manager) setStatus(s Status) {
	m.mu.Lock()
	m.status = s
	m.mu.Unlock()
}

func (m *Manager) currentHandler() func(eventType string, status Status) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.handler
}

// Run connects the WebSocket event stream and keeps it alive with backoff
// reconnection until ctx is cancelled. Every event refreshes the cached
// Status from a fresh GET /status (the event payloads are partial; a full
// re-fetch is simpler and cheap on a local-only API than reconstructing
// state from each event type) and then calls the registered handler.
// On (re)connect it also does one refresh up front, so a missed event
// during a reconnect gap is never silently lost -- the reconciled status is
// always at most one round trip stale.
func (m *Manager) Run(ctx context.Context) {
	backoff := time.Second
	const maxBackoff = 30 * time.Second
	for {
		if ctx.Err() != nil {
			return
		}
		if err := m.runOnce(ctx); err != nil && ctx.Err() == nil {
			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
			}
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
			continue
		}
		backoff = time.Second
	}
}

func (m *Manager) runOnce(ctx context.Context) error {
	conn, _, err := websocket.Dial(ctx, m.wsURL, nil)
	if err != nil {
		return err
	}
	defer conn.CloseNow()
	m.refresh(ctx, "connected")
	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			return err
		}
		var ev wsEvent
		if err := json.Unmarshal(data, &ev); err != nil {
			continue
		}
		m.refresh(ctx, ev.Type)
	}
}

func (m *Manager) refresh(ctx context.Context, eventType string) {
	status, err := m.client.FetchStatus(ctx)
	if err != nil {
		return
	}
	m.setStatus(status)
	if handler := m.currentHandler(); handler != nil {
		handler(eventType, status)
	}
}
