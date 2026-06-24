package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"

	"github.com/mrdulasolutions/skillforge/internal/ai"
	"github.com/mrdulasolutions/skillforge/internal/mcp"
	"github.com/mrdulasolutions/skillforge/internal/schema"
	"github.com/mrdulasolutions/skillforge/internal/skill"
	"github.com/spf13/cobra"
)

var (
	serveExecute bool
	serveName    string
)

var serveMCPCmd = &cobra.Command{
	Use:   "serve-mcp [skill-path...]",
	Short: "Run an MCP server exposing skills as tools",
	Long: "Expose one or more skills as MCP tools over stdio. By default a tool call returns the skill's instructions for the host agent to follow; with --execute it runs the skill via your AI provider. " +
		"Discovers skills in the current directory when no paths are given.",
	RunE: runServeMCP,
}

func init() {
	serveMCPCmd.Flags().BoolVar(&serveExecute, "execute", false, "execute the skill via the AI provider instead of returning instructions")
	serveMCPCmd.Flags().StringVar(&serveName, "name", "skillforge-skills", "MCP server name")
}

func runServeMCP(_ *cobra.Command, args []string) error {
	dirs := discoverSkills(args)
	if len(dirs) == 0 {
		return fmt.Errorf("no skills found (pass skill paths, or run from a folder containing SKILL.md)")
	}

	var provider ai.Provider
	if serveExecute {
		if provider = ai.Select(); provider == nil {
			return fmt.Errorf("--execute needs an AI provider — run `skillforge setup`")
		}
	}

	srv := &mcp.Server{Name: serveName, Version: version}
	var names []string
	for _, dir := range dirs {
		if res := skill.Validate(dir); !res.Valid() {
			fmt.Fprintln(os.Stderr, "skillforge serve-mcp: skipping "+dir+": "+res.FirstError())
			continue
		}
		s, err := skill.Load(dir)
		if err != nil {
			continue
		}
		td := schema.FromSkill(s.Frontmatter.Name, s.Frontmatter.Description)
		handler := instructionsHandler(s.Frontmatter.Name, s.Body)
		if serveExecute {
			handler = executeHandler(provider, s.Body)
		}
		srv.Tools = append(srv.Tools, mcp.Tool{
			Name:        td.Name,
			Description: td.Description,
			InputSchema: td.InputSchema,
			Handler:     handler,
		})
		names = append(names, td.Name)
	}
	if len(srv.Tools) == 0 {
		return fmt.Errorf("no valid skills to serve")
	}

	mode := "instructions"
	if serveExecute {
		mode = "execute via " + provider.Name()
	}
	fmt.Fprintf(os.Stderr, "skillforge serve-mcp: serving %d skill(s) [%s]: %s\n", len(srv.Tools), mode, strings.Join(names, ", "))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	return srv.Serve(ctx, os.Stdin, os.Stdout)
}

func instructionsHandler(name, body string) func(context.Context, map[string]any) (string, error) {
	return func(_ context.Context, args map[string]any) (string, error) {
		request, _ := args["request"].(string)
		return "# Skill: " + name + "\n\n" + body +
			"\n\n---\nUser request: " + request +
			"\n\nFollow the skill above to handle this request.", nil
	}
}

func executeHandler(p ai.Provider, body string) func(context.Context, map[string]any) (string, error) {
	return func(ctx context.Context, args map[string]any) (string, error) {
		request, _ := args["request"].(string)
		resp, err := p.Complete(ctx, ai.Request{
			Model:     ai.DefaultModel(p),
			System:    body,
			Messages:  []ai.Message{{Role: "user", Content: request}},
			MaxTokens: 2000,
		})
		if err != nil {
			return "", err
		}
		return resp.Text, nil
	}
}

func discoverSkills(args []string) []string {
	seen := map[string]bool{}
	var out []string
	add := func(d string) {
		if d != "" && !seen[d] && hasSkillMD(d) {
			seen[d] = true
			out = append(out, d)
		}
	}
	roots := args
	if len(roots) == 0 {
		roots = []string{"."}
	}
	for _, root := range roots {
		add(root)
		if entries, err := os.ReadDir(root); err == nil {
			for _, e := range entries {
				if e.IsDir() {
					add(filepath.Join(root, e.Name()))
				}
			}
		}
	}
	return out
}

func hasSkillMD(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, "SKILL.md"))
	return err == nil
}
