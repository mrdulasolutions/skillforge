package ai

import "testing"

func TestNewOpenRouterTrimsKey(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "  sk-or-test-key\n")
	or := NewOpenRouter()
	if or.APIKey != "sk-or-test-key" {
		t.Fatalf("expected trimmed key, got %q", or.APIKey)
	}
	if !or.Available() {
		t.Fatal("expected Available() true for a non-empty key")
	}
}

func TestNewOpenRouterWhitespaceOnlyKeyIsEmpty(t *testing.T) {
	// Isolate config so the fall-through to a stored secret can't leak a real key.
	t.Setenv("SKILLFORGE_CONFIG_DIR", t.TempDir())
	t.Setenv("SKILLFORGE_NO_KEYCHAIN", "1")
	t.Setenv("OPENROUTER_API_KEY", "   \t  ")
	if or := NewOpenRouter(); or.Available() {
		t.Fatalf("whitespace-only key should be treated as empty, got %q", or.APIKey)
	}
}
