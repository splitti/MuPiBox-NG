package store

import "testing"

func TestTTSGenerationLifecycle(t *testing.T) {
	s, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	if _, ok, err := s.CurrentTTSGeneration(); err != nil || ok {
		t.Fatalf("expected no current generation initially: ok=%v err=%v", ok, err)
	}

	seeds := []TTSJobSeed{
		{Priority: JobPriorityHigh, SourceType: "system", NormalizedText: "Willkommen", TextHash: "h1"},
		{Priority: JobPriorityNormal, SourceType: "category", SourceRef: "books", NormalizedText: "Hörbücher", TextHash: "h2"},
	}
	gen1, err := s.SwitchTTSGeneration("de-DE", "de_DE-thorsten-medium", "piper", "1.8.0", "{}", 30, seeds)
	if err != nil {
		t.Fatal(err)
	}
	if gen1.Status != GenerationBuilding {
		t.Fatalf("new generation should start building, got %s", gen1.Status)
	}

	current, ok, err := s.CurrentTTSGeneration()
	if err != nil || !ok || current.ID != gen1.ID {
		t.Fatalf("current generation mismatch: %#v ok=%v err=%v", current, ok, err)
	}

	// Not ready yet: high job still pending.
	if promoted, err := s.ActivateTTSGenerationIfReady(gen1.ID); err != nil || promoted {
		t.Fatalf("must not activate with pending high job: promoted=%v err=%v", promoted, err)
	}

	job, ok, err := s.DequeueNextTTSJob(gen1.ID)
	if err != nil || !ok || job.Priority != JobPriorityHigh {
		t.Fatalf("expected to dequeue high priority job first: %#v ok=%v err=%v", job, ok, err)
	}
	if err = s.CompleteTTSJob(job.ID); err != nil {
		t.Fatal(err)
	}

	// Still not ready: normal job pending.
	if promoted, err := s.ActivateTTSGenerationIfReady(gen1.ID); err != nil || promoted {
		t.Fatalf("must not activate with pending normal job: promoted=%v err=%v", promoted, err)
	}

	job2, ok, err := s.DequeueNextTTSJob(gen1.ID)
	if err != nil || !ok || job2.Priority != JobPriorityNormal {
		t.Fatalf("expected normal job next: %#v ok=%v err=%v", job2, ok, err)
	}
	if err = s.CompleteTTSJob(job2.ID); err != nil {
		t.Fatal(err)
	}

	promoted, err := s.ActivateTTSGenerationIfReady(gen1.ID)
	if err != nil || !promoted {
		t.Fatalf("expected activation once high/normal backlog empty: promoted=%v err=%v", promoted, err)
	}
	current, _, _ = s.GetTTSGeneration(gen1.ID)
	if current.Status != GenerationActive {
		t.Fatalf("expected active status, got %s", current.Status)
	}
	if current.ActivatedAt.IsZero() {
		t.Fatal("expected activated_at to be set")
	}

	// Switching language marks the previous generation stale immediately,
	// without touching its rows synchronously.
	gen2, err := s.SwitchTTSGeneration("nl-NL", "nl_NL-some-medium", "piper", "1.8.0", "{}", 30, nil)
	if err != nil {
		t.Fatal(err)
	}
	old, _, err := s.GetTTSGeneration(gen1.ID)
	if err != nil || old.Status != GenerationStale {
		t.Fatalf("previous generation must become stale: %#v err=%v", old, err)
	}
	current, ok, err = s.CurrentTTSGeneration()
	if err != nil || !ok || current.ID != gen2.ID {
		t.Fatalf("current generation must be the new one: %#v ok=%v err=%v", current, ok, err)
	}

	// Purge lifecycle for the stale generation.
	if err = s.MarkTTSGenerationPurging(gen1.ID); err != nil {
		t.Fatal(err)
	}
	old, _, _ = s.GetTTSGeneration(gen1.ID)
	if old.Status != GenerationPurging {
		t.Fatalf("expected purging status, got %s", old.Status)
	}
	if err = s.DeleteTTSGeneration(gen1.ID); err != nil {
		t.Fatal(err)
	}
	if _, ok, err = s.GetTTSGeneration(gen1.ID); err != nil || ok {
		t.Fatalf("stale generation should be gone after purge: ok=%v err=%v", ok, err)
	}
	// Deleting again must be a safe no-op (idempotent resume after crash).
	if err = s.DeleteTTSGeneration(gen1.ID); err != nil {
		t.Fatalf("repeat delete must be idempotent: %v", err)
	}
}

