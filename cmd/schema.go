package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/mrdulasolutions/skillforge/internal/schema"
	"github.com/mrdulasolutions/skillforge/internal/skill"
	"github.com/spf13/cobra"
)

var schemaFormat string

var schemaCmd = &cobra.Command{
	Use:   "schema <skill-path>",
	Short: "Emit cross-provider tool schemas for a skill",
	Long:  "Emit the skill as a tool definition for MCP, OpenAI function-calling, and/or Anthropic — Skill Forge's cross-provider output. Prints JSON to stdout.",
	Args:  cobra.ExactArgs(1),
	RunE:  runSchema,
}

func init() {
	schemaCmd.Flags().StringVar(&schemaFormat, "format", "all", "mcp | openai | anthropic | all")
}

func runSchema(_ *cobra.Command, args []string) error {
	dir := args[0]
	if res := skill.Validate(dir); !res.Valid() {
		return fmt.Errorf("invalid skill: %s", res.FirstError())
	}
	s, err := skill.Load(dir)
	if err != nil {
		return err
	}
	td := schema.FromSkill(s.Frontmatter.Name, s.Frontmatter.Description)

	var out any
	switch schemaFormat {
	case "mcp":
		out = td.MCP()
	case "openai":
		out = td.OpenAI()
	case "anthropic":
		out = td.Anthropic()
	case "all", "":
		out = map[string]any{"mcp": td.MCP(), "openai": td.OpenAI(), "anthropic": td.Anthropic()}
	default:
		return fmt.Errorf("unknown format %q (mcp | openai | anthropic | all)", schemaFormat)
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}
