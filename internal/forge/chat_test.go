package forge

import (
	"context"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mrdulasolutions/skillforge/internal/ai"
	"github.com/mrdulasolutions/skillforge/internal/tui"
)

// stubProvider implements ai.Provider (not Streamer).
type stubProvider struct{ reply string }

func (s stubProvider) Name() string    { return "stub" }
func (s stubProvider) Available() bool { return true }
func (s stubProvider) Complete(_ context.Context, _ ai.Request) (*ai.Response, error) {
	return &ai.Response{Text: s.reply}, nil
}

func newTestModel(t *testing.T, draft Drafter) model {
	t.Helper()
	m := newModel(context.Background(), stubProvider{reply: "ready."}, draft, tui.WizardResult{}, t.TempDir())
	nm, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	return nm.(model)
}

func TestModelReadyHeuristic(t *testing.T) {
	// Reply without a question mark => ready (+ hint).
	m := newTestModel(t, nil)
	if mm := mustUpdate(t, m, streamDoneMsg{full: "I have enough to build this."}); mm.phase != phaseReady {
		t.Fatalf("expected phaseReady, got %v", mm.phase)
	}
	// Reply with a question => stays interviewing.
	m2 := newTestModel(t, nil)
	if mm := mustUpdate(t, m2, streamDoneMsg{full: "What should it output?"}); mm.phase != phaseInterview {
		t.Fatalf("expected phaseInterview, got %v", mm.phase)
	}
}

func TestModelReviewConfirm(t *testing.T) {
	m := newTestModel(t, nil)
	m.phase = phaseReview
	m.spec = sampleSpec()
	mm := mustUpdate(t, m, submitMsg{text: "yes"})
	if mm.result == nil {
		t.Fatal("expected a result on confirm")
	}
	if mm.result.Name != "alphabet-reciter" || mm.result.BodyMarkdown != m.spec.Body {
		t.Fatalf("unexpected result: %+v", mm.result)
	}
}

func TestModelDraftPipeline(t *testing.T) {
	draft := func(_ context.Context, _ []ai.Message, _ *ai.SkillSpec, _ string) (*ai.SkillSpec, error) {
		return &ai.SkillSpec{Title: "🔥", Name: "Bad Name", Description: "Has <x>", Type: "weird", Body: "## B\n"}, nil
	}
	m := newTestModel(t, draft)
	m.phase = phaseReady
	dm := draftCmd(context.Background(), draft, m.transcript, nil, "")().(draftDoneMsg)
	mm := mustUpdate(t, m, dm)
	if mm.phase != phaseReview {
		t.Fatalf("expected phaseReview after draft, got %v", mm.phase)
	}
	if mm.spec == nil || mm.spec.Name == "" || mm.spec.Type != "skill" {
		t.Fatalf("spec not repaired: %+v", mm.spec)
	}
}

func TestModelRefineWiring(t *testing.T) {
	var gotPrior *ai.SkillSpec
	var gotInstr string
	draft := func(_ context.Context, _ []ai.Message, prior *ai.SkillSpec, instr string) (*ai.SkillSpec, error) {
		gotPrior, gotInstr = prior, instr
		return sampleSpec(), nil
	}
	m := newTestModel(t, draft)
	m.phase = phaseReview
	m.spec = sampleSpec()
	_, cmd := m.Update(submitMsg{text: "make it shorter"})
	drainCmd(cmd) // executes the draft cmd, which records prior+instruction
	if gotPrior == nil || gotInstr != "make it shorter" {
		t.Fatalf("drafter not wired with prior+instruction: prior=%v instr=%q", gotPrior, gotInstr)
	}
}

func TestModelMultiDeltaNoPanic(t *testing.T) {
	// Regression: a value-copied strings.Builder panicked on the 2nd delta.
	m := newTestModel(t, nil)
	m.busy = true
	var tm tea.Model = m
	for _, d := range []string{"hel", "lo ", "wor", "ld"} {
		tm, _ = tm.Update(streamDeltaMsg{delta: d}) // would panic pre-fix
	}
	if got := tm.(model).pending; got != "hello world" {
		t.Fatalf("pending = %q, want %q", got, "hello world")
	}
}

func TestModelCancel(t *testing.T) {
	m := newTestModel(t, nil)
	if mm := mustUpdate(t, m, submitMsg{text: "/cancel"}); mm.result != nil || mm.degrade {
		t.Fatal("cancel must not set a result or degrade")
	}
}

func TestSlashMenuFilter(t *testing.T) {
	m := newTestModel(t, nil)
	m.ta.SetValue("/")
	m.updateMenu()
	if len(m.menu) != len(slashCmds) {
		t.Fatalf("'/' should list all %d, got %d", len(slashCmds), len(m.menu))
	}
	m.ta.SetValue("/co")
	m.updateMenu()
	if len(m.menu) != 1 || m.menu[0].name != "/compliance" {
		t.Fatalf("'/co' -> %+v", m.menu)
	}
	m.ta.SetValue("/zzz")
	m.updateMenu()
	if len(m.menu) != 0 {
		t.Fatal("'/zzz' should match nothing")
	}
	m.ta.SetValue("hello there")
	m.updateMenu()
	if m.menu != nil {
		t.Fatal("non-slash input should close the menu")
	}
}

func TestSlashRunActions(t *testing.T) {
	if nm, _ := newTestModel(t, nil).runSlash("/plugin"); nm.(model).seed.Type != "plugin" {
		t.Error("/plugin should set type=plugin")
	}
	if nm, _ := newTestModel(t, nil).runSlash("/compliance"); !nm.(model).seed.Compliance {
		t.Error("/compliance should enable compliance")
	}
	if nm, _ := newTestModel(t, nil).runSlash("/cancel"); nm.(model).result != nil {
		t.Error("/cancel must not set a result")
	}
	m := newTestModel(t, nil)
	m.transcript = []ai.Message{{Role: "user", Content: "x"}}
	m.phase = phaseReview
	if nm, _ := m.runSlash("/new"); len(nm.(model).transcript) != 0 || nm.(model).phase != phaseInterview {
		t.Error("/new should reset the conversation")
	}
}

func mustUpdate(t *testing.T, m model, msg tea.Msg) model {
	t.Helper()
	nm, _ := m.Update(msg)
	return nm.(model)
}

// drainCmd executes a (possibly batched) tea.Cmd and returns the produced msgs.
func drainCmd(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		var out []tea.Msg
		for _, c := range batch {
			out = append(out, drainCmd(c)...)
		}
		return out
	}
	if msg == nil {
		return nil
	}
	return []tea.Msg{msg}
}
