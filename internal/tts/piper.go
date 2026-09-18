package tts

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"mupibox/internal/store"
)

// PiperEngine renders text with Piper as a subprocess, one process per job
// (no long-running server, no shell). As of the OHF-Voice/piper1-gpl
// project (rhasspy/piper was archived 2025-10-06 and points here), Piper
// ships only as the "piper-tts" PyPI package -- there is no standalone
// binary release anymore -- so it is invoked as a Python module:
// "<python> -m piper ...". pythonBin should be the interpreter of a
// dedicated venv with piper-tts installed (see
// scripts/install-piper-engine.sh), not the system python3.
//
// Licensing: piper1-gpl is GPL-3.0. MuPiBox-NG only ever invokes it as an
// external subprocess (argv + stdin + files, no linking against libpiper),
// the same arm's-length relationship the project already has with mpv
// (also GPL) -- this does not extend GPL's copyleft to MuPiBox-NG's own
// code. Per-voice licenses (CC0, CC BY 4.0, ...) remain tracked separately
// in internal/tts/manifest and are unrelated to the engine's own license.
//
// CLI flags (-m/--model, -c/--config, -f/--output_file, -s/--speaker,
// stdin text input) are unchanged from the old rhasspy/piper CLI, and the
// voice model format/source (huggingface.co/rhasspy/piper-voices,
// .onnx+.onnx.json) is unchanged too -- only the invocation method and
// --version (removed; see detectPiperVersion) changed.
type PiperEngine struct {
	pythonBin string
	limiter   CPULimiter
	log       *slog.Logger
	version   string
}

// NewPiperEngine locates the Python interpreter that has piper-tts
// installed and probes its version once (used as part of the cache key,
// see TextHash) instead of on every job.
func NewPiperEngine(pythonBin string, limiter CPULimiter, log *slog.Logger) (*PiperEngine, error) {
	if strings.TrimSpace(pythonBin) == "" {
		pythonBin = "python3"
	}
	if limiter == nil {
		limiter = NewNicePriorityLimiter(log)
	}
	if log == nil {
		log = slog.Default()
	}
	e := &PiperEngine{pythonBin: pythonBin, limiter: limiter, log: log}
	version, err := detectPiperVersion(pythonBin)
	if err != nil {
		return nil, fmt.Errorf("detect piper-tts version: %w", err)
	}
	e.version = version
	return e, nil
}

// detectPiperVersion asks Python's own package metadata for the installed
// piper-tts version. piper1-gpl's CLI has no --version flag (unlike the
// old rhasspy/piper binary), so this is the reliable way to identify it.
func detectPiperVersion(pythonBin string) (string, error) {
	out, err := exec.Command(pythonBin, "-c", "import importlib.metadata as m,sys; sys.stdout.write(m.version('piper-tts'))").Output()
	if err != nil {
		return "", fmt.Errorf("run %s: %w", pythonBin, err)
	}
	version := strings.TrimSpace(string(out))
	if version == "" {
		return "unknown", nil
	}
	return version, nil
}

func (p *PiperEngine) Name() string    { return "piper" }
func (p *PiperEngine) Version() string { return p.version }

func (p *PiperEngine) Synthesize(ctx context.Context, voice store.TTSVoice, text string, outPath string, backgroundCPUPercent int) error {
	if _, err := os.Stat(voice.ModelPath); err != nil {
		return fmt.Errorf("voice model %s is missing or unreadable: %w", voice.ModelPath, err)
	}
	args := []string{"-m", "piper", "--model", voice.ModelPath, "--output_file", outPath}
	if strings.TrimSpace(voice.ConfigPath) != "" {
		args = append(args, "--config", voice.ConfigPath)
	}
	if voice.SpeakerID != nil {
		args = append(args, "--speaker", strconv.Itoa(*voice.SpeakerID))
	}
	cmd := exec.CommandContext(ctx, p.pythonBin, args...)
	cmd.Stdin = strings.NewReader(text)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start piper: %w", err)
	}
	if cmd.Process != nil {
		if err := p.limiter.Prepare(cmd.Process.Pid, backgroundCPUPercent); err != nil {
			// Degrade gracefully: an unrestricted render still finishes
			// correctly, it just risks contending with mpv/UI for CPU.
			p.log.Warn("tts cpu limiting failed for job, continuing unrestricted", "error", err)
		}
	}
	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("piper failed: %w (stderr: %s)", err, strings.TrimSpace(stderr.String()))
	}
	return nil
}
