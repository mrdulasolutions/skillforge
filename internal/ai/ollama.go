package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/mrdulasolutions/skillforge/internal/config"
)

// Ollama calls a local Ollama server's chat API for offline completion.
type Ollama struct {
	Host   string
	client *http.Client
}

// NewOllama builds a client. The host comes from OLLAMA_HOST, then the stored
// config, then the default http://localhost:11434.
func NewOllama() *Ollama {
	host := os.Getenv("OLLAMA_HOST")
	if host == "" {
		if cfg := config.Load(); cfg.OllamaHost != "" {
			host = cfg.OllamaHost
		}
	}
	if host == "" {
		host = "http://localhost:11434"
	}
	return &Ollama{Host: host, client: &http.Client{Timeout: 120 * time.Second}}
}

func (o *Ollama) Name() string { return "ollama" }

// ListModels returns the names of locally available Ollama models.
func (o *Ollama) ListModels(ctx context.Context) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, o.Host+"/api/tags", nil)
	if err != nil {
		return nil, err
	}
	resp, err := o.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("ollama: HTTP %d", resp.StatusCode)
	}
	var out struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	names := make([]string, 0, len(out.Models))
	for _, m := range out.Models {
		names = append(names, m.Name)
	}
	return names, nil
}

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
		Error string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("ollama: decode response: %w", err)
	}
	if out.Error != "" {
		return nil, fmt.Errorf("ollama: %s", out.Error)
	}
	return &Response{Text: out.Message.Content, Model: out.Model}, nil
}

// Stream sends a streaming chat request (NDJSON), invoking onDelta per token.
func (o *Ollama) Stream(ctx context.Context, req Request, onDelta func(string)) (*Response, error) {
	payload := map[string]any{
		"model":    req.Model,
		"messages": toOpenAIMessages(req),
		"stream":   true,
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

	var sb strings.Builder
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var chunk struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			Done  bool   `json:"done"`
			Error string `json:"error"`
		}
		if err := json.Unmarshal([]byte(line), &chunk); err != nil {
			continue
		}
		if chunk.Error != "" {
			return nil, fmt.Errorf("ollama: %s", chunk.Error)
		}
		if d := chunk.Message.Content; d != "" {
			sb.WriteString(d)
			if onDelta != nil {
				onDelta(d)
			}
		}
		if chunk.Done {
			break
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return &Response{Text: sb.String(), Model: req.Model}, nil
}
