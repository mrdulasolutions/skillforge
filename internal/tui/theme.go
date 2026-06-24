// Package tui holds Skill Forge's terminal styling: the forge-fire palette,
// the gradient banner, and reusable rendering helpers.
package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Forge-fire palette.
var (
	FireFrom = rgb{0xFF, 0x3D, 0x00} // deep orange-red
	FireTo   = rgb{0xFF, 0xC4, 0x00} // amber/gold

	ColPrimary = lipgloss.Color("#FF8C42")
	ColAccent  = lipgloss.Color("#36D7B7")
	ColMuted   = lipgloss.AdaptiveColor{Light: "#6B7280", Dark: "#8B8B9A"}
	ColOK      = lipgloss.Color("#3FB950")
	ColErr     = lipgloss.Color("#F85149")
	ColWarn    = lipgloss.Color("#E3B341")
	ColText    = lipgloss.AdaptiveColor{Light: "#1F2328", Dark: "#E6EDF3"}
)

// Shared styles.
var (
	Title    = lipgloss.NewStyle().Bold(true).Foreground(ColPrimary)
	Subtitle = lipgloss.NewStyle().Foreground(ColAccent)
	Tagline  = lipgloss.NewStyle().Italic(true).Foreground(ColMuted)
	Muted    = lipgloss.NewStyle().Foreground(ColMuted)
	Code     = lipgloss.NewStyle().Foreground(ColAccent)
	Key      = lipgloss.NewStyle().Foreground(ColMuted)
	Val      = lipgloss.NewStyle().Foreground(ColText)

	okStyle   = lipgloss.NewStyle().Foreground(ColOK).Bold(true)
	errStyle  = lipgloss.NewStyle().Foreground(ColErr).Bold(true)
	warnStyle = lipgloss.NewStyle().Foreground(ColWarn).Bold(true)

	Box = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColPrimary).
		Padding(0, 2)
)

// Glyphs.
const (
	GlyphOK    = "✓"
	GlyphErr   = "✗"
	GlyphWarn  = "▲"
	GlyphSpark = "✦"
	GlyphArrow = "→"
)

// rgb is a simple 0-255 color used for gradient interpolation.
type rgb struct{ r, g, b int }

func (c rgb) hex() string { return fmt.Sprintf("#%02X%02X%02X", c.r, c.g, c.b) }

func lerp(a, b rgb, t float64) rgb {
	return rgb{
		r: int(float64(a.r) + (float64(b.r)-float64(a.r))*t),
		g: int(float64(a.g) + (float64(b.g)-float64(a.g))*t),
		b: int(float64(a.b) + (float64(b.b)-float64(a.b))*t),
	}
}

// gradientLine colors a string left-to-right between two RGB colors. width is
// the reference width for the gradient (use the banner's max width so stacked
// lines share a consistent ramp); pass 0 to use the string's own length.
func gradientLine(s string, from, to rgb, width int) string {
	runes := []rune(s)
	n := width
	if n <= 0 {
		n = len(runes)
	}
	var b strings.Builder
	for i, ru := range runes {
		t := 0.0
		if n > 1 {
			t = float64(i) / float64(n-1)
		}
		col := lerp(from, to, t)
		if ru == ' ' {
			b.WriteRune(ru)
			continue
		}
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(col.hex())).Render(string(ru)))
	}
	return b.String()
}

// 5-row block font for the banner (only the letters we need).
var glyphFont = map[rune][5]string{
	'S': {"█████", "█    ", "█████", "    █", "█████"},
	'K': {"█   █", "█  █ ", "███  ", "█  █ ", "█   █"},
	'I': {"█████", "  █  ", "  █  ", "  █  ", "█████"},
	'L': {"█    ", "█    ", "█    ", "█    ", "█████"},
	'F': {"█████", "█    ", "████ ", "█    ", "█    "},
	'O': {"█████", "█   █", "█   █", "█   █", "█████"},
	'R': {"████ ", "█   █", "████ ", "█  █ ", "█   █"},
	'G': {"█████", "█    ", "█  ██", "█   █", "█████"},
	'E': {"█████", "█    ", "████ ", "█    ", "█████"},
}

// renderWord builds the 5 rows of a word from glyphFont.
func renderWord(word string) []string {
	rows := make([]string, 5)
	for r := 0; r < 5; r++ {
		var parts []string
		for _, ch := range word {
			g, ok := glyphFont[ch]
			if !ok {
				g = [5]string{"     ", "     ", "     ", "     ", "     "}
			}
			parts = append(parts, g[r])
		}
		rows[r] = strings.Join(parts, " ")
	}
	return rows
}

// Banner returns the gradient "SKILL / FORGE" wordmark plus a tagline.
func Banner() string {
	var lines []string
	width := 0
	for _, w := range []string{"SKILL", "FORGE"} {
		for _, row := range renderWord(w) {
			if len(row) > width {
				width = len(row)
			}
		}
	}
	for _, w := range []string{"SKILL", "FORGE"} {
		for _, row := range renderWord(w) {
			lines = append(lines, gradientLine(row, FireFrom, FireTo, width))
		}
	}
	art := strings.Join(lines, "\n")
	tagline := Tagline.Render(GlyphSpark + " forge portable agentic skills & plugins")
	return art + "\n" + tagline
}

// CompactBanner is a one-line wordmark for sub-command headers.
func CompactBanner() string {
	return gradientLine("◆ SKILL FORGE", FireFrom, FireTo, 0)
}

// GradientRule renders a full-width horizontal rule in the forge-fire gradient.
func GradientRule(width int) string {
	if width < 1 {
		return ""
	}
	return gradientLine(strings.Repeat("─", width), FireFrom, FireTo, width)
}
