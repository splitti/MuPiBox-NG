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
	"syscall"
	"time"

	"mupibox/internal/audio"
	"mupibox/internal/connectivity"
	"mupibox/internal/core"
	"mupibox/internal/library"
	"mupibox/internal/server"
	"mupibox/internal/store"
)

var version = "0.1.0-dev"

type config struct {
	Listen       string `json:"listen"`
	MusicDir     string `json:"music_dir"`
	DatabasePath string `json:"database_path"`
	Backend      string `json:"backend"`
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
	flag.Parse()
	if *showVersion {
		fmt.Println(version)
		return nil
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
	}
	stateStore, err := store.Open(cfg.DatabasePath)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer stateStore.Close()
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
		backend = &audio.MPV{}
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
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	workerDone := make(chan struct{})
	go func() { defer close(workerDone); p.Run(ctx) }()
	defer func() { cancel(); <-workerDone }()
	connectivityManager := connectivity.New()
	tuning := connectivity.SystemTuning{PerformanceMode: settings.System.PerformanceMode}
	if settings.System.SwapPolicy != "keep" {
		enabled := settings.System.SwapPolicy == "enabled"
		tuning.SwapEnabled = &enabled
	}
	if settings.System.WaitOnlinePolicy != "keep" {
		enabled := settings.System.WaitOnlinePolicy == "enabled"
		tuning.WaitOnlineEnabled = &enabled
	}
	if _, statErr := os.Stat("/run/mupibox-system-agent/control.sock"); statErr == nil {
		applyContext, applyCancel := context.WithTimeout(ctx, 35*time.Second)
		if applyErr := connectivityManager.ApplySystemTuning(applyContext, tuning); applyErr != nil {
			log.Printf("System tuning could not be applied: %v", applyErr)
		}
		applyCancel()
	}
	if settings.Bluetooth.Enabled {
		if err = connectivityManager.SetBluetoothPower(ctx, true); err != nil {
			log.Printf("Bluetooth could not be enabled: %v", err)
		}
	}
	api := &server.API{Player: p, Library: lib, Store: stateStore, Connectivity: connectivityManager, Version: version, MusicDir: cfg.MusicDir, BackupDir: filepath.Join(filepath.Dir(cfg.DatabasePath), "backups")}
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
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}
