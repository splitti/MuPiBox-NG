package tts

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"mupibox/internal/store"
)

// PiperEngine renders text with the Piper CLI as a subprocess, one process
// per job (no long-running server, no shell). Flags follow the documented
// Piper CLI (--model/--config/--output_file, text on stdin); this must be
// re-verified against the actually installed Piper version on the Pi
// before relying on it (see docs/tts.md open risks).
type PiperEngine struct {
	binary  string
	limiter CPULimiter
	log     *slog.Logger
	version string
}

// NewPiperEngine locates the Piper binary and probes its version once
// (used as part of the cache key, see TextHash) instead of on every job.
func NewPiperEngine(binary string, limiter CPULimiter, log *slog.Logger) (*PiperEngine, error) {
	if strings.TrimSpace(binary) == "" {
		binary = "piper"
	}
	if limiter == nil {
		limiter = NewNicePriorityLimiter(log)
	}
	if log == nil {
		log = slog.Default()
	}
	e := &PiperEngine{binary: binary, limiter: limiter, log: log}
	version, err := detectPiperVersion(binary)
	if err != nil {
		return nil, fmt.Errorf("detect piper version: %w", err)
	}
	e.version = version
	return e, nil
}

var versionPattern = regexp.MustCompile(`\d+\.\d+(\.\d+)?`)

func detectPiperVersion(binary string) (string, error) {
	out, err := exec.Command(binary, "--version").CombinedOutput()
	if err != nil {
		if _, ok := err.(*exec.ExitError); !ok {
			return "", fmt.Errorf("run %s --version: %w", binary, err)
		}
	}
	if match := versionPattern.FindString(string(out)); match != "" {
		return match, nil
	}
	return "unknown", nil
}

func (p *PiperEngine) Name() string    { return "piper" }
func (p *PiperEngine) Version() string { return p.version }

func (p *PiperEngine) Synthesize(ctx context.Context, voice store.TTSVoice, text string, outPath string, backgroundCPUPercent int) error {
	if _, err := os.Stat(voice.ModelPath); err != nil {
		return fmt.Errorf("voice model %s is missing or unreadable: %w", voice.ModelPath, err)
	}
	args := []string{"--model", voice.ModelPath, "--output_file", outPath}
	if strings.TrimSpace(voice.ConfigPath) != "" {
		args = append(args, "--config", voice.ConfigPath)
	}
	if voice.SpeakerID != nil {
		args = append(args, "--speaker", strconv.Itoa(*voice.SpeakerID))
	}
	cmd := exec.CommandContext(ctx, p.binary, args...)
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
