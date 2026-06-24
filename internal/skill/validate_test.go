package skill

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateContent(t *testing.T) {
	cases := []struct {
		name      string
		content   string
		wantValid bool
		wantErr   string // expected FirstError when invalid
	}{
		{
			name:      "valid minimal",
			content:   "---\nname: my-skill\ndescription: Use when the user wants a thing\n---\n\n# Body\n",
			wantValid: true,
		},
		{
			name:    "no frontmatter",
			content: "# Just a heading\n",
			wantErr: "No YAML frontmatter found",
		},
		{
			name:    "malformed frontmatter",
			content: "---\nname: x\n",
			wantErr: "Invalid frontmatter format",
		},
		{
			name:    "unexpected key",
			content: "---\nname: my-skill\ndescription: Use when\nfoo: bar\n---\n",
			wantErr: "Unexpected key(s) in SKILL.md frontmatter: foo. Allowed properties are: allowed-tools, compatibility, description, license, metadata, name",
		},
		{
			name:    "missing name",
			content: "---\ndescription: Use when\n---\n",
			wantErr: "Missing 'name' in frontmatter",
		},
		{
			name:    "missing description",
			content: "---\nname: my-skill\n---\n",
			wantErr: "Missing 'description' in frontmatter",
		},
		{
			name:    "name not kebab-case",
			content: "---\nname: MySkill\ndescription: Use when\n---\n",
			wantErr: "Name 'MySkill' should be kebab-case (lowercase letters, digits, and hyphens only)",
		},
		{
			name:    "name consecutive hyphens",
			content: "---\nname: my--skill\ndescription: Use when\n---\n",
			wantErr: "Name 'my--skill' cannot start/end with hyphen or contain consecutive hyphens",
		},
		{
			name:    "angle brackets in description",
			content: "---\nname: my-skill\ndescription: Use when <thing> happens\n---\n",
			wantErr: "Description cannot contain angle brackets (< or >)",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := ValidateContent(tc.content)
			if r.Valid() != tc.wantValid {
				t.Fatalf("Valid()=%v want %v (issues: %+v)", r.Valid(), tc.wantValid, r.Issues)
			}
			if !tc.wantValid && r.FirstError() != tc.wantErr {
				t.Fatalf("FirstError()=%q\n        want %q", r.FirstError(), tc.wantErr)
			}
		})
	}
}

func TestNameTooLong(t *testing.T) {
	long := make([]byte, 65)
	for i := range long {
		long[i] = 'a'
	}
	content := "---\nname: " + string(long) + "\ndescription: Use when\n---\n"
	r := ValidateContent(content)
	if r.Valid() {
		t.Fatal("expected invalid for >64 char name")
	}
	if got := r.FirstError(); got != "Name is too long (65 characters). Maximum is 64 characters." {
		t.Fatalf("FirstError()=%q", got)
	}
}

func TestValidateMissingFile(t *testing.T) {
	dir := t.TempDir()
	r := Validate(dir)
	if r.Valid() || r.FirstError() != "SKILL.md not found" {
		t.Fatalf("got valid=%v err=%q", r.Valid(), r.FirstError())
	}
}

func TestValidateDirWarnings(t *testing.T) {
	dir := t.TempDir()
	content := "---\nname: my-skill\ndescription: A bare one-line summary with no cue words\n---\n\n# Body\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	r := Validate(dir)
	if !r.Valid() {
		t.Fatalf("expected valid, got %q", r.FirstError())
	}
	if len(r.Warnings()) == 0 {
		t.Fatal("expected a trigger-phrasing warning")
	}
}
