package tts

import (
	"fmt"
	"os"

	"mupibox/internal/store"
	"mupibox/internal/tts/manifest"
)

// SyncReport summarizes what SyncInstalledVoices found on one pass, for the
// installer and the admin UI to display.
type SyncReport struct {
	Installed []string // voice IDs upserted as available
	Missing   []string // manifest voice IDs with no (or incomplete) files on disk
}

// SyncInstalledVoices reconciles the on-disk voices directory against the
// embedded manifest and the tts_voices table: any manifest entry whose
// model+config files are both present on disk is upserted as available;
// anything else is marked unavailable (if it was previously known) rather
// than deleted, so an admin can see what is configured but missing. This is
// safe to call repeatedly (e.g. after every installer run or on service
// start) and never touches tts_generations/tts_jobs.
func SyncInstalledVoices(st *store.Store, voicesDir string) (SyncReport, error) {
	voices, err := manifest.Load()
	if err != nil {
		return SyncReport{}, err
	}
	var report SyncReport
	for _, v := range voices {
		modelPath := manifest.ModelPath(voicesDir, v)
		configPath := manifest.ConfigPath(voicesDir, v)
		available := fileExistsNonEmpty(modelPath) && fileExistsNonEmpty(configPath)
		id := v.ID()
		if available {
			if err := st.UpsertTTSVoice(store.TTSVoice{
				ID: id, Engine: "piper", Language: v.Language, VoiceFamily: v.VoiceFamily, Quality: v.Quality,
				DisplayName: v.DisplayName, ModelPath: modelPath, ConfigPath: configPath,
				License: v.License, SourceURL: v.SourceURL, SpeakerID: v.SpeakerID, Available: true,
			}); err != nil {
				return report, fmt.Errorf("upsert voice %s: %w", id, err)
			}
			report.Installed = append(report.Installed, id)
			continue
		}
		if _, ok, err := st.GetTTSVoice(id); err != nil {
			return report, err
		} else if ok {
			if err := st.SetTTSVoiceAvailable(id, false); err != nil {
				return report, fmt.Errorf("mark voice %s unavailable: %w", id, err)
			}
		}
		report.Missing = append(report.Missing, id)
	}
	return report, nil
}

func fileExistsNonEmpty(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular() && info.Size() > 0
}
