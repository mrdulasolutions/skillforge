package cmd

import (
	"errors"
	"fmt"

	"github.com/mrdulasolutions/skillforge/internal/compliance"
	"github.com/mrdulasolutions/skillforge/internal/skill"
	"github.com/mrdulasolutions/skillforge/internal/tui"
	"github.com/spf13/cobra"
)

var (
	packageOut        string
	packageCompliance bool
)

var packageCmd = &cobra.Command{
	Use:   "package [path]",
	Short: "Validate and bundle a skill into a .skill file",
	Long:  "Validate a skill, then bundle it into a distributable .skill archive, excluding evals/ and build artifacts.",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runPackage,
}

func init() {
	packageCmd.Flags().StringVarP(&packageOut, "out", "o", "", "output directory (default: current dir)")
	packageCmd.Flags().BoolVar(&packageCompliance, "compliance", false, "seal the audit log and write a provenance manifest")
}

func runPackage(_ *cobra.Command, args []string) error {
	path := "."
	if len(args) == 1 {
		path = args[0]
	}

	header("package")

	pr, err := skill.Package(path, packageOut)
	if err != nil {
		var ve *skill.ValidationError
		if errors.As(err, &ve) {
			fmt.Println(tui.ValidationReport(ve.Result))
			return errors.New("packaging aborted — fix validation errors first")
		}
		return err
	}

	fmt.Println(tui.OK("Packaged " + tui.Code.Render(pr.Output)))
	fmt.Println()
	fmt.Println(tui.KV([][2]string{
		{"files", fmt.Sprintf("%d added", len(pr.Added))},
		{"skipped", fmt.Sprintf("%d (evals/build artifacts)", len(pr.Skipped))},
	}))

	if packageCompliance || compliance.HasLog(path) {
		if _, aerr := compliance.Append(path, compliance.Event{
			EventType: "package",
			Tool:      "skillforge package",
			Summary:   "packaged to " + pr.Output,
		}); aerr != nil {
			fmt.Println(tui.Warn("audit log not updated: " + aerr.Error()))
		}
		if v, verr := compliance.Verify(path); verr == nil {
			fmt.Println()
			if v.OK {
				fmt.Println(tui.OK(fmt.Sprintf("audit chain verified (%d entries)", v.Lines)))
			} else {
				fmt.Println(tui.Err(fmt.Sprintf("audit chain broken at entry %d (%s)", v.BrokenAt, v.Reason)))
			}
		}
		if mpath, merr := writeProvenance(path, pr); merr != nil {
			fmt.Println(tui.Warn("provenance manifest not written: " + merr.Error()))
		} else {
			fmt.Println(tui.OK("provenance manifest " + tui.Code.Render(mpath)))
		}
	}
	return nil
}
