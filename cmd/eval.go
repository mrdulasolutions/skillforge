package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/mrdulasolutions/skillforge/internal/ai"
	"github.com/mrdulasolutions/skillforge/internal/eval"
	"github.com/mrdulasolutions/skillforge/internal/skill"
	"github.com/mrdulasolutions/skillforge/internal/tui"
	"github.com/spf13/cobra"
)

var (
	evalBaseline bool
	evalModel    string
	evalHTML     string
	evalJSON     bool
)

var evalCmd = &cobra.Command{
	Use:     "eval <skill-path>",
	Aliases: []string{"test"},
	Short:   "Benchmark a skill across providers (AI-judged)",
	Long:    "Run a skill's evals (evals/evals.json) against your AI provider and grade each expectation with an LLM judge. --baseline also runs each prompt without the skill to measure its lift.",
	Args:    cobra.ExactArgs(1),
	RunE:    runEval,
}

func init() {
	f := evalCmd.Flags()
	f.BoolVar(&evalBaseline, "baseline", false, "also run each prompt without the skill, to measure lift")
	f.StringVar(&evalModel, "model", "", "model id (defaults per provider)")
	f.StringVar(&evalHTML, "html", "", "write an HTML report to this path")
	f.BoolVar(&evalJSON, "json", false, "print the benchmark as JSON")
}

func runEval(_ *cobra.Command, args []string) error {
	dir := args[0]
	if res := skill.Validate(dir); !res.Valid() {
		return fmt.Errorf("invalid skill: %s", res.FirstError())
	}
	s, err := skill.Load(dir)
	if err != nil {
		return err
	}
	f, err := eval.Load(dir)
	if err != nil {
		return fmt.Errorf("no evals found (expected %s/evals/evals.json): %w", dir, err)
	}
	if len(f.Evals) == 0 {
		return fmt.Errorf("evals.json has no cases")
	}

	p := ai.Select()
	if p == nil {
		return fmt.Errorf("eval needs an AI provider — run `skillforge setup`")
	}
	model := evalModel
	if model == "" {
		model = ai.DefaultModel(p)
	}

	header("eval")
	fmt.Println(tui.Info(fmt.Sprintf("running %d eval(s) on %s · %s%s", len(f.Evals), p.Name(), model, baselineNote())))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	cctx, cancel := context.WithTimeout(ctx, time.Duration(len(f.Evals))*120*time.Second+60*time.Second)
	defer cancel()

	rep, err := eval.Run(cctx, p, model, s.Body, f, evalBaseline, func(done, total int) {
		fmt.Println(tui.Muted.Render(fmt.Sprintf("  ✓ case %d/%d", done, total)))
	})
	if err != nil {
		return err
	}

	fmt.Println()
	fmt.Println(renderEvalReport(rep))

	if evalJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(rep); err != nil {
			return err
		}
	}
	if evalHTML != "" {
		html, err := rep.HTML()
		if err != nil {
			return err
		}
		if err := os.WriteFile(evalHTML, []byte(html), 0o644); err != nil {
			return err
		}
		fmt.Println()
		fmt.Println(tui.OK("HTML report written to " + tui.Code.Render(evalHTML)))
	}
	return nil
}

func baselineNote() string {
	if evalBaseline {
		return " (with baseline)"
	}
	return ""
}

func renderEvalReport(r *eval.Report) string {
	var b strings.Builder
	pairs := [][2]string{{"with skill", pct(r.WithSkillPassRate)}}
	if r.Baseline {
		pairs = append(pairs, [2]string{"baseline", pct(r.BaselinePassRate)})
	}
	b.WriteString(tui.Panel("benchmark", tui.KV(pairs)) + "\n\n")
	for _, c := range r.Cases {
		b.WriteString(tui.Subtitle.Render(fmt.Sprintf("eval %d", c.ID)) +
			tui.Muted.Render("  "+clip(c.Prompt, 64)) + "\n")
		for _, e := range c.WithSkill.Expectations {
			if e.Passed {
				b.WriteString("  " + tui.OK(clip(e.Text, 80)) + "\n")
			} else {
				b.WriteString("  " + tui.Err(clip(e.Text, 80)) + "\n")
			}
		}
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

func pct(f float64) string { return fmt.Sprintf("%.0f%%", f*100) }

func clip(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}
