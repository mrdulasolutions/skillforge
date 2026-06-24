package forge

// Slash commands that act on already-built skills found under the chat's output
// directory: /skills (list), /export (portable .skill bundle), and /mcp (MCP
// client config + cross-provider tool schemas).

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mrdulasolutions/skillforge/internal/compliance"
	"github.com/mrdulasolutions/skillforge/internal/schema"
	"github.com/mrdulasolutions/skillforge/internal/skill"
)

// builtSkill is a skill discovered on disk under the chat's output directory.
type builtSkill struct {
	name string
	desc string
	dir  string
	typ  string // "skill" or "plugin"
}

// findSkills scans parent for already-built skills, covering both the standalone
// layout (<parent>/<name>/SKILL.md) and the plugin layout
// (<parent>/<name>/skills/<sub>/SKILL.md). parent itself is checked too.
func findSkills(parent string) []builtSkill {
	if parent == "" {
		parent = "."
	}
	var out []builtSkill
	seen := map[string]bool{}
	add := func(dir, typ string) {
		abs, err := filepath.Abs(dir)
		if err != nil {
			abs = dir
		}
		if seen[abs] {
			return
		}
		if _, err := os.Stat(filepath.Join(dir, "SKILL.md")); err != nil {
			return
		}
		seen[abs] = true
		bs := builtSkill{name: filepath.Base(dir), dir: dir, typ: typ}
		if sk, err := skill.Load(dir); err == nil {
			if sk.Frontmatter.Name != "" {
				bs.name = sk.Frontmatter.Name
			}
			bs.desc = sk.Frontmatter.Description
		}
		out = append(out, bs)
	}

	add(parent, "skill")
	entries, err := os.ReadDir(parent)
	if err != nil {
		return out
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		d := filepath.Join(parent, e.Name())
		add(d, "skill")
		if subs, err := os.ReadDir(filepath.Join(d, "skills")); err == nil {
			for _, s := range subs {
				if s.IsDir() {
					add(filepath.Join(d, "skills", s.Name()), "plugin")
				}
			}
		}
	}
	return out
}

// resolveTarget picks the skill the user named, or returns an explanatory chat
// message when the choice is empty, ambiguous, or unknown.
func resolveTarget(parent string, skills []builtSkill, arg, verb string) (builtSkill, []chatMsg) {
	if len(skills) == 0 {
		return builtSkill{}, []chatMsg{{roleSystem,
			"no built skills in " + dirLabel(parent) + ` — build one first (describe it above and say "go").`}}
	}
	if arg == "" {
		if len(skills) == 1 {
			return skills[0], nil
		}
		return builtSkill{}, []chatMsg{{roleSystem,
			"which skill? run `/" + verb + " <name>`:\n" + skillLines(skills)}}
	}
	var matches []builtSkill
	for _, s := range skills {
		if strings.EqualFold(s.name, arg) || strings.EqualFold(filepath.Base(s.dir), arg) {
			matches = append(matches, s)
		}
	}
	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		return builtSkill{}, []chatMsg{{roleSystem,
			`no built skill named "` + arg + `" — run /skills to see what's available.`}}
	default:
		return builtSkill{}, []chatMsg{{roleSystem,
			`"` + arg + `" matches more than one skill — pick one by its folder:` + "\n" + skillLines(matches)}}
	}
}

func skillLines(skills []builtSkill) string {
	var b strings.Builder
	for _, s := range skills {
		var tags []string
		if base := filepath.Base(s.dir); !strings.EqualFold(base, s.name) {
			tags = append(tags, base+"/") // disambiguate when name != folder
		}
		if s.typ == "plugin" {
			tags = append(tags, "plugin")
		}
		suffix := ""
		if len(tags) > 0 {
			suffix = "  · " + strings.Join(tags, " · ")
		}
		b.WriteString(fmt.Sprintf("  %-22s %s%s\n", s.name, clip(s.desc, 46), suffix))
	}
	return strings.TrimRight(b.String(), "\n")
}

// skillsResult renders the /skills list.
func (m model) skillsResult() []chatMsg {
	skills := findSkills(m.parent)
	if len(skills) == 0 {
		return []chatMsg{{roleSystem,
			"no built skills in " + dirLabel(m.parent) + ` yet — describe one above and say "go", or run ` + "`skillforge new`."}}
	}
	head := fmt.Sprintf("%d built skill", len(skills))
	if len(skills) != 1 {
		head += "s"
	}
	head += " in " + dirLabel(m.parent) + ":"
	tip := "export: /export <name>   ·   serve as MCP: /mcp <name>"
	return []chatMsg{{roleSystem, head + "\n" + skillLines(skills) + "\n\n" + tip}}
}

