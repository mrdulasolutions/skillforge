// Package forge runs the conversational skill-building experience: a full-screen
// Bubble Tea chat (see chat.go) backed by a streaming interview, an AI-drafted
// proposal, natural-language refinement, and a confirm step. This file holds the
// provider-agnostic helpers shared by the chat model.
package forge

import (
	"context"
	"errors"
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
	typ := spec.Type
	if seed.Type == "plugin" { // honor a /plugin or --type plugin override
		typ = "plugin"
	}
	return tui.WizardResult{
		Name:         spec.Name,
		Description:  spec.Description,
		Type:         typ,
		IncludeEvals: spec.Evals,
		Compliance:   spec.Compliance || seed.Compliance,
		BodyMarkdown: spec.Body,
	}
}

// cardString renders a proposed skill as a chat block (the review card).
func cardString(spec *ai.SkillSpec) string {
	card := tui.Panel("proposed skill", tui.KV([][2]string{
		{"name", spec.Name},
		{"description", spec.Description},
		{"type", spec.Type},
	}))
	prev := spec.Body
	if len(prev) > 700 {
		prev = prev[:700] + "\n…"
	}
	body := tui.Muted.Render("body preview") + "\n" + tui.RenderMarkdown(prev)
	cta := tui.Info(`Build it? — "go" to create · tell me what to change · esc to cancel`)
	return card + "\n" + body + "\n\n" + cta
}

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
