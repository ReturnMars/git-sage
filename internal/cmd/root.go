// Package cmd contains the CLI command definitions for GitSage.
package cmd

import (
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
	rootCmd.PersistentFlags().String("config", "", "Config file path (default: ~/.gitsage.yaml)")
	rootCmd.PersistentFlags().String("provider", "", "AI provider to use (openai, deepseek, ollama)")
	rootCmd.PersistentFlags().String("model", "", "AI model to use")

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
