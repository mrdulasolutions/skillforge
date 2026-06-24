package cmd

import (
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
	Short: "Scaffold a skill — AI chat builder, or a quick form / flags",
	Long:  "Scaffold a portable skill (SKILL.md + structure). Builds it conversationally with AI when configured (same as `skillforge chat`), otherwise runs a quick form. --yes is non-interactive.",
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

	if newYes || !isTTY() {
		// Non-interactive: derive the name from the arg or the description.
		if res.Name == "" {
			res.Name = res.Description
		}
		return scaffoldAndReport(res, newOut, newForce)
	}

	r, ok, err := runConversational(res, newOut)
	if err != nil {
		return err
	}
	if !ok {
		return nil // cancelled
	}
	return scaffoldAndReport(r, newOut, newForce)
}
