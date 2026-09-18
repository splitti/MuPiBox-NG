package server

import (
	"context"
	"errors"
	"net/http"
	"os"
	"strings"
	"time"

	"mupibox/internal/core"
	"mupibox/internal/store"
	"mupibox/internal/tts"
	"mupibox/internal/tts/manifest"
)

type ttsVoiceSummary struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	Quality     string `json:"quality"`
}

type ttsGenerationStatus struct {
	ID              int64  `json:"id"`
	Status          string `json:"status"`
	Language        string `json:"language"`
	VoiceID         string `json:"voice_id"`
	PendingHigh     int    `json:"pending_high"`
	PendingNormal   int    `json:"pending_normal"`
	PendingLow      int    `json:"pending_low"`
	Processing      int    `json:"processing"`
	Completed       int    `json:"completed"`
	Failed          int    `json:"failed"`
	ProgressPercent int    `json:"progress_percent"`
}

type ttsStatusResponse struct {
	Enabled             bool                 `json:"enabled"`
	Available           bool                 `json:"available"`
	Engine              string               `json:"engine,omitempty"`
	EngineVersion       string               `json:"engine_version,omitempty"`
	Language            string               `json:"language"`
	Voice               *ttsVoiceSummary     `json:"voice,omitempty"`
	CPULimitPercent     int                  `json:"cpu_limit_percent"`
	PreRenderingEnabled bool                 `json:"pre_rendering_enabled"`
	WorkerRunning       bool                 `json:"worker_running"`
	ActiveGeneration    *ttsGenerationStatus `json:"active_generation,omitempty"`
	BuildingGeneration  *ttsGenerationStatus `json:"building_generation,omitempty"`
	CacheEntries        int                  `json:"cache_entries"`
	CacheSizeBytes      int64                `json:"cache_size_bytes"`
	LastError           string               `json:"last_error,omitempty"`
}

func generationProgressPercent(counts store.TTSJobCounts) int {
	total := counts.Done + counts.Error + counts.HighPending + counts.HighRunning + counts.NormalPending + counts.NormalRunning + counts.LowPending + counts.LowRunning
	if total == 0 {
		return 100
	}
	return (counts.Done + counts.Error) * 100 / total
}

func (a *API) ttsGenerationStatusFor(gen store.TTSGeneration) (ttsGenerationStatus, error) {
	counts, err := a.Store.CountTTSJobs(gen.ID)
	if err != nil {
		return ttsGenerationStatus{}, err
	}
	return ttsGenerationStatus{
		ID: gen.ID, Status: gen.Status, Language: gen.Language, VoiceID: gen.VoiceID,
		PendingHigh: counts.HighPending, PendingNormal: counts.NormalPending, PendingLow: counts.LowPending,
		Processing: counts.HighRunning + counts.NormalRunning + counts.LowRunning,
		Completed:  counts.Done, Failed: counts.Error, ProgressPercent: generationProgressPercent(counts),
	}, nil
}

// buildTTSStatus assembles the admin status view from the store's
// generation/job state and the manager's own runtime state. It never blocks
// on rendering: all figures are point-in-time reads of already-persisted
// state (see internal/tts's building/active/stale/purging state machine).
func (a *API) buildTTSStatus() (ttsStatusResponse, error) {
	settings, err := a.currentSettings()
	if err != nil {
		return ttsStatusResponse{}, err
	}
	out := ttsStatusResponse{
		Enabled: settings.TTS.Enabled, Language: settings.TTS.Language,
		CPULimitPercent: settings.TTS.BackgroundCPUPercent, PreRenderingEnabled: settings.TTS.PreRenderingEnabled,
		Available: a.TTSManager != nil,
	}
	if a.TTSManager != nil {
		out.Engine = a.TTSManager.EngineName()
		out.EngineVersion = a.TTSManager.EngineVersion()
		out.WorkerRunning = a.TTSManager.Running()
		out.LastError = a.TTSManager.LastError()
	}
	if a.Store == nil {
		return out, nil
	}
	gen, ok, err := a.Store.CurrentTTSGeneration()
	if err != nil {
		return ttsStatusResponse{}, err
	}
	if !ok {
		return out, nil
	}
	genStatus, err := a.ttsGenerationStatusFor(gen)
	if err != nil {
		return ttsStatusResponse{}, err
	}
	if gen.Status == store.GenerationActive {
		out.ActiveGeneration = &genStatus
	} else {
		out.BuildingGeneration = &genStatus
	}
	if voice, ok, err := a.Store.GetTTSVoice(gen.VoiceID); err == nil && ok {
		out.Voice = &ttsVoiceSummary{ID: voice.ID, DisplayName: voice.DisplayName, Quality: voice.Quality}
	}
	if a.TTSManager != nil {
		if count, size, err := a.TTSManager.CacheStats(gen.ID); err == nil {
			out.CacheEntries, out.CacheSizeBytes = count, size
		}
	}
	return out, nil
}

