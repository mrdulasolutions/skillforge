package skill

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"
)

// foldMap folds common accented Latin letters to ASCII without pulling in
// golang.org/x/text. Runes not covered (CJK, emoji) fall through to separators.
var foldMap = map[rune]string{
	'á': "a", 'à': "a", 'â': "a", 'ä': "a", 'ã': "a", 'å': "a", 'ā': "a",
	'é': "e", 'è': "e", 'ê': "e", 'ë': "e", 'ē': "e",
	'í': "i", 'ì': "i", 'î': "i", 'ï': "i", 'ī': "i",
	'ó': "o", 'ò': "o", 'ô': "o", 'ö': "o", 'õ': "o", 'ø': "o", 'ō': "o",
	'ú': "u", 'ù': "u", 'û': "u", 'ü': "u", 'ū': "u",
	'ñ': "n", 'ç': "c", 'ß': "ss", 'æ': "ae", 'œ': "oe", 'ý': "y", 'ÿ': "y",
	'Á': "a", 'À': "a", 'Â': "a", 'Ä': "a", 'Ã': "a", 'Å': "a",
	'É': "e", 'È': "e", 'Ê': "e", 'Ë': "e",
	'Í': "i", 'Ì': "i", 'Î': "i", 'Ï': "i",
	'Ó': "o", 'Ò': "o", 'Ô': "o", 'Ö': "o", 'Õ': "o", 'Ø': "o",
	'Ú': "u", 'Ù': "u", 'Û': "u", 'Ü': "u",
	'Ñ': "n", 'Ç': "c", 'Æ': "ae", 'Œ': "oe",
}

// Slugify derives a valid kebab-case skill name from arbitrary text (a title or
// concept). The result always satisfies ValidateName, or is "" when the input
// has no usable letters/digits.
func Slugify(s string) string {
	var b strings.Builder
	prevHyphen := false
	emit := func(r rune) {
		b.WriteRune(r)
		prevHyphen = false
	}
	emitHyphen := func() {
		if b.Len() > 0 && !prevHyphen {
			b.WriteByte('-')
			prevHyphen = true
		}
	}
	for _, r := range s {
		if rep, ok := foldMap[r]; ok {
			for _, rr := range rep {
				emit(rr)
			}
			continue
		}
		lr := unicode.ToLower(r)
		switch {
		case lr >= 'a' && lr <= 'z', lr >= '0' && lr <= '9':
			emit(lr)
		default:
			emitHyphen()
		}
	}
	out := truncateSlug(strings.Trim(b.String(), "-"), 64)
	if ValidateName(out) != nil {
		return ""
	}
	return out
}

// truncateSlug caps s at max bytes, preferring to cut at a hyphen near the end
// rather than mid-word, and never leaves a trailing hyphen.
func truncateSlug(s string, max int) string {
	if len(s) <= max {
		return s
	}
	cut := s[:max]
	if i := strings.LastIndexByte(cut, '-'); i > 0 && i >= max-12 {
		cut = cut[:i]
	}
	return strings.Trim(cut, "-")
}

// UniqueSlug returns base, or base-2/base-3/... when parent/<name> already
// exists, keeping every candidate <=64 and valid.
func UniqueSlug(parent, base string) string {
	if base == "" {
		return ""
	}
	if !pathExists(filepath.Join(parent, base)) {
		return base
	}
	for n := 2; n < 1000; n++ {
		suffix := "-" + strconv.Itoa(n)
		cand := strings.TrimRight(truncateSlug(base, 64-len(suffix)), "-") + suffix
		if ValidateName(cand) == nil && !pathExists(filepath.Join(parent, cand)) {
			return cand
		}
	}
	return base
}

func pathExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
