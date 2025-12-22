// Package cmd contains the CLI command definitions for GitSage.
package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/gitsage/gitsage/internal/pkg/config"
	"github.com/gitsage/gitsage/internal/pkg/pathcheck"
	"github.com/gitsage/gitsage/internal/pkg/ui"
	"github.com/spf13/cobra"
)

// NewRootCmd creates the root command for GitSage CLI.
func NewRootCmd(version, commitHash, date string) *cobra.Command {
	// Create commit command first so we can reference it
	commitCmd := NewCommitCmd()

	rootCmd := &cobra.Command{
		Use:   "gitsage",
		Short: "AI-powered git commit message generator",
		Long: `GitSage is an AI-powered command-line tool that automatically generates
semantic Git commit messages based on staged changes.

It analyzes your git diff output, sends it to configurable AI providers
(OpenAI, DeepSeek, Ollama), and presents you with an interactive interface
to review, edit, and confirm commit messages before execution.`,
		Version: version,
		// PersistentPreRunE runs before any command (including subcommands)
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return runPathCheckIfNeeded(cmd)
		},
		// Default action is to run the commit command
		RunE: func(cmd *cobra.Command, args []string) error {
			// Get commit-specific flags from root command
			dryRun, _ := cmd.Flags().GetBool("dry-run")
			yes, _ := cmd.Flags().GetBool("yes")
			output, _ := cmd.Flags().GetString("output")
			noCache, _ := cmd.Flags().GetBool("no-cache")

			// Create flags struct for commit command
			flags := &CommitFlags{
				DryRun:     dryRun,
				Yes:        yes,
				OutputFile: output,
				NoCache:    noCache,
			}

			return runCommit(cmd, flags)
		},
	}

	// Set version template
	rootCmd.SetVersionTemplate(`GitSage {{.Version}}
Commit: ` + commitHash + `
Built:  ` + date + "\n")

	// Global flags
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "Enable verbose logging")
	rootCmd.PersistentFlags().String("config", "", "Config file path (default: ~/.gitsage/config.yaml)")
	rootCmd.PersistentFlags().String("provider", "", "AI provider to use (openai, deepseek, ollama)")
	rootCmd.PersistentFlags().String("model", "", "AI model to use")
	rootCmd.PersistentFlags().Bool("skip-path-check", false, "Skip PATH detection check")

	// Add commit-specific flags to root command for default action
	rootCmd.Flags().Bool("dry-run", false, "Generate message without committing")
	rootCmd.Flags().BoolP("yes", "y", false, "Skip interactive confirmation and commit immediately")
	rootCmd.Flags().StringP("output", "o", "", "Write generated message to file (implies --dry-run)")
	rootCmd.Flags().Bool("no-cache", false, "Bypass response cache")

	// Add subcommands
	rootCmd.AddCommand(commitCmd)
	rootCmd.AddCommand(NewGenerateCmd())
	rootCmd.AddCommand(NewConfigCmd())
	rootCmd.AddCommand(NewHistoryCmd())

	return rootCmd
}

// runPathCheckIfNeeded performs PATH detection if needed.
// It skips the check for config and help commands, or if --skip-path-check flag is set.
func runPathCheckIfNeeded(cmd *cobra.Command) error {
	// Skip for config, help, and version commands
	cmdName := cmd.Name()
	if cmdName == "config" || cmdName == "help" || cmdName == "version" {
		return nil
	}

	// Also skip if this is a help flag request
	helpFlag, _ := cmd.Flags().GetBool("help")
	if helpFlag {
		return nil
	}

	// Check --skip-path-check flag
	skipPathCheck, _ := cmd.Flags().GetBool("skip-path-check")
	if skipPathCheck {
		return nil
	}

	// Load config to check if PATH check was already done
	configPath, _ := cmd.Flags().GetString("config")
	cfgManager, err := config.NewManager(configPath)
	if err != nil {
		// If we can't load config, skip PATH check but don't fail
		return nil
	}

	cfg, err := cfgManager.Load()
	if err != nil {
		// If we can't load config, skip PATH check but don't fail
		return nil
	}

	// Skip if PATH check was already done
	if cfg.Security.PathCheckDone {
		return nil
	}

	// Perform PATH check
	return performPathCheck(cfgManager)
}

// performPathCheck performs the actual PATH detection and prompts user if needed.
func performPathCheck(cfgManager *config.ViperManager) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create PATH checker
	checker, err := pathcheck.NewChecker()
	if err != nil {
		// If we can't create checker, skip but don't fail
		return nil
	}

	// Check if already in PATH
	inPath, err := checker.IsInPath(ctx)
	if err != nil {
		// If check fails, skip but don't fail
		return nil
	}

	// If already in PATH, mark as done and continue
	if inPath {
		_ = cfgManager.Set("security.path_check_done", "true")
		return nil
	}

	// Create UI manager for user interaction
	uiManager := ui.NewDefaultManager(true, "", false)

	// Get executable directory for display
	execDir, err := checker.GetExecutableDir()
	if err != nil {
		execDir = "<unknown>"
	}

	// Prompt user
	fmt.Println()
	fmt.Printf("GitSage 检测到可执行文件目录 (%s) 不在系统 PATH 中。\n", execDir)
	fmt.Println("添加到 PATH 后，您可以在任何目录直接运行 'gitsage' 命令。")
	fmt.Println()

	confirmed, err := uiManager.PromptConfirm("是否自动添加到 PATH?")
	if err != nil {
		// If prompt fails, mark as done and continue
		_ = cfgManager.Set("security.path_check_done", "true")
		return nil
	}

	if confirmed {
		// Try to add to PATH
		result, err := checker.AddToPath(ctx)
		if err != nil || !result.Success {
			// Show error and manual instructions
			uiManager.ShowError(fmt.Errorf("自动添加失败"))

			// Get shell type for instructions (Unix only)
			shellType := pathcheck.ShellUnknown
			if unixChecker, ok := checker.(interface{ GetShellType() pathcheck.ShellType }); ok {
				shellType = unixChecker.GetShellType()
			}

			instructions := pathcheck.GetManualInstructions(execDir, shellType)
			fmt.Println(pathcheck.FormatInstructions(instructions))
		} else {
			// Show success message
			uiManager.ShowSuccess(result.Message)
			if result.NeedsReload {
				fmt.Println("请重启终端或执行 source 命令使更改生效。")
			}
		}
	} else {
		fmt.Println("已跳过 PATH 配置。您可以稍后手动添加。")
	}

	// Mark PATH check as done regardless of outcome
	_ = cfgManager.Set("security.path_check_done", "true")

	fmt.Println()
	return nil
}
