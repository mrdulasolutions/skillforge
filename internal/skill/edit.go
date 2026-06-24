package skill

import (
	"os"
	"path/filepath"
	"strings"

	yaml "gopkg.in/yaml.v3"
)

// UpdateDescription rewrites a skill's description, preserving the body and any
// other allowed frontmatter keys. Used by `build --optimize --fix`.
func UpdateDescription(dir, newDesc string) error {
	s, err := Load(dir)
	if err != nil {
		return err
	}
	s.Frontmatter.Description = newDesc
	content := emitFrontmatter(s.Frontmatter) + "\n" + strings.TrimLeft(s.Body, "\n")
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	return os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644)
}

// emitFrontmatter renders frontmatter with allowed keys in canonical order.
func emitFrontmatter(fm Frontmatter) string {
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString("name: " + fm.Name + "\n")
	b.WriteString("description: " + yamlDQ(fm.Description) + "\n")
	if fm.License != "" {
		b.WriteString("license: " + yamlDQ(fm.License) + "\n")
	}
	if fm.AllowedTools != "" {
		b.WriteString("allowed-tools: " + yamlDQ(fm.AllowedTools) + "\n")
	}
	if fm.Compatibility != "" {
		b.WriteString("compatibility: " + yamlDQ(fm.Compatibility) + "\n")
	}
	if len(fm.Metadata) > 0 {
		if mb, err := yaml.Marshal(map[string]any{"metadata": fm.Metadata}); err == nil {
			b.Write(mb)
		}
	}
	b.WriteString("---\n")
	return b.String()
}