func (a *API) ttsConfigRequestDefaults() store.TTSSettings {
	settings, err := a.currentSettings()
	if err != nil {
		return store.TTSSettings{BackgroundCPUPercent: 30}
	}
	return settings.TTS
}

func voiceAvailabilityError(voice store.TTSVoice, ok bool, language string) error {
	if !ok || !voice.Available {
		return errors.New("selected voice is not installed or not available")
	}
	if voice.Language != language {
		return errors.New("selected voice does not match the selected language")
	}
	return nil
}

func (a *API) registerTTSRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/admin/tts/status", func(w http.ResponseWriter, r *http.Request) {
		status, err := a.buildTTSStatus()
		if err != nil {
			problem(w, http.StatusInternalServerError, err)
			return
		}
		jsonResponse(w, http.StatusOK, status)
	})

	mux.HandleFunc("GET /api/admin/tts/config", func(w http.ResponseWriter, r *http.Request) {
		settings, err := a.currentSettings()
		if err != nil {
			problem(w, http.StatusInternalServerError, err)
			return
		}
		jsonResponse(w, http.StatusOK, settings.TTS)
	})

	mux.HandleFunc("PUT /api/admin/tts/config", func(w http.ResponseWriter, r *http.Request) {
		if a.Store == nil {
			problem(w, http.StatusServiceUnavailable, errors.New("persistent store unavailable"))
			return
		}
		var input store.TTSSettings
		if err := decode(w, r, &input); err != nil {
			problem(w, http.StatusBadRequest, err)
			return
		}
		input.Language = strings.TrimSpace(input.Language)
		input.VoiceID = strings.TrimSpace(input.VoiceID)
		if input.BackgroundCPUPercent == 0 {
			input.BackgroundCPUPercent = 30
		}
		if input.BackgroundCPUPercent < 1 || input.BackgroundCPUPercent > 100 {
			problem(w, http.StatusBadRequest, errors.New("background_cpu_percent must be 1..100"))
			return
		}
		current, ok, err := a.Store.LoadBoxSettings()
		if err != nil || !ok {
			if err == nil {
				err = errors.New("settings not initialized")
			}
			problem(w, http.StatusInternalServerError, err)
			return
		}
		var voice store.TTSVoice
		if input.Enabled {
			if input.Language == "" || input.VoiceID == "" {
				problem(w, http.StatusBadRequest, errors.New("enabled tts requires language and voice_id"))
				return
			}
			voice, ok, err = a.Store.GetTTSVoice(input.VoiceID)
			if err != nil {
				problem(w, http.StatusInternalServerError, err)
				return
			}
			if verr := voiceAvailabilityError(voice, ok, input.Language); verr != nil {
				problem(w, http.StatusBadRequest, verr)
				return
			}
			if strings.TrimSpace(input.Quality) != "" && input.Quality != voice.Quality {
				problem(w, http.StatusBadRequest, errors.New("quality does not match the selected voice"))
				return
			}
			input.Quality = voice.Quality
			if a.TTSManager == nil {
				problem(w, http.StatusServiceUnavailable, errors.New("tts engine unavailable"))
				return
			}
		}
		input.Provider = current.TTS.Provider // legacy browser-fallback field, unrelated to the Piper engine config here
		next := current
		next.TTS = input
		next = store.NormalizeBoxSettings(next)
		if err = store.ValidateBoxSettings(next); err != nil {
			problem(w, http.StatusBadRequest, err)
			return
		}
		if err = a.Store.SaveBoxSettings(next); err != nil {
			problem(w, http.StatusInternalServerError, err)
			return
		}
		a.TTS = TTSConfig{Enabled: next.TTS.Enabled, Language: next.TTS.Language, Provider: next.TTS.Provider}

		cacheRelevant := input.Enabled && (current.TTS.Language != input.Language || current.TTS.VoiceID != input.VoiceID)
		if input.Enabled && a.TTSManager != nil {
			if _, hasGen, _ := a.Store.CurrentTTSGeneration(); !hasGen {
				cacheRelevant = true
			}
			if cacheRelevant {
				if _, err = a.TTSManager.SwitchLanguage(r.Context(), tts.Config{
					Enabled: true, Language: input.Language, VoiceID: input.VoiceID, BackgroundCPUPercent: input.BackgroundCPUPercent,
				}); err != nil {
					problem(w, http.StatusInternalServerError, err)
					return
				}
			} else if gen, hasGen, _ := a.Store.CurrentTTSGeneration(); hasGen && gen.BackgroundCPUPercent != input.BackgroundCPUPercent {
				if err = a.Store.UpdateTTSGenerationCPUPercent(gen.ID, input.BackgroundCPUPercent); err != nil {
					problem(w, http.StatusInternalServerError, err)
					return
				}
			}
			a.TTSManager.Start()
			// Only the fixed HIGH system texts are seeded by SwitchLanguage
			// itself; category/media/library content for the (possibly
			// new) generation is registered here, in the background so the
			// response is not delayed by walking the whole library.
			a.runBackground(func(ctx context.Context) { _ = a.RegisterAllSpeakableContent(ctx) })
		} else if !input.Enabled && a.TTSManager != nil {
			a.TTSManager.Stop()
		}
		status, err := a.buildTTSStatus()
		if err != nil {
			problem(w, http.StatusInternalServerError, err)
			return
		}
		jsonResponse(w, http.StatusOK, map[string]any{"settings": next.TTS, "status": status})
	})

	mux.HandleFunc("GET /api/admin/tts/voices", func(w http.ResponseWriter, r *http.Request) {
		if a.Store == nil {
			problem(w, http.StatusServiceUnavailable, errors.New("persistent store unavailable"))
			return
		}
		language := strings.TrimSpace(r.URL.Query().Get("language"))
		catalog, err := manifest.Load()
		if err != nil {
			problem(w, http.StatusInternalServerError, err)
			return
		}
		type voiceInfo struct {
			ID             string `json:"id"`
			Language       string `json:"language"`
			VoiceFamily    string `json:"voice_family"`
			DisplayName    string `json:"display_name"`
			Engine         string `json:"engine"`
			Quality        string `json:"quality"`
			Installed      bool   `json:"installed"`
			Available      bool   `json:"available"`
			SpeakerSupport bool   `json:"speaker_support"`
			SpeakerID      *int   `json:"speaker_id,omitempty"`
			License        string `json:"license"`
			ModelSizeBytes int64  `json:"model_size_bytes,omitempty"`
		}
		out := []voiceInfo{}
		for _, v := range catalog {
			if language != "" && v.Language != language {
				continue
			}
			info := voiceInfo{
				ID: v.ID(), Language: v.Language, VoiceFamily: v.VoiceFamily, DisplayName: v.DisplayName, Engine: "piper", Quality: v.Quality,
				SpeakerSupport: v.SpeakerID != nil, SpeakerID: v.SpeakerID, License: v.License,
			}
			if row, ok, err := a.Store.GetTTSVoice(v.ID()); err == nil && ok {
				info.Installed = true
				info.Available = row.Available
				info.License = row.License
				if stat, statErr := os.Stat(row.ModelPath); statErr == nil {
					info.ModelSizeBytes = stat.Size()
				}
			}
			out = append(out, info)
		}
		jsonResponse(w, http.StatusOK, map[string]any{"voices": out})
	})

	mux.HandleFunc("POST /api/admin/tts/test", func(w http.ResponseWriter, r *http.Request) {
		if a.TTSManager == nil || a.Store == nil {
			problem(w, http.StatusServiceUnavailable, errors.New("tts engine unavailable"))
			return
		}
		var input struct {
			Text     string `json:"text"`
			Language string `json:"language,omitempty"`
			VoiceID  string `json:"voice_id,omitempty"`
		}
		if err := decode(w, r, &input); err != nil {
			problem(w, http.StatusBadRequest, err)
			return
		}
		if strings.TrimSpace(input.Text) == "" {
			problem(w, http.StatusBadRequest, errors.New("text is required"))
			return
		}
		voiceID := strings.TrimSpace(input.VoiceID)
		language := strings.TrimSpace(input.Language)
		if voiceID == "" {
			defaults := a.ttsConfigRequestDefaults()
			voiceID, language = defaults.VoiceID, defaults.Language
		}
		if voiceID == "" {
			problem(w, http.StatusBadRequest, errors.New("no voice selected"))
			return
		}
		voice, ok, err := a.Store.GetTTSVoice(voiceID)
		if err != nil {
			problem(w, http.StatusInternalServerError, err)
			return
		}
		if verr := voiceAvailabilityError(voice, ok, cmpLanguage(language, voice)); verr != nil {
			problem(w, http.StatusBadRequest, verr)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
		defer cancel()
		path, cleanup, err := a.TTSManager.RenderTestClip(ctx, voice, input.Text)
		if err != nil {
			problem(w, http.StatusBadGateway, err)
			return
		}
		if err := a.playTestClip(path, cleanup); err != nil {
			cleanup()
			problem(w, http.StatusBadGateway, err)
			return
		}
		jsonResponse(w, http.StatusOK, map[string]bool{"playing": true})
	})

	// Rebuild starts a fresh generation for the current configuration
	// through the exact same state machine as a language/voice change (see
	// PUT .../config): the previous generation is only marked stale, never
	// deleted synchronously, and this call returns as soon as the new
	// generation exists -- it never waits for rendering.
	mux.HandleFunc("POST /api/admin/tts/cache/rebuild", func(w http.ResponseWriter, r *http.Request) {
		if a.TTSManager == nil {
			problem(w, http.StatusServiceUnavailable, errors.New("tts engine unavailable"))
			return
		}
		settings, err := a.currentSettings()
		if err != nil {
			problem(w, http.StatusInternalServerError, err)
			return
		}
		if !settings.TTS.Enabled || settings.TTS.VoiceID == "" {
			problem(w, http.StatusConflict, errors.New("tts is not configured"))
			return
		}
		if _, err := a.TTSManager.SwitchLanguage(r.Context(), tts.Config{
			Enabled: true, Language: settings.TTS.Language, VoiceID: settings.TTS.VoiceID, BackgroundCPUPercent: settings.TTS.BackgroundCPUPercent,
		}); err != nil {
			problem(w, http.StatusInternalServerError, err)
			return
		}
		a.TTSManager.Start()
		a.runBackground(func(ctx context.Context) { _ = a.RegisterAllSpeakableContent(ctx) })
		status, err := a.buildTTSStatus()
		if err != nil {
			problem(w, http.StatusInternalServerError, err)
			return
		}
		jsonResponse(w, http.StatusAccepted, status)
	})

	// FillMissing registers all currently known speakable content against
	// the CURRENT generation without creating a new one: EnsureText is a
	// no-op for text that already has a cache file, so this only ever
	// enqueues genuinely missing (or previously failed, see
	// store.EnqueueTTSJob) entries -- unlike rebuild, which always starts a
	// fresh generation.
	mux.HandleFunc("POST /api/admin/tts/cache/fill-missing", func(w http.ResponseWriter, r *http.Request) {
		if a.TTSManager == nil {
			problem(w, http.StatusServiceUnavailable, errors.New("tts engine unavailable"))
			return
		}
		if _, ok, err := a.Store.CurrentTTSGeneration(); err != nil || !ok {
			problem(w, http.StatusConflict, errors.New("tts is not configured"))
			return
		}
		a.runBackground(func(ctx context.Context) { _ = a.RegisterAllSpeakableContent(ctx) })
		jsonResponse(w, http.StatusAccepted, map[string]bool{"started": true})
	})

	// Cleanup forces the same stale/purging sweep the worker performs
	// periodically on its own; it only ever removes generations that are
	// already not-current, never the active/building one.
	mux.HandleFunc("POST /api/admin/tts/cache/cleanup", func(w http.ResponseWriter, r *http.Request) {
		if a.TTSManager == nil {
			problem(w, http.StatusServiceUnavailable, errors.New("tts engine unavailable"))
			return
		}
		if err := a.TTSManager.PurgeNow(); err != nil {
			problem(w, http.StatusInternalServerError, err)
			return
		}
		jsonResponse(w, http.StatusOK, map[string]bool{"cleaned": true})
	})

	// Speak is the touch-UI on-demand playback path (see docs/tts.md's
	// "USER REQUEST" flow): cache hit plays immediately, a cache miss
	// enqueues a bounded HIGH-priority render and waits for it. Not under
	// /api/admin/ -- the player itself uses this without admin auth, same
	// as /api/command and /api/home.
	mux.HandleFunc("POST /api/speak", func(w http.ResponseWriter, r *http.Request) {
		if a.TTSManager == nil {
			problem(w, http.StatusServiceUnavailable, errors.New("tts unavailable"))
			return
		}
		var input struct {
			SourceType string `json:"source_type"`
			SourceRef  string `json:"source_ref"`
			Text       string `json:"text"`
		}
		if err := decode(w, r, &input); err != nil {
			problem(w, http.StatusBadRequest, err)
			return
		}
		if strings.TrimSpace(input.Text) == "" {
			problem(w, http.StatusBadRequest, errors.New("text is required"))
			return
		}
		// Never let an announcement and music playback talk over each
		// other: pause first, predictably, rather than trying to duck/mix
		// levels. The user can resume music explicitly afterwards.
		if a.Player != nil && a.Player.Status().State == "playing" {
			_ = a.Player.Execute(core.Command{Action: "pause"})
		}
		ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
		defer cancel()
		path, err := a.TTSManager.ResolveSpeech(ctx, input.SourceType, input.SourceRef, input.Text)
		if err != nil {
			problem(w, http.StatusBadGateway, err)
			return
		}
		if err := a.playTestClip(path, func() {}); err != nil {
			problem(w, http.StatusBadGateway, err)
			return
		}
		jsonResponse(w, http.StatusOK, map[string]bool{"speaking": true})
	})
}

