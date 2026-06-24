// Package forge runs the conversational skill-building REPL: a streaming
// interview, an AI-drafted proposal, natural-language refinement, and confirm.
package forge

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/mrdulasolutions/skillforge/internal/ai"
	"github.com/mrdulasolutions/skillforge/internal/skill"
	"github.com/mrdulasolutions/skillforge/internal/tui"
)

// ErrDegrade tells the caller to fall back to the offline form (provider died
// mid-conversation, not a user cancel).
var ErrDegrade = errors.New("ai provider unavailable")

// Drafter is the AI seam (a closure around ai.DraftSkill), injectable for tests.
type Drafter func(ctx context.Context, transcript []ai.Message, prior *ai.SkillSpec, instruction string) (*ai.SkillSpec, error)

// Converse runs the interview → draft → refine → confirm loop. ok=false means
// the user cancelled (nothing written). On confirm it returns a WizardResult
// with BodyMarkdown populated, ready for skill.Scaffold. parent is the directory
// the skill will be created in (used to keep the derived name unique).
func Converse(ctx context.Context, in io.Reader, out io.Writer, p ai.Provider, draft Drafter, seed tui.WizardResult, parent string) (tui.WizardResult, bool, error) {
	r := bufio.NewReader(in)
	transcript := []ai.Message{}
	var spec *ai.SkillSpec
	phase := "interview"

	fail := func(err error) (tui.WizardResult, bool, error) {
		if ctx.Err() != nil {
			fmt.Fprintln(out)
			fmt.Fprintln(out, tui.Muted.Render("cancelled — nothing written"))
			return seed, false, nil
		}
		fmt.Fprintln(out, tui.Warn("model unreachable — switching to the offline form ("+err.Error()+")"))
		return seed, false, ErrDegrade
	}

	fmt.Fprintln(out, tui.Muted.Render("Describe the skill you want — plain words are perfect. (/cancel to quit)"))

	pending := strings.TrimSpace(seed.Description) // optional seed from -d / arg
	for {
		var line string
		if pending != "" {
			line, pending = pending, ""
			fmt.Fprintln(out, userGlyph()+line)
		} else {
			fmt.Fprint(out, userGlyph())
			l, err := readLine(r)
			if err != nil {
				return seed, false, nil // EOF / Ctrl-D
			}
			line = strings.TrimSpace(l)
		}
		if line == "" {
			continue
		}
		if isCancel(line) {
			fmt.Fprintln(out, tui.Muted.Render("cancelled — nothing written"))
			return seed, false, nil
		}

		if phase == "review" {
			if isAffirmative(line) || isCreateCmd(line) {
				return finalize(spec, seed), true, nil
			}
			transcript = append(transcript, ai.Message{Role: "user", Content: line})
			s, err := drafting(ctx, out, draft, transcript, spec, line, parent)
			if err != nil {
				return fail(err)
			}
			spec = s
			renderCard(out, spec)
			continue
		}

		// interview / ready
		if isCreateCmd(line) || (phase == "ready" && isAffirmative(line)) {
			transcript = append(transcript, ai.Message{Role: "user", Content: line})
			s, err := drafting(ctx, out, draft, transcript, nil, "", parent)
			if err != nil {
				return fail(err)
			}
			spec = s
			phase = "review"
			renderCard(out, spec)
			continue
		}

		transcript = append(transcript, ai.Message{Role: "user", Content: line})
		reply, err := interviewTurn(ctx, out, p, transcript)
		if err != nil {
			return fail(err)
		}
		transcript = append(transcript, ai.Message{Role: "assistant", Content: reply})
		if strings.Contains(reply, "?") {
			phase = "interview"
		} else {
			phase = "ready"
			fmt.Fprintln(out, tui.Muted.Render(`(say "go" to build it, or add more detail)`))
		}
	}
}

func interviewTurn(ctx context.Context, out io.Writer, p ai.Provider, transcript []ai.Message) (string, error) {
	resp, err := streamOrComplete(ctx, p, ai.Request{
		Model:       ai.DefaultModel(p),
		System:      ai.InterviewSystem,
		Messages:    transcript,
		Temperature: 0.5,
		MaxTokens:   500,
	}, out)
	fmt.Fprintln(out)
	if err != nil {
		return "", err
	}
	return resp.Text, nil
}

