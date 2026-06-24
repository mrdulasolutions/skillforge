package forge

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mrdulasolutions/skillforge/internal/tui"
)

// writeSkill scaffolds a minimal valid skill under parent and returns its dir.
func writeSkill(t *testing.T, parent, name, desc string) string {
	t.Helper()
	dir := filepath.Join(parent, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	md := "---\nname: " + name + "\ndescription: " + desc + "\n---\n\n# " + name + "\n\nDo the thing.\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(md), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// modelAt builds a ready chat model rooted at a chosen output dir.
func modelAt(t *testing.T, parent string) model {
	t.Helper()
	m := newModel(context.Background(), stubProvider{reply: "x"}, nil, tui.WizardResult{}, parent)
	nm, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	return nm.(model)
}

func lastMsg(m model) string {
	if len(m.msgs) == 0 {
		return ""
	}
	return m.msgs[len(m.msgs)-1].text
}

func TestFindSkills(t *testing.T) {
	dir := t.TempDir()
	writeSkill(t, dir, "alpha-reciter", "Recite the alphabet on request.")
	writeSkill(t, dir, "pdf-extractor", "Pull text out of PDFs.")
	_ = os.MkdirAll(filepath.Join(dir, "notes"), 0o755) // not a skill

	got := findSkills(dir)
	if len(got) != 2 {
		t.Fatalf("want 2 skills, got %d: %+v", len(got), got)
	}
	names := map[string]bool{}
	for _, s := range got {
		names[s.name] = true
	}
	if !names["alpha-reciter"] || !names["pdf-extractor"] {
		t.Fatalf("unexpected names: %+v", names)
	}
}

func TestSlashSkillsListsBuilt(t *testing.T) {
	dir := t.TempDir()
	writeSkill(t, dir, "alpha-reciter", "Recite the alphabet on request.")
	nm, _ := modelAt(t, dir).runSlash("/skills", "")
	if got := lastMsg(nm.(model)); !strings.Contains(got, "alpha-reciter") {
		t.Fatalf("/skills should list the built skill, got: %q", got)
	}
}

func TestSlashSkillsEmpty(t *testing.T) {
	nm, _ := modelAt(t, t.TempDir()).runSlash("/skills", "")
	if got := lastMsg(nm.(model)); !strings.Contains(got, "no built skills") {
		t.Fatalf("expected an empty-state message, got: %q", got)
	}
}

func TestSlashExportPackages(t *testing.T) {
	dir := t.TempDir()
	writeSkill(t, dir, "alpha-reciter", "Recite the alphabet. Use when asked for letters.")
	nm, _ := modelAt(t, dir).runSlash("/export", "alpha-reciter")
	if got := lastMsg(nm.(model)); !strings.Contains(got, "exported") {
		t.Fatalf("expected export success, got: %q", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "alpha-reciter.skill")); err != nil {
		t.Fatalf(".skill bundle not created: %v", err)
	}
}

func TestSlashExportUnknown(t *testing.T) {
	dir := t.TempDir()
	writeSkill(t, dir, "alpha-reciter", "Recite the alphabet.")
	nm, _ := modelAt(t, dir).runSlash("/export", "nope")
	if got := lastMsg(nm.(model)); !strings.Contains(got, "no built skill named") {
		t.Fatalf("expected unknown-skill message, got: %q", got)
	}
}

func TestSlashMCPWritesConfigAndSchemas(t *testing.T) {
	dir := t.TempDir()
	writeSkill(t, dir, "pdf-extractor", "Pull text out of PDFs. Use when given a PDF.")
	nm, _ := modelAt(t, dir).runSlash("/mcp", "pdf-extractor")
	if got := lastMsg(nm.(model)); !strings.Contains(got, "serve-mcp") {
		t.Fatalf("expected serve-mcp instructions, got: %q", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "pdf-extractor", ".mcp.json")); err != nil {
		t.Fatalf(".mcp.json not written: %v", err)
	}
	for _, f := range []string{"pdf-extractor.mcp.json", "pdf-extractor.openai.json", "pdf-extractor.anthropic.json"} {
		if _, err := os.Stat(filepath.Join(dir, "pdf-extractor", "schemas", f)); err != nil {
			t.Errorf("missing cross-provider schema %s: %v", f, err)
		}
	}
}

func TestSlashMCPNeverClobbers(t *testing.T) {
	dir := t.TempDir()
	sd := writeSkill(t, dir, "pdf-extractor", "Pull text out of PDFs. Use when given a PDF.")
	custom := `{"mcpServers":{"my-own":{"command":"x"}}}`
	if err := os.WriteFile(filepath.Join(sd, ".mcp.json"), []byte(custom), 0o644); err != nil {
		t.Fatal(err)
	}
	nm, _ := modelAt(t, dir).runSlash("/mcp", "pdf-extractor")
	if got := lastMsg(nm.(model)); !strings.Contains(got, "left it untouched") {
		t.Fatalf("expected a no-clobber notice, got: %q", got)
	}
	b, _ := os.ReadFile(filepath.Join(sd, ".mcp.json"))
	if string(b) != custom {
		t.Fatalf("existing .mcp.json was overwritten: %q", string(b))
	}
}

func TestResolveTargetAmbiguous(t *testing.T) {
	dir := t.TempDir()
	// Two different folders whose frontmatter names collide.
	writeSkill(t, dir, "dup-a", "First.")
	writeSkill(t, dir, "dup-b", "Second.")
	skills := findSkills(dir)
	for i := range skills { // force a name collision
		skills[i].name = "dup"
	}
	_, deny := resolveTarget(dir, skills, "dup", "export")
	if deny == nil || !strings.Contains(deny[0].text, "more than one") {
		t.Fatalf("expected ambiguity message, got: %+v", deny)
	}
}

func TestSplitSlash(t *testing.T) {
	cases := []struct{ in, name, arg string }{
		{"/export", "/export", ""},
		{"/export my-skill", "/export", "my-skill"},
		{"  /MCP  foo bar ", "/mcp", "foo bar"},
	}
	for _, c := range cases {
		if n, a := splitSlash(c.in); n != c.name || a != c.arg {
			t.Errorf("splitSlash(%q) = (%q,%q), want (%q,%q)", c.in, n, a, c.name, c.arg)
		}
	}
}

// Typing a slash command WITH an argument must route to runSlash, not the AI.
func TestSubmitRoutesSlashWithArg(t *testing.T) {
	dir := t.TempDir()
	writeSkill(t, dir, "alpha-reciter", "Recite the alphabet. Use when asked for letters.")
	m := modelAt(t, dir)
	nm, _ := m.handleSubmit("/export alpha-reciter")
	out := nm.(model)
	if got := lastMsg(out); !strings.Contains(got, "exported") {
		t.Fatalf("typed '/export <name>' should export, got: %q", got)
	}
	// must NOT have been recorded as a chat/transcript turn
	for _, c := range out.msgs {
		if c.role == roleUser && strings.HasPrefix(c.text, "/export") {
			t.Fatal("slash command leaked into the chat as a user message")
		}
	}
}
