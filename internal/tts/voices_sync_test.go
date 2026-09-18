package tts

import (
	"os"
	"path/filepath"
	"testing"

	"mupibox/internal/tts/manifest"
)

func TestSyncInstalledVoicesOnlyRegistersCompletePairs(t *testing.T) {
	s := newTestStore(t)
	dir := t.TempDir()

	voices, err := manifest.Load()
	if err != nil {
		t.Fatal(err)
	}
	de := findVoice(t, voices, "de-DE")
	// Only the German voice gets both files; everything else stays missing.
	if err := os.WriteFile(manifest.ModelPath(dir, de), []byte("model-bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifest.ConfigPath(dir, de), []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	// A voice with only the model file (no config) must not be registered.
	fr := findVoice(t, voices, "fr-FR")
	if err := os.WriteFile(manifest.ModelPath(dir, fr), []byte("model-bytes"), 0600); err != nil {
		t.Fatal(err)
	}

	report, err := SyncInstalledVoices(s, dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Installed) != 1 || report.Installed[0] != de.ID() {
		t.Fatalf("expected only %s installed, got %#v", de.ID(), report.Installed)
	}
	for _, id := range report.Missing {
		if id == de.ID() {
			t.Fatal("de-DE must not be reported missing")
		}
	}

	voice, ok, err := s.GetTTSVoice(de.ID())
	if err != nil || !ok || !voice.Available {
		t.Fatalf("expected de-DE voice available in store: %#v ok=%v err=%v", voice, ok, err)
	}
	langs, err := s.ListTTSLanguagesWithVoices()
	if err != nil || len(langs) != 1 || langs[0] != "de-DE" {
		t.Fatalf("expected only de-DE listed as available: %#v err=%v", langs, err)
	}
}

func TestSyncInstalledVoicesMarksPreviouslyInstalledVoiceUnavailableWhenFileDisappears(t *testing.T) {
	s := newTestStore(t)
	dir := t.TempDir()
	voices, err := manifest.Load()
	if err != nil {
		t.Fatal(err)
	}
	de := findVoice(t, voices, "de-DE")
	model := manifest.ModelPath(dir, de)
	config := manifest.ConfigPath(dir, de)
	if err := os.WriteFile(model, []byte("model-bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config, []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := SyncInstalledVoices(s, dir); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(model); err != nil {
		t.Fatal(err)
	}
	if _, err := SyncInstalledVoices(s, dir); err != nil {
		t.Fatal(err)
	}
	voice, ok, err := s.GetTTSVoice(de.ID())
	if err != nil || !ok || voice.Available {
		t.Fatalf("expected voice to become unavailable, not disappear: %#v ok=%v err=%v", voice, ok, err)
	}
}

func findVoice(t *testing.T, voices []manifest.Voice, language string) manifest.Voice {
	t.Helper()
	for _, v := range voices {
		if v.Language == language {
			return v
		}
	}
	t.Fatalf("no manifest voice for language %s", language)
	return manifest.Voice{}
}

func TestSyncInstalledVoicesIgnoresEmptyFiles(t *testing.T) {
	s := newTestStore(t)
	dir := t.TempDir()
	voices, err := manifest.Load()
	if err != nil {
		t.Fatal(err)
	}
	de := findVoice(t, voices, "de-DE")
	if err := os.WriteFile(manifest.ModelPath(dir, de), nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifest.ConfigPath(dir, de), []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	report, err := SyncInstalledVoices(s, dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Installed) != 0 {
		t.Fatalf("a zero-byte model file must not count as installed: %#v", report.Installed)
	}
	if _, ok, err := s.GetTTSVoice(de.ID()); err != nil || ok {
		t.Fatalf("truncated download must not be registered as a voice: ok=%v err=%v", ok, err)
	}
}

func TestManifestPathsAreFlat(t *testing.T) {
	voices, err := manifest.Load()
	if err != nil {
		t.Fatal(err)
	}
	de := findVoice(t, voices, "de-DE")
	if got := manifest.ModelPath("/x", de); filepath.Dir(got) != "/x" {
		t.Fatalf("expected model path directly under root, got %s", got)
	}
}
