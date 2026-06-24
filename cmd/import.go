package cmd

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/mrdulasolutions/skillforge/internal/skill"
	"github.com/mrdulasolutions/skillforge/internal/tui"
	"github.com/spf13/cobra"
)

const maxDownloadBytes = 256 << 20 // 256 MiB

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

func downloadTemp(rawURL string) (string, func(), error) {
	u, err := url.Parse(rawURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return "", nil, fmt.Errorf("unsupported URL (only http/https): %s", rawURL)
	}
	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Get(rawURL)
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
	n, copyErr := io.Copy(tmp, io.LimitReader(resp.Body, maxDownloadBytes+1))
	tmp.Close()
	if copyErr != nil {
		os.Remove(tmp.Name())
		return "", nil, copyErr
	}
	if n > maxDownloadBytes {
		os.Remove(tmp.Name())
		return "", nil, fmt.Errorf("download exceeds the %d-byte limit", maxDownloadBytes)
	}
	return tmp.Name(), func() { os.Remove(tmp.Name()) }, nil
}
