package server

import (
	"context"
	"errors"
	"net/http"
	"time"

	"mupibox/internal/core"
	"mupibox/internal/providers/spotify"
)

// spotifyStatusResponse is a small, stable JSON shape for the touch/admin
// UI -- deliberately not spotify.Status verbatim, so the wire format does
// not silently change if go-librespot's own schema grows fields we do not
// use yet.
type spotifyStatusResponse struct {
	Connected   bool                `json:"connected"`
	Playing     bool                `json:"playing"`
	Paused      bool                `json:"paused"`
	Buffering   bool                `json:"buffering"`
	Volume      int                 `json:"volume"`
	VolumeSteps int                 `json:"volume_steps"`
	Track       *spotifyTrackFields `json:"track,omitempty"`
}
type spotifyTrackFields struct {
	Name       string   `json:"name"`
	Artists    []string `json:"artists"`
	Album      string   `json:"album"`
	Cover      string   `json:"cover,omitempty"`
	PositionMS int64    `json:"position_ms"`
	DurationMS int64    `json:"duration_ms"`
}

func toSpotifyStatusResponse(s spotify.Status) spotifyStatusResponse {
	out := spotifyStatusResponse{
		Connected: s.Connected, Playing: s.Playing(), Paused: s.Paused,
		Buffering: s.Buffering, Volume: s.Volume, VolumeSteps: s.VolumeSteps,
	}
	if s.Track != nil {
		out.Track = &spotifyTrackFields{
			Name: s.Track.Name, Artists: s.Track.ArtistNames, Album: s.Track.AlbumName,
			Cover: s.Track.AlbumCoverURL, PositionMS: s.Track.PositionMS, DurationMS: s.Track.DurationMS,
		}
	}
	return out
}

// WireSpotifyArbitration is the Spotify-becomes-active-so-local-yields half
// of the audio arbitration (see pauseSpotifyIfPlaying in server.go for the
// other direction). Only reacts to playback genuinely started elsewhere
// (status.ExternallyOriginated(), i.e. the Spotify app or another Connect
// controller picked this box) -- never to the state changes our own pause/
// resume calls produce, which would risk a feedback loop.
func (a *API) WireSpotifyArbitration() {
	if a.Spotify == nil {
		return
	}
	a.Spotify.OnEvent(func(eventType string, status spotify.Status) {
		if a.Player == nil || !status.Playing() || !status.ExternallyOriginated() {
			return
		}
		if a.Player.Status().State == "playing" {
			_ = a.Player.Execute(core.Command{Action: "pause"})
		}
	})
}

func (a *API) registerSpotifyRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/spotify/status", func(w http.ResponseWriter, r *http.Request) {
		if a.Spotify == nil {
			jsonResponse(w, http.StatusOK, spotifyStatusResponse{})
			return
		}
		jsonResponse(w, http.StatusOK, toSpotifyStatusResponse(a.Spotify.Status()))
	})
	mux.HandleFunc("POST /api/spotify/command", func(w http.ResponseWriter, r *http.Request) {
		if a.Spotify == nil {
			problem(w, http.StatusServiceUnavailable, errors.New("spotify unavailable"))
			return
		}
		var input struct {
			Action string `json:"action"`
			Value  int64  `json:"value,omitempty"`
		}
		if err := decode(w, r, &input); err != nil {
			problem(w, http.StatusBadRequest, err)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		client := a.Spotify.Client()
		var err error
		switch input.Action {
		case "pause":
			err = client.Pause(ctx)
		case "resume", "play":
			err = client.Resume(ctx)
		case "next":
			err = client.Next(ctx)
		case "previous":
			err = client.Previous(ctx)
		case "seek":
			err = client.Seek(ctx, input.Value)
		case "volume":
			err = client.SetVolume(ctx, int(input.Value))
		default:
			problem(w, http.StatusBadRequest, errors.New("unknown action"))
			return
		}
		if err != nil {
			problem(w, http.StatusBadGateway, err)
			return
		}
		// Fetch fresh rather than returning the Manager's cache: that cache
		// only updates asynchronously off the WebSocket event stream, so it
		// would echo back the pre-command state and make the touch/admin UI
		// briefly show the wrong toggle position right after tapping it.
		status, statusErr := client.FetchStatus(ctx)
		if statusErr != nil {
			jsonResponse(w, http.StatusOK, toSpotifyStatusResponse(a.Spotify.Status()))
			return
		}
		jsonResponse(w, http.StatusOK, toSpotifyStatusResponse(status))
	})
}
