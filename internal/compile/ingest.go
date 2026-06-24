// Package compile gathers user-provided material (files/folders) into a single
// text corpus for AI skill synthesis.
package compile

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Source records one ingested file.
type Source struct {
	Path  string
	Bytes int
}

// Result is the gathered corpus.
type Result struct {
	Corpus  string
	Sources []Source
	Skipped int
}

var textExts = map[string]bool{
	".md": true, ".markdown": true, ".txt": true, ".text": true, ".rst": true,
	".json": true, ".yaml": true, ".yml": true, ".toml": true, ".ini": true, ".cfg": true,
	".csv": true, ".tsv": true, ".html": true, ".htm": true, ".xml": true,
	".go": true, ".py": true, ".js": true, ".ts": true, ".tsx": true, ".jsx": true,
	".rb": true, ".rs": true, ".java": true, ".c": true, ".h": true, ".cpp": true,
	".sh": true, ".bash": true, ".sql": true, ".org": true,
}

var skipDirs = map[string]bool{
	".git": true, "node_modules": true, "vendor": true, "__pycache__": true,
	"dist": true, "build": true, ".venv": true, "venv": true,
}

const (
	maxFileBytes  = 64 * 1024
	defaultBudget = 60 * 1024
)

// Gather reads the given files/folders into one corpus, capped at budget bytes
// (defaultBudget when <= 0). Binary and non-text files are skipped.
func Gather(paths []string, budget int) (*Result, error) {
	if budget <= 0 {
		budget = defaultBudget
	}
	res := &Result{}
	var b strings.Builder
	used := 0

	add := func(path string) {
		if used >= budget {
			res.Skipped++
			return
		}
		if !textExts[strings.ToLower(filepath.Ext(path))] {
			res.Skipped++
			return
		}
		info, err := os.Stat(path)
		if err != nil || info.Size() > maxFileBytes {
			res.Skipped++
			return
		}
		data, err := os.ReadFile(path)
		if err != nil || !looksTextual(data) {
			res.Skipped++
			return
		}
		chunk := string(data)
		if remaining := budget - used; len(chunk) > remaining {
			chunk = chunk[:remaining]
		}
		b.WriteString("\n\n===== FILE: " + path + " =====\n")
		b.WriteString(chunk)
		used += len(chunk)
		res.Sources = append(res.Sources, Source{Path: path, Bytes: len(chunk)})
	}

	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			add(p)
			continue
		}
		_ = filepath.WalkDir(p, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				// Skip vendor/hidden dirs, but never the explicitly requested root.
				if path != p && (skipDirs[d.Name()] || strings.HasPrefix(d.Name(), ".")) {
					return fs.SkipDir
				}
				return nil
			}
			add(path)
			return nil
		})
	}
	res.Corpus = strings.TrimSpace(b.String())
	return res, nil
}

func looksTextual(data []byte) bool {
	n := len(data)
	if n > 8000 {
		n = 8000
	}
	for i := 0; i < n; i++ {
		if data[i] == 0 {
			return false
		}
	}
	return true
}
