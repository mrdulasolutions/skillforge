package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"

	"github.com/mrdulasolutions/skillforge/internal/ai"
	"github.com/mrdulasolutions/skillforge/internal/compliance"
	"github.com/mrdulasolutions/skillforge/internal/forge"
	"github.com/mrdulasolutions/skillforge/internal/skill"
	"github.com/mrdulasolutions/skillforge/internal/tui"
)

// aiDrafter wraps ai.DraftSkill as a forge.Drafter for the chat flow.
func aiDrafter(p ai.Provider) forge.Drafter {
	return func(ctx context.Context, transcript []ai.Message, prior *ai.SkillSpec, instruction string) (*ai.SkillSpec, error) {
		return ai.DraftSkill(ctx, p, ai.DefaultModel(p), transcript, prior, instruction)
	}
}

// runConversational launches the chat (or the offline form when no provider /
// on degrade) and returns the result. ok=false means the user cancelled.
func runConversational(res tui.WizardResult, outDir string) (tui.WizardResult, bool, error) {
	p := ai.Select()
	if p == nil {
		fmt.Println(tui.Muted.Render("No AI provider configured — using the quick form. Run `skillforge setup` for the conversational builder."))
		r, err := tui.RunWizard(res)
		if err != nil {
			return res, false, err
		}
		return r, true, nil
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	r, ok, err := forge.Chat(ctx, p, aiDrafter(p), res, outDir)
	if errors.Is(err, forge.ErrDegrade) {
		r2, ferr := tui.RunWizard(res)
		if ferr != nil {
			return res, false, ferr
		}
		return r2, true, nil
	}
	if err != nil {
		return res, false, err
	}
	return r, ok, nil
}

// launchChat runs the conversational builder and scaffolds the confirmed skill.
// Used by `chat`, bare `skillforge`, and the interactive path of `new`.
func launchChat(res tui.WizardResult, outDir string, force bool) error {
	header("chat")
	r, ok, err := runConversational(res, outDir)
	if err != nil {
		return err
	}
	if !ok {
		return nil // user cancelled — nothing written
	}
	return scaffoldAndReport(r, outDir, force)
}

// scaffoldAndReport slugifies the name, scaffolds the skill, initializes the
// compliance audit log when enabled, and prints the created tree + a SKILL.md
// preview. Shared by `new` and `compile`.
func scaffoldAndReport(res tui.WizardResult, outDir string, force bool) error {
	slug := skill.Slugify(res.Name)
	if slug == "" {
		return fmt.Errorf("could not derive a valid skill name from %q (try a name or --description)", res.Name)
	}
	res.Name = slug

	sres, err := skill.Scaffold(skill.ScaffoldOptions{
		Name:         res.Name,
		Description:  res.Description,
		Type:         res.Type,
		IncludeEvals: res.IncludeEvals,
		Compliance:   res.Compliance,
		OutDir:       outDir,
		Force:        force,
		BodyOverride: res.BodyMarkdown,
	})
	if err != nil {
		return err
	}

	if res.Compliance {
		if err := compliance.Init(sres.SkillDir, res.Name); err != nil {
			fmt.Println(tui.Warn("compliance enabled but audit log init failed: " + err.Error()))
		}
	}

	fmt.Println()
	fmt.Println(tui.OK("Created " + tui.Code.Render(sres.Root)))
	fmt.Println()
	fmt.Println(tui.FileTree(sres.Created))
	fmt.Println()
	if s, err := skill.Load(sres.SkillDir); err == nil {
		fmt.Println(tui.Muted.Render("SKILL.md preview"))
		fmt.Println(tui.KV([][2]string{
			{"name", s.Frontmatter.Name},
			{"description", s.Frontmatter.Description},
		}))
		fmt.Println()
		fmt.Println(tui.RenderMarkdown(s.Body))
		fmt.Println()
	}
	fmt.Println(tui.Info("Next: " + tui.Code.Render("skillforge build "+sres.Root) +
		tui.Muted.Render("  then  ") + tui.Code.Render("skillforge package "+sres.Root)))
	return nil
}
