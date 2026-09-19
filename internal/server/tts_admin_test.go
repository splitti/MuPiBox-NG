package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"mupibox/internal/audio"
	"mupibox/internal/core"
	"mupibox/internal/library"
	"mupibox/internal/store"
	"mupibox/internal/tts"
)

// fakeTTSEngine is a minimal tts.Engine double: it never shells out, so
// these tests exercise the admin HTTP layer and the generation state
// machine without depending on Piper being installed.
type fakeTTSEngine struct{ delay time.Duration }

func (e *fakeTTSEngine) Name() string    { return "fake" }
func (e *fakeTTSEngine) Version() string { return "1.0" }
func (e *fakeTTSEngine) Synthesize(ctx context.Context, voice store.TTSVoice, text, outPath string, backgroundCPUPercent int) error {
	if e.delay > 0 {
		time.Sleep(e.delay)
	}
	return os.WriteFile(outPath, []byte("wav:"+text), 0600)
}

func newTTSTestAPI(t *testing.T, engine tts.Engine) (*API, *store.Store) {
	t.Helper()
	api, db, _ := newTTSTestAPIWithCacheDir(t, engine)
	return api, db
}

func newTTSTestAPIWithCacheDir(t *testing.T, engine tts.Engine) (*API, *store.Store, string) {
	t.Helper()
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	defaults := store.BoxSettings{Language: "de", AdminLanguage: "de", TTS: store.TTSSettings{Language: "de", BackgroundCPUPercent: 30}, Audio: store.AudioSettings{StartupVolume: 30, MaxVolume: 60}, Display: store.DisplaySettings{Brightness: 100}, Theme: "modern-dark"}
	if _, err := db.EnsureBoxSettings(defaults); err != nil {
		t.Fatal(err)
	}
	cacheDir := t.TempDir()
	manager := tts.NewManager(db, engine, cacheDir, nil, nil)
	manager.Start()
	t.Cleanup(manager.Stop)
	lib := &library.Library{Root: t.TempDir(), Folders: []library.Folder{}}
	return &API{Store: db, Library: lib, TTSManager: manager, NewAudioBackend: func() audio.Backend { return &audio.Simulated{} }}, db, cacheDir
}

func installTestVoice(t *testing.T, db *store.Store, id, language string) {
	t.Helper()
	if err := db.UpsertTTSVoice(store.TTSVoice{
		ID: id, Engine: "fake", Language: language, VoiceFamily: "test", Quality: "medium",
		DisplayName: id, ModelPath: "/models/" + id + ".onnx", ConfigPath: "/models/" + id + ".onnx.json",
		License: "CC0", SourceURL: "https://example.invalid", Available: true,
	}); err != nil {
		t.Fatal(err)
	}
}

