package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/mrdulasolutions/skillforge/internal/config"
)

// OpenRouter calls the OpenRouter chat-completions API (OpenAI-compatible).
type OpenRouter struct {
	APIKey  string
	BaseURL string
	client  *http.Client
}

// NewOpenRouter builds a client. The API key comes from OPENROUTER_API_KEY, or
// the stored secret (keychain/file) written by `skillforge setup`.
func NewOpenRouter() *OpenRouter {
	key := strings.TrimSpace(envOr("OPENROUTER_API_KEY", ""))
	if key == "" {
		if k, err := config.GetSecret(config.SecretOpenRouterKey); err == nil {
			key = strings.TrimSpace(k)
		}
	}
	return &OpenRouter{
		APIKey:  key,
		BaseURL: envOr("OPENROUTER_BASE_URL", "https://openrouter.ai/api/v1"),
		client:  &http.Client{Timeout: 120 * time.Second},
	}
}

func (o *OpenRouter) Name() string    { return "openrouter" }
func (o *OpenRouter) Available() bool { return o.APIKey != "" }

func (o *OpenRouter) Complete(ctx context.Context, req Request) (*Response, error) {
	if o.APIKey == "" {
		return nil, fmt.Errorf("OPENROUTER_API_KEY is not set")
	}
	payload := map[string]any{
		"model":    req.Model,
		"messages": toOpenAIMessages(req),
	}
	if req.Temperature > 0 {
		payload["temperature"] = req.Temperature
	}
	if req.MaxTokens > 0 {
		payload["max_tokens"] = req.MaxTokens
	}
	buf, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, o.BaseURL+"/chat/completions", bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+o.APIKey)
	httpReq.Header.Set("HTTP-Referer", "https://github.com/mrdulasolutions/skillforge")
	httpReq.Header.Set("X-Title", "Skill Forge")

	resp, err := o.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	// Check status before decoding so a non-JSON error body can't mask the
	// 401/429 handling (mirrors Stream).
	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("OpenRouter rejected the API key (401) — verify it at https://openrouter.ai/keys")
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, fmt.Errorf("openrouter: rate-limited (429) — that model is busy or your key hit a limit; try another model or wait")
	}
	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Model string `json:"model"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		if resp.StatusCode >= 400 {
			return nil, fmt.Errorf("openrouter: HTTP %d", resp.StatusCode)
		}
		return nil, fmt.Errorf("openrouter: decode response: %w", err)
	}
	if resp.StatusCode >= 400 {
		if out.Error != nil {
			return nil, fmt.Errorf("openrouter: %s (HTTP %d)", out.Error.Message, resp.StatusCode)
		}
		return nil, fmt.Errorf("openrouter: HTTP %d", resp.StatusCode)
	}
	if len(out.Choices) == 0 {
		return nil, fmt.Errorf("openrouter: empty response")
	}
	return &Response{Text: out.Choices[0].Message.Content, Model: out.Model}, nil
}

// Stream sends a streaming chat-completion request, invoking onDelta per token.
func (o *OpenRouter) Stream(ctx context.Context, req Request, onDelta func(string)) (*Response, error) {
	if o.APIKey == "" {
		return nil, fmt.Errorf("OPENROUTER_API_KEY is not set")
	}
	payload := map[string]any{
		"model":    req.Model,
		"messages": toOpenAIMessages(req),
		"stream":   true,
	}
	if req.Temperature > 0 {
		payload["temperature"] = req.Temperature
	}
	if req.MaxTokens > 0 {
		payload["max_tokens"] = req.MaxTokens
	}
	buf, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, o.BaseURL+"/chat/completions", bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+o.APIKey)
	httpReq.Header.Set("HTTP-Referer", "https://github.com/mrdulasolutions/skillforge")
	httpReq.Header.Set("X-Title", "Skill Forge")

	resp, err := o.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("OpenRouter rejected the API key (401) — verify it at https://openrouter.ai/keys")
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, fmt.Errorf("openrouter: rate-limited (429) — that model is busy or your key hit a limit; try another model or wait")
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("openrouter: HTTP %d", resp.StatusCode)
	}

	var sb strings.Builder
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		if len(chunk.Choices) > 0 {
			if d := chunk.Choices[0].Delta.Content; d != "" {
				sb.WriteString(d)
				if onDelta != nil {
					onDelta(d)
				}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return &Response{Text: sb.String(), Model: req.Model}, nil
}

// toOpenAIMessages prepends the system prompt as a system message.
func toOpenAIMessages(req Request) []map[string]string {
	var msgs []map[string]string
	if req.System != "" {
		msgs = append(msgs, map[string]string{"role": "system", "content": req.System})
	}
	for _, m := range req.Messages {
		msgs = append(msgs, map[string]string{"role": m.Role, "content": m.Content})
	}
	return msgs
}

// ORModel is a model offered by OpenRouter.
type ORModel struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// ListModels fetches the OpenRouter model catalog, sorted by id. The /models
// endpoint is public; the key is sent when available.
func (o *OpenRouter) ListModels(ctx context.Context) ([]ORModel, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, o.BaseURL+"/models", nil)
	if err != nil {
		return nil, err
	}
	if o.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+o.APIKey)
	}
	resp, err := o.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("openrouter: HTTP %d", resp.StatusCode)
	}
	var out struct {
		Data []ORModel `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	sort.Slice(out.Data, func(i, j int) bool { return out.Data[i].ID < out.Data[j].ID })
	return out.Data, nil
}
