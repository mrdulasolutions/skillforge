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
		if err := writeZipFile(f, target); err != nil {
			return "", err
		}
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

func writeZipFile(f *zip.File, target string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	out, err := os.Create(target)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, rc)
	return err
}
