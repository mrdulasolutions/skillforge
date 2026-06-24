package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Ollama calls a local Ollama server's chat API for offline completion.
type Ollama struct {
	Host   string
	client *http.Client
}

// NewOllama builds a client from OLLAMA_HOST (default http://localhost:11434).
func NewOllama() *Ollama {
	return &Ollama{
		Host:   envOr("OLLAMA_HOST", "http://localhost:11434"),
		client: &http.Client{Timeout: 120 * time.Second},
	}
}

func (o *Ollama) Name() string { return "ollama" }

// Available pings the local server with a short timeout so it never blocks the
// CLI for long when Ollama isn't running.
func (o *Ollama) Available() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 600*time.Millisecond)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, o.Host+"/api/version", nil)
	if err != nil {
		return false
	}
	resp, err := o.client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func (o *Ollama) Complete(ctx context.Context, req Request) (*Response, error) {
	payload := map[string]any{
		"model":    req.Model,
		"messages": toOpenAIMessages(req),
		"stream":   false,
	}
	if req.Temperature > 0 {
		payload["options"] = map[string]any{"temperature": req.Temperature}
	}
	buf, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, o.Host+"/api/chat", bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := o.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("ollama: HTTP %d", resp.StatusCode)
	}
	var out struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
		Model string `json:"model"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("ollama: decode response: %w", err)
	}
	return &Response{Text: out.Message.Content, Model: out.Model}, nil
}
