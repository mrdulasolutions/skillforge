package skill

import (
	"os"
	"path/filepath"
	"strings"

	yaml "gopkg.in/yaml.v3"
)

// frontmatterReason is a sentinel describing why frontmatter extraction failed.
type frontmatterReason int

const (
	frontmatterOK frontmatterReason = iota
	frontmatterMissing
	frontmatterMalformed
)

// splitFrontmatter mirrors quick_validate.py's extraction: the body must start
// with "---\n", and the frontmatter runs up to the first "\n---". It returns
// the raw frontmatter YAML, the remaining body, and a reason on failure.
func splitFrontmatter(content string) (frontmatter, body string, reason frontmatterReason) {
	if !strings.HasPrefix(content, "---") {
		return "", "", frontmatterMissing
	}
	if !strings.HasPrefix(content, "---\n") {
		return "", "", frontmatterMalformed
	}
	rest := content[len("---\n"):]
	idx := strings.Index(rest, "\n---")
	if idx < 0 {
		return "", "", frontmatterMalformed
	}
	frontmatter = rest[:idx]
	// Body begins after the entire closing-fence line, so a non-standard fence
	// (----, ---x) doesn't leak stray characters into the body.
	afterFence := rest[idx+1:] // skip the leading '\n'
	if nl := strings.IndexByte(afterFence, '\n'); nl >= 0 {
		body = afterFence[nl+1:]
	}
	return frontmatter, body, frontmatterOK
}

// Load reads and parses the SKILL.md in dir into a Skill. It does not validate.
func Load(dir string) (*Skill, error) {
	content, err := os.ReadFile(filepath.Join(dir, "SKILL.md"))
	if err != nil {
		return nil, err
	}
	front, body, reason := splitFrontmatter(string(content))
	s := &Skill{Dir: dir, Body: body, Raw: map[string]any{}}
	if reason == frontmatterOK {
		_ = yaml.Unmarshal([]byte(front), &s.Raw)
		_ = yaml.Unmarshal([]byte(front), &s.Frontmatter)
	}
	return s, nil
}
