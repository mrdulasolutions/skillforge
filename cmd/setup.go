package cmd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/huh/spinner"
	"github.com/mrdulasolutions/skillforge/internal/ai"
	"github.com/mrdulasolutions/skillforge/internal/config"
	"github.com/mrdulasolutions/skillforge/internal/tui"
	"github.com/spf13/cobra"
)

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Set up AI so Skill Forge can help build skills",
	Long:  "Configure an AI provider (OpenRouter or local Ollama), store the key securely, and verify it works with a live test call.",
	Args:  cobra.NoArgs,
	RunE:  runSetup,
}

func runSetup(_ *cobra.Command, _ []string) error {
	header("setup")
	if !isTTY() {
		return fmt.Errorf("setup is interactive — run it in a terminal, or set OPENROUTER_API_KEY / OLLAMA_HOST directly")
	}
	cfg := config.Load()
	choice := cfg.Provider
	if choice == "" {
		choice = "openrouter"
	}
	if err := huh.NewSelect[string]().
		Title("How should Skill Forge use AI?").
		Description("Powers the conversational skill builder, build --optimize, and more.").
		Options(
			huh.NewOption("OpenRouter — one key, every cloud model (recommended)", "openrouter"),
			huh.NewOption("Ollama — local & offline", "ollama"),
			huh.NewOption("Skip for now", "skip"),
		).
		Value(&choice).
		WithTheme(tui.FormTheme()).
		Run(); err != nil {
		return err
	}

	switch choice {
	case "openrouter":
		return setupOpenRouter(cfg)
	case "ollama":
		return setupOllama(cfg)
	default:
		fmt.Println(tui.Info("Skipped. Run " + tui.Code.Render("skillforge setup") + " anytime."))
		return nil
	}
}

func setupOpenRouter(cfg *config.Config) error {
	existing, _ := config.GetSecret(config.SecretOpenRouterKey)
	existing = strings.TrimSpace(existing)
	key := ""
	keyDesc := "Get one at https://openrouter.ai/keys"
	if existing != "" {
		keyDesc = "Leave blank to keep the saved key"
	}

	if err := huh.NewInput().
		Title("OpenRouter API key").
		Description(keyDesc).
		EchoMode(huh.EchoModePassword).
		Value(&key).
		Validate(func(s string) error {
			if strings.TrimSpace(s) == "" && existing == "" {
				return fmt.Errorf("a key is required")
			}
			return nil
		}).
		WithTheme(tui.FormTheme()).
		Run(); err != nil {
		return err
	}
	key = strings.TrimSpace(key)
	if key == "" {
		key = existing
	}

	or := ai.NewOpenRouter()
	or.APIKey = key

	model, err := chooseOpenRouterModel(cfg, or)
	if err != nil {
		return err
	}

	storage, err := config.SetSecret(config.SecretOpenRouterKey, key)
	if err != nil {
		return fmt.Errorf("storing key: %w", err)
	}

	reply, verr := runVerify("OpenRouter", or, model, 30*time.Second)
	cfg.Provider = "openrouter"
	cfg.OpenRouterModel = model
	if err := cfg.Save(); err != nil {
		return err
	}
	reportSetup(storage, reply, verr)
	return nil
}

// chooseOpenRouterModel loads the catalog and shows a type-to-filter picker,
// falling back to free text if the list can't be loaded.
func chooseOpenRouterModel(cfg *config.Config, or *ai.OpenRouter) (string, error) {
	defaultModel := cfg.OpenRouterModel
	if defaultModel == "" {
		defaultModel = "anthropic/claude-3.5-sonnet"
	}

	var models []ai.ORModel
	var ferr error
	_ = spinner.New().
		Title(" loading OpenRouter models…").
		Action(func() {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			models, ferr = or.ListModels(ctx)
		}).
		Run()

	if ferr != nil || len(models) == 0 {
		fmt.Println(tui.Warn("couldn't load the model list — enter a model id manually"))
		model := defaultModel
		err := huh.NewInput().
			Title("Default model").
			Description("e.g. anthropic/claude-3.5-sonnet, openai/gpt-4o-mini").
			Value(&model).
			WithTheme(tui.FormTheme()).
			Run()
		return strings.TrimSpace(model), err
	}

	opts := make([]huh.Option[string], 0, len(models))
	hasDefault := false
	for _, m := range models {
		opts = append(opts, huh.NewOption(m.ID, m.ID))
		if m.ID == defaultModel {
			hasDefault = true
		}
	}
	model := defaultModel
	if !hasDefault {
		model = models[0].ID
	}
	err := huh.NewSelect[string]().
		Title(fmt.Sprintf("Default model — %d available, type to filter", len(models))).
		Options(opts...).
		Filtering(true).
		Height(12).
		Value(&model).
		WithTheme(tui.FormTheme()).
		Run()
	return model, err
}

func setupOllama(cfg *config.Config) error {
	ol := ai.NewOllama()
	if !ol.Available() {
		fmt.Println(tui.Err("Ollama isn't reachable at " + ol.Host))
		fmt.Println(tui.Muted.Render("Install from https://ollama.com, run `ollama serve`, then re-run setup."))
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	models, err := ol.ListModels(ctx)
	cancel()
	if err != nil || len(models) == 0 {
		fmt.Println(tui.Warn("Connected, but no local models found. Pull one (e.g. `ollama pull llama3.1`), then re-run setup."))
		return nil
	}

	model := cfg.OllamaModel
	if model == "" || !contains(models, model) {
		model = models[0]
	}
	opts := make([]huh.Option[string], 0, len(models))
	for _, m := range models {
		opts = append(opts, huh.NewOption(m, m))
	}
	if err := huh.NewSelect[string]().
		Title("Default Ollama model").
		Options(opts...).
		Value(&model).
		WithTheme(tui.FormTheme()).
		Run(); err != nil {
		return err
	}

	reply, verr := runVerify(model, ol, model, 90*time.Second)
	cfg.Provider = "ollama"
	cfg.OllamaHost = ol.Host
	cfg.OllamaModel = model
	if err := cfg.Save(); err != nil {
		return err
	}
	reportSetup("config", reply, verr)
	return nil
}

func runVerify(label string, p ai.Provider, model string, timeout time.Duration) (string, error) {
	var reply string
	var verr error
	_ = spinner.New().
		Title(" testing " + label + "…").
		Action(func() {
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()
			reply, verr = ai.Verify(ctx, p, model)
		}).
		Run()
	return reply, verr
}

func reportSetup(storage, reply string, verr error) {
	fmt.Println()
	if verr != nil {
		fmt.Println(tui.Err("Verification failed: " + verr.Error()))
		fmt.Println(tui.Muted.Render("Settings were saved; fix the key/model and re-run setup."))
		return
	}
	fmt.Println(tui.OK("AI is working " + tui.Muted.Render("(reply: "+reply+")")))
	switch storage {
	case "keychain":
		fmt.Println(tui.OK("Key stored in your OS keychain"))
	case "file":
		fmt.Println(tui.OK("Key stored (0600) in your config dir"))
	}
	fmt.Println()
	fmt.Println(tui.Info("Next: " + tui.Code.Render("skillforge new") + " to build a skill conversationally"))
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}
