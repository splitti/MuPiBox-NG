package manifest

import "testing"

func TestLoadValidatesAndReturnsAllVoices(t *testing.T) {
	voices, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(voices) == 0 {
		t.Fatal("expected a non-empty voice catalog")
	}
	ids := map[string]bool{}
	for _, v := range voices {
		if ids[v.ID()] {
			t.Fatalf("duplicate voice id %s", v.ID())
		}
		ids[v.ID()] = true
		if v.Tier != TierMandatory && v.Tier != TierOptional {
			t.Fatalf("voice %s has invalid tier %q", v.ID(), v.Tier)
		}
	}
}

func TestMandatoryCoversTheSevenMainLanguages(t *testing.T) {
	voices, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	required := []string{"de-DE", "en-GB", "en-US", "fr-FR", "es-ES", "nl-NL", "sv-SE", "ru-RU"}
	// sv-SE and ru-RU plus the five others: the spec's "at least" list is
	// de, en, fr, es, nl, sv, ru -- en counted once here since en-GB and
	// en-US are both present.
	got := map[string]bool{}
	for _, v := range Mandatory(voices) {
		got[v.Language] = true
		if v.License == "" {
			t.Fatalf("mandatory voice %s must have a resolved license", v.ID())
		}
	}
	for _, lang := range required {
		if !got[lang] {
			t.Fatalf("expected a mandatory voice for %s, mandatory set: %v", lang, got)
		}
	}
}

func TestModelAndConfigPathsAreFlatFilesUnderRoot(t *testing.T) {
	voices, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	v := voices[0]
	model := ModelPath("/srv/voices", v)
	config := ConfigPath("/srv/voices", v)
	if model == config {
		t.Fatal("model and config paths must differ")
	}
	if model != "/srv/voices/"+v.ModelFileName() {
		t.Fatalf("unexpected model path: %s", model)
	}
}
