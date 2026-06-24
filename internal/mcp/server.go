// Package mcp is a minimal stdio JSON-RPC 2.0 MCP server: enough to expose
// tools (initialize / tools/list / tools/call) without a heavy SDK.
package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
)

const protocolVersion = "2024-11-05"

// Tool is a callable exposed by the server.
type Tool struct {
	Name        string
	Description string
	InputSchema map[string]any
	Handler     func(ctx context.Context, args map[string]any) (string, error)
}

// Server is a minimal MCP server over newline-delimited JSON-RPC.
type Server struct {
	Name    string
	Version string
	Tools   []Tool
}

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcError struct {
	code int
	msg  string
}

// Serve reads JSON-RPC messages (one per line) from in and writes responses to
// out until EOF or ctx cancellation.
func (s *Server) Serve(ctx context.Context, in io.Reader, out io.Writer) error {
	r := bufio.NewReader(in)
	enc := json.NewEncoder(out)

	// Read on a goroutine so a blocked ReadBytes can't swallow ctx cancellation.
	lines := make(chan []byte)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			line, err := r.ReadBytes('\n')
			if len(line) > 0 {
				select {
				case lines <- line:
				case <-ctx.Done():
					return
				}
			}
			if err != nil {
				return
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-done:
			return nil // EOF / client disconnected
		case line := <-lines:
			s.handle(ctx, line, enc)
		}
	}
}

func (s *Server) handle(ctx context.Context, line []byte, enc *json.Encoder) {
	if len(bytes.TrimSpace(line)) == 0 {
		return
	}
	var req rpcRequest
	if err := json.Unmarshal(line, &req); err != nil {
		_ = enc.Encode(map[string]any{"jsonrpc": "2.0", "id": nil, "error": map[string]any{"code": -32700, "message": "parse error"}})
		return
	}
	notification := len(req.ID) == 0 || string(req.ID) == "null"
	if req.Method == "" {
		if !notification {
			_ = enc.Encode(map[string]any{"jsonrpc": "2.0", "id": json.RawMessage(req.ID), "error": map[string]any{"code": -32600, "message": "invalid request"}})
		}
		return
	}
	result, rerr := s.dispatch(ctx, req)
	if notification {
		return
	}
	resp := map[string]any{"jsonrpc": "2.0", "id": json.RawMessage(req.ID)}
	if rerr != nil {
		resp["error"] = map[string]any{"code": rerr.code, "message": rerr.msg}
	} else {
		resp["result"] = result
	}
	_ = enc.Encode(resp)
}

func (s *Server) dispatch(ctx context.Context, req rpcRequest) (any, *rpcError) {
	switch req.Method {
	case "initialize":
		return map[string]any{
			"protocolVersion": protocolVersion,
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": s.Name, "version": s.Version},
		}, nil
	case "ping":
		return map[string]any{}, nil
	case "notifications/initialized", "notifications/cancelled":
		return nil, nil
	case "tools/list":
		tools := make([]map[string]any, 0, len(s.Tools))
		for _, t := range s.Tools {
			tools = append(tools, map[string]any{
				"name":        t.Name,
				"description": t.Description,
				"inputSchema": t.InputSchema,
			})
		}
		return map[string]any{"tools": tools}, nil
	case "tools/call":
		var p struct {
			Name      string         `json:"name"`
			Arguments map[string]any `json:"arguments"`
		}
		_ = json.Unmarshal(req.Params, &p)
		for _, t := range s.Tools {
			if t.Name != p.Name {
				continue
			}
			text, err := t.Handler(ctx, p.Arguments)
			if err != nil {
				return map[string]any{
					"content": []any{map[string]any{"type": "text", "text": "error: " + err.Error()}},
					"isError": true,
				}, nil
			}
			return map[string]any{
				"content": []any{map[string]any{"type": "text", "text": text}},
			}, nil
		}
		return nil, &rpcError{code: -32602, msg: "unknown tool: " + p.Name}
	default:
		return nil, &rpcError{code: -32601, msg: "method not found: " + req.Method}
	}
}
