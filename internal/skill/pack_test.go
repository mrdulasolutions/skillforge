package skill

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

func TestScaffoldThenPackage(t *testing.T) {
	parent := t.TempDir()
	res, err := Scaffold(ScaffoldOptions{
		Name:         "demo-skill",
		Description:  "Use when the user wants a demo skill scaffolded",
		IncludeEvals: true,
		OutDir:       parent,
	})
	if err != nil {
		t.Fatalf("scaffold: %v", err)
	}

	// What we scaffold must validate clean.
	if vr := Validate(res.SkillDir); !vr.Valid() {
		t.Fatalf("scaffolded skill invalid: %q", vr.FirstError())
	}

	// Add artifacts that must be excluded from the package.
	mustWrite(t, filepath.Join(res.SkillDir, ".DS_Store"), "junk")
	mustWrite(t, filepath.Join(res.SkillDir, "node_modules", "pkg", "index.js"), "x")
	// Machine-specific MCP artifacts the /mcp command writes — must not travel.
	mustWrite(t, filepath.Join(res.SkillDir, ".mcp.json"), `{"mcpServers":{}}`)
	mustWrite(t, filepath.Join(res.SkillDir, "schemas", "demo-skill.mcp.json"), "{}")

	pr, err := Package(res.SkillDir, parent)
	if err != nil {
		t.Fatalf("package: %v", err)
	}

	got := map[string]bool{}
	zr, err := zip.OpenReader(pr.Output)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	for _, f := range zr.File {
		got[f.Name] = true
	}

	if !got["demo-skill/SKILL.md"] {
		t.Errorf("expected demo-skill/SKILL.md in archive, got %v", keys(got))
	}
	for _, excluded := range []string{
		"demo-skill/evals/evals.json",
		"demo-skill/.DS_Store",
		"demo-skill/node_modules/pkg/index.js",
		"demo-skill/.mcp.json",
		"demo-skill/schemas/demo-skill.mcp.json",
	} {
		if got[excluded] {
			t.Errorf("expected %q to be excluded from archive", excluded)
		}
	}
}

func TestScaffoldRejectsBadName(t *testing.T) {
	if _, err := Scaffold(ScaffoldOptions{Name: "Bad Name", OutDir: t.TempDir()}); err == nil {
		t.Fatal("expected error for invalid name")
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func keys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
