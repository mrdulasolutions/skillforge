package schema

import "testing"

func TestEmitters(t *testing.T) {
	td := FromSkill("pdf-extractor", "Extract tables. Use when the user uploads a PDF.")
	if td.Name != "pdf-extractor" {
		t.Fatalf("name = %q", td.Name)
	}

	mcp := td.MCP()
	if mcp["name"] != "pdf-extractor" || mcp["inputSchema"] == nil {
		t.Fatalf("mcp shape: %+v", mcp)
	}

	oa := td.OpenAI()
	if oa["type"] != "function" {
		t.Fatalf("openai type: %+v", oa)
	}
	fn, ok := oa["function"].(map[string]any)
	if !ok || fn["name"] != "pdf-extractor" || fn["parameters"] == nil {
		t.Fatalf("openai function: %+v", oa)
	}

	an := td.Anthropic()
	if an["name"] != "pdf-extractor" || an["input_schema"] == nil {
		t.Fatalf("anthropic shape: %+v", an)
	}
}

func TestToolName(t *testing.T) {
	cases := map[string]string{
		"pdf-extractor": "pdf-extractor",
		"a b/c.d":       "a_b_c_d",
		"":              "skill",
	}
	for in, want := range cases {
		if got := ToolName(in); got != want {
			t.Errorf("ToolName(%q) = %q, want %q", in, got, want)
		}
	}
}
