package cmd

import (
	"errors"
	"fmt"

	"github.com/mrdulasolutions/skillforge/internal/compliance"
	"github.com/mrdulasolutions/skillforge/internal/skill"
	"github.com/mrdulasolutions/skillforge/internal/tui"
	"github.com/spf13/cobra"
)

var packageOut string

var packageCmd = &cobra.Command{
	Use:   "package [path]",
	Short: "Validate and bundle a skill into a .skill file",
	Long:  "Validate a skill, then bundle it into a distributable .skill archive, excluding evals/ and build artifacts.",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runPackage,
}

func init() {
	packageCmd.Flags().StringVarP(&packageOut, "out", "o", "", "output directory (default: current dir)")
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

	if compliance.HasLog(path) {
		_, _ = compliance.Append(path, compliance.Event{
			EventType: "package",
			Tool:      "skillforge package",
			Summary:   "packaged to " + pr.Output,
		})
		if v, verr := compliance.Verify(path); verr == nil {
			fmt.Println()
			if v.OK {
				fmt.Println(tui.OK(fmt.Sprintf("audit chain verified (%d entries)", v.Lines)))
			} else {
				fmt.Println(tui.Err(fmt.Sprintf("audit chain broken at entry %d (%s)", v.BrokenAt, v.Reason)))
			}
		}
	}
	return nil
}
