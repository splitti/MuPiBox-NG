// Package tts turns configured or requested text into cached speech audio.
//
// A single global (language, voice) pair is active at any time (see
// docs/tts.md). Switching either one starts a new cache generation
// (internal/store's building/active/stale/purging state machine) that is
// populated in the background without ever blocking playback or the UI;
// EnsureText is the one entry point other components (categories, media,
// future providers) use to make sure a piece of text will have cached
// audio, without needing to know anything about generations or the queue.
package tts

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"mupibox/internal/store"
)

type Priority string

const (
	PriorityHigh   Priority = Priority(store.JobPriorityHigh)
	PriorityNormal Priority = Priority(store.JobPriorityNormal)
	PriorityLow    Priority = Priority(store.JobPriorityLow)
)

// Registrar is the public seam other components use to make sure text has
// (or will soon have) cached speech audio. Implementations must be safe to
// call from any goroutine and must never block the caller for background
// (normal/low) priority work.
type Registrar interface {
	EnsureText(ctx context.Context, sourceType, sourceRef, text string, priority Priority) error
}

// Engine renders one piece of text to a WAV file using a concrete voice
// model. Implementations run the actual synthesis subprocess (Piper), are
// responsible for keeping at most one model loaded at a time, and apply
// their own CPULimiter (if any) to the subprocess they start.
type Engine interface {
	Name() string
	Version() string
	Synthesize(ctx context.Context, voice store.TTSVoice, text string, outPath string, backgroundCPUPercent int) error
}

// CPULimiter constrains the resource footprint of one synthesis subprocess
// so a long render cannot starve UI/playback. See NewCgroupCPULimiter and
// NewNicePriorityLimiter.
type CPULimiter interface {
	// Prepare is called by the Engine right after its subprocess has
	// started (pid>0), before waiting for it to finish.
	Prepare(pid int, percent int) error
}

var whitespace = regexp.MustCompile(`\s+`)

// NormalizeText collapses whitespace so that trivially different renderings
// of the same text share a cache entry.
func NormalizeText(text string) string {
	return strings.TrimSpace(whitespace.ReplaceAllString(text, " "))
}

