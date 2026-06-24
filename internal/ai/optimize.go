package ai

import (
	"context"
	"fmt"
	"strings"
)

const optimizeSystem = `You are an expert at writing descriptions for Anthropic Agent Skills.
A skill's "description" is the single most important factor in whether the agent
invokes it at the right time. Good descriptions lead with what the skill does,
then give explicit, assertive "Use when ..." triggers. They are plain text,
one line, contain no angle brackets, and stay under 1024 characters.`

// OptimizeDescription asks the model to rewrite a skill's description so it
// triggers reliably. It returns a cleaned single-line description.
func OptimizeDescription(ctx context.Context, p Provider, model, name, current, body string) (string, error) {
	if p == nil {
		return "", fmt.Errorf("no AI provider available")
	}
	if model == "" {
		model = DefaultModel(p)
	}
	user := fmt.Sprintf(`Skill name: %s

Current description:
%s

SKILL.md body (for context):
%s

Rewrite ONLY the description. Requirements:
- One line, plain text, no angle brackets, max 1024 characters.
- Lead with what it does, then explicit "Use when ..." triggers.
- Be assertive and specific so the agent reliably invokes it.
Return only the rewritten description, with no preamble or quotes.`,
		name, current, truncate(body, 4000))

	resp, err := p.Complete(ctx, Request{
		Model:       model,
		System:      optimizeSystem,
		Messages:    []Message{{Role: "user", Content: user}},
		Temperature: 0.4,
		MaxTokens:   400,
	})
	if err != nil {
		return "", err
	}
	out := cleanDescription(resp.Text)
	if out == "" {
		return "", fmt.Errorf("model returned an empty description")
	}
	return out, nil
}

// cleanDescription extracts a single valid description line from model output.
func cleanDescription(s string) string {
	for _, line := range strings.Split(strings.TrimSpace(s), "\n") {
		line = strings.TrimSpace(line)
		line = strings.TrimPrefix(line, "Description:")
		line = strings.Trim(line, "\"'` ")
		line = strings.ReplaceAll(line, "<", "")
		line = strings.ReplaceAll(line, ">", "")
		line = strings.TrimSpace(line)
		if line != "" {
			if r := []rune(line); len(r) > 1024 {
				line = strings.TrimSpace(string(r[:1024]))
			}
			return line
		}
	}
	return ""
}

func truncate(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n])
	}
	return s
}
