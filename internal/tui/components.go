package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/mrdulasolutions/skillforge/internal/skill"
)

// Status lines ---------------------------------------------------------------

// OK renders a success line.
func OK(s string) string { return okStyle.Render(GlyphOK) + " " + Val.Render(s) }

// Err renders a failure line.
func Err(s string) string { return errStyle.Render(GlyphErr) + " " + Val.Render(s) }

// Warn renders a warning line.
func Warn(s string) string { return warnStyle.Render(GlyphWarn) + " " + Val.Render(s) }

// Info renders an informational line.
func Info(s string) string { return Subtitle.Render(GlyphArrow) + " " + Val.Render(s) }

// Step renders a checklist line keyed on success.
func Step(ok bool, s string) string {
	if ok {
		return OK(s)
	}
	return Err(s)
}

// Panels ---------------------------------------------------------------------

// Panel wraps a titled body in a rounded box.
func Panel(title, body string) string {
	return Box.Render(Title.Render(title) + "\n" + body)
}

// KV renders aligned key/value rows.
func KV(pairs [][2]string) string {
	width := 0
	for _, p := range pairs {
		if len(p[0]) > width {
			width = len(p[0])
		}
	}
	var b strings.Builder
	for _, p := range pairs {
		b.WriteString(Key.Render(fmt.Sprintf("%-*s", width, p[0])) + "  " + Val.Render(p[1]) + "\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

// ValidationReport renders a skill.Result as a colored issue list.
func ValidationReport(res skill.Result) string {
	var b strings.Builder
	if res.Valid() {
		b.WriteString(okStyle.Render(GlyphOK+" valid") + Muted.Render("  — frontmatter & structure OK"))
	} else {
		b.WriteString(errStyle.Render(GlyphErr + " invalid"))
	}
	b.WriteString("\n")
	for _, e := range res.Errors() {
		b.WriteString("  " + errStyle.Render(GlyphErr) + " " + Val.Render(e.Message) + "\n")
	}
	for _, w := range res.Warnings() {
		b.WriteString("  " + warnStyle.Render(GlyphWarn) + " " + Muted.Render(w.Message) + "\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

// FileTree renders relative paths as a nested tree with connectors.
func FileTree(paths []string) string {
	type node struct {
		children map[string]*node
		order    []string
	}
	root := &node{children: map[string]*node{}}
	for _, p := range paths {
		cur := root
		for _, part := range strings.Split(filepath.ToSlash(p), "/") {
			if part == "" {
				continue
			}
			if cur.children[part] == nil {
				cur.children[part] = &node{children: map[string]*node{}}
				cur.order = append(cur.order, part)
			}
			cur = cur.children[part]
		}
	}
	var b strings.Builder
	var walk func(n *node, prefix string)
	walk = func(n *node, prefix string) {
		for i, name := range n.order {
			last := i == len(n.order)-1
			conn := "├─ "
			next := prefix + "│  "
			if last {
				conn = "└─ "
				next = prefix + "   "
			}
			child := n.children[name]
			label := Val.Render(name)
			if len(child.children) > 0 {
				label = Subtitle.Render(name + "/")
			}
			b.WriteString(Muted.Render(prefix+conn) + label + "\n")
			walk(child, next)
		}
	}
	walk(root, "")
	return strings.TrimRight(b.String(), "\n")
}

// RenderMarkdown pretty-prints markdown for terminal preview (best-effort).
func RenderMarkdown(md string) string {
	r, err := glamour.NewTermRenderer(glamour.WithAutoStyle(), glamour.WithWordWrap(82))
	if err != nil {
		return md
	}
	out, err := r.Render(md)
	if err != nil {
		return md
	}
	return strings.TrimRight(out, "\n")
}
