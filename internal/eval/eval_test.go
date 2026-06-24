package eval

import (
	"context"
	"strings"
	"testing"

	"github.com/mrdulasolutions/skillforge/internal/ai"
)

// fakeProvider returns the configured verdict for judge calls (system contains
// "strict evaluator") and a canned output otherwise.
type fakeProvider struct{ verdict string }

func (fakeProvider) Name() string    { return "fake" }
func (fakeProvider) Available() bool { return true }
func (f fakeProvider) Complete(_ context.Context, req ai.Request) (*ai.Response, error) {
	if strings.Contains(req.System, "strict evaluator") {
		return &ai.Response{Text: f.verdict}, nil
	}
	return &ai.Response{Text: "output for " + req.Messages[0].Content}, nil
}

func TestRunAllPassWithBaseline(t *testing.T) {
	f := &File{SkillName: "s", Evals: []Case{{ID: 1, Prompt: "p", Expectations: []string{"a", "b"}}}}
	rep, err := Run(context.Background(), fakeProvider{verdict: "PASS"}, "m", "body", f, true, nil)
	if err != nil {
		t.Fatal(err)
	}
	if rep.WithSkillPassRate != 1.0 || rep.BaselinePassRate != 1.0 {
		t.Fatalf("rates: ws=%v bl=%v", rep.WithSkillPassRate, rep.BaselinePassRate)
	}
	if len(rep.Cases) != 1 || len(rep.Cases[0].WithSkill.Expectations) != 2 || rep.Cases[0].Baseline == nil {
		t.Fatalf("case shape: %+v", rep.Cases)
	}
}

func TestRunAllFailNoBaseline(t *testing.T) {
	f := &File{Evals: []Case{{ID: 1, Expectations: []string{"a"}}}}
	rep, err := Run(context.Background(), fakeProvider{verdict: "FAIL"}, "m", "body", f, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if rep.WithSkillPassRate != 0 {
		t.Fatalf("expected 0 pass rate, got %v", rep.WithSkillPassRate)
	}
	if rep.Baseline || rep.Cases[0].Baseline != nil {
		t.Fatal("baseline should be off")
	}
}

func TestProgressCallback(t *testing.T) {
	f := &File{Evals: []Case{{ID: 1, Expectations: []string{"a"}}, {ID: 2, Expectations: []string{"b"}}}}
	var calls int
	_, _ = Run(context.Background(), fakeProvider{verdict: "PASS"}, "m", "body", f, false, func(done, total int) {
		calls++
		if total != 2 {
			t.Errorf("total = %d", total)
		}
	})
	if calls != 2 {
		t.Fatalf("expected 2 progress calls, got %d", calls)
	}
}

func TestJudgeStrictVerdict(t *testing.T) {
	// Regression: a FAIL verdict containing the word "pass" must score 0.
	f := &File{Evals: []Case{{ID: 1, Expectations: []string{"a"}}}}
	rep, err := Run(context.Background(), fakeProvider{verdict: "FAIL — the output does not pass"}, "m", "body", f, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if rep.WithSkillPassRate != 0 {
		t.Fatalf("expected 0 pass rate, got %v", rep.WithSkillPassRate)
	}
}

func TestHTML(t *testing.T) {
	f := &File{SkillName: "demo", Evals: []Case{{ID: 1, Prompt: "p", Expectations: []string{"a"}}}}
	rep, _ := Run(context.Background(), fakeProvider{verdict: "PASS"}, "m", "body", f, false, nil)
	html, err := rep.HTML()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(html, "demo") || !strings.Contains(html, "100%") {
		t.Fatal("html missing skill name or pass rate")
	}
}
