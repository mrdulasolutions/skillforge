package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigRoundTrip(t *testing.T) {
	t.Setenv("SKILLFORGE_CONFIG_DIR", t.TempDir())
	c := &Config{Provider: "openrouter", OpenRouterModel: "x/y"}
	if err := c.Save(); err != nil {
		t.Fatal(err)
	}
	got := Load()
	if got.Provider != "openrouter" || got.OpenRouterModel != "x/y" {
		t.Fatalf("round trip mismatch: %+v", got)
	}
}

func TestLoadMissingReturnsZero(t *testing.T) {
	t.Setenv("SKILLFORGE_CONFIG_DIR", t.TempDir())
	if c := Load(); c.Provider != "" {
		t.Fatalf("expected zero config, got %+v", c)
	}
}

func TestSecretFileFallback(t *testing.T) {
	t.Setenv("SKILLFORGE_CONFIG_DIR", t.TempDir())
	t.Setenv("SKILLFORGE_NO_KEYCHAIN", "1")

	storage, err := SetSecret(SecretOpenRouterKey, "sk-test-123")
	if err != nil {
		t.Fatal(err)
	}
	if storage != "file" {
		t.Fatalf("expected file storage, got %q", storage)
	}
	if v, err := GetSecret(SecretOpenRouterKey); err != nil || v != "sk-test-123" {
		t.Fatalf("GetSecret = %q, %v", v, err)
	}

	d, _ := Dir()
	info, err := os.Stat(filepath.Join(d, "secrets.json"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("secrets.json perms = %v, want 0600", info.Mode().Perm())
	}

	if err := DeleteSecret(SecretOpenRouterKey); err != nil {
		t.Fatal(err)
	}
	if v, _ := GetSecret(SecretOpenRouterKey); v != "" {
		t.Fatalf("expected empty after delete, got %q", v)
	}
}
