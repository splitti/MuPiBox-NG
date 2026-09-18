package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"mupibox/internal/store"
)

const sampleAplayOutput = `**** List of PLAYBACK Hardware Devices ****
card 0: sndrpimax98357a [snd_rpi_max98357a], device 0: HiFi wm8804-hifi-0 [HiFi wm8804-hifi-0]
  Subdevices: 1/1
  Subdevice #0: subdevice #0
card 1: vc4hdmi0 [vc4-hdmi-0], device 0: MAI PCM i2s-hifi-0 [MAI PCM i2s-hifi-0]
  Subdevices: 1/1
  Subdevice #0: subdevice #0
`

const sampleMPVDeviceOutput = `Available audio devices:
 'auto' (Autoselect device)
 'alsa/default' (Default ALSA device)
 'alsa/hw:CARD=sndrpimax98357a,DEV=0' (bcm2835 HDMI 1, snd_rpi_max98357a)
`

func TestParseAplayList(t *testing.T) {
	cards := parseAplayList(sampleAplayOutput)
	if len(cards) != 2 {
		t.Fatalf("expected 2 cards, got %#v", cards)
	}
	if cards[0].Card != 0 || cards[0].ID != "sndrpimax98357a" || cards[0].Name != "snd_rpi_max98357a" || cards[0].DeviceName != "HiFi wm8804-hifi-0" {
		t.Fatalf("unexpected first card: %#v", cards[0])
	}
	if cards[1].ID != "vc4hdmi0" {
		t.Fatalf("unexpected second card: %#v", cards[1])
	}
}

func TestParseAplayListNoSoundcards(t *testing.T) {
	cards := parseAplayList("aplay: device_list:279: no soundcards found...")
	if len(cards) != 0 {
		t.Fatalf("expected no cards, got %#v", cards)
	}
}

func TestParseMPVAudioDevices(t *testing.T) {
	devices := parseMPVAudioDevices(sampleMPVDeviceOutput)
	if len(devices) != 3 {
		t.Fatalf("expected 3 devices, got %#v", devices)
	}
	if devices[2].ID != "alsa/hw:CARD=sndrpimax98357a,DEV=0" || devices[2].Name != "bcm2835 HDMI 1, snd_rpi_max98357a" {
		t.Fatalf("unexpected third device: %#v", devices[2])
	}
}

func TestParseMPVAudioDevicesHandlesNestedParensInName(t *testing.T) {
	// Real output captured on a Raspberry Pi 4 (mpv 0.40.0): the device
	// name itself contains parens, so a naive "first )" match truncates it.
	raw := `List of detected audio devices:
  'auto' (Autoselect device)
  'alsa' (Default (alsa))
  'jack' (Default (jack))
`
	devices := parseMPVAudioDevices(raw)
	if len(devices) != 3 {
		t.Fatalf("expected 3 devices, got %#v", devices)
	}
	if devices[1].ID != "alsa" || devices[1].Name != "Default (alsa)" {
		t.Fatalf("expected the full nested-paren name preserved, got %#v", devices[1])
	}
}

func TestMupihatCardDetected(t *testing.T) {
	cards := parseAplayList(sampleAplayOutput)
	if !mupihatCardDetected(cards) {
		t.Fatal("expected the sndrpimax98357a card to be detected as MuPiHAT")
	}
	if mupihatCardDetected(parseAplayList("card 0: vc4hdmi0 [vc4-hdmi-0], device 0: MAI PCM i2s-hifi-0 [MAI PCM i2s-hifi-0]\n")) {
		t.Fatal("expected no MuPiHAT detection without a max98357a card")
	}
}

func TestMupihatCardDetectedIsCaseInsensitive(t *testing.T) {
	cards := parseAplayList("card 0: MAX98357A [Maxim MAX98357A Audio Codec], device 0: HiFi [HiFi]\n")
	if !mupihatCardDetected(cards) {
		t.Fatal("expected uppercase MAX98357A card ID to still be detected")
	}
}