func streamOrComplete(ctx context.Context, p ai.Provider, req ai.Request, out io.Writer) (*ai.Response, error) {
	if s, ok := p.(ai.Streamer); ok {
		return s.Stream(ctx, req, func(d string) { io.WriteString(out, d) })
	}
	resp, err := p.Complete(ctx, req)
	if err == nil {
		io.WriteString(out, resp.Text)
	}
	return resp, err
}

func drafting(ctx context.Context, out io.Writer, draft Drafter, transcript []ai.Message, prior *ai.SkillSpec, instr, parent string) (*ai.SkillSpec, error) {
	fmt.Fprintln(out, tui.Muted.Render(tui.GlyphSpark+" drafting your skill…"))
	s, err := draft(ctx, transcript, prior, instr)
	if err != nil {
		return nil, err
	}
	return repair(s, parent), nil
}

// repair makes a model-proposed spec safe to write: a valid, unique name and a
// cleaned description. Slugify guarantees ValidateName passes.
func repair(spec *ai.SkillSpec, parent string) *ai.SkillSpec {
	if spec == nil {
		return nil
	}
	name := skill.Slugify(spec.Title)
	if name == "" {
		name = skill.Slugify(spec.Name)
	}
	if name == "" {
		name = "skill"
		if spec.Type == "plugin" {
			name = "plugin"
		}
	}
	name = skill.UniqueSlug(parent, name)

	desc := ai.CleanDescription(spec.Description)
	if desc == "" {
		desc = "TODO: one-line, trigger-rich description. Use when the user ..."
	}
	typ := spec.Type
	if typ != "plugin" {
		typ = "skill"
	}
	return &ai.SkillSpec{
		Title:       spec.Title,
		Name:        name,
		Description: desc,
		Body:        spec.Body,
		Type:        typ,
		Evals:       spec.Evals,
		Compliance:  spec.Compliance,
	}
}

func finalize(spec *ai.SkillSpec, seed tui.WizardResult) tui.WizardResult {
	return tui.WizardResult{
		Name:         spec.Name,
		Description:  spec.Description,
		Type:         spec.Type,
		IncludeEvals: spec.Evals,
		Compliance:   spec.Compliance || seed.Compliance,
		BodyMarkdown: spec.Body,
	}
}

func renderCard(out io.Writer, spec *ai.SkillSpec) {
	fmt.Fprintln(out)
	fmt.Fprintln(out, tui.Panel("proposed skill", tui.KV([][2]string{
		{"name", spec.Name},
		{"description", spec.Description},
		{"type", spec.Type},
	})))
	prev := spec.Body
	if len(prev) > 700 {
		prev = prev[:700] + "\n…"
	}
	fmt.Fprintln(out, tui.Muted.Render("body preview"))
	fmt.Fprintln(out, tui.RenderMarkdown(prev))
	fmt.Fprintln(out)
	fmt.Fprintln(out, tui.Info(`Build it? ("yes" / tell me what to change / /cancel)`))
}

func readLine(r *bufio.Reader) (string, error) {
	s, err := r.ReadString('\n')
	if err != nil && s == "" {
		return "", err
	}
	return strings.TrimRight(s, "\r\n"), nil
}

func userGlyph() string { return tui.Subtitle.Render("› ") }

func isCancel(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "/cancel", "/quit", "/exit", "/q":
		return true
	}
	return false
}

func isCreateCmd(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "/create", "/build", "/draft", "/make", "/go":
		return true
	}
	return false
}

func isAffirmative(s string) bool {
	t := strings.ToLower(strings.TrimSpace(s))
	switch t {
	case "y", "yes", "yep", "yeah", "yup", "sure", "ok", "okay", "go", "go ahead",
		"do it", "build it", "ship it", "make it", "create it", "looks good",
		"lgtm", "perfect", "great", "sounds good", "draft it":
		return true
	}
	for _, cue := range []string{"looks good", "build it", "ship it", "go ahead", "sounds good"} {
		if strings.Contains(t, cue) {
			return true
		}
	}
	return false
}
