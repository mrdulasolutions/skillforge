// Package assets embeds Skill Forge's scaffold templates into the binary so
// the CLI works fully offline with zero external files.
package assets

import "embed"

// FS holds all scaffold templates. "all:" ensures dotfiles such as
// .claude-plugin templates are included.
//
//go:embed all:templates
var FS embed.FS
