// Package cmd wires Skill Forge's CLI commands.
package cmd

import (
	"fmt"
	"os"

	"github.com/mrdulasolutions/skillforge/internal/tui"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// version is overridden at build time via -ldflags.
var version = "0.0.0-dev"

var rootCmd = &cobra.Command{
	Use:           "skillforge",
	Short:         "Forge portable agentic skills & plugins",
	Long:          "Skill Forge — forge portable agentic skills & plugins, free-form or AI-compiled.",
	Version:       version,
	SilenceUsage:  true,
	SilenceErrors: true,
	Run: func(cmd *cobra.Command, _ []string) {
		fmt.Println()
		fmt.Println(tui.Banner())
		fmt.Println()
		_ = cmd.Help()
	},
}

// Execute runs the root command.
func Execute() {
	rootCmd.SetVersionTemplate(tui.CompactBanner() + "  v{{.Version}}\n")
	rootCmd.AddCommand(newCmd, compileCmd, buildCmd, evalCmd, packageCmd, serveMCPCmd, schemaCmd, setupCmd, doctorCmd)
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, tui.Err(err.Error()))
		os.Exit(1)
	}
}

// header prints a compact, branded sub-command header.
func header(name string) {
	fmt.Println()
	fmt.Println(tui.CompactBanner() + tui.Muted.Render("  "+name))
	fmt.Println()
}

func isTTY() bool {
	return term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd()))
}
