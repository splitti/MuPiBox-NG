// Package manifest is the single, versioned source of truth for which
// Piper voices MuPiBox-NG knows about: language, quality, license, download
// URLs and (for multi-speaker models) a pinned speaker index. Both the
// installer (downloads) and the running service (matches installed files to
// catalog metadata) read this same embedded voices.json, so there is only
// one place to add or correct a voice.
//
// Voice source and format are unaffected by the rhasspy/piper -> OHF-Voice/
// piper1-gpl migration (see internal/tts/piper.go): models still come from
// huggingface.co/rhasspy/piper-voices as .onnx+.onnx.json pairs. Per-voice
// licenses below were verified against each voice's own MODEL_CARD; the
// engine project's own general "personal use and research" usage note is
// about the piper1-gpl *software* (GPL-3.0, invoked as a subprocess) and is
// independent of a voice's own license.
package manifest

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
)

//go:embed voices.json
var voicesJSON []byte

type Tier string

const (
	TierMandatory Tier = "mandatory"
	TierOptional  Tier = "optional"
)

type Voice struct {
	Language    string `json:"language"`
	VoiceFamily string `json:"voice_family"`
	Quality     string `json:"quality"`
	DisplayName string `json:"display_name"`
	ModelURL    string `json:"model_url"`
	ConfigURL   string `json:"config_url"`
	License     string `json:"license"`
	SourceURL   string `json:"source_url"`
	SpeakerID   *int   `json:"speaker_id"`
	Tier        Tier   `json:"tier"`
}

// ID is the stable catalog key used as the tts_voices primary key and as
// the on-disk file base name, so it must be filesystem-safe.
func (v Voice) ID() string {
	return fmt.Sprintf("%s-%s-%s", v.Language, v.VoiceFamily, v.Quality)
}

func (v Voice) ModelFileName() string  { return v.ID() + ".onnx" }
func (v Voice) ConfigFileName() string { return v.ID() + ".onnx.json" }

type catalog struct {
	Engine     string  `json:"engine"`
	Source     string  `json:"source"`
	VerifiedAt string  `json:"verified_at"`
	Voices     []Voice `json:"voices"`
}

// Load parses and validates the embedded voice catalog. It never touches
// the network or the filesystem beyond the compiled-in JSON.
func Load() ([]Voice, error) {
	var c catalog
	if err := json.Unmarshal(voicesJSON, &c); err != nil {
		return nil, fmt.Errorf("decode voice manifest: %w", err)
	}
	seen := map[string]bool{}
	for i, v := range c.Voices {
		if strings.TrimSpace(v.Language) == "" || strings.TrimSpace(v.VoiceFamily) == "" || strings.TrimSpace(v.Quality) == "" {
			return nil, fmt.Errorf("manifest entry %d: language, voice_family and quality are required", i)
		}
		if v.Tier != TierMandatory && v.Tier != TierOptional {
			return nil, fmt.Errorf("manifest entry %s: invalid tier %q", v.ID(), v.Tier)
		}
		if strings.TrimSpace(v.ModelURL) == "" || strings.TrimSpace(v.ConfigURL) == "" {
			return nil, fmt.Errorf("manifest entry %s: model_url and config_url are required", v.ID())
		}
		if strings.TrimSpace(v.License) == "" || strings.TrimSpace(v.SourceURL) == "" {
			return nil, fmt.Errorf("manifest entry %s: license and source_url are required", v.ID())
		}
		if v.SpeakerID != nil && *v.SpeakerID < 0 {
			return nil, fmt.Errorf("manifest entry %s: speaker_id must be >= 0 when set", v.ID())
		}
		if seen[v.ID()] {
			return nil, fmt.Errorf("duplicate manifest entry %s", v.ID())
		}
		seen[v.ID()] = true
	}
	return c.Voices, nil
}

// Mandatory returns only the voices that the installer must install by
// default (the seven main languages), in manifest order.
func Mandatory(voices []Voice) []Voice {
	out := make([]Voice, 0, len(voices))
	for _, v := range voices {
		if v.Tier == TierMandatory {
			out = append(out, v)
		}
	}
	return out
}

// ModelPath and ConfigPath follow the on-disk layout convention shared by
// the installer and the running service: one flat directory of
// <id>.onnx / <id>.onnx.json pairs.
func ModelPath(root string, v Voice) string  { return filepath.Join(root, v.ModelFileName()) }
func ConfigPath(root string, v Voice) string { return filepath.Join(root, v.ConfigFileName()) }
