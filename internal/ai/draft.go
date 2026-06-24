package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// SkillSpec is the model-proposed skill, pre-split into scaffold fields.
type SkillSpec struct {
	Title       string `json:"title"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Body        string `json:"body"`
	Type        string `json:"type"`
	Evals       bool   `json:"evals"`
	Compliance  bool   `json:"compliance"`
}

// InterviewSystem drives the free-form Phase-A conversation. The REPL treats a
// model message containing "?" as "still interviewing"; otherwise it's ready.
const InterviewSystem = `You are Skill Forge's skill-building interviewer. You help a developer turn a
plain-language idea into a complete Anthropic Agent Skill through a short,
friendly conversation, like a sharp pair-programmer.

STYLE
- Warm, brief, concrete. One short paragraph per turn. No bullet lists.
- The user describes what they want in plain language. NEVER ask them to write
  kebab-case, YAML, or markdown — Skill Forge produces all of that for them.
- Ask AT MOST 2 clarifying questions total, and only when the answer would
  materially change the skill (its main output, or its single most important
  trigger). If the idea is already clear, ask ZERO questions.
- Ask at most ONE question per message, and only end a message with a question
  mark when you are genuinely asking. When you have enough to build a good
  skill, do NOT ask a question — say you're ready and invite the user to say
  "go" to see a draft (or to add detail). Mention once, in one line, the
  changeable defaults: a plain skill (not a plugin), eval scaffold included,
  compliance mode off.
- Never output YAML, JSON, or a SKILL.md in this conversation.`

const emitSystem = `You generate Anthropic Agent Skills. Output ONLY a single JSON object — no
prose, no markdown fences, no trailing text. Use exactly these keys:

{
  "title":       "Human readable title in Title Case",
  "name":        "lowercase-kebab-derived-from-title",
  "description": "one line, plain text, NO angle brackets, <=1024 chars",
  "body":        "the SKILL.md body in markdown, NO YAML frontmatter, NO top-level # title",
  "type":        "skill",
  "evals":       true,
  "compliance":  false
}

RULES
- name: lowercase a-z, 0-9, single hyphens, <=64 chars, no leading/trailing/double hyphen.
- description: ONE line. Lead with what the skill DOES, then explicit, assertive
  "Use when ..." triggers. This field alone decides whether the agent fires the
  skill, so make the triggers specific and generous. No "<" or ">" anywhere.
- body: concise, under ~400 lines. Use these level-2 sections in order:
  "## When to use this skill", "## Inputs", "## Instructions" (numbered steps),
  "## Output", "## Examples". Write real, specific content for THIS skill — never
  leave placeholders. Do NOT include frontmatter or a top-level # title.
- type "skill" unless the user clearly wants a plugin. Set evals/compliance from
  the conversation (defaults: evals=true, compliance=false).`

// DraftSkill makes one strict-JSON emission call using the interview transcript.
// prior!=nil with instruction means "refine the existing draft". It brace-scans
// the output and retries once on a parse failure.
func DraftSkill(ctx context.Context, p Provider, model string, transcript []Message, prior *SkillSpec, instruction string) (*SkillSpec, error) {
	if p == nil {
		return nil, fmt.Errorf("no AI provider available")
	}
	if model == "" {
		model = DefaultModel(p)
	}
	msgs := make([]Message, 0, len(transcript)+3)
	msgs = append(msgs, transcript...)
	if prior != nil {
		if pj, err := json.Marshal(prior); err == nil {
			msgs = append(msgs, Message{Role: "assistant", Content: string(pj)})
		}
		if strings.TrimSpace(instruction) != "" {
			msgs = append(msgs, Message{Role: "user", Content: instruction})
		}
	}
	msgs = append(msgs, Message{Role: "user", Content: "Produce the skill now as the single JSON object described. Output JSON only."})

	resp, err := p.Complete(ctx, Request{Model: model, System: emitSystem, Messages: msgs, Temperature: 0.2, MaxTokens: 2000})
	if err != nil {
		return nil, err
	}
	if spec, perr := parseSpec(resp.Text); perr == nil {
		return spec, nil
	}
	retry := append(msgs,
		Message{Role: "assistant", Content: resp.Text},
		Message{Role: "user", Content: "Return ONLY valid JSON matching the schema; your previous reply did not parse."},
	)
	resp2, err := p.Complete(ctx, Request{Model: model, System: emitSystem, Messages: retry, Temperature: 0, MaxTokens: 2000})
	if err != nil {
		return nil, err
	}
	return parseSpec(resp2.Text)
}

// CleanDescription strips angle brackets, single-lines, and clamps a description.
func CleanDescription(s string) string { return cleanDescription(s) }

func parseSpec(text string) (*SkillSpec, error) {
	js := extractJSON(text)
	if js == "" {
		return nil, fmt.Errorf("no JSON object found in model output")
	}
	var spec SkillSpec
	if err := json.Unmarshal([]byte(js), &spec); err != nil {
		return nil, err
	}
	return &spec, nil
}

// extractJSON returns the first complete JSON object in s, ignoring braces
// inside strings and stripping ``` fences.
func extractJSON(s string) string {
	// The brace scanner below is string-aware, so a ```json wrapper before the
	// first '{' is skipped and a trailing ``` after the matching '}' is never
	// reached — no fence stripping needed (stripping fences naively truncates
	// JSON whose body markdown itself contains ``` fences).
	start := strings.IndexByte(s, '{')
	if start < 0 {
		return ""
	}
	depth, inStr, esc := 0, false, false
	for i := start; i < len(s); i++ {
		c := s[i]
		if inStr {
			switch {
			case esc:
				esc = false
			case c == '\\':
				esc = true
			case c == '"':
				inStr = false
			}
			continue
		}
		switch c {
		case '"':
			inStr = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[start : i+1]
			}
		}
	}
	return ""
}
