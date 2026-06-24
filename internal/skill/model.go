// Package skill is the core domain: parsing, validating, scaffolding, and
// packaging SKILL.md-format skills. It keeps strict format parity with the
// official skill-creator tooling so artifacts are interchangeable.
package skill

import "sort"

// Frontmatter mirrors the allowed YAML frontmatter keys of a SKILL.md file.
type Frontmatter struct {
	Name          string         `yaml:"name"`
	Description   string         `yaml:"description"`
	License       string         `yaml:"license,omitempty"`
	AllowedTools  string         `yaml:"allowed-tools,omitempty"`
	Compatibility string         `yaml:"compatibility,omitempty"`
	Metadata      map[string]any `yaml:"metadata,omitempty"`
}

// Skill is a parsed skill loaded from disk.
type Skill struct {
	Dir         string
	Frontmatter Frontmatter
	Body        string
	Raw         map[string]any // top-level frontmatter keys, for key validation
}

// Severity classifies a validation issue.
type Severity int

const (
	// SeverityError makes a skill invalid.
	SeverityError Severity = iota
	// SeverityWarning is advisory and does not affect validity.
	SeverityWarning
)

// Issue is a single validation finding.
type Issue struct {
	Severity Severity
	Message  string
}

// Result is the outcome of validating a skill.
type Result struct {
	Issues []Issue
}

func (r *Result) addError(msg string)   { r.Issues = append(r.Issues, Issue{SeverityError, msg}) }
func (r *Result) addWarning(msg string) { r.Issues = append(r.Issues, Issue{SeverityWarning, msg}) }

// Valid reports whether the skill has no error-severity issues.
func (r Result) Valid() bool {
	for _, i := range r.Issues {
		if i.Severity == SeverityError {
			return false
		}
	}
	return true
}

// Errors returns only error-severity issues.
func (r Result) Errors() []Issue { return r.filter(SeverityError) }

// Warnings returns only warning-severity issues.
func (r Result) Warnings() []Issue { return r.filter(SeverityWarning) }

func (r Result) filter(s Severity) []Issue {
	var out []Issue
	for _, i := range r.Issues {
		if i.Severity == s {
			out = append(out, i)
		}
	}
	return out
}

// FirstError returns the message of the first error, or "" if valid. Used to
// assert parity with quick_validate.py, which returns on the first failure.
func (r Result) FirstError() string {
	for _, i := range r.Issues {
		if i.Severity == SeverityError {
			return i.Message
		}
	}
	return ""
}

// sortedKeys returns map keys in deterministic order.
func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