func TestSwitchDuringBuildingMarksInProgressGenerationStaleImmediately(t *testing.T) {
	s, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	gen1, err := s.SwitchTTSGeneration("de-DE", "voice-a", "piper", "1.8.0", "{}", 30, []TTSJobSeed{
		{Priority: JobPriorityHigh, SourceType: "system", NormalizedText: "x", TextHash: "h1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	// gen1 is still "building" (high job pending) when a second switch happens.
	gen2, err := s.SwitchTTSGeneration("fr-FR", "voice-b", "piper", "1.8.0", "{}", 30, nil)
	if err != nil {
		t.Fatal(err)
	}
	old, _, err := s.GetTTSGeneration(gen1.ID)
	if err != nil || old.Status != GenerationStale {
		t.Fatalf("still-building generation must go straight to stale on re-switch: %#v err=%v", old, err)
	}
	current, ok, err := s.CurrentTTSGeneration()
	if err != nil || !ok || current.ID != gen2.ID {
		t.Fatalf("current generation must be gen2: %#v ok=%v err=%v", current, ok, err)
	}
}

func TestEnqueueTTSJobIsIdempotentAndPromotable(t *testing.T) {
	s, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	gen, err := s.SwitchTTSGeneration("de-DE", "voice-a", "piper", "1.8.0", "{}", 30, nil)
	if err != nil {
		t.Fatal(err)
	}
	seed := TTSJobSeed{Priority: JobPriorityLow, SourceType: "library", SourceRef: "album-1", NormalizedText: "Album Eins", TextHash: "album-1-hash"}
	if err = s.EnqueueTTSJob(gen.ID, seed); err != nil {
		t.Fatal(err)
	}
	if err = s.EnqueueTTSJob(gen.ID, seed); err != nil {
		t.Fatalf("re-enqueue of same text must be a no-op, not an error: %v", err)
	}
	counts, err := s.CountTTSJobs(gen.ID)
	if err != nil || counts.LowPending != 1 {
		t.Fatalf("expected exactly one low job, got %#v err=%v", counts, err)
	}

	// Cache-miss fallback: promote to high priority.
	if err = s.PromoteTTSJob(gen.ID, "album-1-hash"); err != nil {
		t.Fatal(err)
	}
	counts, err = s.CountTTSJobs(gen.ID)
	if err != nil || counts.LowPending != 0 || counts.HighPending != 1 {
		t.Fatalf("expected job promoted to high: %#v err=%v", counts, err)
	}
}

func TestResetRunningTTSJobsOnRestart(t *testing.T) {
	s, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	gen, err := s.SwitchTTSGeneration("de-DE", "voice-a", "piper", "1.8.0", "{}", 30, []TTSJobSeed{
		{Priority: JobPriorityHigh, SourceType: "system", NormalizedText: "x", TextHash: "h1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	job, ok, err := s.DequeueNextTTSJob(gen.ID)
	if err != nil || !ok {
		t.Fatalf("dequeue: %#v ok=%v err=%v", job, ok, err)
	}
	// Simulate a crash: job stays "running" forever unless we recover it.
	if err = s.ResetRunningTTSJobs(); err != nil {
		t.Fatal(err)
	}
	counts, err := s.CountTTSJobs(gen.ID)
	if err != nil || counts.HighPending != 1 || counts.HighRunning != 0 {
		t.Fatalf("expected running job requeued to pending: %#v err=%v", counts, err)
	}
}

func TestEnqueueTTSJobNeverDowngradesAPendingHighPriorityJob(t *testing.T) {
	s, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	gen, err := s.SwitchTTSGeneration("de-DE", "voice-a", "piper", "1.8.0", "{}", 30, nil)
	if err != nil {
		t.Fatal(err)
	}
	seed := TTSJobSeed{SourceType: "category", NormalizedText: "LEGO Ninjago", TextHash: "shared-hash"}
	low := seed
	low.Priority = JobPriorityLow
	if err = s.EnqueueTTSJob(gen.ID, low); err != nil {
		t.Fatal(err)
	}
	if err = s.PromoteTTSJob(gen.ID, "shared-hash"); err != nil {
		t.Fatal(err)
	}
	// A background bulk content scan re-registering the same text at NORMAL
	// must not undo the user-triggered HIGH promotion.
	normal := seed
	normal.Priority = JobPriorityNormal
	if err = s.EnqueueTTSJob(gen.ID, normal); err != nil {
		t.Fatal(err)
	}
	counts, err := s.CountTTSJobs(gen.ID)
	if err != nil || counts.HighPending != 1 || counts.NormalPending != 0 {
		t.Fatalf("expected the job to stay HIGH, got %#v err=%v", counts, err)
	}
}

func TestEnqueueTTSJobResetsErrorStatusForRetry(t *testing.T) {
	s, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	gen, err := s.SwitchTTSGeneration("de-DE", "voice-a", "piper", "1.8.0", "{}", 30, nil)
	if err != nil {
		t.Fatal(err)
	}
	seed := TTSJobSeed{Priority: JobPriorityLow, SourceType: "library", NormalizedText: "Kaputt", TextHash: "broken-hash"}
	if err = s.EnqueueTTSJob(gen.ID, seed); err != nil {
		t.Fatal(err)
	}
	job, ok, err := s.DequeueNextTTSJob(gen.ID)
	if err != nil || !ok {
		t.Fatalf("dequeue: ok=%v err=%v", ok, err)
	}
	if err = s.FailTTSJob(job.ID, "synthesis failed"); err != nil {
		t.Fatal(err)
	}
	counts, err := s.CountTTSJobs(gen.ID)
	if err != nil || counts.Error != 1 {
		t.Fatalf("expected the job marked as errored: %#v err=%v", counts, err)
	}
	// "Fehlende Texte erzeugen" re-registers everything; a previously
	// failed job must become retryable again, not stay stuck forever.
	if err = s.EnqueueTTSJob(gen.ID, seed); err != nil {
		t.Fatal(err)
	}
	counts, err = s.CountTTSJobs(gen.ID)
	if err != nil || counts.Error != 0 || counts.LowPending != 1 {
		t.Fatalf("expected the errored job reset to pending: %#v err=%v", counts, err)
	}
}

func TestMarkOrphanTTSGenerationsStale(t *testing.T) {
	s, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	// No current generation at all: nothing to do, must not error.
	if err = s.MarkOrphanTTSGenerationsStale(); err != nil {
		t.Fatal(err)
	}
	gen, err := s.SwitchTTSGeneration("de-DE", "voice-a", "piper", "1.8.0", "{}", 30, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.MarkOrphanTTSGenerationsStale(); err != nil {
		t.Fatal(err)
	}
	current, _, _ := s.GetTTSGeneration(gen.ID)
	if current.Status != GenerationBuilding {
		t.Fatalf("current generation must remain untouched by the orphan sweep: %s", current.Status)
	}
}

func TestTTSVoiceCatalog(t *testing.T) {
	s, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	v := TTSVoice{
		ID: "de_DE-thorsten-medium", Engine: "piper", Language: "de-DE", VoiceFamily: "thorsten", Quality: "medium",
		DisplayName: "Thorsten (medium)", ModelPath: "/usr/local/share/mupibox-ng/voices/de_DE-thorsten-medium.onnx",
		ConfigPath: "/usr/local/share/mupibox-ng/voices/de_DE-thorsten-medium.onnx.json", License: "CC0", SourceURL: "https://example.invalid/voice",
		Available: true,
	}
	if err = s.UpsertTTSVoice(v); err != nil {
		t.Fatal(err)
	}
	langs, err := s.ListTTSLanguagesWithVoices()
	if err != nil || len(langs) != 1 || langs[0] != "de-DE" {
		t.Fatalf("expected de-DE in language list: %#v err=%v", langs, err)
	}
	voices, err := s.ListTTSVoicesByLanguage("de-DE")
	if err != nil || len(voices) != 1 || voices[0].Quality != "medium" {
		t.Fatalf("expected one medium voice: %#v err=%v", voices, err)
	}
	if err = s.SetTTSVoiceAvailable(v.ID, false); err != nil {
		t.Fatal(err)
	}
	voices, err = s.ListTTSVoicesByLanguage("de-DE")
	if err != nil || len(voices) != 0 {
		t.Fatalf("unavailable voice must not be listed: %#v err=%v", voices, err)
	}
	langs, err = s.ListTTSLanguagesWithVoices()
	if err != nil || len(langs) != 0 {
		t.Fatalf("language must disappear once its only voice is unavailable: %#v err=%v", langs, err)
	}
}

func TestTTSVoiceSpeakerIDRoundTrip(t *testing.T) {
	s, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	// Single-speaker voice: SpeakerID stays nil.
	if err = s.UpsertTTSVoice(TTSVoice{ID: "single", Language: "de-DE", VoiceFamily: "thorsten", Quality: "medium", DisplayName: "Thorsten", ModelPath: "/m/single.onnx", Available: true}); err != nil {
		t.Fatal(err)
	}
	single, ok, err := s.GetTTSVoice("single")
	if err != nil || !ok || single.SpeakerID != nil {
		t.Fatalf("expected nil speaker id for single-speaker voice: %#v ok=%v err=%v", single, ok, err)
	}

	// Multi-speaker voice: a pinned speaker index round-trips exactly.
	pinned := 42
	if err = s.UpsertTTSVoice(TTSVoice{ID: "multi", Language: "en-GB", VoiceFamily: "vctk", Quality: "medium", DisplayName: "VCTK", ModelPath: "/m/multi.onnx", SpeakerID: &pinned, Available: true}); err != nil {
		t.Fatal(err)
	}
	multi, ok, err := s.GetTTSVoice("multi")
	if err != nil || !ok || multi.SpeakerID == nil || *multi.SpeakerID != 42 {
		t.Fatalf("expected speaker id 42 to round-trip: %#v ok=%v err=%v", multi, ok, err)
	}

	if err = s.UpsertTTSVoice(TTSVoice{ID: "bad", Language: "de-DE", ModelPath: "/m/bad.onnx", SpeakerID: func() *int { n := -1; return &n }()}); err == nil {
		t.Fatal("expected negative speaker id to be rejected")
	}
}
