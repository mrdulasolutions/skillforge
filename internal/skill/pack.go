package skill

import (
	"archive/zip"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Exclusion rules ported from package_skill.py.
var (
	excludeDirs     = map[string]bool{"__pycache__": true, "node_modules": true}
	rootExcludeDirs = map[string]bool{"evals": true}
	excludeFiles    = map[string]bool{".DS_Store": true}
)

// PackResult reports what a packaging run produced.
type PackResult struct {
	Output  string
	Added   []string
	Skipped []string
}

// ValidationError wraps a failed validation so callers can render the issues.
type ValidationError struct{ Result Result }

func (e *ValidationError) Error() string { return "validation failed: " + e.Result.FirstError() }

// shouldExclude decides whether a path (relative to the skill's parent dir, so
// parts[0] is the skill folder name) is excluded from the package.
func shouldExclude(rel string) bool {
	parts := strings.Split(rel, string(os.PathSeparator))
	for _, p := range parts {
		if excludeDirs[p] {
			return true
		}
	}
	if len(parts) > 1 && rootExcludeDirs[parts[1]] {
		return true
	}
	base := parts[len(parts)-1]
	if excludeFiles[base] {
		return true
	}
	return strings.HasSuffix(base, ".pyc")
}

// Package validates the skill in skillDir, then zips it into <name>.skill under
// outDir (current dir if empty), excluding build artifacts and evals/. The
// archive preserves the top-level skill folder name, matching package_skill.py.
func Package(skillDir, outDir string) (*PackResult, error) {
	abs, err := filepath.Abs(filepath.Clean(skillDir))
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return nil, fmt.Errorf("skill folder not found: %s", abs)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("path is not a directory: %s", abs)
	}
	if res := Validate(abs); !res.Valid() {
		return nil, &ValidationError{Result: res}
	}

	parent := filepath.Dir(abs)
	name := filepath.Base(abs)
	if outDir == "" {
		if outDir, err = os.Getwd(); err != nil {
			return nil, err
		}
	} else if err := os.MkdirAll(outDir, 0o755); err != nil {
		return nil, err
	}

	out := filepath.Join(outDir, name+".skill")
	zf, err := os.Create(out)
	if err != nil {
		return nil, err
	}
	defer zf.Close()
	zw := zip.NewWriter(zf)

	result := &PackResult{Output: out}
	walkErr := filepath.WalkDir(abs, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(parent, path)
		if err != nil {
			return err
		}
		if shouldExclude(rel) {
			result.Skipped = append(result.Skipped, filepath.ToSlash(rel))
			return nil
		}
		w, err := zw.Create(filepath.ToSlash(rel))
		if err != nil {
			return err
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		if _, err := io.Copy(w, f); err != nil {
			return err
		}
		result.Added = append(result.Added, filepath.ToSlash(rel))
		return nil
	})
	if walkErr != nil {
		zw.Close()
		return nil, walkErr
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return result, nil
}
