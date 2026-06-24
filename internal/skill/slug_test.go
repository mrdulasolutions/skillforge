package skill

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSlugify(t *testing.T) {
	cases := map[string]string{
		"Alphabet Reciter": "alphabet-reciter",
		"a skill for reciting the alphabet in every language": "a-skill-for-reciting-the-alphabet-in-every-language",
		"pdf-extractor":    "pdf-extractor",
		"PDF → Markdown":   "pdf-markdown",
		"café Über":        "cafe-uber",
		"  --Foo__Bar--  ": "foo-bar",
		"C++ & Rust!!!":    "c-rust",
		"日本語":              "",
		"🔥🔥":               "",
		"!!!":              "",
		"":                 "",
	}
	for in, want := range cases {
		if got := Slugify(in); got != want {
			t.Errorf("Slugify(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSlugifyIdempotentAndValid(t *testing.T) {
	inputs := []string{
		"Alphabet Reciter", "PDF → Markdown", "café Über",
		"A Very Long Title That Goes On And On And On Past The Sixty Four Character Limit For Sure",
		"pdf-extractor", "  --Foo__Bar--  ",
	}
	for _, in := range inputs {
		s := Slugify(in)
		if s == "" {
			continue
		}
		if err := ValidateName(s); err != nil {
			t.Errorf("Slugify(%q)=%q is not a valid name: %v", in, s, err)
		}
		if Slugify(s) != s {
			t.Errorf("Slugify not idempotent: Slugify(%q)=%q", s, Slugify(s))
		}
		if len(s) > 64 {
			t.Errorf("Slugify(%q)=%q exceeds 64 chars", in, s)
		}
	}
}

func TestUniqueSlug(t *testing.T) {
	parent := t.TempDir()
	if got := UniqueSlug(parent, "foo"); got != "foo" {
		t.Fatalf("expected foo, got %q", got)
	}
	mustMkdir(t, filepath.Join(parent, "foo"))
	if got := UniqueSlug(parent, "foo"); got != "foo-2" {
		t.Fatalf("expected foo-2, got %q", got)
	}
	mustMkdir(t, filepath.Join(parent, "foo-2"))
	if got := UniqueSlug(parent, "foo"); got != "foo-3" {
		t.Fatalf("expected foo-3, got %q", got)
	}

	long := strings.Repeat("a", 64)
	mustMkdir(t, filepath.Join(parent, long))
	got := UniqueSlug(parent, long)
	if len(got) > 64 || ValidateName(got) != nil {
		t.Fatalf("UniqueSlug long base produced invalid %q (len %d)", got, len(got))
	}
}

func mustMkdir(t *testing.T, p string) {
	t.Helper()
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatal(err)
	}
}
