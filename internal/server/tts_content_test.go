package server

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"mupibox/internal/library"
	"mupibox/internal/store"
)

// setTestLibrary mirrors rescanLibrary()'s own locked write: a raw
// `api.Library = ...` assignment races with the background goroutine a
// prior request's rescanLibrary() may still have running (it reads Library
// through the same lock via librarySnapshot()).
func setTestLibrary(api *API, lib *library.Library) {
	api.libraryMu.Lock()
	api.Library = lib
	api.libraryMu.Unlock()
}

func TestNavigationSaveRegistersCategoryLabelsAsNormalPriority(t *testing.T) {
	api, db := newTTSTestAPI(t, &fakeTTSEngine{})
	installTestVoice(t, db, "de-voice", "de-DE")
	if w := ttsRequest(t, api.Handler(), http.MethodPut, "/api/admin/tts/config", `{"enabled":true,"language":"de-DE","voice_id":"de-voice","background_cpu_percent":30}`); w.Code != 200 {
		t.Fatalf("enable failed: %d %s", w.Code, w.Body.String())
	}
	gen, _, err := db.CurrentTTSGeneration()
	if err != nil {
		t.Fatal(err)
	}
	body := `{"categories":[{"id":"stories","labels":{"de":"Meine Hörspiele"},"rows":[]}]}`
	if w := ttsRequest(t, api.Handler(), http.MethodPut, "/api/admin/navigation", body); w.Code != 200 {
		t.Fatalf("navigation save failed: %d %s", w.Code, w.Body.String())
	}
	counts, err := db.CountTTSJobs(gen.ID)
	if err != nil || counts.NormalPending+counts.NormalRunning+counts.Done != 1 {
		t.Fatalf("expected one NORMAL job for the category label: %#v err=%v", counts, err)
	}
}

func TestNavigationRenameRegistersTheNewLabelAsANewJob(t *testing.T) {
	api, db := newTTSTestAPI(t, &fakeTTSEngine{})
	installTestVoice(t, db, "de-voice", "de-DE")
	if w := ttsRequest(t, api.Handler(), http.MethodPut, "/api/admin/tts/config", `{"enabled":true,"language":"de-DE","voice_id":"de-voice","background_cpu_percent":30}`); w.Code != 200 {
		t.Fatalf("enable failed: %d %s", w.Code, w.Body.String())
	}
	gen, _, err := db.CurrentTTSGeneration()
	if err != nil {
		t.Fatal(err)
	}
	if w := ttsRequest(t, api.Handler(), http.MethodPut, "/api/admin/navigation", `{"categories":[{"id":"stories","labels":{"de":"Meine Hörspiele"},"rows":[]}]}`); w.Code != 200 {
		t.Fatalf("first save failed: %d %s", w.Code, w.Body.String())
	}
	countsBefore, err := db.CountTTSJobs(gen.ID)
	if err != nil {
		t.Fatal(err)
	}
	if w := ttsRequest(t, api.Handler(), http.MethodPut, "/api/admin/navigation", `{"categories":[{"id":"stories","labels":{"de":"Hörspiele"},"rows":[]}]}`); w.Code != 200 {
		t.Fatalf("rename save failed: %d %s", w.Code, w.Body.String())
	}
	countsAfter, err := db.CountTTSJobs(gen.ID)
	if err != nil {
		t.Fatal(err)
	}
	total := func(c store.TTSJobCounts) int { return c.NormalPending + c.NormalRunning + c.Done }
	if total(countsAfter) <= total(countsBefore) {
		t.Fatalf("expected a new job for the renamed label: before=%#v after=%#v", countsBefore, countsAfter)
	}
}

