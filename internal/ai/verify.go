package ai

import (
	"context"
	"strings"
	"time"
)

// Verify makes a tiny completion to confirm a provider + model actually works,
// returning the model's trimmed reply. A transient 429 (rate limit) is retried
// once after a short backoff. Used by `skillforge setup`.
func Verify(ctx context.Context, p Provider, model string) (string, error) {
	if model == "" {
		model = DefaultModel(p)
	}
	reply, err := verifyOnce(ctx, p, model)
	if err != nil && strings.Contains(err.Error(), "429") {
		select {
		case <-time.After(2 * time.Second):
		case <-ctx.Done():
			return "", ctx.Err()
		}
		reply, err = verifyOnce(ctx, p, model)
	}
	return reply, err
}

func verifyOnce(ctx context.Context, p Provider, model string) (string, error) {
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
