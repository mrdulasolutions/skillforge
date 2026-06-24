// Package schema emits a single canonical tool definition to multiple provider
// formats (MCP, OpenAI, Anthropic) — Skill Forge's cross-provider output.
package schema

import "strings"

// ToolDef is the canonical, provider-agnostic tool definition.
type ToolDef struct {
	Name        string
	Description string
	InputSchema map[string]any // a JSON Schema object
}

// FromSkill builds a ToolDef that exposes a skill as a callable tool. Skills are
// instruction sets, so the tool takes a free-form natural-language request.
func FromSkill(name, description string) ToolDef {
	return ToolDef{
		Name:        ToolName(name),
		Description: description,
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"request": map[string]any{
					"type":        "string",
					"description": "What you want this skill to do, described in natural language.",
				},
			},
			"required": []any{"request"},
		},
	}
}

// MCP renders the tool as an MCP Tool object.
func (t ToolDef) MCP() map[string]any {
	return map[string]any{
		"name":        t.Name,
		"description": t.Description,
		"inputSchema": t.InputSchema,
	}
}

// OpenAI renders the tool as an OpenAI function-calling tool.
func (t ToolDef) OpenAI() map[string]any {
	return map[string]any{
		"type": "function",
		"function": map[string]any{
			"name":        t.Name,
			"description": t.Description,
			"parameters":  t.InputSchema,
		},
	}
}

// Anthropic renders the tool as an Anthropic tool definition.
func (t ToolDef) Anthropic() map[string]any {
	return map[string]any{
		"name":         t.Name,
		"description":  t.Description,
		"input_schema": t.InputSchema,
	}
}

// ToolName sanitizes a skill name into a valid tool name (^[A-Za-z0-9_-]{1,64}$).
func ToolName(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	n := b.String()
	if len(n) > 64 {
		n = n[:64]
	}
	if n == "" {
		n = "skill"
	}
	return n
}
