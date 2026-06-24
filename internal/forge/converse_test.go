package forge

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/mrdulasolutions/skillforge/internal/ai"
	"github.com/mrdulasolutions/skillforge/internal/tui"
)

// stubProvider returns a fixed interview reply and does not implement Streamer,
// so Converse uses Complete.
type stubProvider struct{ reply string }

func (s stubProvider) Name() string    { return "stub" }
func (s stubProvider) Available() bool { return true }
func (s stubProvider) Complete(_ context.Context, _ ai.Request) (*ai.Response, error) {
	return &ai.Response{Text: s.reply, Model: "stub"}, nil
}

func goodSpec() *ai.SkillSpec {
	return &ai.SkillSpec{
		Title:       "Alphabet Reciter",
		Name:        "alphabet-reciter",
		Description: "Recites the alphabet. Use when asked to list letters.",
		Body:        "## When to use this skill\n\nUse when asked.\n",
		Type:        "skill",
		Evals:       true,
	}
}

func TestConverseHappyPath(t *testing.T) {
	p := stubProvider{reply: "Great — I have enough to build this. Defaults: plain skill, evals on, compliance off."}
	drafted := 0
	draft := func(_ context.Context, _ []ai.Message, _ *ai.SkillSpec, _ string) (*ai.SkillSpec, error) {
		drafted++
		return goodSpec(), nil
	}
	in := strings.NewReader("a skill that recites the alphabet\ngo\nyes\n")
	var out bytes.Buffer
	res, ok, err := Converse(context.Background(), in, &out, p, draft, tui.WizardResult{}, t.TempDir())
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	if res.Name != "alphabet-reciter" || res.BodyMarkdown == "" {
		t.Fatalf("unexpected result: %+v", res)
	}
	if drafted != 1 {
		t.Fatalf("expected 1 draft, got %d", drafted)
	}
}

func TestConverseCancel(t *testing.T) {
	p := stubProvider{reply: "Tell me more — what should it output?"}
	draft := func(_ context.Context, _ []ai.Message, _ *ai.SkillSpec, _ string) (*ai.SkillSpec, error) {
		t.Fatal("draft must not be called on cancel")
		return nil, nil
	}
	in := strings.NewReader("an idea\n/cancel\n")
	var out bytes.Buffer
	if _, ok, err := Converse(context.Background(), in, &out, p, draft, tui.WizardResult{}, t.TempDir()); ok || err != nil {
		t.Fatalf("expected clean cancel, ok=%v err=%v", ok, err)
	}
}

func TestConverseRefine(t *testing.T) {
	p := stubProvider{reply: "Ready to draft. Defaults set."}
	var instrs []string
	draft := func(_ context.Context, _ []ai.Message, prior *ai.SkillSpec, instr string) (*ai.SkillSpec, error) {
		instrs = append(instrs, instr)
		s := goodSpec()
		if prior != nil && instr != "" {
			s.Description = "Refined: " + s.Description
		}
		return s, nil
	}
	in := strings.NewReader("idea\ngo\nmake it shorter\nyes\n")
	var out bytes.Buffer
	res, ok, err := Converse(context.Background(), in, &out, p, draft, tui.WizardResult{}, t.TempDir())
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	if len(instrs) < 2 || instrs[1] != "make it shorter" {
		t.Fatalf("expected refine instruction, got %v", instrs)
	}
	if !strings.HasPrefix(res.Description, "Refined:") {
		t.Fatalf("expected refined description, got %q", res.Description)
	}
}

func TestConverseRepairsInvalidSpec(t *testing.T) {
	p := stubProvider{reply: "Ready."}
	draft := func(_ context.Context, _ []ai.Message, _ *ai.SkillSpec, _ string) (*ai.SkillSpec, error) {
		// All-emoji title, angle brackets in description, junk name.
		return &ai.SkillSpec{Title: "🔥🔥", Name: "Bad Name!", Description: "Does <stuff>", Body: "## When\n", Type: "weird", Evals: true}, nil
	}
	in := strings.NewReader("idea\ngo\nyes\n")
	var out bytes.Buffer
	res, ok, err := Converse(context.Background(), in, &out, p, draft, tui.WizardResult{}, t.TempDir())
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	if res.Name == "" {
		t.Fatal("repair should have produced a fallback name")
	}
	if strings.ContainsAny(res.Description, "<>") {
		t.Fatalf("angle brackets not stripped: %q", res.Description)
	}
	if res.Type != "skill" {
		t.Fatalf("type not normalized: %q", res.Type)
	}
}
