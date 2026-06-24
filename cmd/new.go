package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"

	"github.com/mrdulasolutions/skillforge/internal/ai"
	"github.com/mrdulasolutions/skillforge/internal/forge"
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
	Long:  "Scaffold a new portable skill (SKILL.md + structure). Builds it conversationally with AI when configured, otherwise runs a quick form. --yes is non-interactive.",
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
			r, ok, err := forge.Chat(ctx, p, aiDrafter(p), res, newOut)
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

	return scaffoldAndReport(res, newOut, newForce)
}