// TextHash is the cache key for one generation: engine + engine version +
// language + voice + voice settings are fixed per generation, so only the
// normalized text needs to be hashed per call.
func TextHash(engine, engineVersion, language, voiceID, voiceSettings, normalizedText string) string {
	h := sha256.New()
	for _, part := range []string{engine, engineVersion, language, voiceID, voiceSettings, normalizedText} {
		h.Write([]byte(part))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

// Config is the user-facing, desired TTS configuration (Admin > Text-to-Speech).
type Config struct {
	Enabled              bool
	Language             string
	VoiceID              string
	BackgroundCPUPercent int
}

// SystemTexts supplies the fixed, high-priority system/UI strings that must
// be ready first after any language switch. Implementations are expected to
// return quickly (no I/O beyond an in-memory/embedded lookup).
type SystemTexts interface {
	ForLanguage(language string) map[string]string
}

type Manager struct {
	store       *store.Store
	engine      Engine
	cacheRoot   string
	systemTexts SystemTexts
	log         *slog.Logger

	mu       sync.Mutex
	waiters  map[waiterKey][]chan struct{}
	stopWork chan struct{}
	wake     chan struct{}
	wg       sync.WaitGroup
	running  bool
	lastErr  string
}

type waiterKey struct {
	generationID int64
	textHash     string
}

func NewManager(st *store.Store, engine Engine, cacheRoot string, systemTexts SystemTexts, log *slog.Logger) *Manager {
	if systemTexts == nil {
		systemTexts = staticSystemTexts{}
	}
	if log == nil {
		log = slog.Default()
	}
	return &Manager{
		store: st, engine: engine, cacheRoot: cacheRoot, systemTexts: systemTexts, log: log,
		waiters: map[waiterKey][]chan struct{}{},
	}
}

type staticSystemTexts struct{}

func (staticSystemTexts) ForLanguage(string) map[string]string { return nil }

func (m *Manager) generationDir(id int64) string {
	return filepath.Join(m.cacheRoot, "gen-"+strconv.FormatInt(id, 10))
}

// Recover must be called once at startup, before Start, to bring a possibly
// crash-interrupted state back into a well-defined one: stuck "running"
// jobs are requeued, any generation left dangling by a partial switch is
// marked stale, and stale/purging generations are cleaned up.
func (m *Manager) Recover() error {
	if err := m.store.ResetRunningTTSJobs(); err != nil {
		return fmt.Errorf("reset running tts jobs: %w", err)
	}
	if err := m.store.MarkOrphanTTSGenerationsStale(); err != nil {
		return fmt.Errorf("mark orphan tts generations stale: %w", err)
	}
	return m.purgeStaleGenerations()
}

// PurgeNow runs the same stale/purging cleanup sweep the idle worker loop
// performs periodically, but immediately. Used by the admin "clean up now"
// action; safe to call at any time, including while the worker is stopped.
func (m *Manager) PurgeNow() error {
	return m.purgeStaleGenerations()
}

func (m *Manager) purgeStaleGenerations() error {
	for _, status := range []string{store.GenerationStale, store.GenerationPurging} {
		gens, err := m.store.ListTTSGenerationsByStatus(status)
		if err != nil {
			return err
		}
		for _, g := range gens {
			if status == store.GenerationStale {
				if err := m.store.MarkTTSGenerationPurging(g.ID); err != nil {
					return err
				}
			}
			if err := os.RemoveAll(m.generationDir(g.ID)); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("remove generation directory: %w", err)
			}
			if err := m.store.DeleteTTSGeneration(g.ID); err != nil {
				return err
			}
		}
	}
	return nil
}

// SwitchLanguage starts a new cache generation for the given configuration.
// It always succeeds structurally (the DB transaction either commits fully
// or not at all); the previous generation is left on disk and cleaned up
// asynchronously by the worker loop, never synchronously here.
func (m *Manager) SwitchLanguage(ctx context.Context, cfg Config) (store.TTSGeneration, error) {
	voice, ok, err := m.store.GetTTSVoice(cfg.VoiceID)
	if err != nil {
		return store.TTSGeneration{}, err
	}
	if !ok || !voice.Available || voice.Language != cfg.Language {
		return store.TTSGeneration{}, fmt.Errorf("voice %q is not available for language %q", cfg.VoiceID, cfg.Language)
	}
	voiceSettings := "{}"
	seeds := make([]store.TTSJobSeed, 0)
	for key, text := range m.systemTexts.ForLanguage(cfg.Language) {
		normalized := NormalizeText(text)
		seeds = append(seeds, store.TTSJobSeed{
			Priority: string(PriorityHigh), SourceType: "system", SourceRef: key,
			NormalizedText: normalized,
			TextHash:       TextHash(m.engine.Name(), m.engine.Version(), cfg.Language, voice.ID, voiceSettings, normalized),
		})
	}
	gen, err := m.store.SwitchTTSGeneration(cfg.Language, voice.ID, m.engine.Name(), m.engine.Version(), voiceSettings, cfg.BackgroundCPUPercent, seeds)
	if err != nil {
		return store.TTSGeneration{}, err
	}
	if err := os.MkdirAll(m.generationDir(gen.ID), 0750); err != nil {
		return store.TTSGeneration{}, fmt.Errorf("create generation directory: %w", err)
	}
	m.poke()
	// The generation that just became stale (if any) is picked up by the
	// worker loop's idle purge sweep; nothing to purge synchronously here.
	return gen, nil
}

// EnsureText registers a piece of text for synthesis against the current
// generation. HIGH priority additionally waits (bounded by ctx) for a
// cache hit, matching the "click => usually a cache hit, otherwise
// immediate HIGH fallback" contract; NORMAL/LOW return as soon as the job
// is queued and never block the caller.
func (m *Manager) EnsureText(ctx context.Context, sourceType, sourceRef, text string, priority Priority) error {
	gen, ok, err := m.store.CurrentTTSGeneration()
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("tts is not configured yet")
	}
	normalized := NormalizeText(text)
	if normalized == "" {
		return nil
	}
	voice, ok, err := m.store.GetTTSVoice(gen.VoiceID)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("current tts voice %q is missing", gen.VoiceID)
	}
	hash := TextHash(gen.Engine, gen.EngineVersion, gen.Language, voice.ID, gen.VoiceSettings, normalized)

	if _, err := os.Stat(m.cacheFilePath(gen.ID, hash)); err == nil {
		return nil // cache hit, nothing to do
	}

	if priority == PriorityHigh {
		if err := m.store.PromoteTTSJob(gen.ID, hash); err != nil {
			return err
		}
	}
	if err := m.store.EnqueueTTSJob(gen.ID, store.TTSJobSeed{
		Priority: string(priority), SourceType: sourceType, SourceRef: sourceRef, NormalizedText: normalized, TextHash: hash,
	}); err != nil {
		return err
	}
	m.poke()
	if priority != PriorityHigh {
		return nil
	}
	return m.waitForCacheHit(ctx, gen.ID, hash)
}

