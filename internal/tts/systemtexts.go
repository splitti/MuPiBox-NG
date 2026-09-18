package tts

import "strings"

// StaticSystemTexts is the small, fixed set of touch-UI phrases that are not
// data-driven by navigation/media content and should therefore be ready
// with HIGH priority right after any language/voice switch (see
// docs/tts.md). Kept deliberately minimal: MuPiBox-NG's player UI is
// data-driven (categories/media come from navigation, not hardcoded
// strings -- see CLAUDE.md), so today the only genuinely fixed, spoken
// phrase is the welcome greeting used as the canonical "voice test" sample.
// Only languages the project already ships admin-UI translations for (de,
// en) are covered here, to avoid guessing unverified translations for the
// other manifest languages; extend this alongside a verified translation
// once one is available.
type StaticSystemTexts struct{}

var staticTexts = map[string]map[string]string{
	"de": {"welcome": "Hallo! Ich bin deine MuPiBox."},
	"en": {"welcome": "Hello! I am your MuPiBox."},
}

func (StaticSystemTexts) ForLanguage(language string) map[string]string {
	base := strings.ToLower(strings.SplitN(language, "-", 2)[0])
	return staticTexts[base]
}
