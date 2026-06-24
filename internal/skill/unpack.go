package skill

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Unpack extracts a .skill (zip) into destDir and returns the extracted skill
// directory. It guards against zip-slip and validates the result.
func Unpack(skillFile, destDir string) (string, error) {
	zr, err := zip.OpenReader(skillFile)
	if err != nil {
		return "", fmt.Errorf("open %s: %w", skillFile, err)
	}
	defer zr.Close()

	if destDir == "" {
		destDir = "."
	}
	destAbs, err := filepath.Abs(destDir)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(destAbs, 0o755); err != nil {
		return "", err
	}

	topDir := ""
	var total int64
	for _, f := range zr.File {
		clean := filepath.Clean(f.Name)
		if clean == "." || strings.HasPrefix(clean, "..") || filepath.IsAbs(clean) {
			continue // zip-slip / absolute path
		}
		target := filepath.Join(destAbs, clean)
		if target != destAbs && !strings.HasPrefix(target, destAbs+string(os.PathSeparator)) {
			continue // escapes destDir
		}
		if topDir == "" {
			topDir = strings.SplitN(clean, string(os.PathSeparator), 2)[0]
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return "", err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return "", err
		}
		limit := int64(maxEntryBytes)
		if rem := int64(maxTotalBytes) - total; rem < limit {
			limit = rem
		}
		if limit <= 0 {
			return "", fmt.Errorf("archive exceeds the %d-byte limit", maxTotalBytes)
		}
		n, err := writeZipFile(f, target, limit)
		if err != nil {
			return "", err
		}
		total += n
	}
	if topDir == "" {
		return "", fmt.Errorf("%s contains no files", skillFile)
	}

	skillDir := filepath.Join(destAbs, topDir)
	if res := Validate(skillDir); !res.Valid() {
		return skillDir, fmt.Errorf("imported skill is invalid: %s", res.FirstError())
	}
	return skillDir, nil
}

const (
	maxEntryBytes = 64 << 20  // 64 MiB per file
	maxTotalBytes = 256 << 20 // 256 MiB per archive
)

// writeZipFile copies one entry to target, capped at limit bytes (decompression
// bomb guard).
func writeZipFile(f *zip.File, target string, limit int64) (int64, error) {
	rc, err := f.Open()
	if err != nil {
		return 0, err
	}
	defer rc.Close()
	out, err := os.Create(target)
	if err != nil {
		return 0, err
	}
	defer out.Close()
	n, err := io.Copy(out, io.LimitReader(rc, limit+1))
	if err != nil {
		return n, err
	}
	if n > limit {
		return n, fmt.Errorf("archive entry too large: %s", f.Name)
	}
	return n, nil
}
