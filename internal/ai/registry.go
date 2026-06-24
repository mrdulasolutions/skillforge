package ai

// DefaultModel returns the default model id for a provider, overridable by env.
func DefaultModel(p Provider) string {
	switch p.(type) {
	case *OpenRouter:
		return envOr("OPENROUTER_MODEL", "anthropic/claude-3.5-sonnet")
	case *Ollama:
		return envOr("OLLAMA_MODEL", "llama3.1")
	default:
		return ""
	}
}

// Select picks a provider: prefer OpenRouter (cloud gateway) when a key is set,
// otherwise local Ollama if reachable. Returns nil if neither is available.
func Select() Provider {
	if or := NewOpenRouter(); or.Available() {
		return or
	}
	if ol := NewOllama(); ol.Available() {
		return ol
	}
	return nil
}

// Probe is a provider availability snapshot for `doctor`.
type Probe struct {
	Name      string
	Available bool
	Detail    string
}

// ProbeAll reports availability of both providers.
func ProbeAll() []Probe {
	or := NewOpenRouter()
	ol := NewOllama()
	orDetail := "OPENROUTER_API_KEY not set"
	if or.Available() {
		orDetail = "key set"
	}
	return []Probe{
		{Name: "openrouter", Available: or.Available(), Detail: orDetail},
		{Name: "ollama", Available: ol.Available(), Detail: ol.Host},
	}
}