func TestLibraryRescanRegistersFoldersAsLowPriorityInBackground(t *testing.T) {
	api, db := newTTSTestAPI(t, &fakeTTSEngine{})
	installTestVoice(t, db, "de-voice", "de-DE")
	if w := ttsRequest(t, api.Handler(), http.MethodPut, "/api/admin/tts/config", `{"enabled":true,"language":"de-DE","voice_id":"de-voice","background_cpu_percent":30}`); w.Code != 200 {
		t.Fatalf("enable failed: %d %s", w.Code, w.Body.String())
	}
	gen, _, err := db.CurrentTTSGeneration()
	if err != nil {
		t.Fatal(err)
	}
	setTestLibrary(api, &library.Library{Root: "/media", Folders: []library.Folder{
		{ID: "f1", Name: "Die drei ???", Relative: "f1"},
		{ID: "f2", Name: "Benjamin Blümchen", Relative: "f2"},
	}})
	// Exercise the exact mechanism rescanLibrary() backgrounds.
	api.registerLibrarySpeech(context.Background())
	waitForCondition(t, 2*time.Second, func() bool {
		c, err := db.CountTTSJobs(gen.ID)
		return err == nil && c.LowPending+c.LowRunning+c.Done == 2
	})
}

func TestIdenticalTextAcrossManyFoldersProducesExactlyOneJob(t *testing.T) {
	api, db := newTTSTestAPI(t, &fakeTTSEngine{})
	installTestVoice(t, db, "de-voice", "de-DE")
	if w := ttsRequest(t, api.Handler(), http.MethodPut, "/api/admin/tts/config", `{"enabled":true,"language":"de-DE","voice_id":"de-voice","background_cpu_percent":30}`); w.Code != 200 {
		t.Fatalf("enable failed: %d %s", w.Code, w.Body.String())
	}
	gen, _, err := db.CurrentTTSGeneration()
	if err != nil {
		t.Fatal(err)
	}
	folders := make([]library.Folder, 0, 100)
	for i := 0; i < 100; i++ {
		folders = append(folders, library.Folder{ID: fmt.Sprintf("f%d", i), Name: "LEGO Ninjago", Relative: fmt.Sprintf("f%d", i)})
	}
	setTestLibrary(api, &library.Library{Root: "/media", Folders: folders})
	api.registerLibrarySpeech(context.Background())
	// The worker may already have rendered the single deduplicated job by
	// the time we check, so count pending+done together.
	counts, err := db.CountTTSJobs(gen.ID)
	if err != nil || counts.LowPending+counts.LowRunning+counts.Done != 1 {
		t.Fatalf("expected exactly one deduplicated job for 100 identically-named folders: %#v err=%v", counts, err)
	}
}

func TestContentRegistrationIsANoOpWithoutTTSManager(t *testing.T) {
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.EnsureBoxSettings(store.BoxSettings{Language: "de", AdminLanguage: "de", TTS: store.TTSSettings{Language: "de"}, Audio: store.AudioSettings{StartupVolume: 30, MaxVolume: 60}, Display: store.DisplaySettings{Brightness: 100}, Theme: "modern-dark"}); err != nil {
		t.Fatal(err)
	}
	a := &API{Store: db, Library: &library.Library{Root: "/media", Folders: []library.Folder{{ID: "f1", Name: "X"}}}}
	if err := a.RegisterAllSpeakableContent(context.Background()); err != nil {
		t.Fatalf("must be a silent no-op without a TTS manager (piper unavailable): %v", err)
	}
	w := ttsRequest(t, a.Handler(), http.MethodPut, "/api/admin/navigation", `{"categories":[{"id":"stories","labels":{"de":"Hörspiele"},"rows":[]}]}`)
	if w.Code != 200 {
		t.Fatalf("navigation save must still work without TTS: %d %s", w.Code, w.Body.String())
	}
}

func TestContentRegistrationIsANoOpBeforeTTSIsConfigured(t *testing.T) {
	// TTSManager exists (piper available) but no generation was ever
	// switched to yet -- content registration must not error out.
	api, _ := newTTSTestAPI(t, &fakeTTSEngine{})
	setTestLibrary(api, &library.Library{Root: "/media", Folders: []library.Folder{{ID: "f1", Name: "X"}}})
	if err := api.RegisterAllSpeakableContent(context.Background()); err != nil {
		t.Fatalf("must be a no-op before tts is configured: %v", err)
	}
}

