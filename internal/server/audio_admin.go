package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// audioCard is one playback device as reported by `aplay -l`.
type audioCard struct {
	Card       int    `json:"card"`
	ID         string `json:"id"`
	Name       string `json:"name"`
	Device     int    `json:"device"`
	DeviceName string `json:"device_name"`
}

// audioDevice is one mpv --audio-device candidate.
type audioDevice struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type audioStatus struct {
	AplayInstalled           bool          `json:"aplay_installed"`
	MPVInstalled             bool          `json:"mpv_installed"`
	Cards                    []audioCard   `json:"cards"`
	MPVDevices               []audioDevice `json:"mpv_devices"`
	ConfiguredDevice         string        `json:"configured_device"`
	RecommendedDevice        string        `json:"recommended_device,omitempty"`
	MuPiHATOverlayConfigured bool          `json:"mupihat_overlay_configured"`
	MuPiHATDetected          bool          `json:"mupihat_detected"`
	RebootRequired           bool          `json:"reboot_required"`
	CollectedAt              time.Time     `json:"collected_at"`
}

var aplayCardLine = regexp.MustCompile(`^card (\d+): (\S+) \[([^\]]*)\], device (\d+): (.+?) \[([^\]]*)\]$`)

func parseAplayList(raw string) []audioCard {
	out := []audioCard{}
	for _, line := range strings.Split(raw, "\n") {
		match := aplayCardLine.FindStringSubmatch(strings.TrimSpace(line))
		if match == nil {
			continue
		}
		card, _ := strconv.Atoi(match[1])
		device, _ := strconv.Atoi(match[4])
		out = append(out, audioCard{Card: card, ID: match[2], Name: match[3], Device: device, DeviceName: match[6]})
	}
	return out
}

// Greedy .* (not [^)]*) is deliberate: real mpv output nests parens in the
// name itself, e.g. "'alsa' (Default (alsa))" -- matching to the *last*
// ')' on the line is what actually captures the full name.
var mpvDeviceLine = regexp.MustCompile(`^\s*'([^']+)'\s*\((.*)\)\s*$`)

func parseMPVAudioDevices(raw string) []audioDevice {
	out := []audioDevice{}
	for _, line := range strings.Split(raw, "\n") {
		match := mpvDeviceLine.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		out = append(out, audioDevice{ID: match[1], Name: match[2]})
	}
	return out
}

// mupihatOverlayConfigured reports whether the managed MuPiHAT audio block
// (see cmd/mupibox-system-agent's setMuPiHATAudioBlock) is currently present
// in the Raspberry Pi boot config -- read-only, no root required.
func mupihatOverlayConfigured() bool {
	content := readFirst("/boot/firmware/config.txt", "/boot/config.txt")
	return strings.Contains(content, "BEGIN MUPIBOX-NG MUPIHAT AUDIO")
}

func mupihatCardDetected(cards []audioCard) bool {
	for _, card := range cards {
		if strings.Contains(strings.ToLower(card.ID), "max98357a") {
			return true
		}
	}
	return false
}

// recommendedAudioDevice prefers a "dmix" playback device for a detected
// MuPiHAT (MAX98357A) card over the default "plughw" one: plughw is
// exclusive, so a second mpv process (TTS/announcements) cannot open it
// while the main player still holds it, even paused -- observed for real
// on Pi 4/MuPiHAT V3.1 hardware. dmix allows both to play concurrently.
// plughw stays selectable directly, it just isn't the suggested default.
func recommendedAudioDevice(cards []audioCard, devices []audioDevice) string {
	for _, card := range cards {
		if !strings.Contains(strings.ToLower(card.ID), "max98357a") {
			continue
		}
		want := "alsa/dmix:CARD=" + card.ID + ",DEV=" + strconv.Itoa(card.Device)
		for _, device := range devices {
			if device.ID == want {
				return want
			}
		}
	}
	return ""
}

