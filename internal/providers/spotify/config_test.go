package spotify

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteConfigProducesExpectedFields(t *testing.T) {
	dir := t.TempDir()
	if err := WriteConfig(Config{
		ConfigDir: dir, AudioDevice: "alsa/dmix:CARD=MAX98357A,DEV=0", DeviceName: "MuPiBox",
		APIPort: 3678, PersistCredentials: true,
	}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "config.yml"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	for _, want := range []string{
		`device_name: "MuPiBox"`,
		`audio_device: "dmix:CARD=MAX98357A,DEV=0"`,
		"zeroconf_enabled: true",
		"persist_credentials: true",
		"port: 3678",
		"address: 127.0.0.1",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("config.yml missing %q, got:\n%s", want, content)
		}
	}
}

func TestWriteConfigIsIdempotentAndRewritesOnDeviceChange(t *testing.T) {
	dir := t.TempDir()
	base := Config{ConfigDir: dir, DeviceName: "MuPiBox", APIPort: 3678}
	base.AudioDevice = "alsa/dmix:CARD=MAX98357A,DEV=0"
	if err := WriteConfig(base); err != nil {
		t.Fatal(err)
	}
	base.AudioDevice = "alsa/plughw:CARD=MAX98357A,DEV=0"
	if err := WriteConfig(base); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "config.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `audio_device: "plughw:CARD=MAX98357A,DEV=0"`) {
		t.Fatalf("rewrite did not pick up the new device: %s", data)
	}
}

func TestWriteConfigRequiresConfigDir(t *testing.T) {
	if err := WriteConfig(Config{}); err == nil {
		t.Fatal("expected an error without a config dir")
	}
}

func TestALSADeviceFromMPV(t *testing.T) {
	cases := map[string]string{
		"":                                 "default",
		"auto":                             "default",
		"alsa/dmix:CARD=MAX98357A,DEV=0":   "dmix:CARD=MAX98357A,DEV=0",
		"alsa/plughw:CARD=MAX98357A,DEV=0": "plughw:CARD=MAX98357A,DEV=0",
		"dmix:CARD=MAX98357A,DEV=0":        "dmix:CARD=MAX98357A,DEV=0",
	}
	for in, want := range cases {
		if got := ALSADeviceFromMPV(in); got != want {
			t.Errorf("ALSADeviceFromMPV(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestWriteConfigQuotesDeviceNameSafely(t *testing.T) {
	dir := t.TempDir()
	if err := WriteConfig(Config{ConfigDir: dir, DeviceName: `Kids" Room\`, APIPort: 3678}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "config.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `device_name: "Kids\" Room\\"`) {
		t.Fatalf("expected escaped quotes/backslash in device_name, got:\n%s", data)
	}
}

// TestWriteConfigEscapesEmbeddedNewline covers a real finding from Local-AI
// review: a raw newline inside a double-quoted YAML scalar is legal YAML
// (folded to a space) and could let a crafted device name span onto a new
// line, potentially injecting additional YAML keys.
func TestWriteConfigEscapesEmbeddedNewline(t *testing.T) {
	dir := t.TempDir()
	if err := WriteConfig(Config{ConfigDir: dir, DeviceName: "Kids\nzeroconf_enabled: false", APIPort: 3678}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "config.yml"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if strings.Contains(content, "Kids\nzeroconf_enabled: false") {
		t.Fatalf("raw newline in device_name was not escaped, config.yml is injectable:\n%s", content)
	}
	if !strings.Contains(content, `Kids\nzeroconf_enabled: false"`) {
		t.Fatalf("expected the newline escaped as \\n within the quoted scalar, got:\n%s", content)
	}
}
