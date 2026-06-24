package cmd

import (
	"fmt"

	"github.com/mrdulasolutions/skillforge/internal/compliance"
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

	if !newYes && isTTY() {
		r, err := tui.RunWizard(res)
		if err != nil {
			return err
		}
		res = r
	}
	if res.Name == "" {
		return fmt.Errorf("a skill name is required (pass it as an argument, or run interactively without --yes)")
	}

	sres, err := skill.Scaffold(skill.ScaffoldOptions{
		Name:         res.Name,
		Description:  res.Description,
		Type:         res.Type,
		IncludeEvals: res.IncludeEvals,
		Compliance:   res.Compliance,
		OutDir:       newOut,
		Force:        newForce,
	})
	if err != nil {
		return err
	}

	if res.Compliance {
		if err := compliance.Init(sres.SkillDir, res.Name); err != nil {
			fmt.Println(tui.Warn("compliance enabled but audit log init failed: " + err.Error()))
		}
	}

	header("new")
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
