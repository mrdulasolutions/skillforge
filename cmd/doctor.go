package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mrdulasolutions/skillforge/internal/ai"
	"github.com/mrdulasolutions/skillforge/internal/config"
	"github.com/mrdulasolutions/skillforge/internal/tui"
	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check the Skill Forge environment",
	Long:  "Report Skill Forge version, AI provider availability, and config writability. Read-only; never requires a network.",
	Args:  cobra.NoArgs,
	RunE:  runDoctor,
}

func runDoctor(_ *cobra.Command, _ []string) error {
	header("doctor")

	fmt.Println(tui.KV([][2]string{
		{"version", version},
		{"tty", fmt.Sprintf("%v", isTTY())},
	}))
	fmt.Println()

	fmt.Println(tui.Muted.Render("AI providers"))
	for _, p := range ai.ProbeAll() {
		line := fmt.Sprintf("%-11s %s", p.Name, tui.Muted.Render(p.Detail))
		fmt.Println("  " + tui.Step(p.Available, line))
	}
	if ai.Select() == nil {
		fmt.Println("  " + tui.Warn("no provider available — `build --optimize` will be skipped"))
	}
	fmt.Println()

	fmt.Println(tui.Muted.Render("configured"))
	c := config.Load()
	prov := c.Provider
	model := ""
	switch c.Provider {
	case "openrouter":
		model = c.OpenRouterModel
	case "ollama":
		model = c.OllamaModel
	case "":
		prov = "(none — run skillforge setup)"
	}
	fmt.Println("  " + tui.Step(c.Provider != "", fmt.Sprintf("%-11s %s", "provider", tui.Muted.Render(strings.TrimSpace(prov+" "+model)))))
	if dir, derr := config.Dir(); derr == nil {
		fmt.Println("  " + tui.Step(canWrite(dir), fmt.Sprintf("%-11s %s", "writable", tui.Muted.Render(dir))))
	}
	return nil
}

func canWrite(dir string) bool {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return false
	}
	probe := filepath.Join(dir, ".write-probe")
	if err := os.WriteFile(probe, []byte("ok"), 0o600); err != nil {
		return false
	}
	_ = os.Remove(probe)
	return true
}