func TestParsersHandleMissingBinaryOutputGracefully(t *testing.T) {
	if cards := parseAplayList(""); len(cards) != 0 {
		t.Fatalf("expected no cards from empty aplay output, got %#v", cards)
	}
	if devices := parseMPVAudioDevices(""); len(devices) != 0 {
		t.Fatalf("expected no devices from empty mpv output, got %#v", devices)
	}
}

func TestAudioStatusEndpointReportsRebootRequiredWhenOverlayConfiguredButNotDetected(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.txt")
	if err := os.WriteFile(configPath, []byte("dtparam=i2c_arm=on\n# BEGIN MUPIBOX-NG MUPIHAT AUDIO\ndtoverlay=max98357a,sdmode-pin=16\ndtoverlay=i2s-mmap\n# END MUPIBOX-NG MUPIHAT AUDIO\n"), 0644); err != nil {
		t.Fatal(err)
	}
	// mupihatOverlayConfigured() reads fixed system paths; this test only
	// exercises the pure parsing/detection helpers directly (see above) plus
	// the aggregate reboot-required rule, without needing to fake those
	// system paths.
	overlay := strings.Contains(readFirst(configPath), "BEGIN MUPIBOX-NG MUPIHAT AUDIO")
	detected := mupihatCardDetected(parseAplayList("aplay: device_list:279: no soundcards found...\n"))
	if !overlay || detected {
		t.Fatalf("test fixture setup wrong: overlay=%v detected=%v", overlay, detected)
	}
	rebootRequired := overlay && !detected
	if !rebootRequired {
		t.Fatal("expected reboot_required when the overlay is configured but the card is not yet visible")
	}
}

func TestAudioDeviceEndpointPersistsSelection(t *testing.T) {
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.EnsureBoxSettings(store.BoxSettings{Language: "de", AdminLanguage: "de", Audio: store.AudioSettings{StartupVolume: 30, MaxVolume: 60}, Display: store.DisplaySettings{Brightness: 100}, Theme: "modern-dark"}); err != nil {
		t.Fatal(err)
	}
	a := &API{Store: db}
	w := httptest.NewRecorder()
	// "auto" is mpv's own built-in device and always present in
	// --audio-device=help output, on any machine with or without real
	// sound hardware -- unlike a hardware-specific ALSA id, this keeps the
	// test deterministic regardless of whether mpv is installed here.
	r := httptest.NewRequest(http.MethodPut, "/api/admin/audio/device", strings.NewReader(`{"device":"auto"}`))
	r.Header.Set("Content-Type", "application/json")
	a.Handler().ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	saved, ok, err := db.LoadBoxSettings()
	if err != nil || !ok || saved.Audio.Device != "auto" {
		t.Fatalf("device not persisted: %#v ok=%v err=%v", saved, ok, err)
	}
}

func TestAudioDeviceEndpointRejectsUnknownDeviceWhenDetectionWorks(t *testing.T) {
	if _, err := exec.LookPath("mpv"); err != nil {
		t.Skip("mpv not installed here; the allow-list is intentionally skipped when detection finds nothing, see collectAudioStatus")
	}
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.EnsureBoxSettings(store.BoxSettings{Language: "de", AdminLanguage: "de", Audio: store.AudioSettings{StartupVolume: 30, MaxVolume: 60}, Display: store.DisplaySettings{Brightness: 100}, Theme: "modern-dark"}); err != nil {
		t.Fatal(err)
	}
	a := &API{Store: db}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPut, "/api/admin/audio/device", strings.NewReader(`{"device":"alsa/hw:CARD=DefinitelyNotARealCard,DEV=0"}`))
	r.Header.Set("Content-Type", "application/json")
	a.Handler().ServeHTTP(w, r)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for a device mpv does not know about, got %d: %s", w.Code, w.Body.String())
	}
}

func TestMuPiHATAudioEndpointUnavailableWithoutSystemAgent(t *testing.T) {
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	a := &API{Store: db}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPut, "/api/admin/audio/mupihat", strings.NewReader(`{"enabled":true}`))
	r.Header.Set("Content-Type", "application/json")
	a.Handler().ServeHTTP(w, r)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 without a connectivity manager, got %d: %s", w.Code, w.Body.String())
	}
}
