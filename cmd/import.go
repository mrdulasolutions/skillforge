package cmd

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/mrdulasolutions/skillforge/internal/skill"
	"github.com/mrdulasolutions/skillforge/internal/tui"
	"github.com/spf13/cobra"
)

var importDir string

var importCmd = &cobra.Command{
	Use:   "import <file-or-url>",
	Short: "Install a skill from a .skill file or URL",
	Args:  cobra.ExactArgs(1),
	RunE:  runImport,
}

func init() {
	importCmd.Flags().StringVarP(&importDir, "dir", "C", ".", "directory to install the skill into")
}

func runImport(_ *cobra.Command, args []string) error {
	header("import")
	file := args[0]
	if isURL(file) {
		fmt.Println(tui.Info("downloading " + file))
		f, cleanup, err := downloadTemp(file)
		if err != nil {
			return err
		}
		defer cleanup()
		file = f
	}

	skillDir, err := skill.Unpack(file, importDir)
	if err != nil {
		if skillDir != "" {
			fmt.Println(tui.Warn("extracted to " + skillDir + ", but " + err.Error()))
		}
		return err
	}

	fmt.Println(tui.OK("Imported " + tui.Code.Render(skillDir)))
	if s, lerr := skill.Load(skillDir); lerr == nil {
		fmt.Println()
		fmt.Println(tui.KV([][2]string{
			{"name", s.Frontmatter.Name},
			{"description", s.Frontmatter.Description},
		}))
	}
	return nil
}

func isURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}

func downloadTemp(url string) (string, func(), error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return "", nil, fmt.Errorf("download failed: HTTP %d", resp.StatusCode)
	}
	tmp, err := os.CreateTemp("", "skillforge-*.skill")
	if err != nil {
		return "", nil, err
	}
	if _, err := io.Copy(tmp, resp.Body); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return "", nil, err
	}
	tmp.Close()
	return tmp.Name(), func() { os.Remove(tmp.Name()) }, nil
}
