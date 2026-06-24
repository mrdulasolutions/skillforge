package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mrdulasolutions/skillforge/internal/compliance"
	"github.com/mrdulasolutions/skillforge/internal/skill"
)

// scaffoldSkill creates a valid skill under parent and returns its dir.
func scaffoldSkill(t *testing.T, parent, name string) string {
	t.Helper()
	res, err := skill.Scaffold(skill.ScaffoldOptions{
		Name:        name,
		Description: "Use when the user wants a " + name + " demo scaffolded for tests",
		OutDir:      parent,
	})
	if err != nil {
		t.Fatalf("scaffold: %v", err)
	}
	return res.SkillDir
}

func TestRunBuildValidAndInvalid(t *testing.T) {
	dir := scaffoldSkill(t, t.TempDir(), "build-demo")
	buildJSON = false
	if err := runBuild(nil, []string{dir}); err != nil {
		t.Fatalf("build of a scaffolded skill should pass: %v", err)
	}
	// Corrupt the frontmatter -> build must fail.
	bad := "---\nname: Bad Name\ndescription: has <angle> brackets\n---\n\n# x\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(bad), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runBuild(nil, []string{dir}); err == nil {
		t.Fatal("build of an invalid skill should error")
	}
}

func TestRunPackageWithComplianceWritesProvenance(t *testing.T) {
	parent := t.TempDir()
	dir := scaffoldSkill(t, parent, "pkg-demo")
	out := t.TempDir()
	packageOut = out
	packageCompliance = true
	defer func() { packageOut = ""; packageCompliance = false }()

	if err := runPackage(nil, []string{dir}); err != nil {
		t.Fatalf("package --compliance: %v", err)
	}
	if _, err := os.Stat(filepath.Join(out, "pkg-demo.skill")); err != nil {
		t.Fatalf(".skill not created: %v", err)
	}
	if _, err := os.Stat(filepath.Join(out, "pkg-demo.skill.provenance.json")); err != nil {
		t.Fatalf("provenance manifest not written: %v", err)
	}
}

func TestRunAuditVerify(t *testing.T) {
	parent := t.TempDir()
	dir := scaffoldSkill(t, parent, "audit-demo")
	// No log yet -> error.
	if err := runAuditVerify(nil, []string{dir}); err == nil {
		t.Fatal("audit verify without a log should error")
	}
	// Create a log entry, then it should verify clean.
	if _, err := compliance.Append(dir, compliance.Event{EventType: "test", Tool: "test"}); err != nil {
		t.Fatalf("append: %v", err)
	}
	if err := runAuditVerify(nil, []string{dir}); err != nil {
		t.Fatalf("audit verify of an intact log should pass: %v", err)
	}
}

func TestRunImportRoundTrip(t *testing.T) {
	parent := t.TempDir()
	dir := scaffoldSkill(t, parent, "rt-demo")
	out := t.TempDir()
	packageOut = out
	packageCompliance = false
	defer func() { packageOut = "" }()
	if err := runPackage(nil, []string{dir}); err != nil {
		t.Fatalf("package: %v", err)
	}

	dest := t.TempDir()
	importDir = dest
	defer func() { importDir = "." }()
	if err := runImport(nil, []string{filepath.Join(out, "rt-demo.skill")}); err != nil {
		t.Fatalf("import: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dest, "rt-demo", "SKILL.md")); err != nil {
		t.Fatalf("imported skill missing: %v", err)
	}
}

func TestRunSchema(t *testing.T) {
	dir := scaffoldSkill(t, t.TempDir(), "schema-demo")
	schemaFormat = "all"
	for _, f := range []string{"all", "mcp", "openai", "anthropic"} {
		schemaFormat = f
		if err := runSchema(nil, []string{dir}); err != nil {
			t.Fatalf("schema --format %s: %v", f, err)
		}
	}
	schemaFormat = "bogus"
	if err := runSchema(nil, []string{dir}); err == nil {
		t.Fatal("schema with an unknown format should error")
	}
	schemaFormat = "all"
}

func TestBlockPrivateDial(t *testing.T) {
	blocked := []string{"127.0.0.1:443", "10.0.0.5:443", "192.168.1.1:80", "169.254.1.1:80", "[::1]:443", "0.0.0.0:80"}
	for _, a := range blocked {
		if err := blockPrivateDial("tcp", a, nil); err == nil {
			t.Errorf("expected %s to be blocked", a)
		}
	}
	for _, a := range []string{"8.8.8.8:443", "1.1.1.1:80"} {
		if err := blockPrivateDial("tcp", a, nil); err != nil {
			t.Errorf("expected %s to be allowed, got %v", a, err)
		}
	}
}

func TestIsURL(t *testing.T) {
	for _, u := range []string{"http://x", "https://x"} {
		if !isURL(u) {
			t.Errorf("%s should be a URL", u)
		}
	}
	for _, u := range []string{"./file.skill", "file:///x", "ftp://x"} {
		if isURL(u) {
			t.Errorf("%s should not be a URL", u)
		}
	}
}
