package store

import (
	"path/filepath"
	"testing"
	"time"
)

func TestMigrationsSettingsNavigationAndProgress(t *testing.T) {
	s, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	defaults := BoxSettings{Language: "de", AdminLanguage: "de", TTS: TTSSettings{Enabled: true, Language: "de", Provider: "offline"}, Power: PowerSettings{IdleShutdownMinutes: 30}, Audio: AudioSettings{StartupVolume: 25, MaxVolume: 60}, Display: DisplaySettings{IdleOffMinutes: 5, Brightness: 80}, Theme: "modern-dark"}
	got, err := s.EnsureBoxSettings(defaults)
	if err != nil || got.Audio.MaxVolume != 60 {
		t.Fatalf("settings: %#v %v", got, err)
	}
	got.Audio.MaxVolume = 70
	if err = s.SaveBoxSettings(got); err != nil {
		t.Fatal(err)
	}
	got, ok, err := s.LoadBoxSettings()
	if err != nil || !ok || got.Audio.MaxVolume != 70 {
		t.Fatalf("saved settings: %#v %v", got, err)
	}

	nav := Navigation{Categories: []Category{{ID: "books", Labels: map[string]string{"de": "Hörbücher", "en": "Audiobooks"}, Rows: []Row{{ID: "local-books", Labels: map[string]string{"de": "Lokal"}, Provider: "spotify", SourceType: "artist", SourceRef: "spotify:artist:example"}}}}}
	if err = s.EnsureNavigation(nav); err != nil {
		t.Fatal(err)
	}
	loaded, err := s.LoadNavigation()
	if err != nil || len(loaded.Categories) != 1 || len(loaded.Categories[0].Rows) != 1 || loaded.Categories[0].Rows[0].SourceType != "artist" || loaded.Categories[0].Rows[0].SourceRef != "spotify:artist:example" {
		t.Fatalf("navigation: %#v %v", loaded, err)
	}

	p := Progress{Provider: "spotify", AccountID: "box-account", MediaID: "episode-1", PositionMS: 21600000, DurationMS: 82800000}
	if err = s.SaveProgress(p); err != nil {
		t.Fatal(err)
	}
	pg, ok, err := s.LoadProgress("spotify", "box-account", "episode-1")
	if err != nil || !ok || pg.PositionMS != 21600000 {
		t.Fatalf("progress: %#v %v", pg, err)
	}
	if err = s.SaveProgress(Progress{Provider: "radio", MediaID: "live", PositionMS: 1000}); err != nil {
		t.Fatal(err)
	}
	if _, ok, err = s.LoadProgress("radio", "", "live"); err != nil || ok {
		t.Fatal("radio progress must not be stored")
	}
	older := time.Now().UTC().Add(-time.Hour)
	newer := time.Now().UTC()
	if err = s.SaveProgress(Progress{Provider: "local", MediaID: "old", ContextID: "folder-old", PositionMS: 5000, UpdatedAt: older}); err != nil {
		t.Fatal(err)
	}
	if err = s.SaveProgress(Progress{Provider: "local", MediaID: "new", ContextID: "folder-new", PositionMS: 6000, UpdatedAt: newer}); err != nil {
		t.Fatal(err)
	}
	if err = s.SaveProgress(Progress{Provider: "local", MediaID: "done", ContextID: "folder-done", PositionMS: 7000, Completed: true}); err != nil {
		t.Fatal(err)
	}
	recent, err := s.ListRecentProgress(10)
	if err != nil || len(recent) != 3 || recent[0].MediaID != "new" {
		t.Fatalf("recent progress: %#v %v", recent, err)
	}
}

