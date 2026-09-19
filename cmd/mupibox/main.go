package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"mupibox/internal/audio"
	"mupibox/internal/connectivity"
	"mupibox/internal/core"
	"mupibox/internal/library"
	"mupibox/internal/providers/spotify"
	"mupibox/internal/server"
	"mupibox/internal/store"
	"mupibox/internal/tts"
	"mupibox/internal/tts/manifest"
)

var version = "0.1.0-dev"

type config struct {
	Listen       string `json:"listen"`
	MusicDir     string `json:"music_dir"`
	DatabasePath string `json:"database_path"`
	Backend      string `json:"backend"`
	// TTS bootstrap paths (deployment layout, not user-editable settings --
	// see CLAUDE.md's config.json/SQLite split). Empty means the default
	// under DatabasePath's directory (cache) or the standard install
	// location (voices, matching scripts/install-tts-voices.sh).
	TTSVoicesDir string `json:"tts_voices_dir"`
	TTSCacheDir  string `json:"tts_cache_dir"`
	// TTSPiperPython is the Python interpreter with piper-tts installed
	// (see scripts/install-piper-engine.sh). piper1-gpl (the maintained
	// successor to the archived rhasspy/piper) ships only as a PyPI
	// package invoked as "<python> -m piper", not a standalone binary, so
	// this must point at an interpreter, not an executable named "piper".
	TTSPiperPython string `json:"tts_piper_python"`
	// TTSPiperBinaryDeprecated is the pre-piper1-gpl field name. Accepted
	// (not rejected by DisallowUnknownFields) purely so a config.json
	// written before that migration keeps starting the service; migrate()
	// copies it into TTSPiperPython and logs a deprecation warning. Real
	// typos in any *other* field are still caught, since only this one
	// known-renamed name is special-cased here.
	TTSPiperBinaryDeprecated string `json:"tts_piper_binary,omitempty"`
}

// migrate normalizes deprecated config field names into their current
// equivalent. Called once right after decoding; every other reader of cfg
// only ever sees the current field.
func (c *config) migrate() {
	if c.TTSPiperPython == "" && c.TTSPiperBinaryDeprecated != "" {
		log.Printf("config: \"tts_piper_binary\" is deprecated, use \"tts_piper_python\" instead (still honoring it for now)")
		c.TTSPiperPython = c.TTSPiperBinaryDeprecated
	}
}

