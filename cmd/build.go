package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/mrdulasolutions/skillforge/internal/ai"
	"github.com/mrdulasolutions/skillforge/internal/compliance"
	"github.com/mrdulasolutions/skillforge/internal/skill"
	"github.com/mrdulasolutions/skillforge/internal/tui"
	"github.com/spf13/cobra"
)

var (
	buildOptimize bool
	buildFix      bool
	buildJSON     bool
	buildModel    string
)

var buildCmd = &cobra.Command{
	Use:   "build [path]",
	Short: "Validate (and optionally AI-optimize) a skill",
	Long:  "Validate a skill's SKILL.md against the canonical rules and surface best-practice warnings. With --optimize, refine the description via an AI provider.",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runBuild,
}

func init() {
	f := buildCmd.Flags()
	f.BoolVar(&buildOptimize, "optimize", false, "use an AI provider to improve the description")
	f.BoolVar(&buildFix, "fix", false, "apply the optimized description to SKILL.md")
	f.BoolVar(&buildJSON, "json", false, "emit machine-readable JSON")
	f.StringVar(&buildModel, "model", "", "model id (defaults per provider)")
}

func runBuild(_ *cobra.Command, args []string) error {
	path := "."
	if len(args) == 1 {
		path = args[0]
	}
	res := skill.Validate(path)

	if buildJSON {
		return emitBuildJSON(path, res)
	}

	header("build")
	fmt.Println(tui.Key.Render("skill") + "  " + tui.Val.Render(path))
	fmt.Println()
	fmt.Println(tui.ValidationReport(res))

	if buildOptimize && res.Valid() {
		fmt.Println()
		if err := runOptimize(path); err != nil {
			fmt.Println(tui.Warn("optimize: " + err.Error()))
		}
	}

	if compliance.HasLog(path) {
		status := "valid"
		if !res.Valid() {
			status = "invalid"
		}
		if _, aerr := compliance.Append(path, compliance.Event{
			EventType: "build",
			Tool:      "skillforge build",
			Summary:   "validation " + status,
			Metadata: map[string]any{
				"errors":   len(res.Errors()),
				"warnings": len(res.Warnings()),
			},
		}); aerr != nil {
			fmt.Println(tui.Warn("audit log not updated: " + aerr.Error()))
		}
	}

	if !res.Valid() {
		return fmt.Errorf("validation failed (%d error(s))", len(res.Errors()))
	}
	fmt.Println()
	fmt.Println(tui.OK("build passed"))
	return nil
}

func runOptimize(path string) error {
	provider := ai.Select()
	if provider == nil {
		return fmt.Errorf("no AI provider available — set OPENROUTER_API_KEY or start Ollama")
	}
	s, err := skill.Load(path)
	if err != nil {
		return err
	}
	fmt.Println(tui.Info(fmt.Sprintf("optimizing description via %s…", provider.Name())))
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	newDesc, err := ai.OptimizeDescription(ctx, provider, buildModel, s.Frontmatter.Name, s.Frontmatter.Description, s.Body)
	if err != nil {
		return err
	}

	fmt.Println()
	fmt.Println(tui.Muted.Render("before") + "  " + tui.Val.Render(s.Frontmatter.Description))
	fmt.Println(tui.Subtitle.Render("after ") + "  " + tui.Val.Render(newDesc))

	if buildFix {
		if err := skill.UpdateDescription(path, newDesc); err != nil {
			return err
		}
		fmt.Println()
		fmt.Println(tui.OK("applied to SKILL.md"))
	} else {
		fmt.Println()
		fmt.Println(tui.Muted.Render("re-run with --fix to apply"))
	}
	return nil
}

type buildIssueJSON struct {
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

func emitBuildJSON(path string, res skill.Result) error {
	out := struct {
		Path     string           `json:"path"`
		Valid    bool             `json:"valid"`
		Errors   []buildIssueJSON `json:"errors"`
		Warnings []buildIssueJSON `json:"warnings"`
	}{Path: path, Valid: res.Valid(), Errors: []buildIssueJSON{}, Warnings: []buildIssueJSON{}}
	for _, e := range res.Errors() {
		out.Errors = append(out.Errors, buildIssueJSON{"error", e.Message})
	}
	for _, w := range res.Warnings() {
		out.Warnings = append(out.Warnings, buildIssueJSON{"warning", w.Message})
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		return err
	}
	if !res.Valid() {
		return fmt.Errorf("validation failed")
	}
	return nil
}