func TestSnapshotAndRestore(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(filepath.Join(dir, "live.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	settings := BoxSettings{Language: "de", AdminLanguage: "de", Audio: AudioSettings{StartupVolume: 30, MaxVolume: 60}, Display: DisplaySettings{Brightness: 80, UISize: "normal"}, Theme: "modern-dark"}
	if err = s.SaveBoxSettings(settings); err != nil {
		t.Fatal(err)
	}
	if err = s.SaveProgress(Progress{Provider: "local", MediaID: "story", PositionMS: 12345}); err != nil {
		t.Fatal(err)
	}
	snapshot := filepath.Join(dir, "snapshot.db")
	if err = s.Snapshot(snapshot); err != nil {
		t.Fatal(err)
	}
	settings.Audio.MaxVolume = 25
	settings.Audio.StartupVolume = 20
	if err = s.SaveBoxSettings(settings); err != nil {
		t.Fatal(err)
	}
	if err = s.RestoreSnapshot(snapshot); err != nil {
		t.Fatal(err)
	}
	restored, ok, err := s.LoadBoxSettings()
	if err != nil || !ok || restored.Audio.MaxVolume != 60 {
		t.Fatalf("restored settings: %#v ok=%v err=%v", restored, ok, err)
	}
	progress, ok, err := s.LoadProgress("local", "", "story")
	if err != nil || !ok || progress.PositionMS != 12345 {
		t.Fatalf("restored progress: %#v ok=%v err=%v", progress, ok, err)
	}
}

func TestValidation(t *testing.T) {
	s, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	bad := BoxSettings{Language: "de", Audio: AudioSettings{StartupVolume: 70, MaxVolume: 60}, Display: DisplaySettings{Brightness: 80}, Theme: "modern-dark"}
	if err = s.SaveBoxSettings(bad); err == nil {
		t.Fatal("invalid settings accepted")
	}
	invalidWiFi := BoxSettings{Language: "de", AdminLanguage: "de", Audio: AudioSettings{StartupVolume: 30, MaxVolume: 60}, Display: DisplaySettings{Brightness: 80, UISize: "normal"}, WiFi: WiFiSettings{PrimaryInterface: "wlan0", DisabledInterfaces: []string{"wlan0"}}, Theme: "modern-dark"}
	if err = s.SaveBoxSettings(invalidWiFi); err == nil {
		t.Fatal("disabled primary Wi-Fi adapter accepted")
	}
	if err = s.SaveNavigation(Navigation{Categories: []Category{{ID: "bad id", Labels: map[string]string{"de": "Bad"}}}}); err == nil {
		t.Fatal("invalid navigation accepted")
	}
	if err = s.SaveNavigation(Navigation{Categories: []Category{{ID: "books", Labels: map[string]string{"de": "Bücher"}, Rows: []Row{{ID: "escape", Labels: map[string]string{"de": "Escape"}, Provider: "local-library", SourceType: "path", SourceRef: "../private"}}}}}); err == nil {
		t.Fatal("escaping local path accepted")
	}
	if err = s.SaveNavigation(Navigation{Categories: []Category{{ID: "resume", Labels: map[string]string{"de": "Weiterhören"}, Rows: []Row{{ID: "resume-list", Labels: map[string]string{"de": "Zuletzt gehört"}, Provider: "resume-list", SourceType: "limit", SourceRef: "101"}}}}}); err == nil {
		t.Fatal("resume limit above 100 accepted")
	}
}

func TestNewAdminSettingsAreNormalizedAndValidated(t *testing.T) {
	settings := NormalizeBoxSettings(BoxSettings{Language: "de", AdminLanguage: "de", Audio: AudioSettings{StartupVolume: 20, MaxVolume: 60}, Display: DisplaySettings{Brightness: 80}, Theme: "modern-dark"})
	if settings.WiFi.IPv4.Mode != "dhcp" || settings.MQTT.Port != 1883 || len(settings.MuPiHAT.Profiles) != 4 || settings.System.PerformanceMode != "balanced" {
		t.Fatalf("new settings were not normalized: %#v", settings)
	}
	if err := ValidateBoxSettings(settings); err != nil {
		t.Fatal(err)
	}
	settings.WiFi.IPv4 = IPv4Settings{Mode: "static", Interface: "wlan0", Address: "not-an-address", Gateway: "192.168.1.1"}
	if err := ValidateBoxSettings(settings); err == nil {
		t.Fatal("invalid static address accepted")
	}
}