// cmpLanguage lets a voice test omit language and just validate against the
// voice's own language, while still enforcing a match when a caller
// supplies one explicitly (e.g. testing a voice before saving it as the new
// global language).
func cmpLanguage(requested string, voice store.TTSVoice) string {
	if requested == "" {
		return voice.Language
	}
	return requested
}

// playTestClip plays a short preview through a dedicated one-shot backend
// instance (the same audio.Backend/mpv adapter the main player uses), never
// through the player's queue: a voice test must not interrupt or pollute
// whatever the box is currently playing. The clip file is removed once
// playback ends or after a safety timeout, whichever comes first.
func (a *API) playTestClip(path string, cleanup func()) error {
	if a.NewAudioBackend == nil {
		return errors.New("audio backend unavailable")
	}
	backend := a.NewAudioBackend()
	volume := 70
	if a.Player != nil {
		if max := a.Player.Status().MaxVolume; max > 0 && max < volume {
			volume = max
		}
	}
	if err := backend.Load(path, volume); err != nil {
		_ = backend.Close()
		return err
	}
	go func() {
		deadline := time.Now().Add(30 * time.Second)
		for time.Now().Before(deadline) {
			snapshot, err := backend.Snapshot()
			if err != nil || snapshot.Ended {
				break
			}
			time.Sleep(300 * time.Millisecond)
		}
		_ = backend.Close()
		cleanup()
	}()
	return nil
}
