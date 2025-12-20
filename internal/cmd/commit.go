// Package cmd contains the CLI command definitions for GitSage.
package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/gitsage/gitsage/internal/app"
	"github.com/gitsage/gitsage/internal/pkg/ai"
	"github.com/gitsage/gitsage/internal/pkg/config"
	apperrors "github.com/gitsage/gitsage/internal/pkg/errors"
	"github.com/gitsage/gitsage/internal/pkg/git"
	"github.com/gitsage/gitsage/internal/pkg/history"
	"github.com/gitsage/gitsage/internal/pkg/processor"
	"github.com/gitsage/gitsage/internal/pkg/security"
	"github.com/gitsage/gitsage/internal/pkg/ui"
	"github.com/spf13/cobra"
)

// CommitFlags holds the flags for the commit command.
type CommitFlags struct {
	DryRun     bool
	Yes        bool
	OutputFile string
	NoCache    bool
}

// NewCommitCmd creates the commit command.
func NewCommitCmd() *cobra.Command {
	flags := &CommitFlags{}

	cmd := &cobra.Command{
		Use:   "commit",
		Short: "Generate and commit with an AI-generated message",
		Long: `Generate a commit message using AI based on your staged changes,
then optionally commit with that message.

The command analyzes your staged git diff, sends it to the configured
AI provider, and presents you with an interactive interface to review,
edit, and confirm the commit message.

Examples:
  gitsage commit              # Interactive commit
  gitsage commit --yes        # Auto-accept generated message
  gitsage commit --dry-run    # Generate without committing
  gitsage commit -o msg.txt   # Save message to file`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCommit(cmd, flags)
		},
	}

	// Add commit-specific flags
	cmd.Flags().BoolVar(&flags.DryRun, "dry-run", false, "Generate message without committing")
	cmd.Flags().BoolVarP(&flags.Yes, "yes", "y", false, "Skip interactive confirmation and commit immediately")
	cmd.Flags().StringVarP(&flags.OutputFile, "output", "o", "", "Write generated message to file (implies --dry-run)")
	cmd.Flags().BoolVar(&flags.NoCache, "no-cache", false, "Bypass response cache")

	return cmd
}

