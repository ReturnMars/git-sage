// Package cmd contains the CLI command definitions for GitSage.
package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/gitsage/gitsage/internal/app/workflow"
	"github.com/gitsage/gitsage/internal/core/services/cache"
	"github.com/gitsage/gitsage/internal/core/services/diff"
	"github.com/gitsage/gitsage/internal/core/services/generator"
	"github.com/gitsage/gitsage/internal/core/services/prompt"
	"github.com/gitsage/gitsage/internal/core/services/retry"
	"github.com/gitsage/gitsage/internal/core/services/validator"
	infra_ai "github.com/gitsage/gitsage/internal/infra/ai"
	infra_git "github.com/gitsage/gitsage/internal/infra/git"
	infra_ui "github.com/gitsage/gitsage/internal/infra/ui"
	"github.com/gitsage/gitsage/internal/pkg/config"
	apperrors "github.com/gitsage/gitsage/internal/pkg/errors"
	"github.com/gitsage/gitsage/internal/pkg/security"
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
	// providerOverride, _ := cmd.Flags().GetString("provider")
	// modelOverride, _ := cmd.Flags().GetString("model")

	// Enable verbose logging if flag is set
	apperrors.SetVerbose(verbose)

	// Load configuration
	cfgMgr, err := config.NewManager(configPath)
	if err != nil {
		return apperrors.Wrap(err, apperrors.ErrInvalidConfig, "failed to create config manager")
	}

	if !cfgMgr.ConfigExists() {
		// Launch interactive setup if config doesn't exist
		// Note: We're reusing the old setup logic for now strictly for config file creation
		if err := runInteractiveSetup(cfgMgr); err != nil {
			return fmt.Errorf("setup failed: %w", err)
		}
	}

	cfg, err := cfgMgr.Load()
	if err != nil {
		return apperrors.Wrap(err, apperrors.ErrInvalidConfig, "failed to load config")
	}

	// 1. Initialize Adapters (Infrastructure Layer)
	gitAdapter := infra_git.NewAdapter("")

	aiConfig := infra_ai.Config{
		ProviderName: cfg.Provider.Name,
		APIKey:       cfg.Provider.APIKey,
		Model:        cfg.Provider.Model,
		Temperature:  float64(cfg.Provider.Temperature),
		MaxTokens:    cfg.Provider.MaxTokens,
	}
	// Support both BaseURL and Endpoint for backwards compatibility
	if cfg.Provider.BaseURL != "" {
		aiConfig.BaseURL = cfg.Provider.BaseURL
	} else if cfg.Provider.Endpoint != "" {
		aiConfig.BaseURL = cfg.Provider.Endpoint
	}
	aiAdapter, err := infra_ai.NewAdapter(&aiConfig)
	if err != nil {
		return fmt.Errorf("failed to init AI adapter: %w", err)
	}

	uiAdapter := infra_ui.NewConsoleUI()

	// 2. Initialize Core Services (Core Layer)
	diffProcessor := diff.NewProcessor(cfg.Git.DiffSizeThreshold, cfg.Git.ExcludePatterns)
	retryStrategy := retry.NewExponentialBackoff(1*time.Second, 10*time.Second)
	promptBuilder, err := prompt.NewBuilder()
	if err != nil {
		return fmt.Errorf("failed to init prompt builder: %w", err)
	}

	var lruCache *cache.LRUCache
	if !flags.NoCache {
		// Initialize Cache (default 100 items, 24h TTL)
		lruCache = cache.NewLRUCache(100, 24*time.Hour)
	}

	// Create SmartGenerator
	smartGenerator := generator.NewSmartGenerator(
		aiAdapter,
		promptBuilder,
		diffProcessor,
		retryStrategy,
		lruCache,
	)

	conventionalValidator := validator.NewConventionalValidator()

	// 3. Initialize Workflow (Application Layer)
	commitWorkflow := workflow.NewCommitWorkflow(
		gitAdapter,
		uiAdapter,
		diffProcessor,
		smartGenerator,
		conventionalValidator,
	)

	// 4. Run Workflow
	// For now, we ignore hint passing from CLI args (future feature)
	hint := ""
	if err := commitWorkflow.Run(ctx, hint, flags.Yes); err != nil {
		return err
	}

	return nil
}

// Helper to run interactive setup reusing old ui package if needed,
// or we can just fail if no config.
// For minimizing changes, we assume config exists or user runs 'gitsage config init'.
func runInteractiveSetup(cfgMgr *config.ViperManager) error {
	// Simple instruction for now
	fmt.Println("Config not found. Please run 'gitsage config init' or create ~/.gitsage/config.yaml")
	return fmt.Errorf("config missing")
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