func (m *Manager) waitForCacheHit(ctx context.Context, generationID int64, hash string) error {
	key := waiterKey{generationID, hash}
	ch := make(chan struct{}, 1)
	m.mu.Lock()
	m.waiters[key] = append(m.waiters[key], ch)
	m.mu.Unlock()
	defer func() {
		m.mu.Lock()
		list := m.waiters[key]
		for i, c := range list {
			if c == ch {
				m.waiters[key] = append(list[:i], list[i+1:]...)
				break
			}
		}
		m.mu.Unlock()
	}()
	select {
	case <-ch:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (m *Manager) notifyWaiters(generationID int64, hash string) {
	key := waiterKey{generationID, hash}
	m.mu.Lock()
	list := m.waiters[key]
	delete(m.waiters, key)
	m.mu.Unlock()
	for _, ch := range list {
		close(ch)
	}
}

func (m *Manager) cacheFilePath(generationID int64, hash string) string {
	return filepath.Join(m.generationDir(generationID), hash+".wav")
}

func (m *Manager) poke() {
	select {
	case m.wake <- struct{}{}:
	default:
	}
}

// Start launches the single background worker goroutine. Concurrency is
// intentionally fixed at one: it is sufficient on Pi 3/4, and it avoids any
// chance of two synthesis jobs competing with mpv/UI for CPU at once.
// Safe to call from any goroutine (e.g. an admin HTTP handler toggling TTS
// on) and safe to call again while already running, which is then a no-op.
func (m *Manager) Start() {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return
	}
	m.running = true
	m.stopWork = make(chan struct{})
	m.wake = make(chan struct{}, 1)
	m.mu.Unlock()
	m.wg.Add(1)
	go m.runWorker()
}

// Stop halts the worker and waits for the in-flight job (if any) to finish.
// Safe to call when not running (no-op) and safe to call from any goroutine.
func (m *Manager) Stop() {
	m.mu.Lock()
	if !m.running {
		m.mu.Unlock()
		return
	}
	m.running = false
	stopWork := m.stopWork
	m.mu.Unlock()
	close(stopWork)
	m.wg.Wait()
}

// Running reports whether the background worker goroutine is active.
func (m *Manager) Running() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.running
}

// LastError returns the most recent worker-loop error message, or "" if
// none occurred since startup. Meant for admin status display, not control
// flow: EnsureText/SwitchLanguage report their own errors directly.
func (m *Manager) LastError() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.lastErr
}

func (m *Manager) setLastError(err error) {
	m.mu.Lock()
	if err != nil {
		m.lastErr = err.Error()
	}
	m.mu.Unlock()
}

// EngineName and EngineVersion identify the synthesis engine for admin
// status display; they do not require the worker to be running.
func (m *Manager) EngineName() string    { return m.engine.Name() }
func (m *Manager) EngineVersion() string { return m.engine.Version() }

// CacheStats reports the number of rendered cache files and their total
// size for the given generation, read directly from its directory (the
// same directory EnsureText checks for cache hits) rather than a separate
// bookkeeping table.
func (m *Manager) CacheStats(generationID int64) (count int, sizeBytes int64, err error) {
	entries, err := os.ReadDir(m.generationDir(generationID))
	if err != nil {
		if os.IsNotExist(err) {
			return 0, 0, nil
		}
		return 0, 0, err
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".wav") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		count++
		sizeBytes += info.Size()
	}
	return count, sizeBytes, nil
}

// ResolveSpeech is the on-demand playback path for the touch UI (see
// docs/tts.md "USER REQUEST" flow): it ensures cached audio exists for text
// with HIGH priority (waiting, bounded by ctx, exactly like EnsureText) and
// then returns the path to that now-guaranteed-to-exist cache file so the
// caller can play it. Unlike RenderTestClip this uses the real production
// cache (the file is meant to stay and be reused), not a scratch directory.
func (m *Manager) ResolveSpeech(ctx context.Context, sourceType, sourceRef, text string) (path string, err error) {
	if err := m.EnsureText(ctx, sourceType, sourceRef, text, PriorityHigh); err != nil {
		return "", err
	}
	gen, ok, err := m.store.CurrentTTSGeneration()
	if err != nil {
		return "", err
	}
	if !ok {
		return "", errors.New("tts is not configured")
	}
	voice, ok, err := m.store.GetTTSVoice(gen.VoiceID)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", fmt.Errorf("current tts voice %q is missing", gen.VoiceID)
	}
	normalized := NormalizeText(text)
	hash := TextHash(gen.Engine, gen.EngineVersion, gen.Language, voice.ID, gen.VoiceSettings, normalized)
	return m.cacheFilePath(gen.ID, hash), nil
}

