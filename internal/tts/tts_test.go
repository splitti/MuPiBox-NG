package tts

import (
	"context"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"mupibox/internal/store"
)

type fakeEngine struct {
	calls int32
	delay time.Duration
	fail  bool
}

func (e *fakeEngine) Name() string    { return "fake" }
func (e *fakeEngine) Version() string { return "1.0" }
func (e *fakeEngine) Synthesize(ctx context.Context, voice store.TTSVoice, text string, outPath string, backgroundCPUPercent int) error {
	atomic.AddInt32(&e.calls, 1)
	if e.delay > 0 {
		time.Sleep(e.delay)
	}
	if e.fail {
		return context.DeadlineExceeded
	}
	return os.WriteFile(outPath, []byte("wav:"+text), 0600)
}

type mapSystemTexts map[string]map[string]string

func (m mapSystemTexts) ForLanguage(lang string) map[string]string { return m[lang] }

func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func mustUpsertVoice(t *testing.T, s *store.Store, id, language string) {
	t.Helper()
	if err := s.UpsertTTSVoice(store.TTSVoice{
		ID: id, Engine: "fake", Language: language, VoiceFamily: "test", Quality: "medium",
		DisplayName: id, ModelPath: "/models/" + id + ".onnx", ConfigPath: "/models/" + id + ".onnx.json",
		License: "CC0", SourceURL: "https://example.invalid", Available: true,
	}); err != nil {
		t.Fatal(err)
	}
}

func waitUntil(t *testing.T, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("condition not met before timeout")
}

func TestSwitchLanguageBuildsAndActivatesGeneration(t *testing.T) {
	s := newTestStore(t)
	mustUpsertVoice(t, s, "de-voice", "de-DE")
	engine := &fakeEngine{}
	texts := mapSystemTexts{"de-DE": {"welcome": "Willkommen", "menu": "Menü"}}
	m := NewManager(s, engine, t.TempDir(), texts, nil)
	m.Start()
	defer m.Stop()

	gen, err := m.SwitchLanguage(context.Background(), Config{Enabled: true, Language: "de-DE", VoiceID: "de-voice", BackgroundCPUPercent: 30})
	if err != nil {
		t.Fatal(err)
	}

	waitUntil(t, 2*time.Second, func() bool {
		g, _, err := s.GetTTSGeneration(gen.ID)
		return err == nil && g.Status == store.GenerationActive
	})
	if atomic.LoadInt32(&engine.calls) != 2 {
		t.Fatalf("expected 2 system text jobs rendered, got %d", engine.calls)
	}
}

func TestEnsureTextCacheHitAfterBuild(t *testing.T) {
	s := newTestStore(t)
	mustUpsertVoice(t, s, "de-voice", "de-DE")
	engine := &fakeEngine{}
	m := NewManager(s, engine, t.TempDir(), mapSystemTexts{}, nil)
	m.Start()
	defer m.Stop()

	if _, err := m.SwitchLanguage(context.Background(), Config{Enabled: true, Language: "de-DE", VoiceID: "de-voice", BackgroundCPUPercent: 30}); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := m.EnsureText(ctx, "category", "books", "Hörbücher", PriorityHigh); err != nil {
		t.Fatalf("first EnsureText (cache miss -> render) failed: %v", err)
	}
	if atomic.LoadInt32(&engine.calls) != 1 {
		t.Fatalf("expected exactly one render call, got %d", engine.calls)
	}

	// Second call for the same text must be an immediate cache hit: no new render.
	ctx2, cancel2 := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel2()
	if err := m.EnsureText(ctx2, "category", "books", "Hörbücher", PriorityHigh); err != nil {
		t.Fatal(err)
	}
	if atomic.LoadInt32(&engine.calls) != 1 {
		t.Fatalf("expected cache hit to avoid a second render, got %d calls", engine.calls)
	}
}

func TestEnsureTextNormalPriorityDoesNotBlock(t *testing.T) {
	s := newTestStore(t)
	mustUpsertVoice(t, s, "de-voice", "de-DE")
	engine := &fakeEngine{delay: 200 * time.Millisecond}
	m := NewManager(s, engine, t.TempDir(), mapSystemTexts{}, nil)
	m.Start()
	defer m.Stop()

	if _, err := m.SwitchLanguage(context.Background(), Config{Enabled: true, Language: "de-DE", VoiceID: "de-voice", BackgroundCPUPercent: 30}); err != nil {
		t.Fatal(err)
	}

	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := m.EnsureText(ctx, "category", "books", "Hörbücher (normal)", PriorityNormal); err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(start); elapsed > 100*time.Millisecond {
		t.Fatalf("NORMAL priority EnsureText must return immediately without waiting for render, took %v", elapsed)
	}
}