func ttsRequest(t *testing.T, h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		r.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func waitForCondition(t *testing.T, timeout time.Duration, cond func() bool) {
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

func TestTTSStatusReportsUnavailableWithoutManager(t *testing.T) {
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.EnsureBoxSettings(store.BoxSettings{Language: "de", AdminLanguage: "de", TTS: store.TTSSettings{Language: "de"}, Audio: store.AudioSettings{StartupVolume: 30, MaxVolume: 60}, Display: store.DisplaySettings{Brightness: 100}, Theme: "modern-dark"}); err != nil {
		t.Fatal(err)
	}
	a := &API{Store: db}
	w := ttsRequest(t, a.Handler(), http.MethodGet, "/api/admin/tts/status", "")
	if w.Code != 200 {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var status ttsStatusResponse
	if err := json.Unmarshal(w.Body.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	if status.Enabled || status.Available || status.WorkerRunning {
		t.Fatalf("expected a fully inactive status without a manager: %#v", status)
	}
}

func TestTTSConfigRejectsUnknownVoice(t *testing.T) {
	api, _ := newTTSTestAPI(t, &fakeTTSEngine{})
	w := ttsRequest(t, api.Handler(), http.MethodPut, "/api/admin/tts/config", `{"enabled":true,"language":"de-DE","voice_id":"does-not-exist","background_cpu_percent":30}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for unknown voice, got %d: %s", w.Code, w.Body.String())
	}
}

func TestTTSConfigRejectsVoiceLanguageMismatch(t *testing.T) {
	api, db := newTTSTestAPI(t, &fakeTTSEngine{})
	installTestVoice(t, db, "de-voice", "de-DE")
	w := ttsRequest(t, api.Handler(), http.MethodPut, "/api/admin/tts/config", `{"enabled":true,"language":"fr-FR","voice_id":"de-voice","background_cpu_percent":30}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for language/voice mismatch, got %d: %s", w.Code, w.Body.String())
	}
}

func TestTTSConfigRejectsOutOfRangeCPUPercent(t *testing.T) {
	api, db := newTTSTestAPI(t, &fakeTTSEngine{})
	installTestVoice(t, db, "de-voice", "de-DE")
	w := ttsRequest(t, api.Handler(), http.MethodPut, "/api/admin/tts/config", `{"enabled":true,"language":"de-DE","voice_id":"de-voice","background_cpu_percent":150}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for out-of-range cpu percent, got %d: %s", w.Code, w.Body.String())
	}
}

func TestTTSConfigChangeReturnsWithoutWaitingForRebuild(t *testing.T) {
	// A slow engine simulates a large library rebuild; the PUT must still
	// return quickly because it only starts the generation, it never waits
	// for the background worker to finish rendering it.
	api, db := newTTSTestAPI(t, &fakeTTSEngine{delay: 2 * time.Second})
	installTestVoice(t, db, "de-voice", "de-DE")
	start := time.Now()
	w := ttsRequest(t, api.Handler(), http.MethodPut, "/api/admin/tts/config", `{"enabled":true,"language":"de-DE","voice_id":"de-voice","background_cpu_percent":30}`)
	elapsed := time.Since(start)
	if w.Code != http.StatusOK {
		t.Fatalf("config status=%d body=%s", w.Code, w.Body.String())
	}
	if elapsed > 500*time.Millisecond {
		t.Fatalf("PUT /api/admin/tts/config must not block on rebuild, took %v", elapsed)
	}
	// With no seed jobs the generation is free to activate immediately --
	// that is correct (nothing to wait for), the point of this test is that
	// the HTTP handler itself never blocks on the worker regardless.
	if _, ok, err := db.CurrentTTSGeneration(); err != nil || !ok {
		t.Fatalf("expected a current generation right after the switch: ok=%v err=%v", ok, err)
	}
}

func TestTTSConfigEnableThenCPUOnlyChangeKeepsSameGeneration(t *testing.T) {
	api, db := newTTSTestAPI(t, &fakeTTSEngine{})
	installTestVoice(t, db, "de-voice", "de-DE")
	if w := ttsRequest(t, api.Handler(), http.MethodPut, "/api/admin/tts/config", `{"enabled":true,"language":"de-DE","voice_id":"de-voice","background_cpu_percent":30}`); w.Code != 200 {
		t.Fatalf("initial enable failed: %d %s", w.Code, w.Body.String())
	}
	waitForCondition(t, 2*time.Second, func() bool {
		g, _, err := db.CurrentTTSGeneration()
		return err == nil && g.Status == store.GenerationActive
	})
	before, _, err := db.CurrentTTSGeneration()
	if err != nil {
		t.Fatal(err)
	}
	w := ttsRequest(t, api.Handler(), http.MethodPut, "/api/admin/tts/config", `{"enabled":true,"language":"de-DE","voice_id":"de-voice","background_cpu_percent":55}`)
	if w.Code != 200 {
		t.Fatalf("cpu-only change failed: %d %s", w.Code, w.Body.String())
	}
	after, _, err := db.CurrentTTSGeneration()
	if err != nil {
		t.Fatal(err)
	}
	if after.ID != before.ID {
		t.Fatalf("a cpu-only change must not create a new generation: before=%d after=%d", before.ID, after.ID)
	}
	if after.BackgroundCPUPercent != 55 {
		t.Fatalf("expected the cpu percent to update in place, got %d", after.BackgroundCPUPercent)
	}
}

func TestTTSConfigLanguageChangeMarksPreviousGenerationStale(t *testing.T) {
	api, db := newTTSTestAPI(t, &fakeTTSEngine{})
	installTestVoice(t, db, "de-voice", "de-DE")
	installTestVoice(t, db, "nl-voice", "nl-NL")
	if w := ttsRequest(t, api.Handler(), http.MethodPut, "/api/admin/tts/config", `{"enabled":true,"language":"de-DE","voice_id":"de-voice","background_cpu_percent":30}`); w.Code != 200 {
		t.Fatalf("initial enable failed: %d %s", w.Code, w.Body.String())
	}
	first, _, err := db.CurrentTTSGeneration()
	if err != nil {
		t.Fatal(err)
	}
	w := ttsRequest(t, api.Handler(), http.MethodPut, "/api/admin/tts/config", `{"enabled":true,"language":"nl-NL","voice_id":"nl-voice","background_cpu_percent":30}`)
	if w.Code != 200 {
		t.Fatalf("language switch failed: %d %s", w.Code, w.Body.String())
	}
	// The old generation must never remain the current/serving one again; it
	// may already have progressed from stale to purging by the time we
	// check, since an idle worker purges promptly -- both are correct.
	if stale, ok, err := db.GetTTSGeneration(first.ID); err != nil || (ok && stale.Status != store.GenerationStale && stale.Status != store.GenerationPurging) {
		t.Fatalf("expected the previous generation to be stale/purging/gone, not still current: %#v ok=%v err=%v", stale, ok, err)
	}
	second, _, err := db.CurrentTTSGeneration()
	if err != nil || second.ID == first.ID || second.Language != "nl-NL" {
		t.Fatalf("expected a new current generation for nl-NL: %#v err=%v", second, err)
	}
}

func TestTTSConfigDisableStopsWorker(t *testing.T) {
	api, db := newTTSTestAPI(t, &fakeTTSEngine{})
	installTestVoice(t, db, "de-voice", "de-DE")
	if w := ttsRequest(t, api.Handler(), http.MethodPut, "/api/admin/tts/config", `{"enabled":true,"language":"de-DE","voice_id":"de-voice","background_cpu_percent":30}`); w.Code != 200 {
		t.Fatalf("enable failed: %d %s", w.Code, w.Body.String())
	}
	if !api.TTSManager.Running() {
		t.Fatal("expected worker running after enable")
	}
	w := ttsRequest(t, api.Handler(), http.MethodPut, "/api/admin/tts/config", `{"enabled":false,"language":"de-DE","voice_id":"de-voice","background_cpu_percent":30}`)
	if w.Code != 200 {
		t.Fatalf("disable failed: %d %s", w.Code, w.Body.String())
	}
	if api.TTSManager.Running() {
		t.Fatal("expected worker stopped after disable")
	}
}

func TestTTSVoicesListsManifestWithInstalledFlag(t *testing.T) {
	api, db := newTTSTestAPI(t, &fakeTTSEngine{})
	w := ttsRequest(t, api.Handler(), http.MethodGet, "/api/admin/tts/voices?language=de-DE", "")
	if w.Code != 200 {
		t.Fatalf("voices status=%d body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Voices []struct {
			ID        string `json:"id"`
			Language  string `json:"language"`
			Installed bool   `json:"installed"`
			Available bool   `json:"available"`
		} `json:"voices"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Voices) == 0 {
		t.Fatal("expected at least one manifest voice for de-DE")
	}
	for _, v := range resp.Voices {
		if v.Language != "de-DE" {
			t.Fatalf("language filter leaked a non-de-DE voice: %#v", v)
		}
		if v.Installed || v.Available {
			t.Fatalf("no voice should be marked installed before syncing the store: %#v", v)
		}
	}
	installTestVoice(t, db, resp.Voices[0].ID, "de-DE")
	w = ttsRequest(t, api.Handler(), http.MethodGet, "/api/admin/tts/voices?language=de-DE", "")
	json.Unmarshal(w.Body.Bytes(), &resp)
	if !resp.Voices[0].Installed || !resp.Voices[0].Available {
		t.Fatalf("expected the installed voice to be reported installed+available: %#v", resp.Voices[0])
	}
}

func TestTTSTestEndpointWorksWithoutSavedConfig(t *testing.T) {
	api, db := newTTSTestAPI(t, &fakeTTSEngine{})
	installTestVoice(t, db, "de-voice", "de-DE")
	w := ttsRequest(t, api.Handler(), http.MethodPost, "/api/admin/tts/test", `{"text":"Hallo! Ich bin deine MuPiBox.","voice_id":"de-voice","language":"de-DE"}`)
	if w.Code != 200 {
		t.Fatalf("test status=%d body=%s", w.Code, w.Body.String())
	}
	if _, ok, err := db.CurrentTTSGeneration(); err != nil || ok {
		t.Fatalf("voice test must not create a production generation: ok=%v err=%v", ok, err)
	}
}

func TestTTSTestEndpointRejectsUnavailableVoice(t *testing.T) {
	api, _ := newTTSTestAPI(t, &fakeTTSEngine{})
	w := ttsRequest(t, api.Handler(), http.MethodPost, "/api/admin/tts/test", `{"text":"Hallo","voice_id":"missing"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for an unavailable voice, got %d: %s", w.Code, w.Body.String())
	}
}

func TestTTSTestEndpointUnavailableWithoutManager(t *testing.T) {
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	a := &API{Store: db}
	w := ttsRequest(t, a.Handler(), http.MethodPost, "/api/admin/tts/test", `{"text":"Hallo"}`)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 without a manager, got %d: %s", w.Code, w.Body.String())
	}
}

// TestSpeakPausesRunningMusicWithoutAutoResume covers Phase 2 item 8: an
// announcement must pause playback instead of talking over it, and must
// never auto-resume music on its own -- the user resumes explicitly.
func TestSpeakPausesRunningMusicWithoutAutoResume(t *testing.T) {
	api, db := newTTSTestAPI(t, &fakeTTSEngine{})
	installTestVoice(t, db, "de-voice", "de-DE")
	if w := ttsRequest(t, api.Handler(), http.MethodPut, "/api/admin/tts/config", `{"enabled":true,"language":"de-DE","voice_id":"de-voice","background_cpu_percent":30}`); w.Code != 200 {
		t.Fatalf("enable failed: %d %s", w.Code, w.Body.String())
	}
	dir := t.TempDir()
	if err := os.WriteFile(dir+"/track.wav", []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	lib, err := library.Scan(dir)
	if err != nil {
		t.Fatal(err)
	}
	player, err := core.New(lib, &audio.Simulated{}, 60)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = player.Close() })
	api.Player = player
	if err := player.Execute(core.Command{Action: "folder", FolderID: lib.Folders[0].ID}); err != nil {
		t.Fatal(err)
	}
	if player.Status().State != "playing" {
		t.Fatalf("expected music to be playing, got %q", player.Status().State)
	}
	w := ttsRequest(t, api.Handler(), http.MethodPost, "/api/speak", `{"text":"Hallo, das ist ein Test mit Umlauten: äöüß."}`)
	if w.Code != 200 {
		t.Fatalf("speak status=%d body=%s", w.Code, w.Body.String())
	}
	if player.Status().State != "paused" {
		t.Fatalf("expected music paused for the announcement, got %q", player.Status().State)
	}
	time.Sleep(50 * time.Millisecond)
	if player.Status().State != "paused" {
		t.Fatalf("music must not auto-resume after the announcement, got %q", player.Status().State)
	}
}

func TestTTSRecoversAfterSimulatedRestartDuringLanguageSwitch(t *testing.T) {
	api, db, cacheDir := newTTSTestAPIWithCacheDir(t, &fakeTTSEngine{})
	installTestVoice(t, db, "de-voice", "de-DE")
	if w := ttsRequest(t, api.Handler(), http.MethodPut, "/api/admin/tts/config", `{"enabled":true,"language":"de-DE","voice_id":"de-voice","background_cpu_percent":30}`); w.Code != 200 {
		t.Fatalf("enable failed: %d %s", w.Code, w.Body.String())
	}
	gen, _, err := db.CurrentTTSGeneration()
	if err != nil {
		t.Fatal(err)
	}
	// Simulate a crash mid-render: a job stuck in "running" because the
	// process died before completing it.
	if err := db.EnqueueTTSJob(gen.ID, store.TTSJobSeed{Priority: store.JobPriorityHigh, SourceType: "system", NormalizedText: "stuck", TextHash: "stuck-hash"}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := db.DequeueNextTTSJob(gen.ID); err != nil {
		t.Fatal(err)
	}
	api.TTSManager.Stop() // simulate the old process's worker being gone

	// A brand-new Manager against the same store and cache directory
	// simulates a process restart.
	restarted := tts.NewManager(db, &fakeTTSEngine{}, cacheDir, nil, nil)
	if err := restarted.Recover(); err != nil {
		t.Fatal(err)
	}
	counts, err := db.CountTTSJobs(gen.ID)
	if err != nil || counts.HighRunning != 0 {
		t.Fatalf("expected the stuck job requeued (not left running) after recovery: %#v err=%v", counts, err)
	}
	restarted.Start()
	defer restarted.Stop()
	waitForCondition(t, 2*time.Second, func() bool {
		c, err := db.CountTTSJobs(gen.ID)
		return err == nil && c.HighPending == 0 && c.HighRunning == 0
	})
}

func TestTTSConcurrentConfigPUTsLeaveExactlyOneCurrentGeneration(t *testing.T) {
	api, db := newTTSTestAPI(t, &fakeTTSEngine{})
	installTestVoice(t, db, "de-voice", "de-DE")
	installTestVoice(t, db, "nl-voice", "nl-NL")
	bodies := []string{
		`{"enabled":true,"language":"de-DE","voice_id":"de-voice","background_cpu_percent":30}`,
		`{"enabled":true,"language":"nl-NL","voice_id":"nl-voice","background_cpu_percent":30}`,
	}
	var wg sync.WaitGroup
	codes := make([]int, len(bodies))
	for i, body := range bodies {
		wg.Add(1)
		go func(i int, body string) {
			defer wg.Done()
			codes[i] = ttsRequest(t, api.Handler(), http.MethodPut, "/api/admin/tts/config", body).Code
		}(i, body)
	}
	wg.Wait()
	for _, code := range codes {
		if code != 200 {
			t.Fatalf("expected both concurrent PUTs to succeed, got codes=%v", codes)
		}
	}
	generations, err := db.ListTTSGenerationsByStatus(store.GenerationBuilding)
	if err != nil {
		t.Fatal(err)
	}
	active, err := db.ListTTSGenerationsByStatus(store.GenerationActive)
	if err != nil {
		t.Fatal(err)
	}
	if len(generations)+len(active) != 1 {
		t.Fatalf("expected exactly one non-stale generation after concurrent switches, got building=%d active=%d", len(generations), len(active))
	}
	if _, ok, err := db.CurrentTTSGeneration(); err != nil || !ok {
		t.Fatalf("expected a well-defined current generation: ok=%v err=%v", ok, err)
	}
}

func TestTTSCacheRebuildCreatesNewGenerationWithoutBlocking(t *testing.T) {
	api, db := newTTSTestAPI(t, &fakeTTSEngine{delay: 2 * time.Second})
	installTestVoice(t, db, "de-voice", "de-DE")
	if w := ttsRequest(t, api.Handler(), http.MethodPut, "/api/admin/tts/config", `{"enabled":true,"language":"de-DE","voice_id":"de-voice","background_cpu_percent":30}`); w.Code != 200 {
		t.Fatalf("enable failed: %d %s", w.Code, w.Body.String())
	}
	first, _, err := db.CurrentTTSGeneration()
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	w := ttsRequest(t, api.Handler(), http.MethodPost, "/api/admin/tts/cache/rebuild", "")
	if w.Code != http.StatusAccepted {
		t.Fatalf("rebuild status=%d body=%s", w.Code, w.Body.String())
	}
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Fatalf("rebuild must not block on rendering, took %v", elapsed)
	}
	second, _, err := db.CurrentTTSGeneration()
	if err != nil || second.ID == first.ID {
		t.Fatalf("expected a new generation after rebuild: first=%d second=%#v err=%v", first.ID, second, err)
	}
}

func TestTTSCacheRebuildRequiresConfiguredTTS(t *testing.T) {
	api, _ := newTTSTestAPI(t, &fakeTTSEngine{})
	w := ttsRequest(t, api.Handler(), http.MethodPost, "/api/admin/tts/cache/rebuild", "")
	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409 when tts is not configured yet, got %d: %s", w.Code, w.Body.String())
	}
}

func TestTTSCacheCleanupRemovesStaleGenerationButKeepsCurrent(t *testing.T) {
	api, db := newTTSTestAPI(t, &fakeTTSEngine{})
	installTestVoice(t, db, "de-voice", "de-DE")
	installTestVoice(t, db, "nl-voice", "nl-NL")
	if w := ttsRequest(t, api.Handler(), http.MethodPut, "/api/admin/tts/config", `{"enabled":true,"language":"de-DE","voice_id":"de-voice","background_cpu_percent":30}`); w.Code != 200 {
		t.Fatalf("enable failed: %d %s", w.Code, w.Body.String())
	}
	first, _, err := db.CurrentTTSGeneration()
	if err != nil {
		t.Fatal(err)
	}
	if w := ttsRequest(t, api.Handler(), http.MethodPut, "/api/admin/tts/config", `{"enabled":true,"language":"nl-NL","voice_id":"nl-voice","background_cpu_percent":30}`); w.Code != 200 {
		t.Fatalf("switch failed: %d %s", w.Code, w.Body.String())
	}
	w := ttsRequest(t, api.Handler(), http.MethodPost, "/api/admin/tts/cache/cleanup", "")
	if w.Code != 200 {
		t.Fatalf("cleanup status=%d body=%s", w.Code, w.Body.String())
	}
	if _, ok, err := db.GetTTSGeneration(first.ID); err != nil || ok {
		t.Fatalf("expected the stale generation to be gone after cleanup: ok=%v err=%v", ok, err)
	}
	if current, ok, err := db.CurrentTTSGeneration(); err != nil || !ok || current.Language != "nl-NL" {
		t.Fatalf("cleanup must not touch the current generation: %#v ok=%v err=%v", current, ok, err)
	}
}

func TestExistingAdminSettingsEndpointStillWorksAlongsideTTSRoutes(t *testing.T) {
	api, db := newTTSTestAPI(t, &fakeTTSEngine{})
	w := ttsRequest(t, api.Handler(), http.MethodGet, "/api/admin/settings", "")
	if w.Code != 200 {
		t.Fatalf("existing settings endpoint broke: %d %s", w.Code, w.Body.String())
	}
	var settings store.BoxSettings
	if err := json.Unmarshal(w.Body.Bytes(), &settings); err != nil || settings.Language != "de" {
		t.Fatalf("unexpected settings payload: %#v err=%v", settings, err)
	}
	_ = db
}
