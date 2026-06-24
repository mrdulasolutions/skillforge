package cmd

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/mrdulasolutions/skillforge/internal/skill"
	"github.com/mrdulasolutions/skillforge/internal/tui"
	"github.com/spf13/cobra"
)

var publishOut string

var publishCmd = &cobra.Command{
	Use:   "publish [skill-path]",
	Short: "Package a skill for sharing (.skill + manifest)",
	Long:  "Validate and package a skill into a .skill archive with a JSON manifest (name, description, sha256), and print how others install it.",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runPublish,
}

func init() {
	publishCmd.Flags().StringVarP(&publishOut, "out", "o", "", "output directory (default: current dir)")
}

func runPublish(_ *cobra.Command, args []string) error {
	path := "."
	if len(args) == 1 {
		path = args[0]
	}
	header("publish")

	pr, err := skill.Package(path, publishOut)
	if err != nil {
		var ve *skill.ValidationError
		if errors.As(err, &ve) {
			fmt.Println(tui.ValidationReport(ve.Result))
			return errors.New("publish aborted — fix validation errors first")
		}
		return err
	}

	sum, err := sha256File(pr.Output)
	if err != nil {
		return err
	}
	info, err := os.Stat(pr.Output)
	if err != nil {
		return err
	}

	manifest := map[string]any{
		"file":   filepath.Base(pr.Output),
		"sha256": sum,
		"bytes":  info.Size(),
	}
	if s, lerr := skill.Load(path); lerr == nil {
		manifest["name"] = s.Frontmatter.Name
		manifest["description"] = s.Frontmatter.Description
	}
	manifestPath := pr.Output + ".json"
	manifestWritten := false
	if b, merr := json.MarshalIndent(manifest, "", "  "); merr == nil {
		if werr := os.WriteFile(manifestPath, b, 0o644); werr == nil {
			manifestWritten = true
		} else {
			fmt.Println(tui.Warn("could not write manifest: " + werr.Error()))
		}
	}

	fmt.Println(tui.OK("Published " + tui.Code.Render(pr.Output)))
	fmt.Println()
	rows := [][2]string{
		{"sha256", sum[:16] + "…"},
		{"size", fmt.Sprintf("%d bytes", info.Size())},
	}
	if manifestWritten {
		rows = append(rows, [2]string{"manifest", filepath.Base(manifestPath)})
	}
	fmt.Println(tui.KV(rows))
	fmt.Println()
	fmt.Println(tui.Info("Share the .skill file. Others install it with: " + tui.Code.Render("skillforge import "+pr.Output)))
	return nil
}

func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