func TestLanguageSwitchMarksPreviousGenerationStaleAndPurgesIt(t *testing.T) {
	s := newTestStore(t)
	mustUpsertVoice(t, s, "de-voice", "de-DE")
	mustUpsertVoice(t, s, "nl-voice", "nl-NL")
	engine := &fakeEngine{}
	root := t.TempDir()
	m := NewManager(s, engine, root, mapSystemTexts{}, nil)
	m.Start()
	defer m.Stop()

	gen1, err := m.SwitchLanguage(context.Background(), Config{Enabled: true, Language: "de-DE", VoiceID: "de-voice", BackgroundCPUPercent: 30})
	if err != nil {
		t.Fatal(err)
	}
	waitUntil(t, 2*time.Second, func() bool {
		g, _, err := s.GetTTSGeneration(gen1.ID)
		return err == nil && g.Status == store.GenerationActive
	})
	dir1 := m.generationDir(gen1.ID)
	if _, err := os.Stat(dir1); err != nil {
		t.Fatalf("expected generation directory to exist: %v", err)
	}

	gen2, err := m.SwitchLanguage(context.Background(), Config{Enabled: true, Language: "nl-NL", VoiceID: "nl-voice", BackgroundCPUPercent: 30})
	if err != nil {
		t.Fatal(err)
	}
	if gen2.ID == gen1.ID {
		t.Fatal("expected a new generation id")
	}

	waitUntil(t, 3*time.Second, func() bool {
		_, ok, err := s.GetTTSGeneration(gen1.ID)
		return err == nil && !ok
	})
	if _, err := os.Stat(dir1); !os.IsNotExist(err) {
		t.Fatalf("expected old generation directory removed, stat err=%v", err)
	}
}

func TestRecoverRequeuesRunningJobsAndCleansOrphans(t *testing.T) {
	s := newTestStore(t)
	mustUpsertVoice(t, s, "de-voice", "de-DE")
	gen, err := s.SwitchTTSGeneration("de-DE", "de-voice", "fake", "1.0", "{}", 30, []store.TTSJobSeed{
		{Priority: store.JobPriorityHigh, SourceType: "system", NormalizedText: "x", TextHash: "h1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.DequeueNextTTSJob(gen.ID); err != nil {
		t.Fatal(err)
	}
	m := NewManager(s, &fakeEngine{}, t.TempDir(), mapSystemTexts{}, nil)
	if err := m.Recover(); err != nil {
		t.Fatal(err)
	}
	counts, err := s.CountTTSJobs(gen.ID)
	if err != nil || counts.HighRunning != 0 || counts.HighPending != 1 {
		t.Fatalf("expected running job requeued by Recover: %#v err=%v", counts, err)
	}
}

func TestStartStopIsIdempotentAndSafeUnderConcurrentToggling(t *testing.T) {
	s := newTestStore(t)
	mustUpsertVoice(t, s, "de-voice", "de-DE")
	m := NewManager(s, &fakeEngine{}, t.TempDir(), mapSystemTexts{}, nil)
	if m.Running() {
		t.Fatal("must not be running before Start")
	}
	m.Start()
	m.Start() // second call must be a harmless no-op, not a second goroutine
	if !m.Running() {
		t.Fatal("expected Running() true after Start")
	}
	m.Stop()
	if m.Running() {
		t.Fatal("expected Running() false after Stop")
	}
	m.Stop() // must not panic/block on an already-stopped manager
	m.Start()
	m.Stop()
}

func TestLastErrorReportsWorkerFailures(t *testing.T) {
	s := newTestStore(t)
	mustUpsertVoice(t, s, "de-voice", "de-DE")
	m := NewManager(s, &fakeEngine{fail: true}, t.TempDir(), mapSystemTexts{"de-DE": {"x": "Text"}}, nil)
	if m.LastError() != "" {
		t.Fatal("expected no error before anything ran")
	}
	m.Start()
	defer m.Stop()
	if _, err := m.SwitchLanguage(context.Background(), Config{Enabled: true, Language: "de-DE", VoiceID: "de-voice", BackgroundCPUPercent: 30}); err != nil {
		t.Fatal(err)
	}
	waitUntil(t, 2*time.Second, func() bool { return m.LastError() != "" })
}

func TestCacheStatsCountsRenderedFiles(t *testing.T) {
	s := newTestStore(t)
	mustUpsertVoice(t, s, "de-voice", "de-DE")
	m := NewManager(s, &fakeEngine{}, t.TempDir(), mapSystemTexts{"de-DE": {"a": "Eins", "b": "Zwei"}}, nil)
	m.Start()
	defer m.Stop()
	gen, err := m.SwitchLanguage(context.Background(), Config{Enabled: true, Language: "de-DE", VoiceID: "de-voice", BackgroundCPUPercent: 30})
	if err != nil {
		t.Fatal(err)
	}
	waitUntil(t, 2*time.Second, func() bool {
		g, _, err := s.GetTTSGeneration(gen.ID)
		return err == nil && g.Status == store.GenerationActive
	})
	count, size, err := m.CacheStats(gen.ID)
	if err != nil || count != 2 || size <= 0 {
		t.Fatalf("expected 2 cached files with non-zero size, got count=%d size=%d err=%v", count, size, err)
	}
	if count, _, err := m.CacheStats(gen.ID + 999); err != nil || count != 0 {
		t.Fatalf("expected zero stats for a non-existent generation directory: count=%d err=%v", count, err)
	}
}

func TestRenderTestClipDoesNotTouchProductionCacheOrQueue(t *testing.T) {
	s := newTestStore(t)
	mustUpsertVoice(t, s, "de-voice", "de-DE")
	engine := &fakeEngine{}
	m := NewManager(s, engine, t.TempDir(), mapSystemTexts{}, nil)
	if _, err := m.SwitchLanguage(context.Background(), Config{Enabled: true, Language: "de-DE", VoiceID: "de-voice", BackgroundCPUPercent: 30}); err != nil {
		t.Fatal(err)
	}
	gen, _, err := s.CurrentTTSGeneration()
	if err != nil {
		t.Fatal(err)
	}
	voice, _, err := s.GetTTSVoice("de-voice")
	if err != nil {
		t.Fatal(err)
	}
	path, cleanup, err := m.RenderTestClip(context.Background(), voice, "Hallo! Ich bin deine MuPiBox.")
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected test clip file to exist: %v", err)
	}
	counts, err := s.CountTTSJobs(gen.ID)
	if err != nil || counts.Done != 0 {
		t.Fatalf("test clip must not create a production job: %#v err=%v", counts, err)
	}
	cleanup()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected cleanup to remove the test clip, stat err=%v", err)
	}
}

