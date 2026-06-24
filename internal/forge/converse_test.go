package forge

import (
	"strings"
	"testing"

	"github.com/mrdulasolutions/skillforge/internal/ai"
	"github.com/mrdulasolutions/skillforge/internal/tui"
)

func sampleSpec() *ai.SkillSpec {
	return &ai.SkillSpec{
		Title:       "Alphabet Reciter",
		Name:        "alphabet-reciter",
		Description: "Recites the alphabet. Use when asked to list letters.",
		Body:        "## When to use this skill\n\nUse when asked.\n",
		Type:        "skill",
		Evals:       true,
	}
}

func TestRepair(t *testing.T) {
	got := repair(&ai.SkillSpec{Title: "🔥🔥", Name: "Bad Name!", Description: "Does <stuff>", Type: "weird"}, t.TempDir())
	if got.Name == "" {
		t.Fatal("expected a fallback name")
	}
	if strings.ContainsAny(got.Description, "<>") {
		t.Fatalf("angle brackets not stripped: %q", got.Description)
	}
	if got.Type != "skill" {
		t.Fatalf("type not normalized: %q", got.Type)
	}
}

func TestFinalizeComplianceOR(t *testing.T) {
	spec := sampleSpec()
	res := finalize(spec, tui.WizardResult{Compliance: true})
	if !res.Compliance {
		t.Fatal("expected compliance OR'd with seed")
	}
	if res.BodyMarkdown != spec.Body {
		t.Fatal("body not carried into result")
	}
}

func TestCardStringTruncates(t *testing.T) {
	spec := sampleSpec()
	spec.Body = strings.Repeat("x", 1000)
	out := cardString(spec)
	if !strings.Contains(out, "…") {
		t.Fatal("expected body truncation marker")
	}
	if !strings.Contains(out, spec.Name) {
		t.Fatal("expected name in card")
	}
}

func TestIsAffirmative(t *testing.T) {
	for _, y := range []string{"yes", "go", "build it", "LGTM", "looks good"} {
		if !isAffirmative(y) {
			t.Errorf("expected affirmative: %q", y)
		}
	}
	for _, n := range []string{"no", "make it shorter", "what about X"} {
		if isAffirmative(n) {
			t.Errorf("expected non-affirmative: %q", n)
		}
	}
}
