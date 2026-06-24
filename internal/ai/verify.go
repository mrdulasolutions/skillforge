package ai

import (
	"context"
	"strings"
)

// Verify makes a tiny completion to confirm a provider + model actually works,
// returning the model's trimmed reply. Used by `skillforge setup`.
func Verify(ctx context.Context, p Provider, model string) (string, error) {
	if model == "" {
		model = DefaultModel(p)
	}
	resp, err := p.Complete(ctx, Request{
		Model:       model,
		Messages:    []Message{{Role: "user", Content: "Reply with exactly: OK"}},
		MaxTokens:   16,
		Temperature: 0,
	})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(resp.Text), nil
}
