package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Generation statuses form a strict lifecycle:
// building -> active -> stale -> purging -> (row deleted).
// Exactly one generation can be "current" (tts_state.current_generation_id)
// at a time; a current generation is always in status building or active.
const (
	GenerationBuilding = "building"
	GenerationActive   = "active"
	GenerationStale    = "stale"
	GenerationPurging  = "purging"
)

const (
	JobPriorityHigh   = "high"
	JobPriorityNormal = "normal"
	JobPriorityLow    = "low"
)

const (
	JobPending = "pending"
	JobRunning = "running"
	JobDone    = "done"
	JobError   = "error"
)

type TTSGeneration struct {
	ID                   int64
	Language             string
	VoiceID              string
	Engine               string
	EngineVersion        string
	VoiceSettings        string
	BackgroundCPUPercent int
	Status               string
	CreatedAt            time.Time
	ActivatedAt          time.Time
}

type TTSJobSeed struct {
	Priority       string
	SourceType     string
	SourceRef      string
	NormalizedText string
	TextHash       string
}

type TTSJob struct {
	ID             int64
	GenerationID   int64
	Priority       string
	SourceType     string
	SourceRef      string
	NormalizedText string
	TextHash       string
	Status         string
	Attempts       int
	LastError      string
}

type TTSJobCounts struct {
	HighPending   int
	NormalPending int
	LowPending    int
	HighRunning   int
	NormalRunning int
	LowRunning    int
	Done          int
	Error         int
}

func (c TTSJobCounts) ReadyForActive() bool {
	return c.HighPending == 0 && c.HighRunning == 0 && c.NormalPending == 0 && c.NormalRunning == 0
}

type TTSVoice struct {
	ID          string
	Engine      string
	Language    string
	VoiceFamily string
	Quality     string
	DisplayName string
	ModelPath   string
	ConfigPath  string
	License     string
	SourceURL   string
	SpeakerID   *int // nil for single-speaker models, otherwise the pinned Piper --speaker index
	Available   bool
	InstalledAt time.Time
}

func isValidGenerationStatus(status string) bool {
	switch status {
	case GenerationBuilding, GenerationActive, GenerationStale, GenerationPurging:
		return true
	}
	return false
}

func isValidJobPriority(p string) bool {
	switch p {
	case JobPriorityHigh, JobPriorityNormal, JobPriorityLow:
		return true
	}
	return false
}

// SwitchTTSGeneration atomically starts a new generation for the given
// configuration and marks the previous current generation (if any) stale.
// Both changes, plus the seed jobs, commit together: a crash before commit
// leaves the previous configuration fully intact, a crash after commit
// leaves the new generation as the sole current one.
func (s *Store) SwitchTTSGeneration(language, voiceID, engine, engineVersion, voiceSettings string, backgroundCPUPercent int, seeds []TTSJobSeed) (TTSGeneration, error) {
	if strings.TrimSpace(language) == "" || strings.TrimSpace(voiceID) == "" || strings.TrimSpace(engine) == "" {
		return TTSGeneration{}, errors.New("language, voice_id and engine are required")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return TTSGeneration{}, err
	}
	defer tx.Rollback()

	var previousID sql.NullInt64
	if err = tx.QueryRow(`SELECT current_generation_id FROM tts_state WHERE id=1`).Scan(&previousID); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return TTSGeneration{}, err
	}
	if previousID.Valid {
		if _, err = tx.Exec(`UPDATE tts_generations SET status=? WHERE id=? AND status IN (?,?)`,
			GenerationStale, previousID.Int64, GenerationBuilding, GenerationActive); err != nil {
			return TTSGeneration{}, err
		}
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	res, err := tx.Exec(`INSERT INTO tts_generations(language,voice_id,engine,engine_version,voice_settings,background_cpu_percent,status,created_at,activated_at)
 VALUES(?,?,?,?,?,?,?,?,'')`, language, voiceID, engine, engineVersion, voiceSettings, backgroundCPUPercent, GenerationBuilding, now)
	if err != nil {
		return TTSGeneration{}, err
	}
	newID, err := res.LastInsertId()
	if err != nil {
		return TTSGeneration{}, err
	}

	if _, err = tx.Exec(`INSERT INTO tts_state(id,current_generation_id) VALUES(1,?)
 ON CONFLICT(id) DO UPDATE SET current_generation_id=excluded.current_generation_id`, newID); err != nil {
		return TTSGeneration{}, err
	}

	for _, seed := range seeds {
		if err = insertJobSeed(tx, newID, seed, now); err != nil {
			return TTSGeneration{}, err
		}
	}

	if err = tx.Commit(); err != nil {
		return TTSGeneration{}, err
	}
	return TTSGeneration{
		ID: newID, Language: language, VoiceID: voiceID, Engine: engine, EngineVersion: engineVersion,
		VoiceSettings: voiceSettings, BackgroundCPUPercent: backgroundCPUPercent, Status: GenerationBuilding,
	}, nil
}

