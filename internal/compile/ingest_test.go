package compile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGather(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "notes.md"), "# Notes\nimportant procedure")
	write(t, filepath.Join(dir, "data.json"), `{"k":"v"}`)
	write(t, filepath.Join(dir, "image.png"), "\x00\x01\x02binary")    // skipped (binary)
	write(t, filepath.Join(dir, "skip.bin"), "whatever")               // skipped (ext)
	write(t, filepath.Join(dir, ".git", "config"), "[core]")           // skipped (dir)
	write(t, filepath.Join(dir, "node_modules", "x", "i.js"), "var x") // skipped (dir)

	res, err := Gather([]string{dir}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.Corpus, "important procedure") || !strings.Contains(res.Corpus, `"k":"v"`) {
		t.Fatalf("expected md+json content, got:\n%s", res.Corpus)
	}
	if strings.Contains(res.Corpus, "var x") || strings.Contains(res.Corpus, "[core]") {
		t.Fatal("expected skipped dirs to be excluded")
	}
	if len(res.Sources) != 2 {
		t.Fatalf("expected 2 sources, got %d (%+v)", len(res.Sources), res.Sources)
	}
}

func TestGatherExplicitSkipDir(t *testing.T) {
	// Pointing compile directly at a normally-skipped dir must still read it.
	dir := t.TempDir()
	vendor := filepath.Join(dir, "vendor")
	write(t, filepath.Join(vendor, "lib.md"), "vendor docs here")
	res, err := Gather([]string{vendor}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Sources) != 1 || !strings.Contains(res.Corpus, "vendor docs here") {
		t.Fatalf("explicit skip-dir root should be ingested: %+v", res)
	}
}

func TestGatherBudget(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "big.txt"), strings.Repeat("x", 5000))
	res, err := Gather([]string{dir}, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Corpus) > 300 { // header + 100 bytes of content
		t.Fatalf("corpus not capped to budget: %d bytes", len(res.Corpus))
	}
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
