package skill

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/mrdulasolutions/skillforge/internal/assets"
)

// ScaffoldOptions controls what `skillforge new` generates.
type ScaffoldOptions struct {
	Name         string
	Description  string
	Type         string // "skill" (default) or "plugin"
	IncludeEvals bool
	Compliance   bool
	OutDir       string // parent directory; defaults to "."
	Force        bool
}

// ScaffoldResult reports what was generated.
type ScaffoldResult struct {
	Root     string   // the created top-level folder
	SkillDir string   // where SKILL.md lives (== Root for skills)
	Created  []string // paths relative to the parent dir
}

type tmplData struct {
	Name        string
	Description string
	Title       string
	Compliance  bool
	Plugin      bool
}

var scaffoldFuncs = template.FuncMap{
	// q renders a value as a JSON string literal (quoted + escaped).
	"q": func(s string) (string, error) {
		b, err := json.Marshal(s)
		return string(b), err
	},
}

// ValidateName enforces the same naming rules as the validator.
func ValidateName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("name is required")
	}
	if !nameRe.MatchString(name) {
		return fmt.Errorf("name %q must be kebab-case (lowercase letters, digits, and hyphens only)", name)
	}
	if strings.HasPrefix(name, "-") || strings.HasSuffix(name, "-") || strings.Contains(name, "--") {
		return fmt.Errorf("name %q cannot start/end with a hyphen or contain consecutive hyphens", name)
	}
	if len(name) > 64 {
		return fmt.Errorf("name is too long (%d characters); maximum is 64", len(name))
	}
	return nil
}

// Scaffold creates a new skill (or plugin-wrapped skill) on disk.
func Scaffold(opts ScaffoldOptions) (*ScaffoldResult, error) {
	name := strings.TrimSpace(opts.Name)
	if err := ValidateName(name); err != nil {
		return nil, err
	}
	desc := strings.TrimSpace(opts.Description)
	if desc == "" {
		desc = "TODO: one-line, trigger-rich description. Use when the user ..."
	}
	if strings.ContainsAny(desc, "<>") {
		return nil, fmt.Errorf("description cannot contain angle brackets (< or >)")
	}

	parent := opts.OutDir
	if parent == "" {
		parent = "."
	}
	plugin := opts.Type == "plugin"
	root := filepath.Join(parent, name)
	if _, err := os.Stat(root); err == nil && !opts.Force {
		return nil, fmt.Errorf("%s already exists (use --force to overwrite)", root)
	}
	skillDir := root
	if plugin {
		skillDir = filepath.Join(root, "skills", name)
	}

	data := tmplData{
		Name:        name,
		Description: desc,
		Title:       titleFromName(name),
		Compliance:  opts.Compliance,
		Plugin:      plugin,
	}
	res := &ScaffoldResult{Root: root, SkillDir: skillDir}
	add := func(p string) {
		if rel, err := filepath.Rel(parent, p); err == nil {
			res.Created = append(res.Created, rel)
		} else {
			res.Created = append(res.Created, p)
		}
	}

	// SKILL.md = handwritten frontmatter (guaranteed valid) + rendered body.
	body, err := render("skill/SKILL.body.md.tmpl", data)
	if err != nil {
		return nil, err
	}
	skillMD := buildFrontmatter(name, desc) + "\n" + body
	p := filepath.Join(skillDir, "SKILL.md")
	if err := writeFile(p, skillMD); err != nil {
		return nil, err
	}
	add(p)

	// references/reference.md
	if err := renderToFile("skill/reference.md.tmpl", filepath.Join(skillDir, "references", "reference.md"), data, add); err != nil {
		return nil, err
	}

	if opts.IncludeEvals {
		if err := renderToFile("skill/evals.json.tmpl", filepath.Join(skillDir, "evals", "evals.json"), data, add); err != nil {
			return nil, err
		}
	}

	if opts.Compliance {
		if err := renderToFile("compliance/disclosure.md.tmpl", filepath.Join(skillDir, "references", "disclosure.md"), data, add); err != nil {
			return nil, err
		}
	}

	if plugin {
		if err := renderToFile("plugin/plugin.json.tmpl", filepath.Join(root, ".claude-plugin", "plugin.json"), data, add); err != nil {
			return nil, err
		}
		if err := renderToFile("plugin/marketplace.json.tmpl", filepath.Join(root, "marketplace.json"), data, add); err != nil {
			return nil, err
		}
	}
	return res, nil
}

func buildFrontmatter(name, desc string) string {
	return "---\nname: " + name + "\ndescription: " + yamlDQ(desc) + "\n---\n"
}

// yamlDQ renders s as a YAML double-quoted scalar (always valid on one line).
func yamlDQ(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '"':
			b.WriteString(`\"`)
		case '\n':
			b.WriteString(`\n`)
		case '\t':
			b.WriteString(`\t`)
		case '\r':
			b.WriteString(`\r`)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

func titleFromName(name string) string {
	parts := strings.Split(name, "-")
	for i, p := range parts {
		if p != "" {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, " ")
}

func render(tmplPath string, data tmplData) (string, error) {
	b, err := assets.FS.ReadFile("templates/" + tmplPath)
	if err != nil {
		return "", err
	}
	t, err := template.New(filepath.Base(tmplPath)).Funcs(scaffoldFuncs).Parse(string(b))
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func renderToFile(tmplPath, outPath string, data tmplData, add func(string)) error {
	out, err := render(tmplPath, data)
	if err != nil {
		return err
	}
	if err := writeFile(outPath, out); err != nil {
		return err
	}
	add(outPath)
	return nil
}

func writeFile(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}
