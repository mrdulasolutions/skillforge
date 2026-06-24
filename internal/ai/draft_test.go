package ai

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

type fakeProvider struct {
	replies []string
	calls   int
}

func (f *fakeProvider) Name() string    { return "fake" }
func (f *fakeProvider) Available() bool { return true }
func (f *fakeProvider) Complete(_ context.Context, _ Request) (*Response, error) {
	if f.calls >= len(f.replies) {
		return nil, fmt.Errorf("no more replies")
	}
	r := f.replies[f.calls]
	f.calls++
	return &Response{Text: r, Model: "fake"}, nil
}

func TestExtractJSON(t *testing.T) {
	obj := `{"title":"T","name":"t","description":"d","body":"has } a brace","type":"skill","evals":true,"compliance":false}`
	cases := []string{
		obj,
		"```json\n" + obj + "\n```",
		"Here is your skill:\n" + obj + "\nHope that helps!",
		"```\n" + obj + "\n```",
	}
	for _, in := range cases {
		if got := extractJSON(in); got != obj {
			t.Errorf("extractJSON mismatch\n in=%q\nout=%q", in, got)
		}
	}
	if extractJSON("no json here") != "" {
		t.Error("expected empty for input with no JSON")
	}
}

func TestDraftSkill(t *testing.T) {
	good := `{"title":"Alphabet Reciter","name":"alphabet-reciter","description":"Recites the alphabet. Use when asked.","body":"## When to use\n...","type":"skill","evals":true,"compliance":false}`

	p := &fakeProvider{replies: []string{good}}
	spec, err := DraftSkill(context.Background(), p, "m", nil, nil, "")
	if err != nil || spec.Name != "alphabet-reciter" || !spec.Evals {
		t.Fatalf("clean draft: spec=%+v err=%v", spec, err)
	}

	p = &fakeProvider{replies: []string{"```json\n" + good + "\n```"}}
	if spec, err = DraftSkill(context.Background(), p, "m", nil, nil, ""); err != nil || spec.Title != "Alphabet Reciter" {
		t.Fatalf("fenced draft: spec=%+v err=%v", spec, err)
	}

	p = &fakeProvider{replies: []string{"sorry, no json", good}}
	if spec, err = DraftSkill(context.Background(), p, "m", nil, nil, ""); err != nil || spec.Name != "alphabet-reciter" {
		t.Fatalf("retry draft: spec=%+v err=%v", spec, err)
	}
	if p.calls != 2 {
		t.Fatalf("expected 2 calls (one retry), got %d", p.calls)
	}

	p = &fakeProvider{replies: []string{"nope", "still nope"}}
	if _, err = DraftSkill(context.Background(), p, "m", nil, nil, ""); err == nil {
		t.Fatal("expected error after two unparseable replies")
	}
}

func TestCleanDescriptionExported(t *testing.T) {
	got := CleanDescription("Does <stuff>.\nSecond line")
	if strings.ContainsAny(got, "<>") {
		t.Fatalf("angle brackets not stripped: %q", got)
	}
	if strings.Contains(got, "\n") {
		t.Fatalf("not single-lined: %q", got)
	}
}