func TestFillMissingOnlyRendersMissingEntriesAndStaysOnCurrentGeneration(t *testing.T) {
	api, db := newTTSTestAPI(t, &fakeTTSEngine{})
	installTestVoice(t, db, "de-voice", "de-DE")
	if w := ttsRequest(t, api.Handler(), http.MethodPut, "/api/admin/tts/config", `{"enabled":true,"language":"de-DE","voice_id":"de-voice","background_cpu_percent":30}`); w.Code != 200 {
		t.Fatalf("enable failed: %d %s", w.Code, w.Body.String())
	}
	gen, _, err := db.CurrentTTSGeneration()
	if err != nil {
		t.Fatal(err)
	}
	if w := ttsRequest(t, api.Handler(), http.MethodPut, "/api/admin/navigation", `{"categories":[{"id":"stories","labels":{"de":"Hörspiele"},"rows":[]}]}`); w.Code != 200 {
		t.Fatalf("navigation save failed: %d %s", w.Code, w.Body.String())
	}
	waitForCondition(t, 2*time.Second, func() bool {
		c, err := db.CountTTSJobs(gen.ID)
		return err == nil && c.Done >= 1
	})
	setTestLibrary(api, &library.Library{Root: "/media", Folders: []library.Folder{{ID: "f1", Name: "Neuer Ordner"}}})
	w := ttsRequest(t, api.Handler(), http.MethodPost, "/api/admin/tts/cache/fill-missing", "")
	if w.Code != http.StatusAccepted {
		t.Fatalf("fill-missing status=%d body=%s", w.Code, w.Body.String())
	}
	waitForCondition(t, 2*time.Second, func() bool {
		c, err := db.CountTTSJobs(gen.ID)
		return err == nil && c.LowPending+c.Done >= 1
	})
	after, _, err := db.CurrentTTSGeneration()
	if err != nil || after.ID != gen.ID {
		t.Fatalf("fill-missing must stay on the current generation, not rebuild: before=%d after=%#v err=%v", gen.ID, after, err)
	}
}

func TestFillMissingIsDistinctFromRebuild(t *testing.T) {
	api, db := newTTSTestAPI(t, &fakeTTSEngine{})
	installTestVoice(t, db, "de-voice", "de-DE")
	if w := ttsRequest(t, api.Handler(), http.MethodPut, "/api/admin/tts/config", `{"enabled":true,"language":"de-DE","voice_id":"de-voice","background_cpu_percent":30}`); w.Code != 200 {
		t.Fatalf("enable failed: %d %s", w.Code, w.Body.String())
	}
	before, _, err := db.CurrentTTSGeneration()
	if err != nil {
		t.Fatal(err)
	}
	if w := ttsRequest(t, api.Handler(), http.MethodPost, "/api/admin/tts/cache/fill-missing", ""); w.Code != http.StatusAccepted {
		t.Fatalf("fill-missing status=%d", w.Code)
	}
	afterFill, _, err := db.CurrentTTSGeneration()
	if err != nil || afterFill.ID != before.ID {
		t.Fatalf("fill-missing must never create a new generation: before=%d after=%#v", before.ID, afterFill)
	}
	if w := ttsRequest(t, api.Handler(), http.MethodPost, "/api/admin/tts/cache/rebuild", ""); w.Code != http.StatusAccepted {
		t.Fatalf("rebuild status=%d", w.Code)
	}
	afterRebuild, _, err := db.CurrentTTSGeneration()
	if err != nil || afterRebuild.ID == before.ID {
		t.Fatalf("rebuild must create a new generation: before=%d after=%#v", before.ID, afterRebuild)
	}
}

func TestHighPriorityUserRequestOvertakesExistingLowBacklog(t *testing.T) {
	api, db := newTTSTestAPI(t, &fakeTTSEngine{delay: 50 * time.Millisecond})
	installTestVoice(t, db, "de-voice", "de-DE")
	if w := ttsRequest(t, api.Handler(), http.MethodPut, "/api/admin/tts/config", `{"enabled":true,"language":"de-DE","voice_id":"de-voice","background_cpu_percent":30}`); w.Code != 200 {
		t.Fatalf("enable failed: %d %s", w.Code, w.Body.String())
	}
	api.TTSManager.Stop() // hold the backlog so it does not drain before we submit the HIGH request
	gen, _, err := db.CurrentTTSGeneration()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 20; i++ {
		if err := db.EnqueueTTSJob(gen.ID, store.TTSJobSeed{Priority: store.JobPriorityLow, SourceType: "library-folder", NormalizedText: fmt.Sprintf("Folder %d", i), TextHash: fmt.Sprintf("folder-%d-hash", i)}); err != nil {
			t.Fatal(err)
		}
	}
	api.TTSManager.Start()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	start := time.Now()
	if _, err := api.TTSManager.ResolveSpeech(ctx, "category", "urgent", "Dringend"); err != nil {
		t.Fatal(err)
	}
	elapsed := time.Since(start)
	if elapsed > 500*time.Millisecond {
		t.Fatalf("HIGH request should overtake a 20-item LOW backlog quickly, took %v", elapsed)
	}
}

