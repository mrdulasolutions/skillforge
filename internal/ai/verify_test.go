package ai

import (
	"context"
	"fmt"
	"testing"
)

type seqResult struct {
	text string
	err  error
}

type seqProvider struct {
	results []seqResult
	i       int
}

func (s *seqProvider) Name() string    { return "seq" }
func (s *seqProvider) Available() bool { return true }
func (s *seqProvider) Complete(_ context.Context, _ Request) (*Response, error) {
	if s.i >= len(s.results) {
		return nil, fmt.Errorf("seq exhausted")
	}
	r := s.results[s.i]
	s.i++
	if r.err != nil {
		return nil, r.err
	}
	return &Response{Text: r.text}, nil
}

func TestVerifyRetriesOn429(t *testing.T) {
	p := &seqProvider{results: []seqResult{
		{err: fmt.Errorf("openrouter: rate-limited (429) — busy")},
		{text: "OK"},
	}}
	reply, err := Verify(context.Background(), p, "m")
	if err != nil || reply != "OK" {
		t.Fatalf("expected success after retry: reply=%q err=%v", reply, err)
	}
	if p.i != 2 {
		t.Fatalf("expected 2 calls (one retry), got %d", p.i)
	}
}

func TestVerifyNoRetryOnAuthError(t *testing.T) {
	p := &seqProvider{results: []seqResult{
		{err: fmt.Errorf("OpenRouter rejected the API key (401)")},
		{text: "OK"},
	}}
	if _, err := Verify(context.Background(), p, "m"); err == nil {
		t.Fatal("expected the 401 error to propagate")
	}
	if p.i != 1 {
		t.Fatalf("should not retry on a non-429 error, calls=%d", p.i)
	}
}
