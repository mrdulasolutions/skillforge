// Package config persists Skill Forge preferences and secrets. Non-secret
// preferences live in a JSON file under the user config dir; API keys go to the
// OS keychain when available, falling back to a 0600 file. This is what
// `skillforge setup` writes and what the AI provider layer reads.
package config

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/zalando/go-keyring"
)

const (
	keychainService = "skillforge"
	// SecretOpenRouterKey is the secret name for the OpenRouter API key.
	SecretOpenRouterKey = "openrouter_api_key"
)

// Config holds non-secret preferences.
type Config struct {
	Provider        string `json:"provider,omitempty"` // "openrouter" | "ollama"
	OpenRouterModel string `json:"openrouter_model,omitempty"`
	OllamaHost      string `json:"ollama_host,omitempty"`
	OllamaModel     string `json:"ollama_model,omitempty"`
}

// Dir returns the Skill Forge config directory. SKILLFORGE_CONFIG_DIR overrides
// it (useful for tests and relocating config).
func Dir() (string, error) {
	if d := os.Getenv("SKILLFORGE_CONFIG_DIR"); d != "" {
		return d, nil
	}
	c, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(c, "skillforge"), nil
}

func configPath() (string, error) {
	d, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "config.json"), nil
}

// Load reads preferences, returning a zero-value Config if none exist (or the
// file is unreadable/corrupt — setup can always rewrite it).
func Load() *Config {
	p, err := configPath()
	if err != nil {
		return &Config{}
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return &Config{}
	}
	var c Config
	if err := json.Unmarshal(b, &c); err != nil {
		return &Config{}
	}
	return &c
}

// Save writes preferences with 0600 permissions.
func (c *Config) Save() error {
	d, err := Dir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(d, 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(d, "config.json"), b, 0o600)
}

// --- secrets: keychain first, 0600 file fallback ---

func useKeychain() bool { return os.Getenv("SKILLFORGE_NO_KEYCHAIN") == "" }

// SetSecret stores a secret and reports where it landed ("keychain" or "file").
func SetSecret(name, value string) (string, error) {
	if useKeychain() {
		if err := keyring.Set(keychainService, name, value); err == nil {
			return "keychain", nil
		}
	}
	if err := setSecretFile(name, value); err != nil {
		return "", err
	}
	return "file", nil
}

// GetSecret reads a secret from the keychain, then the file fallback. Returns
// "" (no error) when absent.
func GetSecret(name string) (string, error) {
	if useKeychain() {
		if v, err := keyring.Get(keychainService, name); err == nil && v != "" {
			return v, nil
		}
	}
	return getSecretFile(name)
}

// DeleteSecret removes a secret from both stores.
func DeleteSecret(name string) error {
	if useKeychain() {
		_ = keyring.Delete(keychainService, name)
	}
	return deleteSecretFile(name)
}

func secretsPath() (string, error) {
	d, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "secrets.json"), nil
}

func loadSecrets() (map[string]string, error) {
	p, err := secretsPath()
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]string{}, nil
		}
		return nil, err
	}
	m := map[string]string{}
	_ = json.Unmarshal(b, &m)
	return m, nil
}

func writeSecrets(m map[string]string) error {
	d, err := Dir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(d, 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	p, _ := secretsPath()
	return os.WriteFile(p, b, 0o600)
}

func setSecretFile(name, value string) error {
	m, err := loadSecrets()
	if err != nil {
		return err
	}
	m[name] = value
	return writeSecrets(m)
}

func getSecretFile(name string) (string, error) {
	m, err := loadSecrets()
	if err != nil {
		return "", err
	}
	return m[name], nil
}

func deleteSecretFile(name string) error {
	m, err := loadSecrets()
	if err != nil {
		return err
	}
	delete(m, name)
	return writeSecrets(m)
}