func TestShutdownWaitsForInFlightBackgroundContentScan(t *testing.T) {
	api, _ := newTTSTestAPI(t, &fakeTTSEngine{})
	started := make(chan struct{})
	finished := false
	api.runBackground(func(ctx context.Context) {
		close(started)
		<-ctx.Done() // simulate a long scan that only stops on cancellation
		finished = true
	})
	<-started
	api.Shutdown()
	if !finished {
		t.Fatal("Shutdown must cancel and wait for in-flight background tasks")
	}
}

func TestRegisterLibrarySpeechStopsWhenContextIsCancelled(t *testing.T) {
	api, db := newTTSTestAPI(t, &fakeTTSEngine{delay: 50 * time.Millisecond})
	installTestVoice(t, db, "de-voice", "de-DE")
	if w := ttsRequest(t, api.Handler(), http.MethodPut, "/api/admin/tts/config", `{"enabled":true,"language":"de-DE","voice_id":"de-voice","background_cpu_percent":30}`); w.Code != 200 {
		t.Fatalf("enable failed: %d %s", w.Code, w.Body.String())
	}
	folders := make([]library.Folder, 0, 200)
	for i := 0; i < 200; i++ {
		folders = append(folders, library.Folder{ID: fmt.Sprintf("f%d", i), Name: fmt.Sprintf("Ordner %d", i)})
	}
	setTestLibrary(api, &library.Library{Root: "/media", Folders: folders})
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled: must stop immediately, registering nothing
	api.registerLibrarySpeech(ctx)
	gen, _, err := db.CurrentTTSGeneration()
	if err != nil {
		t.Fatal(err)
	}
	counts, err := db.CountTTSJobs(gen.ID)
	if err != nil || counts.LowPending+counts.LowRunning+counts.Done != 0 {
		t.Fatalf("expected no jobs registered once the context was already cancelled: %#v err=%v", counts, err)
	}
}

func TestLargeLibraryRegistrationDoesNotBlockCaller(t *testing.T) {
	api, db := newTTSTestAPI(t, &fakeTTSEngine{})
	installTestVoice(t, db, "de-voice", "de-DE")
	if w := ttsRequest(t, api.Handler(), http.MethodPut, "/api/admin/tts/config", `{"enabled":true,"language":"de-DE","voice_id":"de-voice","background_cpu_percent":30}`); w.Code != 200 {
		t.Fatalf("enable failed: %d %s", w.Code, w.Body.String())
	}
	gen, _, err := db.CurrentTTSGeneration()
	if err != nil {
		t.Fatal(err)
	}
	const total = 2000
	folders := make([]library.Folder, 0, total)
	for i := 0; i < total; i++ {
		folders = append(folders, library.Folder{ID: fmt.Sprintf("f%d", i), Name: fmt.Sprintf("Ordner %d", i), Relative: fmt.Sprintf("f%d", i)})
	}
	setTestLibrary(api, &library.Library{Root: "/media", Folders: folders})

	done := make(chan time.Duration, 1)
	go func() {
		start := time.Now()
		api.registerLibrarySpeech(context.Background())
		done <- time.Since(start)
	}()
	select {
	case elapsed := <-done:
		if elapsed > 15*time.Second {
			t.Fatalf("registering %d folders took too long for a Pi 3 budget: %v", total, elapsed)
		}
		t.Logf("registered %d folders in %v (%s per item)", total, elapsed, elapsed/total)
	case <-time.After(20 * time.Second):
		t.Fatal("registration did not complete in time")
	}
	// The worker is running concurrently and may have already rendered some
	// of these, so what matters is that all `total` got registered at all
	// (pending+running+done), not that none of them started yet.
	waitForCondition(t, 10*time.Second, func() bool {
		c, err := db.CountTTSJobs(gen.ID)
		return err == nil && c.LowPending+c.LowRunning+c.Done == total
	})
}
