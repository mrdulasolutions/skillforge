package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/huh"
)

// WizardResult is the data collected by the `new` wizard (or the conversational
// flow). Name holds a plain title in the form path; the caller slugifies it.
type WizardResult struct {
	Name         string
	Description  string
	Type         string
	IncludeEvals bool
	Compliance   bool
	BodyMarkdown string // AI-generated SKILL.md body (conversational flow only)
}

// RunWizard runs the interactive `new` form, seeded with defaults.
func RunWizard(defaults WizardResult) (WizardResult, error) {
	r := defaults
	if r.Type == "" {
		r.Type = "skill"
	}
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("What's this skill called?").
				Description(`Plain words, e.g. "Alphabet Reciter" — we'll slug it for you.`).
				Value(&r.Name).
				Validate(func(s string) error {
					if strings.TrimSpace(s) == "" {
						return fmt.Errorf("a name is required")
					}
					return nil
				}),
			huh.NewText().
				Title("Description").
				Description("One line. Be specific & assertive — say when to use it.").
				Lines(3).
				Value(&r.Description).
				Validate(func(s string) error {
					s = strings.TrimSpace(s)
					if s == "" {
						return fmt.Errorf("description is required")
					}
					if strings.ContainsAny(s, "<>") {
						return fmt.Errorf("no angle brackets (< or >)")
					}
					if len(s) > 1024 {
						return fmt.Errorf("max 1024 characters (have %d)", len(s))
					}
					return nil
				}),
			huh.NewSelect[string]().
				Title("Type").
				Options(
					huh.NewOption("Skill", "skill"),
					huh.NewOption("Plugin (wraps the skill)", "plugin"),
				).
				Value(&r.Type),
			huh.NewConfirm().
				Title("Include eval scaffold?").
				Value(&r.IncludeEvals).
				Affirmative("Yes").Negative("No"),
			huh.NewConfirm().
				Title("Enable compliance profile?").
				Description("Audit log + sanitization + disclosure template").
				Value(&r.Compliance).
				Affirmative("Yes").Negative("No"),
		),
	).WithTheme(forgeHuhTheme())

	if err := form.Run(); err != nil {
		return r, err
	}
	return r, nil
}

// FormTheme returns the Skill Forge huh theme for use by other commands.
func FormTheme() *huh.Theme { return forgeHuhTheme() }

func forgeHuhTheme() *huh.Theme {
	t := huh.ThemeCharm()
	t.Focused.Base = t.Focused.Base.BorderForeground(ColPrimary)
	t.Focused.Title = t.Focused.Title.Foreground(ColPrimary).Bold(true)
	return t
}
