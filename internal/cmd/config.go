package cmd

import (
	"fmt"
	"strings"

	"github.com/gitsage/gitsage/internal/pkg/config"
	"github.com/spf13/cobra"
)

// NewConfigCmd creates the config command and its subcommands.
func NewConfigCmd() *cobra.Command {
	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Manage GitSage configuration",
		Long: `Manage GitSage configuration settings.

Use subcommands to initialize, view, or modify configuration values.
Configuration is stored in ~/.gitsage.yaml by default.`,
	}

	configCmd.AddCommand(newConfigInitCmd())
	configCmd.AddCommand(newConfigSetCmd())
	configCmd.AddCommand(newConfigListCmd())

	return configCmd
}

// newConfigInitCmd creates the 'config init' subcommand.
func newConfigInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Initialize configuration file",
		Long: `Create a new configuration file at ~/.gitsage.yaml with default values.

The configuration file will be created with permissions 0600 (user read/write only)
for security, as it may contain API keys.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			configPath, _ := cmd.Flags().GetString("config")
			mgr, err := config.NewManager(configPath)
			if err != nil {
				return fmt.Errorf("failed to create config manager: %w", err)
			}

			if err := mgr.Init(); err != nil {
				return err
			}

			fmt.Printf("Configuration file created at %s\n", mgr.GetConfigPath())
			fmt.Println("Edit this file to set your API key and customize settings.")
			return nil
		},
	}
}

// newConfigSetCmd creates the 'config set' subcommand.
func newConfigSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a configuration value",
		Long: `Set a configuration value by key.

Supports nested keys using dot notation (e.g., "provider.name", "git.diff_size_threshold").

Examples:
  gitsage config set provider.name openai
  gitsage config set provider.api_key sk-xxx
  gitsage config set provider.model gpt-4o-mini
  gitsage config set git.diff_size_threshold 20480`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			key := args[0]
			value := args[1]

			configPath, _ := cmd.Flags().GetString("config")
			mgr, err := config.NewManager(configPath)
			if err != nil {
				return fmt.Errorf("failed to create config manager: %w", err)
			}

			// Check if config file exists
			if !mgr.ConfigExists() {
				return fmt.Errorf("config file not found. Run 'gitsage config init' first")
			}

			if err := mgr.Set(key, value); err != nil {
				return err
			}

			// Mask API key in output
			displayValue := value
			if strings.Contains(strings.ToLower(key), "api_key") {
				displayValue = config.MaskAPIKey(value)
			}

			fmt.Printf("Set %s = %s\n", key, displayValue)
			return nil
		},
	}
}

// newConfigListCmd creates the 'config list' subcommand.
func newConfigListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all configuration values",
		Long: `Display all current configuration values.

API keys are masked for security, showing only the last 4 characters.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			configPath, _ := cmd.Flags().GetString("config")
			mgr, err := config.NewManager(configPath)
			if err != nil {
				return fmt.Errorf("failed to create config manager: %w", err)
			}

			settings := mgr.List()
			printSettings("", settings)
			return nil
		},
	}
}

// printSettings recursively prints configuration settings with proper formatting.
func printSettings(prefix string, settings map[string]interface{}) {
	for key, value := range settings {
		fullKey := key
		if prefix != "" {
			fullKey = prefix + "." + key
		}

		switch v := value.(type) {
		case map[string]interface{}:
			fmt.Printf("%s:\n", key)
			printSettingsIndented("  ", fullKey, v)
		default:
			// Mask API keys
			displayValue := fmt.Sprintf("%v", value)
			if strings.Contains(strings.ToLower(key), "api_key") && displayValue != "" {
				displayValue = config.MaskAPIKey(displayValue)
			}
			fmt.Printf("%s: %s\n", key, displayValue)
		}
	}
}

// printSettingsIndented prints settings with indentation for nested values.
func printSettingsIndented(indent, prefix string, settings map[string]interface{}) {
	for key, value := range settings {
		fullKey := prefix + "." + key

		switch v := value.(type) {
		case map[string]interface{}:
			fmt.Printf("%s%s:\n", indent, key)
			printSettingsIndented(indent+"  ", fullKey, v)
		default:
			// Mask API keys
			displayValue := fmt.Sprintf("%v", value)
			if strings.Contains(strings.ToLower(key), "api_key") && displayValue != "" {
				displayValue = config.MaskAPIKey(displayValue)
			}
			fmt.Printf("%s%s: %s\n", indent, key, displayValue)
		}
	}
}
