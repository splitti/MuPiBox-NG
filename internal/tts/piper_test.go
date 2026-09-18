package tts

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mupibox/internal/store"
)

// writeFakePiper installs a tiny shell script standing in for the real
// Piper CLI, so these tests exercise argument construction, stdin/stdout
// wiring and error propagation without needing Piper actually installed
// (not available on the amd64 dev LXC).
func writeFakePiper(t *testing.T, script string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fake-piper")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+script), 0700); err != nil {
		t.Fatal(err)
	}
	return path
}

type recordingLimiter struct {
	pid     int
	percent int
	called  bool
}

func (r *recordingLimiter) Prepare(pid int, percent int) error {
	r.pid, r.percent, r.called = pid, percent, true
	return nil
}

func TestPiperEngineDetectsVersionAndRenders(t *testing.T) {
	bin := writeFakePiper(t, `
if [ "$1" = "--version" ]; then echo "piper 1.8.0"; exit 0; fi
out=""
while [ $# -gt 0 ]; do
  case "$1" in
    --output_file) out="$2"; shift 2;;
    *) shift;;
  esac
done
cat > "$out"
`)
	limiter := &recordingLimiter{}
	engine, err := NewPiperEngine(bin, limiter, nil)
	if err != nil {
		t.Fatal(err)
	}
	if engine.Version() != "1.8.0" {
		t.Fatalf("expected detected version 1.8.0, got %q", engine.Version())
	}

	modelDir := t.TempDir()
	modelPath := filepath.Join(modelDir, "voice.onnx")
	if err := os.WriteFile(modelPath, []byte("model"), 0600); err != nil {
		t.Fatal(err)
	}
	voice := store.TTSVoice{ID: "v1", ModelPath: modelPath, ConfigPath: modelPath + ".json"}
	out := filepath.Join(t.TempDir(), "out.wav")
	if err := engine.Synthesize(context.Background(), voice, "Hallo Welt", out, 30); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(out)
	if err != nil || string(content) != "Hallo Welt" {
		t.Fatalf("expected rendered file to contain the input text: %q err=%v", content, err)
	}
	if !limiter.called || limiter.percent != 30 {
		t.Fatalf("expected CPU limiter to be applied with 30%%: %#v", limiter)
	}
}

func TestPiperEngineMissingVoiceModelFailsFast(t *testing.T) {
	bin := writeFakePiper(t, `exit 0`)
	engine, err := NewPiperEngine(bin, &recordingLimiter{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	voice := store.TTSVoice{ID: "v1", ModelPath: filepath.Join(t.TempDir(), "missing.onnx")}
	if err := engine.Synthesize(context.Background(), voice, "text", filepath.Join(t.TempDir(), "out.wav"), 30); err == nil {
		t.Fatal("expected an error for a missing voice model file")
	}
}

func TestPiperEngineSurfacesStderrOnFailure(t *testing.T) {
	bin := writeFakePiper(t, `
if [ "$1" = "--version" ]; then echo "piper 1.8.0"; exit 0; fi
echo "boom: bad input" >&2
exit 1
`)
	engine, err := NewPiperEngine(bin, &recordingLimiter{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	modelPath := filepath.Join(t.TempDir(), "voice.onnx")
	if err := os.WriteFile(modelPath, []byte("model"), 0600); err != nil {
		t.Fatal(err)
	}
	voice := store.TTSVoice{ID: "v1", ModelPath: modelPath}
	err = engine.Synthesize(context.Background(), voice, "text", filepath.Join(t.TempDir(), "out.wav"), 30)
	if err == nil {
		t.Fatal("expected an error")
	}
	if got := err.Error(); !strings.Contains(got, "boom: bad input") {
		t.Fatalf("expected error to include piper's stderr, got %q", got)
	}
}