func collectAudioStatus(ctx context.Context, configuredDevice string) audioStatus {
	_, aplayErr := exec.LookPath("aplay")
	_, mpvErr := exec.LookPath("mpv")
	cards := parseAplayList(commandText(ctx, "aplay", "-l"))
	devices := parseMPVAudioDevices(commandText(ctx, "mpv", "--audio-device=help"))
	overlay := mupihatOverlayConfigured()
	detected := mupihatCardDetected(cards)
	return audioStatus{
		AplayInstalled: aplayErr == nil, MPVInstalled: mpvErr == nil,
		Cards: cards, MPVDevices: devices, ConfiguredDevice: configuredDevice,
		RecommendedDevice:        recommendedAudioDevice(cards, devices),
		MuPiHATOverlayConfigured: overlay, MuPiHATDetected: detected,
		RebootRequired: overlay && !detected,
		CollectedAt:    time.Now().UTC(),
	}
}

func (a *API) registerAudioRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/admin/audio/status", func(w http.ResponseWriter, r *http.Request) {
		settings, err := a.currentSettings()
		if err != nil {
			problem(w, http.StatusInternalServerError, err)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
		defer cancel()
		jsonResponse(w, http.StatusOK, collectAudioStatus(ctx, settings.Audio.Device))
	})

	mux.HandleFunc("PUT /api/admin/audio/device", func(w http.ResponseWriter, r *http.Request) {
		if a.Store == nil {
			problem(w, http.StatusServiceUnavailable, errors.New("persistent store unavailable"))
			return
		}
		var input struct {
			Device string `json:"device"`
		}
		if err := decode(w, r, &input); err != nil {
			problem(w, http.StatusBadRequest, err)
			return
		}
		device := strings.TrimSpace(input.Device)
		if device != "" {
			ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
			detected := collectAudioStatus(ctx, "")
			cancel()
			// Only enforce the allow-list when detection actually found
			// something: an empty result usually means mpv/aplay are
			// missing or unreachable right now, and rejecting a value the
			// admin knows is correct just because detection failed would
			// lock them out of fixing the configuration.
			if len(detected.MPVDevices) > 0 {
				known := false
				for _, candidate := range detected.MPVDevices {
					if candidate.ID == device {
						known = true
						break
					}
				}
				if !known {
					problem(w, http.StatusBadRequest, fmt.Errorf("device %q was not found by mpv --audio-device=help", device))
					return
				}
			}
		}
		settings, ok, err := a.Store.LoadBoxSettings()
		if err != nil || !ok {
			if err == nil {
				err = errors.New("settings not initialized")
			}
			problem(w, http.StatusInternalServerError, err)
			return
		}
		settings.Audio.Device = device
		if err = a.Store.SaveBoxSettings(settings); err != nil {
			problem(w, http.StatusBadRequest, err)
			return
		}
		jsonResponse(w, http.StatusOK, map[string]any{"settings": settings.Audio, "restart_required": true})
	})

	mux.HandleFunc("PUT /api/admin/audio/mupihat", func(w http.ResponseWriter, r *http.Request) {
		if a.Store == nil || a.Connectivity == nil {
			problem(w, http.StatusServiceUnavailable, errors.New("MuPiHAT audio management unavailable"))
			return
		}
		var input struct {
			Enabled bool `json:"enabled"`
		}
		if err := decode(w, r, &input); err != nil {
			problem(w, http.StatusBadRequest, err)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		changed, err := a.Connectivity.SetMuPiHATAudio(ctx, input.Enabled)
		cancel()
		if err != nil {
			problem(w, http.StatusBadGateway, err)
			return
		}
		settings, ok, err := a.Store.LoadBoxSettings()
		if err != nil || !ok {
			if err == nil {
				err = errors.New("settings not initialized")
			}
			problem(w, http.StatusInternalServerError, err)
			return
		}
		settings.MuPiHAT.AudioEnabled = input.Enabled
		if err = a.Store.SaveBoxSettings(settings); err != nil {
			problem(w, http.StatusBadRequest, err)
			return
		}
		jsonResponse(w, http.StatusOK, map[string]any{"enabled": input.Enabled, "reboot_required": changed})
	})
}
