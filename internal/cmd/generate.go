// Package cmd contains the CLI command definitions for GitSage.
package cmd

import (
	"github.com/spf13/cobra"
)

// NewGenerateCmd creates the generate command as an alias for commit --dry-run.
func NewGenerateCmd() *cobra.Command {
	flags := &CommitFlags{
		DryRun: true, // Always dry-run for generate command
	}

	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate a commit message without committing",
		Long: `Generate a commit message using AI based on your staged changes
without actually committing.

This is equivalent to running 'gitsage commit --dry-run'.

The generated message is displayed to stdout by default, or can be
written to a file using the --output flag.

Examples:
  gitsage generate              # Generate and display message
  gitsage generate -o msg.txt   # Save message to file
  gitsage generate --yes        # Skip interactive prompt`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCommit(cmd, flags)
		},
	}

	// Add generate-specific flags (subset of commit flags)
	cmd.Flags().BoolVarP(&flags.Yes, "yes", "y", false, "Skip interactive confirmation")
	cmd.Flags().StringVarP(&flags.OutputFile, "output", "o", "", "Write generated message to file")

	return cmd
}
