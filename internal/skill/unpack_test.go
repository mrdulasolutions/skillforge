package skill

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPackUnpackRoundTrip(t *testing.T) {
	src := t.TempDir()
	res, err := Scaffold(ScaffoldOptions{
		Name:         "round-trip",
		Description:  "Use when round-tripping a skill through pack/unpack.",
		IncludeEvals: true,
		OutDir:       src,
	})
	if err != nil {
		t.Fatal(err)
	}

	outDir := t.TempDir()
	pr, err := Package(res.SkillDir, outDir)
	if err != nil {
		t.Fatal(err)
	}

	dest := t.TempDir()
	skillDir, err := Unpack(pr.Output, dest)
	if err != nil {
		t.Fatalf("unpack: %v", err)
	}
	if filepath.Base(skillDir) != "round-trip" {
		t.Fatalf("unexpected skill dir: %s", skillDir)
	}
	if vr := Validate(skillDir); !vr.Valid() {
		t.Fatalf("unpacked skill invalid: %q", vr.FirstError())
	}
	if _, err := os.Stat(filepath.Join(skillDir, "references", "reference.md")); err != nil {
		t.Fatalf("expected references/reference.md: %v", err)
	}
	// evals/ was excluded by Package, so it must be absent after unpack.
	if _, err := os.Stat(filepath.Join(skillDir, "evals")); err == nil {
		t.Fatal("evals/ should not be present in the packaged skill")
	}
}
