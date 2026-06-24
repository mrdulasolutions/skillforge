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
	"github.com/spf13/cobra"
)

var (
	newType       string
	newDesc       string
	newCompliance bool
	newEvals      bool
	newYes        bool
	newForce      bool
	newOut        string
)

var newCmd = &cobra.Command{
	Use:   "new [name]",
	Short: "Scaffold a new skill (or plugin)",
	Long:  "Scaffold a new portable skill (SKILL.md + structure). Runs an interactive wizard unless --yes is set.",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runNew,
}

func init() {
	f := newCmd.Flags()
	f.StringVar(&newType, "type", "skill", "skill | plugin")
	f.StringVarP(&newDesc, "description", "d", "", "skill description")
	f.BoolVar(&newCompliance, "compliance", false, "enable the compliance profile (audit + disclosure)")
	f.BoolVar(&newEvals, "evals", true, "include an eval scaffold")
	f.BoolVarP(&newYes, "yes", "y", false, "skip the wizard; use flags and defaults")
	f.BoolVar(&newForce, "force", false, "overwrite the target directory if it exists")
	f.StringVarP(&newOut, "out", "o", ".", "parent directory to create the skill in")
}

func runNew(_ *cobra.Command, args []string) error {
	res := tui.WizardResult{
		Type:         newType,
		Description:  newDesc,
		Compliance:   newCompliance,
		IncludeEvals: newEvals,
	}
	if len(args) == 1 {
		res.Name = args[0]
	}

	header("new")

	switch {
	case newYes || !isTTY():
		// Non-interactive: derive the name from the arg or the description.
		if res.Name == "" {
			res.Name = res.Description
		}
	default:
		if p := ai.Select(); p != nil {
			// Conversational, AI-driven flow.
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
			defer stop()
			drafter := func(c context.Context, transcript []ai.Message, prior *ai.SkillSpec, instruction string) (*ai.SkillSpec, error) {
				return ai.DraftSkill(c, p, ai.DefaultModel(p), transcript, prior, instruction)
			}
			r, ok, err := forge.Converse(ctx, os.Stdin, os.Stdout, p, drafter, res, newOut)
			switch {
			case errors.Is(err, forge.ErrDegrade):
				r2, ferr := tui.RunWizard(res)
				if ferr != nil {
					return ferr
				}
				res = r2
			case err != nil:
				return err
			case !ok:
				return nil // user cancelled — nothing written
			default:
				res = r
			}
		} else {
			fmt.Println(tui.Muted.Render("No AI provider configured — using the quick form. Run `skillforge setup` for the conversational builder."))
			r, err := tui.RunWizard(res)
			if err != nil {
				return err
			}
			res = r
		}
	}

	// Derive a valid kebab name from whatever the source produced. Idempotent
	// for the conversational flow, which already returns a unique slug.
	slug := skill.Slugify(res.Name)
	if slug == "" {
		return fmt.Errorf("could not derive a valid skill name from %q (try a name argument or --description)", res.Name)
	}
	res.Name = slug

	sres, err := skill.Scaffold(skill.ScaffoldOptions{
		Name:         res.Name,
		Description:  res.Description,
		Type:         res.Type,
		IncludeEvals: res.IncludeEvals,
		Compliance:   res.Compliance,
		OutDir:       newOut,
		Force:        newForce,
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