func TestResolveSpeechReturnsPlayablePathAfterCacheMiss(t *testing.T) {
	s := newTestStore(t)
	mustUpsertVoice(t, s, "de-voice", "de-DE")
	m := NewManager(s, &fakeEngine{}, t.TempDir(), mapSystemTexts{}, nil)
	m.Start()
	defer m.Stop()
	if _, err := m.SwitchLanguage(context.Background(), Config{Enabled: true, Language: "de-DE", VoiceID: "de-voice", BackgroundCPUPercent: 30}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	path, err := m.ResolveSpeech(ctx, "category", "books", "Hörbücher")
	if err != nil {
		t.Fatal(err)
	}
	if _, statErr := os.Stat(path); statErr != nil {
		t.Fatalf("expected the resolved path to exist: %v", statErr)
	}
}

func TestResolveSpeechFailsWithoutConfiguredTTS(t *testing.T) {
	s := newTestStore(t)
	m := NewManager(s, &fakeEngine{}, t.TempDir(), mapSystemTexts{}, nil)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err := m.ResolveSpeech(ctx, "category", "books", "Hörbücher"); err == nil {
		t.Fatal("expected an error when no generation is configured")
	}
}

func TestConcurrentEnsureTextCallsForSameTextShareOneRender(t *testing.T) {
	s := newTestStore(t)
	mustUpsertVoice(t, s, "de-voice", "de-DE")
	engine := &fakeEngine{delay: 100 * time.Millisecond}
	m := NewManager(s, engine, t.TempDir(), mapSystemTexts{}, nil)
	m.Start()
	defer m.Stop()
	if _, err := m.SwitchLanguage(context.Background(), Config{Enabled: true, Language: "de-DE", VoiceID: "de-voice", BackgroundCPUPercent: 30}); err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	errs := make([]error, 5)
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			errs[i] = m.EnsureText(ctx, "category", "same", "Gleicher Text", PriorityHigh)
		}(i)
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if atomic.LoadInt32(&engine.calls) != 1 {
		t.Fatalf("expected a single render for the shared text hash, got %d", engine.calls)
	}
}
