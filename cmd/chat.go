package cmd

import (
	"fmt"

	"github.com/mrdulasolutions/skillforge/internal/tui"
	"github.com/spf13/cobra"
)

var (
	chatType       string
	chatCompliance bool
	chatForce      bool
	chatOut        string
)

var chatCmd = &cobra.Command{
	Use:     "chat",
	Aliases: []string{"run"},
	Short:   "Build a skill by chatting with AI (the main builder)",
	Long:    "Open the full-screen conversational builder: describe a skill in plain words, AI drafts it, and you refine by chatting. This is the default entry point — bare `skillforge` does the same.",
	Args:    cobra.NoArgs,
	RunE:    runChatCmd,
}

func init() {
	f := chatCmd.Flags()
	f.StringVar(&chatType, "type", "skill", "skill | plugin")
	f.BoolVar(&chatCompliance, "compliance", false, "enable the compliance profile (audit + disclosure)")
	f.BoolVar(&chatForce, "force", false, "overwrite the target directory if it exists")
	f.StringVarP(&chatOut, "out", "o", ".", "parent directory to create the skill in")
}

func runChatCmd(_ *cobra.Command, _ []string) error {
	if !isTTY() {
		return fmt.Errorf("chat is interactive — use `skillforge new <name> -y` for non-interactive scaffolding")
	}
	return launchChat(tui.WizardResult{Type: chatType, Compliance: chatCompliance, IncludeEvals: true}, chatOut, chatForce)
}