func insertJobSeed(tx *sql.Tx, generationID int64, seed TTSJobSeed, now string) error {
	if !isValidJobPriority(seed.Priority) {
		return fmt.Errorf("invalid job priority %q", seed.Priority)
	}
	if strings.TrimSpace(seed.TextHash) == "" {
		return errors.New("job text_hash is required")
	}
	_, err := tx.Exec(`INSERT OR IGNORE INTO tts_jobs(generation_id,priority,source_type,source_ref,normalized_text,text_hash,status,created_at,updated_at)
 VALUES(?,?,?,?,?,?,?,?,?)`, generationID, seed.Priority, seed.SourceType, seed.SourceRef, seed.NormalizedText, seed.TextHash, JobPending, now, now)
	return err
}

func scanGeneration(row interface {
	Scan(dest ...any) error
}) (TTSGeneration, error) {
	var g TTSGeneration
	var created, activated string
	if err := row.Scan(&g.ID, &g.Language, &g.VoiceID, &g.Engine, &g.EngineVersion, &g.VoiceSettings, &g.BackgroundCPUPercent, &g.Status, &created, &activated); err != nil {
		return TTSGeneration{}, err
	}
	g.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
	if activated != "" {
		g.ActivatedAt, _ = time.Parse(time.RFC3339Nano, activated)
	}
	return g, nil
}

const generationColumns = `id,language,voice_id,engine,engine_version,voice_settings,background_cpu_percent,status,created_at,activated_at`

func (s *Store) CurrentTTSGeneration() (TTSGeneration, bool, error) {
	var id sql.NullInt64
	if err := s.db.QueryRow(`SELECT current_generation_id FROM tts_state WHERE id=1`).Scan(&id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return TTSGeneration{}, false, nil
		}
		return TTSGeneration{}, false, err
	}
	if !id.Valid {
		return TTSGeneration{}, false, nil
	}
	return s.GetTTSGeneration(id.Int64)
}

func (s *Store) GetTTSGeneration(id int64) (TTSGeneration, bool, error) {
	row := s.db.QueryRow(`SELECT `+generationColumns+` FROM tts_generations WHERE id=?`, id)
	g, err := scanGeneration(row)
	if errors.Is(err, sql.ErrNoRows) {
		return TTSGeneration{}, false, nil
	}
	if err != nil {
		return TTSGeneration{}, false, err
	}
	return g, true, nil
}

