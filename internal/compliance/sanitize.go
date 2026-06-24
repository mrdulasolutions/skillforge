package compliance

import (
	"regexp"
	"strings"
)

// SanitizeResult reports the cleaned string and any flags raised.
type SanitizeResult struct {
	Cleaned        string
	Flags          []string
	OriginalLength int
	CleanedLength  int
}

var (
	reZeroWidth  = regexp.MustCompile(`[\x{200B}-\x{200D}\x{2060}-\x{206F}\x{FEFF}]`)
	reBidi       = regexp.MustCompile(`[\x{202A}-\x{202E}\x{2066}-\x{2069}\x{200E}\x{200F}\x{061C}]`)
	reControl    = regexp.MustCompile(`[\x{00}-\x{08}\x{0B}\x{0C}\x{0E}-\x{1F}\x{7F}]`)
	reWhitespace = regexp.MustCompile(`\s`)
	reShellMeta  = regexp.MustCompile("[;|&$`\n\r]")
)

// homoglyphs maps Cyrillic/Greek look-alikes to ASCII (party-name spoofing).
var homoglyphs = map[rune]rune{
	'А': 'A', 'В': 'B', 'Е': 'E', 'К': 'K',
	'М': 'M', 'Н': 'H', 'О': 'O', 'Р': 'P',
	'С': 'C', 'Т': 'T', 'Х': 'X',
	'а': 'a', 'е': 'e', 'к': 'k', 'о': 'o',
	'р': 'p', 'с': 'c', 'х': 'x', 'у': 'y',
	'Α': 'A', 'Β': 'B', 'Ε': 'E', 'Ζ': 'Z',
	'Η': 'H', 'Ι': 'I', 'Κ': 'K', 'Μ': 'M',
	'Ν': 'N', 'Ο': 'O', 'Ρ': 'P', 'Τ': 'T',
	'Υ': 'Y', 'Χ': 'X',
}

var injectionPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\b(ignore|disregard|forget)\b.{0,20}\b(previous|prior|above|earlier)\b.{0,20}\b(instruction|prompt|rule|system)`),
	regexp.MustCompile(`(?i)\bsystem\s*[:>]\s*you\s+are\b`),
	regexp.MustCompile(`(?i)\b(jailbreak|developer\s*mode|DAN\b|godmode)\b`),
	regexp.MustCompile(`<\|im_(start|end)\|>`),
	regexp.MustCompile(`(?i)\[\[?(SYSTEM|INST|ASSISTANT)\]?\]`),
}

// Sanitize treats input as data (never instructions), strips invisible/spoofing
// characters, and flags injection patterns. field tightens strict fields like
// "eccn", "country_iso2", and "path".
func Sanitize(input, field string) SanitizeResult {
	flags := []string{}
	cleaned := input

	if reZeroWidth.MatchString(cleaned) {
		flags = append(flags, "zero_width")
	}
	if reBidi.MatchString(cleaned) {
		flags = append(flags, "bidi_override")
	}
	if reControl.MatchString(cleaned) {
		flags = append(flags, "control_chars")
	}
	cleaned = reZeroWidth.ReplaceAllString(cleaned, "")
	cleaned = reBidi.ReplaceAllString(cleaned, "")
	cleaned = reControl.ReplaceAllString(cleaned, "")

	homoHit := false
	var b strings.Builder
	for _, r := range cleaned {
		if mapped, ok := homoglyphs[r]; ok {
			homoHit = true
			b.WriteRune(mapped)
		} else {
			b.WriteRune(r)
		}
	}
	cleaned = b.String()
	if homoHit {
		flags = append(flags, "homoglyph_normalized")
	}

	for _, p := range injectionPatterns {
		if p.MatchString(cleaned) {
			flags = append(flags, "injection_pattern")
			break
		}
	}

	switch field {
	case "eccn", "country_iso2", "path":
		if reWhitespace.MatchString(cleaned) {
			flags = append(flags, "whitespace_in_strict_field")
		}
	}
	if field == "path" && reShellMeta.MatchString(cleaned) {
		flags = append(flags, "shell_metacharacters")
	}

	return SanitizeResult{
		Cleaned:        cleaned,
		Flags:          flags,
		OriginalLength: len([]rune(input)),
		CleanedLength:  len([]rune(cleaned)),
	}
}

// IsBlocking reports whether flags indicate a deliberate injection/shell escape.
func IsBlocking(flags []string) bool {
	for _, f := range flags {
		if f == "injection_pattern" || f == "shell_metacharacters" || f == "bidi_override" {
			return true
		}
	}
	return false
}
