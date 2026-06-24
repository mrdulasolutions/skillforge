package mcp

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestServe(t *testing.T) {
	srv := &Server{
		Name: "t", Version: "1",
		Tools: []Tool{{
			Name:        "echo",
			Description: "echoes the request",
			InputSchema: map[string]any{"type": "object"},
			Handler: func(_ context.Context, args map[string]any) (string, error) {
				req, _ := args["request"].(string)
				return "got: " + req, nil
			},
		}},
	}

	in := strings.NewReader(strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize"}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`, // notification: no response
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"echo","arguments":{"request":"hi"}}}`,
		`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"nope","arguments":{}}}`,
	}, "\n") + "\n")

	var out bytes.Buffer
	if err := srv.Serve(context.Background(), in, &out); err != nil {
		t.Fatal(err)
	}
	s := out.String()

	if !strings.Contains(s, `"protocolVersion"`) {
		t.Error("missing initialize result")
	}
	if !strings.Contains(s, `"echo"`) {
		t.Error("missing tool in tools/list")
	}
	if !strings.Contains(s, "got: hi") {
		t.Error("missing tools/call result")
	}
	if !strings.Contains(s, "unknown tool: nope") {
		t.Error("missing error for unknown tool")
	}
	// 4 responses (the notification produces none).
	if n := strings.Count(strings.TrimSpace(s), "\n") + 1; n != 4 {
		t.Fatalf("expected 4 responses, got %d:\n%s", n, s)
	}
}

func TestServeErrors(t *testing.T) {
	srv := &Server{Name: "t", Version: "1"}
	in := strings.NewReader(strings.Join([]string{
		`{"jsonrpc":"2.0","id":7}`,                  // request with id, no method -> -32600
		`not json at all`,                           // parse error -> -32700
		`{"jsonrpc":"2.0","method":"x"}`,            // notification, no id -> no response
		`{"jsonrpc":"2.0","id":8,"method":"bogus"}`, // unknown method -> -32601
	}, "\n") + "\n")
	var out bytes.Buffer
	if err := srv.Serve(context.Background(), in, &out); err != nil {
		t.Fatal(err)
	}
	s := out.String()
	for _, want := range []string{"-32600", "-32700", "-32601"} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %s in:\n%s", want, s)
		}
	}
}
