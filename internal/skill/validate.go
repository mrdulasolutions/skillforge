package skill

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	yaml "gopkg.in/yaml.v3"
)

// allowedKeys is the exact set the official validator accepts.
var allowedKeys = map[string]bool{
	"name":          true,
	"description":   true,
	"license":       true,
	"allowed-tools": true,
	"metadata":      true,
	"compatibility": true,
}

var nameRe = regexp.MustCompile(`^[a-z0-9-]+$`)

// Validate validates the skill in dir. It reads SKILL.md, runs the format
// checks (parity with quick_validate.py), and adds advisory warnings.
func Validate(dir string) Result {
	var r Result
	content, err := os.ReadFile(filepath.Join(dir, "SKILL.md"))
	if err != nil {
		r.addError("SKILL.md not found")
		return r
	}
	r = ValidateContent(string(content))
	if !r.Valid() {
		return r
	}
	addStructureWarnings(&r, dir, string(content))
	return r
}

// ValidateContent runs the SKILL.md frontmatter checks against raw content,
// preserving the order and messages of quick_validate.py so the first error
// matches exactly. Best-practice warnings are appended only when valid.
func ValidateContent(content string) Result {
	var r Result

	front, _, reason := splitFrontmatter(content)
	switch reason {
	case frontmatterMissing:
		r.addError("No YAML frontmatter found")
		return r
	case frontmatterMalformed:
		r.addError("Invalid frontmatter format")
		return r
	}

	var raw map[string]any
	if err := yaml.Unmarshal([]byte(front), &raw); err != nil {
		r.addError(fmt.Sprintf("Invalid YAML in frontmatter: %v", err))
		return r
	}
	if raw == nil {
		r.addError("Frontmatter must be a YAML dictionary")
		return r
	}

	// Unexpected keys.
	var unexpected []string
	for k := range raw {
		if !allowedKeys[k] {
			unexpected = append(unexpected, k)
		}
	}
	if len(unexpected) > 0 {
		sort.Strings(unexpected)
		r.addError(fmt.Sprintf(
			"Unexpected key(s) in SKILL.md frontmatter: %s. Allowed properties are: %s",
			strings.Join(unexpected, ", "),
			strings.Join(sortedKeys(allowedKeys), ", "),
		))
		return r
	}

	// Required fields.
	if _, ok := raw["name"]; !ok {
		r.addError("Missing 'name' in frontmatter")
		return r
	}
	if _, ok := raw["description"]; !ok {
		r.addError("Missing 'description' in frontmatter")
		return r
	}

	// Name.
	name, ok := raw["name"].(string)
	if !ok {
		r.addError(fmt.Sprintf("Name must be a string, got %s", yamlTypeName(raw["name"])))
		return r
	}
	name = strings.TrimSpace(name)
	if name != "" {
		if !nameRe.MatchString(name) {
			r.addError(fmt.Sprintf("Name '%s' should be kebab-case (lowercase letters, digits, and hyphens only)", name))
			return r
		}
		if strings.HasPrefix(name, "-") || strings.HasSuffix(name, "-") || strings.Contains(name, "--") {
			r.addError(fmt.Sprintf("Name '%s' cannot start/end with hyphen or contain consecutive hyphens", name))
			return r
		}
		if len(name) > 64 {
			r.addError(fmt.Sprintf("Name is too long (%d characters). Maximum is 64 characters.", len(name)))
			return r
		}
	}

	// Description.
	desc, ok := raw["description"].(string)
	if !ok {
		r.addError(fmt.Sprintf("Description must be a string, got %s", yamlTypeName(raw["description"])))
		return r
	}
	desc = strings.TrimSpace(desc)
	if desc != "" {
		if strings.ContainsAny(desc, "<>") {
			r.addError("Description cannot contain angle brackets (< or >)")
			return r
		}
		if len(desc) > 1024 {
			r.addError(fmt.Sprintf("Description is too long (%d characters). Maximum is 1024 characters.", len(desc)))
			return r
		}
	}

	// Compatibility (optional, only checked when truthy).
	if v, ok := raw["compatibility"]; ok && isTruthy(v) {
		compat, ok := v.(string)
		if !ok {
			r.addError(fmt.Sprintf("Compatibility must be a string, got %s", yamlTypeName(v)))
			return r
		}
		if len(compat) > 500 {
			r.addError(fmt.Sprintf("Compatibility is too long (%d characters). Maximum is 500 characters.", len(compat)))
			return r
		}
	}

	// --- Advisory warnings (Skill Forge additions; never affect validity). ---
	if desc != "" && !hasTriggerPhrasing(desc) {
		r.addWarning(`Description has no explicit trigger phrasing (e.g. "Use when…"); skills under-trigger without it`)
	}
	return r
}

// addStructureWarnings adds advisory warnings about file size and references.
func addStructureWarnings(r *Result, dir, content string) {
	if lines := strings.Count(content, "\n") + 1; lines > 500 {
		r.addWarning(fmt.Sprintf("SKILL.md is %d lines; keep it under 500 and move detail into references/", lines))
	}
	refDir := filepath.Join(dir, "references")
	entries, err := os.ReadDir(refDir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(refDir, e.Name()))
		if err != nil {
			continue
		}
		text := string(b)
		lines := strings.Count(text, "\n") + 1
		if lines > 300 && !hasTOC(text) {
			r.addWarning(fmt.Sprintf("references/%s is %d lines without a table of contents", e.Name(), lines))
		}
	}
}

func hasTOC(text string) bool {
	low := strings.ToLower(text)
	return strings.Contains(low, "table of contents") || strings.Contains(low, "## contents")
}

func hasTriggerPhrasing(desc string) bool {
	low := strings.ToLower(desc)
	for _, cue := range []string{"use when", "use this", "when the user", "use for", "trigger", "invoke"} {
		if strings.Contains(low, cue) {
			return true
		}
	}
	return false
}

// isTruthy mirrors Python truthiness for the values yaml.v3 produces.
func isTruthy(v any) bool {
	switch t := v.(type) {
	case nil:
		return false
	case string:
		return t != ""
	case bool:
		return t
	case int:
		return t != 0
	case int64:
		return t != 0
	case float64:
		return t != 0
	case []any:
		return len(t) > 0
	case map[string]any:
		return len(t) > 0
	default:
		return true
	}
}

// yamlTypeName maps a decoded YAML value to a Python-style type name so error
// messages read the same as the reference validator's.
func yamlTypeName(v any) string {
	switch v.(type) {
	case nil:
		return "NoneType"
	case string:
		return "str"
	case bool:
		return "bool"
	case int, int64:
		return "int"
	case float64:
		return "float"
	case []any:
		return "list"
	case map[string]any:
		return "dict"
	default:
		return fmt.Sprintf("%T", v)
	}
}