func (s *Store) ListTTSGenerationsByStatus(status string) ([]TTSGeneration, error) {
	if !isValidGenerationStatus(status) {
		return nil, fmt.Errorf("invalid generation status %q", status)
	}
	rows, err := s.db.Query(`SELECT `+generationColumns+` FROM tts_generations WHERE status=? ORDER BY id`, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []TTSGeneration{}
	for rows.Next() {
		g, err := scanGeneration(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

// UpdateTTSGenerationCPUPercent changes the background CPU limit applied to
// future jobs of a generation in place. Unlike language/voice, the CPU
// limit is not cache-relevant (it never changes which audio bytes are
// valid), so it must not trigger a new generation.
func (s *Store) UpdateTTSGenerationCPUPercent(id int64, percent int) error {
	if percent < 1 || percent > 100 {
		return fmt.Errorf("background cpu percent must be 1..100")
	}
	_, err := s.db.Exec(`UPDATE tts_generations SET background_cpu_percent=? WHERE id=?`, percent, id)
	return err
}

// ActivateTTSGenerationIfReady flips a building generation to active once
// its high/normal priority backlog is empty. Safe to call repeatedly; it is
// a pure function of the current job counts, so it is idempotent and can be
// re-evaluated after a crash without any extra bookkeeping.
func (s *Store) ActivateTTSGenerationIfReady(id int64) (bool, error) {
	g, ok, err := s.GetTTSGeneration(id)
	if err != nil || !ok || g.Status != GenerationBuilding {
		return false, err
	}
	counts, err := s.CountTTSJobs(id)
	if err != nil {
		return false, err
	}
	if !counts.ReadyForActive() {
		return false, nil
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	res, err := s.db.Exec(`UPDATE tts_generations SET status=?,activated_at=? WHERE id=? AND status=?`, GenerationActive, now, id, GenerationBuilding)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n > 0, err
}

// MarkTTSGenerationsStale transitions any current-pointer-detached generation
// still sitting in building/active into stale, so it becomes eligible for
// purge. Used on startup to catch a generation orphaned by a crash between
// the current-pointer update and its own status update (both happen in the
// same transaction in SwitchTTSGeneration, so this is a defensive sweep).
func (s *Store) MarkOrphanTTSGenerationsStale() error {
	var currentID sql.NullInt64
	if err := s.db.QueryRow(`SELECT current_generation_id FROM tts_state WHERE id=1`).Scan(&currentID); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if currentID.Valid {
		_, err := s.db.Exec(`UPDATE tts_generations SET status=? WHERE status IN (?,?) AND id<>?`, GenerationStale, GenerationBuilding, GenerationActive, currentID.Int64)
		return err
	}
	_, err := s.db.Exec(`UPDATE tts_generations SET status=? WHERE status IN (?,?)`, GenerationStale, GenerationBuilding, GenerationActive)
	return err
}

func (s *Store) MarkTTSGenerationPurging(id int64) error {
	_, err := s.db.Exec(`UPDATE tts_generations SET status=? WHERE id=? AND status=?`, GenerationPurging, id, GenerationStale)
	return err
}

// DeleteTTSGeneration removes a purging generation's cache and job rows and
// then the generation row itself. Deleting on-disk files is the caller's
// responsibility (internal/tts owns the cache directory layout); this only
// touches the database and is safe to call again if a previous attempt was
// interrupted, since DELETE on already-absent rows is a no-op.
func (s *Store) DeleteTTSGeneration(id int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.Exec(`DELETE FROM tts_cache WHERE generation_id=?`, id); err != nil {
		return err
	}
	if _, err = tx.Exec(`DELETE FROM tts_jobs WHERE generation_id=?`, id); err != nil {
		return err
	}
	if _, err = tx.Exec(`DELETE FROM tts_generations WHERE id=? AND status=?`, id, GenerationPurging); err != nil {
		return err
	}
	return tx.Commit()
}

// EnqueueTTSJob idempotently registers a text for synthesis against the
// given generation. Re-registering the same (generation, text_hash) is a
// no-op, matching EnsureText's contract for provider/media hooks.
func (s *Store) EnqueueTTSJob(generationID int64, seed TTSJobSeed) error {
	if !isValidJobPriority(seed.Priority) {
		return fmt.Errorf("invalid job priority %q", seed.Priority)
	}
	if strings.TrimSpace(seed.TextHash) == "" {
		return errors.New("job text_hash is required")
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	// Priority is a ratchet (high > normal > low) that never downgrades a
	// still-pending job -- important once multiple callers (an on-demand
	// HIGH request and a background bulk content scan) can race to
	// register the exact same text. A job already 'done' keeps its
	// priority untouched (moot, it is finished); a previously failed job
	// is reset from 'error' back to 'pending' so it gets retried instead
	// of being stuck forever once something re-registers the same text.
	_, err := s.db.Exec(`INSERT INTO tts_jobs(generation_id,priority,source_type,source_ref,normalized_text,text_hash,status,created_at,updated_at)
 VALUES(?,?,?,?,?,?,?,?,?)
 ON CONFLICT(generation_id,text_hash) DO UPDATE SET
 priority=CASE
   WHEN tts_jobs.status='done' THEN tts_jobs.priority
   WHEN tts_jobs.priority='high' OR excluded.priority='high' THEN 'high'
   WHEN tts_jobs.priority='normal' OR excluded.priority='normal' THEN 'normal'
   ELSE excluded.priority
 END,
 status=CASE WHEN tts_jobs.status='error' THEN 'pending' ELSE tts_jobs.status END,
 updated_at=excluded.updated_at`,
		generationID, seed.Priority, seed.SourceType, seed.SourceRef, seed.NormalizedText, seed.TextHash, JobPending, now, now)
	return err
}

// PromoteTTSJob raises an existing job (any status other than done) to high
// priority and back to pending, used for the on-demand cache-miss fallback.
func (s *Store) PromoteTTSJob(generationID int64, textHash string) error {
	_, err := s.db.Exec(`UPDATE tts_jobs SET priority=?,status=?,updated_at=? WHERE generation_id=? AND text_hash=? AND status<>?`,
		JobPriorityHigh, JobPending, time.Now().UTC().Format(time.RFC3339Nano), generationID, textHash, JobDone)
	return err
}

func scanJob(row interface{ Scan(dest ...any) error }) (TTSJob, error) {
	var j TTSJob
	if err := row.Scan(&j.ID, &j.GenerationID, &j.Priority, &j.SourceType, &j.SourceRef, &j.NormalizedText, &j.TextHash, &j.Status, &j.Attempts, &j.LastError); err != nil {
		return TTSJob{}, err
	}
	return j, nil
}

const jobColumns = `id,generation_id,priority,source_type,source_ref,normalized_text,text_hash,status,attempts,last_error`

// DequeueNextTTSJob atomically claims the highest-priority pending job for a
// generation (high > normal > low, then FIFO) and marks it running.
func (s *Store) DequeueNextTTSJob(generationID int64) (TTSJob, bool, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return TTSJob{}, false, err
	}
	defer tx.Rollback()
	row := tx.QueryRow(`SELECT `+jobColumns+` FROM tts_jobs WHERE generation_id=? AND status=?
 ORDER BY CASE priority WHEN 'high' THEN 0 WHEN 'normal' THEN 1 ELSE 2 END, id LIMIT 1`, generationID, JobPending)
	j, err := scanJob(row)
	if errors.Is(err, sql.ErrNoRows) {
		return TTSJob{}, false, nil
	}
	if err != nil {
		return TTSJob{}, false, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err = tx.Exec(`UPDATE tts_jobs SET status=?,updated_at=? WHERE id=?`, JobRunning, now, j.ID); err != nil {
		return TTSJob{}, false, err
	}
	if err = tx.Commit(); err != nil {
		return TTSJob{}, false, err
	}
	j.Status = JobRunning
	return j, true, nil
}

func (s *Store) CompleteTTSJob(id int64) error {
	_, err := s.db.Exec(`UPDATE tts_jobs SET status=?,updated_at=? WHERE id=?`, JobDone, time.Now().UTC().Format(time.RFC3339Nano), id)
	return err
}

func (s *Store) FailTTSJob(id int64, message string) error {
	_, err := s.db.Exec(`UPDATE tts_jobs SET status=?,attempts=attempts+1,last_error=?,updated_at=? WHERE id=?`,
		JobError, message, time.Now().UTC().Format(time.RFC3339Nano), id)
	return err
}

// ResetRunningTTSJobs requeues jobs left in "running" state by an unclean
// shutdown, so they are retried instead of stuck forever.
func (s *Store) ResetRunningTTSJobs() error {
	_, err := s.db.Exec(`UPDATE tts_jobs SET status=?,updated_at=? WHERE status=?`, JobPending, time.Now().UTC().Format(time.RFC3339Nano), JobRunning)
	return err
}

func (s *Store) CountTTSJobs(generationID int64) (TTSJobCounts, error) {
	rows, err := s.db.Query(`SELECT priority,status,COUNT(*) FROM tts_jobs WHERE generation_id=? GROUP BY priority,status`, generationID)
	if err != nil {
		return TTSJobCounts{}, err
	}
	defer rows.Close()
	var c TTSJobCounts
	for rows.Next() {
		var priority, status string
		var n int
		if err = rows.Scan(&priority, &status, &n); err != nil {
			return TTSJobCounts{}, err
		}
		switch {
		case status == JobDone:
			c.Done += n
		case status == JobError:
			c.Error += n
		case priority == JobPriorityHigh && status == JobPending:
			c.HighPending += n
		case priority == JobPriorityHigh && status == JobRunning:
			c.HighRunning += n
		case priority == JobPriorityNormal && status == JobPending:
			c.NormalPending += n
		case priority == JobPriorityNormal && status == JobRunning:
			c.NormalRunning += n
		case priority == JobPriorityLow && status == JobPending:
			c.LowPending += n
		case priority == JobPriorityLow && status == JobRunning:
			c.LowRunning += n
		}
	}
	return c, rows.Err()
}

// --- Voice catalog ---

func (s *Store) UpsertTTSVoice(v TTSVoice) error {
	if strings.TrimSpace(v.ID) == "" || strings.TrimSpace(v.Language) == "" || strings.TrimSpace(v.ModelPath) == "" {
		return errors.New("voice id, language and model_path are required")
	}
	if v.SpeakerID != nil && *v.SpeakerID < 0 {
		return errors.New("voice speaker_id must be nil (single-speaker) or a non-negative index")
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := s.db.Exec(`INSERT INTO tts_voices(id,engine,language,voice_family,quality,display_name,model_path,config_path,license,source_url,speaker_id,available,installed_at)
 VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)
 ON CONFLICT(id) DO UPDATE SET engine=excluded.engine,language=excluded.language,voice_family=excluded.voice_family,quality=excluded.quality,
 display_name=excluded.display_name,model_path=excluded.model_path,config_path=excluded.config_path,license=excluded.license,
 source_url=excluded.source_url,speaker_id=excluded.speaker_id,available=excluded.available`,
		v.ID, v.Engine, v.Language, v.VoiceFamily, v.Quality, v.DisplayName, v.ModelPath, v.ConfigPath, v.License, v.SourceURL, v.SpeakerID, v.Available, now)
	return err
}

func (s *Store) SetTTSVoiceAvailable(id string, available bool) error {
	_, err := s.db.Exec(`UPDATE tts_voices SET available=? WHERE id=?`, available, id)
	return err
}

func scanVoice(row interface{ Scan(dest ...any) error }) (TTSVoice, error) {
	var v TTSVoice
	var speakerID sql.NullInt64
	var available int
	var installed string
	if err := row.Scan(&v.ID, &v.Engine, &v.Language, &v.VoiceFamily, &v.Quality, &v.DisplayName, &v.ModelPath, &v.ConfigPath, &v.License, &v.SourceURL, &speakerID, &available, &installed); err != nil {
		return TTSVoice{}, err
	}
	if speakerID.Valid {
		id := int(speakerID.Int64)
		v.SpeakerID = &id
	}
	v.Available = available == 1
	v.InstalledAt, _ = time.Parse(time.RFC3339Nano, installed)
	return v, nil
}

const voiceColumns = `id,engine,language,voice_family,quality,display_name,model_path,config_path,license,source_url,speaker_id,available,installed_at`

func (s *Store) GetTTSVoice(id string) (TTSVoice, bool, error) {
	row := s.db.QueryRow(`SELECT `+voiceColumns+` FROM tts_voices WHERE id=?`, id)
	v, err := scanVoice(row)
	if errors.Is(err, sql.ErrNoRows) {
		return TTSVoice{}, false, nil
	}
	if err != nil {
		return TTSVoice{}, false, err
	}
	return v, true, nil
}

func (s *Store) ListTTSVoicesByLanguage(language string) ([]TTSVoice, error) {
	rows, err := s.db.Query(`SELECT `+voiceColumns+` FROM tts_voices WHERE language=? AND available=1 ORDER BY voice_family,quality`, language)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []TTSVoice{}
	for rows.Next() {
		v, err := scanVoice(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (s *Store) ListTTSLanguagesWithVoices() ([]string, error) {
	rows, err := s.db.Query(`SELECT DISTINCT language FROM tts_voices WHERE available=1 ORDER BY language`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var lang string
		if err = rows.Scan(&lang); err != nil {
			return nil, err
		}
		out = append(out, lang)
	}
	return out, rows.Err()
}
