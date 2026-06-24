package ai

import (
	"context"
	"fmt"
)

const compileIntro = `Below is reference material the user provided (their own files). Synthesize a single Anthropic Agent Skill that captures the reusable procedure, knowledge, or capability this material implies — something that lets an agent reproduce this kind of work on new inputs. Infer a good name and trigger-rich description from the material. Then output the skill as the JSON object described.`

// CompileSkill synthesizes a skill from a corpus of user-provided material.
// hint is an optional steer about what the user wants.
func CompileSkill(ctx context.Context, p Provider, model, corpus, hint string) (*SkillSpec, error) {
	if p == nil {
		return nil, fmt.Errorf("no AI provider available")
	}
	msg := compileIntro
	if hint != "" {
		msg += "\n\nUser hint about the skill they want: " + hint
	}
	msg += "\n\n--- MATERIAL ---\n" + truncate(corpus, 14000)
	return DraftSkill(ctx, p, model, []Message{{Role: "user", Content: msg}}, nil, "")
}
