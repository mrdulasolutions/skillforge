// Package ai is a tiny provider layer for OpenRouter (cloud gateway) and Ollama
// (local/offline). It speaks plain HTTP+JSON — no SDK — to stay lightweight.
package ai

import (
	"context"
	"os"
)

// Message is one chat turn.
type Message struct {
	Role    string
	Content string
}

// Request is a model completion request.
type Request struct {
	Model       string
	System      string
	Messages    []Message
	Temperature float64
	MaxTokens   int
}

// Response is a model completion result.
type Response struct {
	Text  string
	Model string
}

// Provider is a chat-completion backend.
type Provider interface {
	Name() string
	// Available reports whether the provider is usable right now (key set,
	// host reachable). It must be fast and must not block for long.
	Available() bool
	Complete(ctx context.Context, req Request) (*Response, error)
}

// Streamer is an optional capability: providers that implement it stream tokens
// via onDelta as they arrive, and still return the full accumulated Response.
type Streamer interface {
	Stream(ctx context.Context, req Request, onDelta func(string)) (*Response, error)
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
