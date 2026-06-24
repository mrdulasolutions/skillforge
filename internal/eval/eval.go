// Package eval runs a skill's evals across an AI provider, comparing the skill
// against a baseline and grading each expectation with an LLM judge.
package eval

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/mrdulasolutions/skillforge/internal/ai"
)

// Case is one eval (mirrors the skill-creator evals.json schema).
type Case struct {
	ID             int      `json:"id"`
	Prompt         string   `json:"prompt"`
	ExpectedOutput string   `json:"expected_output"`
	Files          []string `json:"files"`
	Expectations   []string `json:"expectations"`
}

// File is the evals.json document.
type File struct {
	SkillName string `json:"skill_name"`
	Evals     []Case `json:"evals"`
}

// ExpResult is the graded verdict for one expectation.
type ExpResult struct {
	Text   string `json:"text"`
	Passed bool   `json:"passed"`
}

// RunResult is one model run (with-skill or baseline) and its grading.
type RunResult struct {
	Output       string      `json:"output"`
	Expectations []ExpResult `json:"expectations"`
	PassRate     float64     `json:"pass_rate"`
}

// CaseResult pairs a case with its with-skill (and optional baseline) runs.
type CaseResult struct {
	ID        int        `json:"id"`
	Prompt    string     `json:"prompt"`
	WithSkill RunResult  `json:"with_skill"`
	Baseline  *RunResult `json:"baseline,omitempty"`
}

// Report is the full benchmark.
type Report struct {
	SkillName         string       `json:"skill_name"`
	Provider          string       `json:"provider"`
	Model             string       `json:"model"`
	Baseline          bool         `json:"baseline"`
	Cases             []CaseResult `json:"cases"`
	WithSkillPassRate float64      `json:"with_skill_pass_rate"`
	BaselinePassRate  float64      `json:"baseline_pass_rate,omitempty"`
}

// Load reads evals/evals.json from a skill directory.
func Load(dir string) (*File, error) {
	b, err := os.ReadFile(filepath.Join(dir, "evals", "evals.json"))
	if err != nil {
		return nil, err
	}
	var f File
	if err := json.Unmarshal(b, &f); err != nil {
		return nil, err
	}
	return &f, nil
}

// Progress is invoked after each case completes.
type Progress func(done, total int)

// Run executes every case against the provider and grades it.
func Run(ctx context.Context, p ai.Provider, model, skillBody string, f *File, baseline bool, prog Progress) (*Report, error) {
	rep := &Report{SkillName: f.SkillName, Provider: p.Name(), Model: model, Baseline: baseline}
	var wsPass, wsTotal, blPass, blTotal int
	for i, c := range f.Evals {
		cr := CaseResult{ID: c.ID, Prompt: c.Prompt}

		out, err := complete(ctx, p, model, skillBody, c.Prompt)
		if err != nil {
			return nil, err
		}
		exps := grade(ctx, p, model, out, c.Expectations)
		cr.WithSkill = RunResult{Output: out, Expectations: exps, PassRate: rate(exps)}
		wsPass += passes(exps)
		wsTotal += len(exps)

		if baseline {
			bout, err := complete(ctx, p, model, "", c.Prompt)
			if err != nil {
				return nil, err
			}
			bexps := grade(ctx, p, model, bout, c.Expectations)
			br := RunResult{Output: bout, Expectations: bexps, PassRate: rate(bexps)}
			cr.Baseline = &br
			blPass += passes(bexps)
			blTotal += len(bexps)
		}

		rep.Cases = append(rep.Cases, cr)
		if prog != nil {
			prog(i+1, len(f.Evals))
		}
	}
	rep.WithSkillPassRate = ratio(wsPass, wsTotal)
	if baseline {
		rep.BaselinePassRate = ratio(blPass, blTotal)
	}
	return rep, nil
}

func complete(ctx context.Context, p ai.Provider, model, system, prompt string) (string, error) {
	resp, err := p.Complete(ctx, ai.Request{
		Model:       model,
		System:      system,
		Messages:    []ai.Message{{Role: "user", Content: prompt}},
		MaxTokens:   1500,
		Temperature: 0,
	})
	if err != nil {
		return "", err
	}
	return resp.Text, nil
}

const judgeSystem = `You are a strict evaluator. Given an OUTPUT and an EXPECTATION about that output, reply with exactly "PASS" if the output satisfies the expectation, or "FAIL" otherwise. Reply with only that one word.`

func grade(ctx context.Context, p ai.Provider, model, output string, expectations []string) []ExpResult {
	out := make([]ExpResult, 0, len(expectations))
	for _, e := range expectations {
		out = append(out, ExpResult{Text: e, Passed: judge(ctx, p, model, output, e)})
	}
	return out
}

func judge(ctx context.Context, p ai.Provider, model, output, expectation string) bool {
	user := "EXPECTATION:\n" + expectation + "\n\nOUTPUT:\n" + truncate(output, 4000) + "\n\nVerdict (PASS or FAIL):"
	resp, err := p.Complete(ctx, ai.Request{
		Model:       model,
		System:      judgeSystem,
		Messages:    []ai.Message{{Role: "user", Content: user}},
		MaxTokens:   4,
		Temperature: 0,
	})
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToUpper(resp.Text), "PASS")
}

func rate(e []ExpResult) float64 { return ratio(passes(e), len(e)) }

func passes(e []ExpResult) int {
	n := 0
	for _, x := range e {
		if x.Passed {
			n++
		}
	}
	return n
}

func ratio(a, b int) float64 {
	if b == 0 {
		return 0
	}
	return float64(a) / float64(b)
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}
