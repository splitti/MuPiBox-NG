package main

import (
	"bytes"
	"encoding/json"
	"io"
	"testing"
)

// decodeConfig mirrors run()'s own decode+migrate sequence, so this test
// exercises exactly the code path a real config.json goes through.
func decodeConfig(t *testing.T, jsonText string) (config, error) {
	t.Helper()
	cfg := config{Listen: "127.0.0.1:8090", MusicDir: "./music", DatabasePath: "./var/mupibox.db", Backend: "mpv"}
	d := json.NewDecoder(bytes.NewReader([]byte(jsonText)))
	d.DisallowUnknownFields()
	if err := d.Decode(&cfg); err != nil {
		return config{}, err
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		t.Fatal("test config must be a single JSON object")
	}
	cfg.migrate()
	return cfg, nil
}

func TestOldConfigWithDeprecatedTTSPiperBinaryFieldStillStarts(t *testing.T) {
	cfg, err := decodeConfig(t, `{
		"listen": "0.0.0.0:8090",
		"database_path": "/var/lib/mupibox-ng/mupibox.db",
		"music_dir": "/srv/mupibox/music",
		"backend": "mpv",
		"tts_piper_binary": "/usr/local/lib/mupibox-ng/piper/piper",
		"tts_voices_dir": "/usr/local/share/mupibox-ng/voices",
		"tts_cache_dir": "/var/lib/mupibox-ng/tts"
	}`)
	if err != nil {
		t.Fatalf("old config with the deprecated field must still decode: %v", err)
	}
	if cfg.TTSPiperPython != "/usr/local/lib/mupibox-ng/piper/piper" {
		t.Fatalf("expected the deprecated value migrated into TTSPiperPython, got %q", cfg.TTSPiperPython)
	}
}

func TestCurrentConfigWithTTSPiperPythonIsUnaffectedByMigration(t *testing.T) {
	cfg, err := decodeConfig(t, `{
		"listen": "0.0.0.0:8090",
		"database_path": "/var/lib/mupibox-ng/mupibox.db",
		"music_dir": "/srv/mupibox/music",
		"backend": "mpv",
		"tts_piper_python": "/usr/local/lib/mupibox-ng/piper-venv/bin/python3"
	}`)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.TTSPiperPython != "/usr/local/lib/mupibox-ng/piper-venv/bin/python3" {
		t.Fatalf("current field must pass through unchanged, got %q", cfg.TTSPiperPython)
	}
	if cfg.TTSPiperBinaryDeprecated != "" {
		t.Fatalf("expected the deprecated field to stay empty, got %q", cfg.TTSPiperBinaryDeprecated)
	}
}

func TestCurrentFieldWinsIfBothOldAndNewArePresent(t *testing.T) {
	cfg, err := decodeConfig(t, `{
		"listen": "0.0.0.0:8090",
		"database_path": "/var/lib/mupibox-ng/mupibox.db",
		"music_dir": "/srv/mupibox/music",
		"backend": "mpv",
		"tts_piper_python": "/current/python3",
		"tts_piper_binary": "/old/piper"
	}`)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.TTSPiperPython != "/current/python3" {
		t.Fatalf("the current field must win over the deprecated one, got %q", cfg.TTSPiperPython)
	}
}

func TestGenuineUnknownConfigFieldIsStillRejected(t *testing.T) {
	// A real typo/unsupported field must not be silently swallowed just
	// because we now tolerate one specific known-renamed field.
	_, err := decodeConfig(t, `{
		"listen": "0.0.0.0:8090",
		"database_path": "/var/lib/mupibox-ng/mupibox.db",
		"music_dir": "/srv/mupibox/music",
		"backend": "mpv",
		"tts_piper_pythn": "/typo/path"
	}`)
	if err == nil {
		t.Fatal("expected a genuine unknown field (typo) to be rejected")
	}
}