// runCommit executes the commit command logic.
func runCommit(cmd *cobra.Command, flags *CommitFlags) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Get global flags
	verbose, _ := cmd.Flags().GetBool("verbose")
	configPath, _ := cmd.Flags().GetString("config")
	providerOverride, _ := cmd.Flags().GetString("provider")
	modelOverride, _ := cmd.Flags().GetString("model")

	// Enable verbose logging if flag is set
	apperrors.SetVerbose(verbose)

	// Load configuration with custom path if specified
	// The --config flag allows using a different config file for this execution
	cfgMgr, err := config.NewManager(configPath)
	if err != nil {
		apperrors.Error("Failed to create config manager: %v", err)
		return apperrors.Wrap(err, apperrors.ErrInvalidConfig, "failed to create config manager")
	}

	// Log custom config path if specified
	if configPath != "" {
		apperrors.Debug("Using custom config path: %s", configPath)
	}

	// Check if config exists
	if !cfgMgr.ConfigExists() {
		// Launch interactive setup if config doesn't exist
		if err := ui.RunInteractiveSetup(cfgMgr); err != nil {
			return fmt.Errorf("setup failed: %w", err)
		}
	}

	// Apply command-line flag overrides BEFORE loading config
	// This ensures flags take highest priority (flags > env > file > defaults)
	// These overrides are temporary and don't persist to the config file
	if providerOverride != "" {
		cfgMgr.SetOverride("provider.name", providerOverride)
		apperrors.Debug("Provider overridden via flag: %s", providerOverride)
	}
	if modelOverride != "" {
		cfgMgr.SetOverride("provider.model", modelOverride)
		apperrors.Debug("Model overridden via flag: %s", modelOverride)
	}

	cfg, err := cfgMgr.Load()
	if err != nil {
		apperrors.Error("Failed to load config: %v", err)
		return apperrors.Wrap(err, apperrors.ErrInvalidConfig, "failed to load config")
	}

	// If output file is specified, enable dry-run mode
	if flags.OutputFile != "" {
		flags.DryRun = true
	}

	// Validate API key format before making requests (fail fast)
	if err := security.ValidateAPIKeyFormat(cfg.Provider.Name, cfg.Provider.APIKey); err != nil {
		apperrors.Error("API key validation failed: %v", err)
		return apperrors.Wrap(err, apperrors.ErrInvalidConfig, "invalid API key")
	}

	// Check and show first-use security warning for external providers
	if cfg.Provider.Name != "ollama" && !cfg.Security.WarningAcknowledged {
		if err := showSecurityWarning(cfgMgr, flags.Yes); err != nil {
			return err
		}
	}

	// Verbose logging
	if verbose {
		apperrors.Info("Using provider: %s", cfg.Provider.Name)
		apperrors.Info("Using model: %s", cfg.Provider.Model)
		if cfg.Provider.APIKey != "" {
			apperrors.Info("API key: %s", security.MaskAPIKey(cfg.Provider.APIKey))
		}
		if flags.DryRun {
			apperrors.Info("Dry-run mode enabled")
		}
	}

	// Create dependencies
	gitClient := git.NewClient()

	aiProvider, err := ai.NewProvider(&cfg.Provider)
	if err != nil {
		apperrors.Error("Failed to create AI provider: %v", err)
		return apperrors.NewAIProviderError(cfg.Provider.Name, err)
	}
	apperrors.Debug("AI provider created: %s", aiProvider.Name())

	diffProcessor := processor.NewProcessorWithConfig(processor.ProcessorConfig{
		DiffSizeThreshold: cfg.Git.DiffSizeThreshold,
	})

	// Create UI manager based on --yes flag
	var uiMgr ui.Manager
	if flags.Yes {
		uiMgr = ui.NewNonInteractiveManager(cfg.UI.ColorEnabled)
	} else {
		uiMgr = ui.NewDefaultManager(cfg.UI.ColorEnabled, cfg.UI.Editor)
	}

	// Create history manager
	var historyMgr history.Manager
	if cfg.History.Enabled {
		historyMgr = history.NewFileManager(cfg.History.FilePath, cfg.History.MaxEntries)
	}

	// Create commit service
	service := app.NewCommitService(
		gitClient,
		aiProvider,
		diffProcessor,
		uiMgr,
		historyMgr,
		cfg,
	)

	// Execute the commit workflow
	opts := &app.CommitOptions{
		DryRun:      flags.DryRun,
		OutputFile:  flags.OutputFile,
		SkipConfirm: flags.Yes,
		NoCache:     flags.NoCache,
	}

	return service.GenerateAndCommit(ctx, opts)
}

// showSecurityWarning displays the first-use security warning and prompts for acknowledgment.
func showSecurityWarning(cfgMgr *config.ViperManager, autoAccept bool) error {
	fmt.Print(security.FirstUseWarning)

	if autoAccept {
		// In non-interactive mode, auto-acknowledge
		fmt.Println("Auto-acknowledging security warning (--yes flag)")
	} else {
		// Prompt for acknowledgment
		fmt.Print("Do you understand and wish to continue? [y/N]: ")
		reader := bufio.NewReader(os.Stdin)
		response, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read response: %w", err)
		}

		response = strings.TrimSpace(strings.ToLower(response))
		if response != "y" && response != "yes" {
			return fmt.Errorf("security warning not acknowledged - operation cancelled")
		}
	}

	// Save acknowledgment to config
	if err := cfgMgr.AcknowledgeSecurityWarning(); err != nil {
		apperrors.Warn("Failed to save security acknowledgment: %v", err)
		// Don't fail the operation, just warn
	}

	fmt.Println(security.FirstUseAcknowledgment)
	fmt.Println()

	return nil
}