// RenderTestClip synthesizes a one-off preview clip for the admin "test
// voice" action. It intentionally bypasses tts_jobs/tts_cache and the
// generation directories entirely (a dedicated "test" scratch directory
// under the cache root) so previewing a voice never pollutes the
// production cache or competes with its priority queue. The caller is
// responsible for invoking the returned cleanup func once done.
func (m *Manager) RenderTestClip(ctx context.Context, voice store.TTSVoice, text string) (path string, cleanup func(), err error) {
	normalized := NormalizeText(text)
	if normalized == "" {
		return "", nil, errors.New("text is required")
	}
	dir := filepath.Join(m.cacheRoot, "test")
	if err := os.MkdirAll(dir, 0750); err != nil {
		return "", nil, fmt.Errorf("create test cache directory: %w", err)
	}
	hash := TextHash("test", "test", voice.Language, voice.ID, "{}", normalized)
	out := filepath.Join(dir, hash+".wav")
	if err := m.engine.Synthesize(ctx, voice, normalized, out, 100); err != nil {
		_ = os.Remove(out)
		return "", nil, err
	}
	return out, func() { _ = os.Remove(out) }, nil
}

func (m *Manager) runWorker() {
	defer m.wg.Done()
	idle := time.NewTicker(5 * time.Second)
	defer idle.Stop()
	for {
		processed, err := m.processOneJob()
		if err != nil {
			m.log.Error("tts job failed", "error", err)
			m.setLastError(err)
		}
		if processed {
			continue // keep draining without waiting on the ticker
		}
		if err := m.purgeStaleGenerations(); err != nil {
			m.log.Error("tts purge failed", "error", err)
		}
		select {
		case <-m.stopWork:
			return
		case <-m.wake:
		case <-idle.C:
		}
	}
}

// processOneJob dequeues and renders a single job for the current
// generation, if any is pending. It returns processed=false when there is
// nothing to do right now (idle), so the caller can wait instead of
// busy-looping.
func (m *Manager) processOneJob() (processed bool, err error) {
	gen, ok, err := m.store.CurrentTTSGeneration()
	if err != nil || !ok {
		return false, err
	}
	job, ok, err := m.store.DequeueNextTTSJob(gen.ID)
	if err != nil {
		return false, err
	}
	if !ok {
		if _, err := m.store.ActivateTTSGenerationIfReady(gen.ID); err != nil {
			m.log.Error("tts activation check failed", "error", err)
		}
		return false, nil
	}
	voice, ok, err := m.store.GetTTSVoice(gen.VoiceID)
	if err != nil {
		return true, err
	}
	if !ok || !voice.Available {
		_ = m.store.FailTTSJob(job.ID, "voice unavailable")
		return true, fmt.Errorf("voice %q unavailable while rendering job %d", gen.VoiceID, job.ID)
	}
	out := m.cacheFilePath(gen.ID, job.TextHash)
	tmp := out + ".tmp"
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	renderErr := m.engine.Synthesize(ctx, voice, job.NormalizedText, tmp, gen.BackgroundCPUPercent)
	if renderErr != nil {
		_ = os.Remove(tmp)
		_ = m.store.FailTTSJob(job.ID, renderErr.Error())
		return true, fmt.Errorf("synthesize job %d: %w", job.ID, renderErr)
	}
	if err := os.Rename(tmp, out); err != nil {
		_ = os.Remove(tmp)
		_ = m.store.FailTTSJob(job.ID, err.Error())
		return true, fmt.Errorf("finalize cache file for job %d: %w", job.ID, err)
	}
	if err := m.store.CompleteTTSJob(job.ID); err != nil {
		return true, err
	}
	m.notifyWaiters(gen.ID, job.TextHash)
	if _, err := m.store.ActivateTTSGenerationIfReady(gen.ID); err != nil {
		m.log.Error("tts activation check failed", "error", err)
	}
	return true, nil
}
