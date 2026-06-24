package skill

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScaffoldBodyOverride(t *testing.T) {
	parent := t.TempDir()
	body := "## When to use this skill\n\nUse when testing overrides.\n\n## Instructions\n\n1. Do the thing.\n"
	res, err := Scaffold(ScaffoldOptions{
		Name:         "override-demo",
		Description:  "Test the body override. Use when verifying AI body injection.",
		BodyOverride: body,
		OutDir:       parent,
	})
	if err != nil {
		t.Fatal(err)
	}
	if vr := Validate(res.SkillDir); !vr.Valid() {
		t.Fatalf("scaffolded skill invalid: %q", vr.FirstError())
	}
	content := readSkillMD(t, res.SkillDir)
	if !strings.HasPrefix(content, "---\nname: override-demo\n") {
		t.Fatalf("frontmatter not produced by buildFrontmatter:\n%.80s", content)
	}
	if !strings.Contains(content, "# Override Demo") {
		t.Fatal("expected H1 derived from the name")
	}
	if !strings.Contains(content, "Use when testing overrides.") {
		t.Fatal("expected the override body content")
	}
}

func TestScaffoldOverrideStripsFrontmatterAndH1(t *testing.T) {
	parent := t.TempDir()
	// The model wrongly included frontmatter and its own H1.
	body := "---\nname: wrong\n---\n\n# Wrong Title\n\n## When to use this skill\n\nBody.\n"
	res, err := Scaffold(ScaffoldOptions{
		Name:         "strip-demo",
		Description:  "Use when stripping bad model output.",
		BodyOverride: body,
		OutDir:       parent,
	})
	if err != nil {
		t.Fatal(err)
	}
	content := readSkillMD(t, res.SkillDir)
	if strings.Contains(content, "name: wrong") {
		t.Fatal("model frontmatter leaked into SKILL.md")
	}
	if strings.Contains(content, "# Wrong Title") {
		t.Fatal("model H1 leaked into SKILL.md")
	}
	if !strings.Contains(content, "# Strip Demo") {
		t.Fatal("expected the scaffold-owned H1")
	}
	if vr := Validate(res.SkillDir); !vr.Valid() {
		t.Fatalf("invalid: %q", vr.FirstError())
	}
}

func readSkillMD(t *testing.T, dir string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