func defaultNavigation() store.Navigation {
	return store.Navigation{Categories: []store.Category{
		{ID: "audiobooks", Labels: map[string]string{"de": "Hörbücher", "en": "Audiobooks"}, Rows: []store.Row{}},
		{ID: "music", Labels: map[string]string{"de": "Musik", "en": "Music"}, Rows: []store.Row{{ID: "local-library", Labels: map[string]string{"de": "Lokale Medien", "en": "Local media"}, Provider: "local-library", SourceType: "library"}}},
		{ID: "radio", Labels: map[string]string{"de": "Radio", "en": "Radio"}, Rows: []store.Row{}},
		{ID: "podcasts", Labels: map[string]string{"de": "Podcasts", "en": "Podcasts"}, Rows: []store.Row{}},
	}}
}
func run() error {
	path := flag.String("config", "", "JSON configuration file")
	showVersion := flag.Bool("version", false, "print version")
	ttsPrintManifest := flag.Bool("tts-print-manifest", false, "print the built-in Piper voice manifest as TSV (id, language, voice_family, quality, tier, model_url, config_url, license, source_url, speaker_id) and exit")
	ttsSyncVoicesDir := flag.String("tts-sync-voices", "", "reconcile a downloaded voices directory against the manifest into the database (requires -config) and exit")
	flag.Parse()
	if *showVersion {
		fmt.Println(version)
		return nil
	}
	if *ttsPrintManifest {
		return printTTSManifest()
	}
	cfg := config{Listen: "127.0.0.1:8090", MusicDir: "./music", DatabasePath: "./var/mupibox.db", Backend: "mpv"}
	if *path != "" {
		f, err := os.Open(*path)
		if err != nil {
			return err
		}
		defer f.Close()
		d := json.NewDecoder(f)
		d.DisallowUnknownFields()
		if err = d.Decode(&cfg); err != nil {
			return err
		}
		var extra any
		if d.Decode(&extra) != io.EOF {
			return errors.New("expected one config object")
		}
		cfg.migrate()
	}
	stateStore, err := store.Open(cfg.DatabasePath)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer stateStore.Close()
	if *ttsSyncVoicesDir != "" {
		return syncTTSVoices(stateStore, *ttsSyncVoicesDir)
	}
	settings, err := stateStore.EnsureBoxSettings(store.BoxSettings{
		Language:      "de",
		AdminLanguage: "de",
		TTS:           store.TTSSettings{Enabled: false, Language: "de", Provider: ""},
		Power:         store.PowerSettings{IdleShutdownMinutes: 0},
		Audio:         store.AudioSettings{StartupVolume: 30, MaxVolume: 60},
		Display:       store.DisplaySettings{IdleOffMinutes: 0, Brightness: 100, UISize: "normal"},
		Bluetooth:     store.BluetoothSettings{Enabled: false},
		Theme:         "modern-dark",
	})
	if err != nil {
		return fmt.Errorf("initialize settings: %w", err)
	}
	if err = stateStore.EnsureNavigation(defaultNavigation()); err != nil {
		return fmt.Errorf("initialize navigation: %w", err)
	}
	lib, err := library.Scan(cfg.MusicDir)
	if err != nil {
		return fmt.Errorf("scan music: %w", err)
	}
	var backend audio.Backend
	switch cfg.Backend {
	case "mpv":
		backend = &audio.MPV{Device: settings.Audio.Device}
	case "simulated":
		backend = &audio.Simulated{}
	default:
		return fmt.Errorf("unknown backend %q", cfg.Backend)
	}
	p, err := core.New(lib, backend, settings.Audio.MaxVolume)
	if err != nil {
		return err
	}
	p.SetProgressRepository(stateStore)
	defer p.Close()
	newAudioBackend := func() audio.Backend {
		if cfg.Backend == "mpv" {
			return &audio.MPV{Device: settings.Audio.Device}
		}
		return &audio.Simulated{}
	}
	ttsManager := newTTSManager(stateStore, cfg)
	if ttsManager != nil {
		defer ttsManager.Stop()
		if settings.TTS.Enabled && settings.TTS.VoiceID != "" {
			if _, ok, genErr := stateStore.CurrentTTSGeneration(); genErr == nil && !ok {
				if _, switchErr := ttsManager.SwitchLanguage(context.Background(), tts.Config{
					Enabled: true, Language: settings.TTS.Language, VoiceID: settings.TTS.VoiceID, BackgroundCPUPercent: settings.TTS.BackgroundCPUPercent,
				}); switchErr != nil {
					log.Printf("tts: could not start initial cache generation: %v", switchErr)
				}
			}
			ttsManager.Start()
		}
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	workerDone := make(chan struct{})
	go func() { defer close(workerDone); p.Run(ctx) }()
	defer func() { cancel(); <-workerDone }()
	connectivityManager := connectivity.New()
	tuning := connectivity.SystemTuning{PerformanceMode: settings.System.PerformanceMode}
	if _, statErr := os.Stat("/run/mupibox-system-agent/control.sock"); statErr == nil {
		applyContext, applyCancel := context.WithTimeout(ctx, 35*time.Second)
		if applyErr := connectivityManager.ApplySystemTuning(applyContext, tuning); applyErr != nil {
			log.Printf("System tuning could not be applied: %v", applyErr)
		}
		applyCancel()
		if settings.WiFi.PrimaryMAC != "" || settings.WiFi.PrimaryInterface != "" {
			if adapters, listErr := connectivityManager.ListWiFiAdapters(); listErr == nil {
				selected := ""
				for _, adapter := range adapters {
					if settings.WiFi.PrimaryMAC != "" && strings.EqualFold(adapter.MAC, settings.WiFi.PrimaryMAC) {
						selected = adapter.Interface
						break
					}
				}
				if selected == "" {
					for _, adapter := range adapters {
						if adapter.Interface == settings.WiFi.PrimaryInterface {
							selected = adapter.Interface
							break
						}
					}
				}
				if selected != "" {
					wifiContext, wifiCancel := context.WithTimeout(ctx, 35*time.Second)
					if applyErr := connectivityManager.SelectWiFiAdapter(wifiContext, selected); applyErr != nil {
						log.Printf("Preferred Wi-Fi adapter could not be selected: %v", applyErr)
					}
					wifiCancel()
				}
			}
		}
		if settings.Samba.Enabled {
			sambaContext, sambaCancel := context.WithTimeout(ctx, 65*time.Second)
			if applyErr := connectivityManager.ApplySamba(sambaContext, connectivity.SambaConfig{Enabled: true, Mode: settings.Samba.Mode, ShareName: settings.Samba.ShareName, Workgroup: settings.Samba.Workgroup}); applyErr != nil {
				log.Printf("Samba configuration could not be applied: %v", applyErr)
			}
			sambaCancel()
		}
	}
	if settings.Bluetooth.Enabled {
		if err = connectivityManager.SetBluetoothPower(ctx, true); err != nil {
			log.Printf("Bluetooth could not be enabled: %v", err)
		}
	}
	spotifyConfigDir := filepath.Join(filepath.Dir(cfg.DatabasePath), "spotify")
	if spotifyErr := spotify.WriteConfig(spotify.Config{
		ConfigDir: spotifyConfigDir, AudioDevice: settings.Audio.Device, DeviceName: "MuPiBox",
		APIPort: spotify.DefaultAPIPort, PersistCredentials: true,
	}); spotifyErr != nil {
		log.Printf("spotify: could not write go-librespot config: %v", spotifyErr)
	}
	spotifyManager := spotify.NewManager(fmt.Sprintf("http://127.0.0.1:%d", spotify.DefaultAPIPort))
	api := &server.API{Player: p, Library: lib, Store: stateStore, Connectivity: connectivityManager, Version: version, MusicDir: cfg.MusicDir, BackupDir: filepath.Join(filepath.Dir(cfg.DatabasePath), "backups"), TTSManager: ttsManager, Spotify: spotifyManager, NewAudioBackend: newAudioBackend}
	// Wire arbitration before starting Run: an already-active Connect session
	// at startup (box restarted while Spotify was playing) fires its first
	// status refresh as soon as the event stream connects, and events are not
	// replayed -- registering the handler after Run started could miss it.
	api.WireSpotifyArbitration()
	go spotifyManager.Run(ctx)
	if ttsManager != nil {
		// Startup/recovery: register whatever content is already known so a
		// generation surviving a restart (or one built before this process
		// last ran) is caught up, without blocking server startup.
		go func() { _ = api.RegisterAllSpeakableContent(context.Background()) }()
	}
	httpServer := &http.Server{Addr: cfg.Listen, Handler: api.Handler(), ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second}
	shutdownDone := make(chan struct{})
	go func() {
		defer close(shutdownDone)
		<-ctx.Done()
		c, stop := context.WithTimeout(context.Background(), 15*time.Second)
		defer stop()
		if err := httpServer.Shutdown(c); err != nil {
			_ = httpServer.Close()
		}
	}()
	log.Printf("MuPiBox %s: http://%s, backend=%s, folders=%d, database=%s", version, cfg.Listen, backend.Name(), len(lib.Folders), cfg.DatabasePath)
	err = httpServer.ListenAndServe()
	cancel()
	<-shutdownDone
	// Cancel and wait for any in-flight background content registration
	// (library rescan, language switch, rebuild, fill-missing) before the
	// deferred TTS manager stop and database close run below -- otherwise
	// those goroutines would keep writing to a store that is about to
	// disappear underneath them.
	api.Shutdown()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
func printTTSManifest() error {
	voices, err := manifest.Load()
	if err != nil {
		return err
	}
	for _, v := range voices {
		speaker := ""
		if v.SpeakerID != nil {
			speaker = fmt.Sprint(*v.SpeakerID)
		}
		fmt.Printf("%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			v.ID(), v.Language, v.VoiceFamily, v.Quality, v.Tier, v.ModelURL, v.ConfigURL, v.License, v.SourceURL, speaker)
	}
	return nil
}

// newTTSManager builds the internal/tts.Manager used for the runtime
// server. It never returns an error: any failure (engine binary missing,
// cgroup delegation unavailable, ...) is logged and results in a nil
// manager, so a broken TTS setup can never stop MuPiBox from starting --
// the admin API and UI report the failure instead (see
// internal/server/tts_admin.go).
func newTTSManager(stateStore *store.Store, cfg config) *tts.Manager {
	voicesDir := cfg.TTSVoicesDir
	if voicesDir == "" {
		voicesDir = "/usr/local/share/mupibox-ng/voices"
	}
	if _, err := tts.SyncInstalledVoices(stateStore, voicesDir); err != nil {
		log.Printf("tts: voice sync failed, continuing without it: %v", err)
	}
	cacheDir := cfg.TTSCacheDir
	if cacheDir == "" {
		cacheDir = filepath.Join(filepath.Dir(cfg.DatabasePath), "tts")
	}
	cgroupPath, cgErr := tts.DetectOwnCgroupPath()
	if cgErr != nil {
		cgroupPath = ""
	}
	limiter := tts.NewAutoCPULimiter(cgroupPath, nil)
	pythonBin := cfg.TTSPiperPython
	if pythonBin == "" {
		pythonBin = "/usr/local/lib/mupibox-ng/piper-venv/bin/python3"
	}
	engine, err := tts.NewPiperEngine(pythonBin, limiter, nil)
	if err != nil {
		log.Printf("tts: engine unavailable, TTS stays disabled until this is fixed: %v", err)
		return nil
	}
	manager := tts.NewManager(stateStore, engine, cacheDir, tts.StaticSystemTexts{}, nil)
	if err := manager.Recover(); err != nil {
		log.Printf("tts: startup recovery failed: %v", err)
	}
	return manager
}

func syncTTSVoices(stateStore *store.Store, voicesDir string) error {
	report, err := tts.SyncInstalledVoices(stateStore, voicesDir)
	if err != nil {
		return err
	}
	log.Printf("tts voices synced: installed=%d missing=%d", len(report.Installed), len(report.Missing))
	for _, id := range report.Missing {
		log.Printf("tts voice not installed (model/config file missing): %s", id)
	}
	return nil
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}
