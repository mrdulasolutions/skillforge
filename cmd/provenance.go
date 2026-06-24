package cmd

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/mrdulasolutions/skillforge/internal/skill"
)

// writeProvenance emits a sidecar manifest next to a packaged bundle pinning the
// tool version, a timestamp, the skill name, the bundle sha256, and the sha256 of
// every entry. Hashes are read from the bundle itself (not re-read from the source
// tree) so the manifest attests exactly what was packaged — no TOCTOU window and
// no silently-missing files.
func writeProvenance(skillDir string, pr *skill.PackResult) (string, error) {
	absSkill, err := filepath.Abs(skillDir)
	if err != nil {
		return "", err
	}

	bundleSum, err := sha256File(pr.Output)
	if err != nil {
		return "", err
	}

	zr, err := zip.OpenReader(pr.Output)
	if err != nil {
		return "", err
	}
	defer zr.Close()

	type fileHash struct {
		Path   string `json:"path"`
		SHA256 string `json:"sha256"`
	}
	files := make([]fileHash, 0, len(zr.File))
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		sum, err := sha256ZipEntry(f)
		if err != nil {
			return "", err
		}
		files = append(files, fileHash{Path: f.Name, SHA256: sum})
	}

	name := filepath.Base(absSkill)
	if s, lerr := skill.Load(absSkill); lerr == nil && s.Frontmatter.Name != "" {
		name = s.Frontmatter.Name
	}

	manifest := map[string]any{
		"tool":          "skillforge",
		"tool_version":  version,
		"generated_at":  time.Now().UTC().Format(time.RFC3339),
		"skill":         name,
		"bundle":        filepath.Base(pr.Output),
		"bundle_sha256": bundleSum,
		"files":         files,
	}
	out := pr.Output + ".provenance.json"
	b, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(out, append(b, '\n'), 0o644); err != nil {
		return "", err
	}
	return out, nil
}

func sha256ZipEntry(f *zip.File) (string, error) {
	rc, err := f.Open()
	if err != nil {
		return "", err
	}
	defer rc.Close()
	h := sha256.New()
	if _, err := io.Copy(h, rc); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