// exportResult packages the named skill into a portable .skill bundle.
func (m model) exportResult(arg string) []chatMsg {
	skills := findSkills(m.parent)
	target, deny := resolveTarget(m.parent, skills, arg, "export")
	if deny != nil {
		return deny
	}
	pr, err := skill.Package(target.dir, m.parent)
	if err != nil {
		return []chatMsg{{roleSystem, "couldn't export " + target.name + ": " + firstLine(err.Error())}}
	}
	msgs := []chatMsg{{roleSystem, "exported " + target.name + " → " + displayPath(pr.Output)}}
	if compliance.HasLog(target.dir) {
		if _, err := compliance.Append(target.dir, compliance.Event{
			EventType: "package",
			Skill:     target.name,
			Tool:      "chat /export",
			Summary:   "packaged to " + pr.Output,
		}); err == nil {
			msgs = append(msgs, chatMsg{roleSystem, "audit log updated (compliance profile)"})
		} else {
			msgs = append(msgs, chatMsg{roleSystem, "exported, but couldn't update the audit log: " + firstLine(err.Error())})
		}
	}
	return msgs
}

// mcpResult writes an MCP client config + cross-provider tool schemas for the
// named skill and shows the ready-to-paste config and the serve command.
func (m model) mcpResult(arg string) []chatMsg {
	skills := findSkills(m.parent)
	target, deny := resolveTarget(m.parent, skills, arg, "mcp")
	if deny != nil {
		return deny
	}
	sk, err := skill.Load(target.dir)
	if err != nil {
		return []chatMsg{{roleSystem, "couldn't read " + target.name + ": " + firstLine(err.Error())}}
	}
	absDir, err := filepath.Abs(target.dir)
	if err != nil {
		absDir = target.dir
	}
	td := schema.FromSkill(sk.Frontmatter.Name, sk.Frontmatter.Description)
	server := "skillforge-" + td.Name

	// MCP client config (Claude Code project .mcp.json shape). Build it first so a
	// marshal failure aborts before we touch the filesystem.
	cfg := map[string]any{
		"mcpServers": map[string]any{
			server: map[string]any{
				"command": "skillforge",
				"args":    []any{"serve-mcp", absDir},
			},
		},
	}
	cfgJSON, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return []chatMsg{{roleSystem, "couldn't build the MCP config: " + firstLine(err.Error())}}
	}

	var warnings []string

	// Cross-provider tool schemas (deterministic — safe to regenerate).
	var wrote []string
	for _, f := range []struct {
		name string
		v    any
	}{
		{td.Name + ".mcp.json", td.MCP()},
		{td.Name + ".openai.json", td.OpenAI()},
		{td.Name + ".anthropic.json", td.Anthropic()},
	} {
		if werr := writeJSON(filepath.Join(target.dir, "schemas", f.name), f.v); werr == nil {
			wrote = append(wrote, "schemas/"+f.name)
		} else {
			warnings = append(warnings, "couldn't write schemas/"+f.name+": "+firstLine(werr.Error()))
		}
	}

	// Never clobber a user's existing .mcp.json — the config is always shown
	// inline below, so they can merge it by hand.
	cfgPath := filepath.Join(target.dir, ".mcp.json")
	cfgNote := ""
	if _, serr := os.Stat(cfgPath); serr == nil {
		cfgNote = " (you already have one here — left it untouched; merge the block below)"
	} else if werr := os.WriteFile(cfgPath, append(cfgJSON, '\n'), 0o644); werr == nil {
		cfgNote = " (written to " + displayPath(cfgPath) + ")"
	} else {
		warnings = append(warnings, "couldn't write "+displayPath(cfgPath)+": "+firstLine(werr.Error()))
	}

	var b strings.Builder
	b.WriteString("**MCP setup for `" + target.name + "`**\n\n")
	b.WriteString("Run the server:\n\n```\nskillforge serve-mcp " + absDir + "\n```\n\n")
	b.WriteString("Add to your MCP client" + cfgNote + ":\n\n```json\n")
	b.Write(cfgJSON)
	b.WriteString("\n```")
	if len(wrote) > 0 {
		b.WriteString("\n\nCross-provider tool schemas: " + strings.Join(wrote, " · "))
	}
	msgs := []chatMsg{{roleAssistant, b.String()}}
	for _, w := range warnings {
		msgs = append(msgs, chatMsg{roleSystem, w})
	}
	return msgs
}

func writeJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

// splitSlash splits "/cmd some args" into ("/cmd", "some args"); the command is
// lower-cased, the argument keeps its case.
func splitSlash(line string) (name, arg string) {
	line = strings.TrimSpace(line)
	if i := strings.IndexAny(line, " \t"); i >= 0 {
		return strings.ToLower(line[:i]), strings.TrimSpace(line[i+1:])
	}
	return strings.ToLower(line), ""
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

func clip(s string, n int) string {
	s = strings.TrimSpace(s)
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	if n <= 1 {
		return string(r[:n])
	}
	return string(r[:n-1]) + "…"
}

// dirLabel is displayPath for a directory, but names the cwd "this folder".
func dirLabel(p string) string {
	if d := displayPath(p); d != "." && d != "" {
		return d
	}
	return "this folder"
}

// displayPath shortens a path for display: relative to the cwd when inside it,
// else ~-relative to home, else the absolute path.
func displayPath(p string) string {
	if abs, err := filepath.Abs(p); err == nil {
		p = abs
	}
	if cwd, err := os.Getwd(); err == nil {
		if rel, err := filepath.Rel(cwd, p); err == nil && !strings.HasPrefix(rel, "..") {
			return rel
		}
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" && strings.HasPrefix(p, home) {
		return "~" + p[len(home):]
	}
	return p
}
