package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/gitsage/gitsage/internal/pkg/config"
)

// RunInteractiveSetup runs the interactive setup wizard using Bubble Tea (huh).
func RunInteractiveSetup(cfgMgr *config.ViperManager) error {
	fmt.Println("No configuration found. Let's set up GitSage!")
	fmt.Println()

	// Initialize config file directory structure
	// We ignore error here because if it exists, it's fine.
	// We just want to ensure the directory is created.
	_ = cfgMgr.Init()

	var provider string

	// Stage 1: Select Provider
	err := huh.NewSelect[string]().
		Title("Select AI Provider").
		Options(
			huh.NewOption("OpenAI", "openai"),
			huh.NewOption("DeepSeek", "deepseek"),
			huh.NewOption("Ollama (Local)", "ollama"),
		).
		Value(&provider).
		Run()
	if err != nil {
		return err
	}

	var apiKey string
	var model string
	var endpoint string

	// Set defaults based on provider
	switch provider {
	case "openai":
		model = "gpt-4o-mini"
	case "deepseek":
		model = "deepseek-chat"
		endpoint = "https://api.deepseek.com"
	case "ollama":
		model = "llama2" // or codellama
		endpoint = "http://localhost:11434"
	}

	// Stage 2: Details
	fields := []huh.Field{}

	if provider != "ollama" {
		fields = append(fields,
			huh.NewInput().
				Title("API Key").
				Description("Enter your API key").
				Value(&apiKey).
				Password(true).
				Validate(func(s string) error {
					if len(strings.TrimSpace(s)) < 5 {
						return fmt.Errorf("api key too short")
					}
					return nil
				}),
		)
	}

	fields = append(fields,
		huh.NewInput().
			Title("Model Name").
			Description("Model to use").
			Value(&model).
			Validate(func(s string) error {
				if strings.TrimSpace(s) == "" {
					return fmt.Errorf("model name cannot be empty")
				}
				return nil
			}),
	)

	if provider == "ollama" || provider == "deepseek" {
		fields = append(fields,
			huh.NewInput().
				Title("API Endpoint").
				Description("Optional custom endpoint").
				Value(&endpoint),
		)
	}

	err = huh.NewForm(huh.NewGroup(fields...)).Run()
	if err != nil {
		return err
	}

	// Save configuration
	if err := cfgMgr.Set("provider.name", provider); err != nil {
		return fmt.Errorf("failed to set provider: %w", err)
	}

	if apiKey != "" {
		if err := cfgMgr.Set("provider.api_key", apiKey); err != nil {
			return fmt.Errorf("failed to set api key: %w", err)
		}
	} else if provider == "ollama" {
		// Clear API key for Ollama
		if err := cfgMgr.Set("provider.api_key", ""); err != nil {
			return fmt.Errorf("failed to generic api key: %w", err)
		}
	}

	if err := cfgMgr.Set("provider.model", model); err != nil {
		return fmt.Errorf("failed to set model: %w", err)
	}

	if endpoint != "" {
		if err := cfgMgr.Set("provider.endpoint", endpoint); err != nil {
			return fmt.Errorf("failed to set endpoint: %w", err)
		}
	}

	// Auto-acknowledge security warning since user just set it up
	if err := cfgMgr.AcknowledgeSecurityWarning(); err != nil {
		// Non-critical
	}

	fmt.Printf("\nConfiguration saved to %s\n", cfgMgr.GetConfigPath())
	fmt.Println("Setup complete! You can now use GitSage.")
	fmt.Println()

	return nil
}
