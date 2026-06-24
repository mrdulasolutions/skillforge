package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/charmbracelet/huh/spinner"
	"github.com/mrdulasolutions/skillforge/internal/ai"
	"github.com/mrdulasolutions/skillforge/internal/compile"
	"github.com/mrdulasolutions/skillforge/internal/forge"
	"github.com/mrdulasolutions/skillforge/internal/tui"
	"github.com/spf13/cobra"
)

var (
	compileHint       string
	compileName       string
	compileType       string
	compileCompliance bool
	compileEvals      bool
	compileYes        bool
	compileForce      bool
	compileOut        string
)

var compileCmd = &cobra.Command{
	Use:   "compile <path...>",
	Short: "Compile a skill from your files/data using AI",
	Long:  "Read the given files/folders and synthesize a skill with AI, then refine it conversationally (or write it directly with --yes).",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runCompile,
}

func init() {
	f := compileCmd.Flags()
	f.StringVarP(&compileHint, "hint", "d", "", "hint about the skill you want")
	f.StringVar(&compileName, "name", "", "skill name (default: AI-derived)")
	f.StringVar(&compileType, "type", "skill", "skill | plugin")
	f.BoolVar(&compileCompliance, "compliance", false, "enable the compliance profile")
	f.BoolVar(&compileEvals, "evals", true, "include an eval scaffold")
	f.BoolVarP(&compileYes, "yes", "y", false, "non-interactive: draft and write without refining")
	f.BoolVar(&compileForce, "force", false, "overwrite the target directory if it exists")
	f.StringVarP(&compileOut, "out", "o", ".", "parent directory to create the skill in")
}

func runCompile(_ *cobra.Command, args []string) error {
	header("compile")
	p := ai.Select()
	if p == nil {
		return fmt.Errorf("compile needs an AI provider — run `skillforge setup` (or set OPENROUTER_API_KEY / start Ollama)")
	}

	gathered, err := compile.Gather(args, 0)
	if err != nil {
		return err
	}
	if strings.TrimSpace(gathered.Corpus) == "" {
		return fmt.Errorf("no readable text found in %v", args)
	}
	fmt.Println(tui.OK(fmt.Sprintf("read %d file(s)%s", len(gathered.Sources), skippedNote(gathered.Skipped))))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	var spec *ai.SkillSpec
	var cerr error
	runWork(" synthesizing a skill from your material…", func() {
		c, cancel := context.WithTimeout(ctx, 120*time.Second)
		defer cancel()
		spec, cerr = ai.CompileSkill(c, p, ai.DefaultModel(p), gathered.Corpus, compileHint)
	})
	if cerr != nil {
		return fmt.Errorf("synthesis failed: %w", cerr)
	}

	res := tui.WizardResult{Type: compileType, Compliance: compileCompliance, IncludeEvals: compileEvals}

	if compileYes || !isTTY() {
		applyDraft(&res, spec)
	} else {
		r, ok, cerr := forge.ChatFromDraft(ctx, p, aiDrafter(p), res, compileOut, spec)
		switch {
		case errors.Is(cerr, forge.ErrDegrade):
			applyDraft(&res, spec) // model died mid-refine: keep the draft
		case cerr != nil:
			return cerr
		case !ok:
			return nil // user cancelled
		default:
			res = r
		}
	}

	if compileName != "" {
		res.Name = compileName
	}
	return scaffoldAndReport(res, compileOut, compileForce)
}

// applyDraft fills a WizardResult from a freshly compiled spec (non-interactive).
func applyDraft(res *tui.WizardResult, spec *ai.SkillSpec) {
	res.Name = firstNonEmpty(compileName, spec.Title, spec.Name)
	res.Description = ai.CleanDescription(spec.Description)
	res.BodyMarkdown = spec.Body
	if spec.Type == "plugin" {
		res.Type = "plugin"
	}
}

func skippedNote(n int) string {
	if n == 0 {
		return ""
	}
	return fmt.Sprintf(", %d skipped", n)
}

// runWork shows a spinner around fn on a TTY, otherwise just runs it.
func runWork(title string, fn func()) {
	if isTTY() {
		_ = spinner.New().Title(title).Action(fn).Run()
	} else {
		fn()
	}
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
