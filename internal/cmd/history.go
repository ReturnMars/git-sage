package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/gitsage/gitsage/internal/pkg/config"
	"github.com/gitsage/gitsage/internal/pkg/history"
	"github.com/spf13/cobra"
)

const (
	// DefaultHistoryLimit is the default number of history entries to display.
	DefaultHistoryLimit = 20
)

// NewHistoryCmd creates the history command and its subcommands.
func NewHistoryCmd() *cobra.Command {
	historyCmd := &cobra.Command{
		Use:   "history",
		Short: "View commit message history",
		Long: `View the history of generated commit messages.

By default, displays the most recent 20 entries. Use --limit to change the number of entries shown.

Examples:
  gitsage history           # Show last 20 entries
  gitsage history --limit 5 # Show last 5 entries
  gitsage history clear     # Clear all history`,
		RunE: runHistoryList,
	}

	// Add --limit flag
	historyCmd.Flags().IntP("limit", "l", DefaultHistoryLimit, "Number of entries to display")

	// Add subcommands
	historyCmd.AddCommand(newHistoryClearCmd())

	return historyCmd
}

// runHistoryList displays the history entries.
func runHistoryList(cmd *cobra.Command, args []string) error {
	limit, _ := cmd.Flags().GetInt("limit")

	// Load configuration to get history file path
	configPath, _ := cmd.Flags().GetString("config")
	mgr, err := config.NewManager(configPath)
	if err != nil {
		return fmt.Errorf("failed to create config manager: %w", err)
	}

	cfg, err := mgr.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Check if history is enabled
	if !cfg.History.Enabled {
		fmt.Println("History is disabled. Enable it with: gitsage config set history.enabled true")
		return nil
	}

	// Create history manager
	historyMgr := history.NewFileManager(cfg.History.FilePath, cfg.History.MaxEntries)

	// Get entries
	entries, err := historyMgr.List(limit)
	if err != nil {
		return fmt.Errorf("failed to load history: %w", err)
	}

	if len(entries) == 0 {
		fmt.Println("No history entries found.")
		return nil
	}

	// Display entries (most recent first)
	fmt.Printf("Showing %d most recent entries:\n\n", len(entries))

	// Reverse to show most recent first
	for i := len(entries) - 1; i >= 0; i-- {
		entry := entries[i]
		printHistoryEntry(entry, len(entries)-i)
	}

	return nil
}

// printHistoryEntry formats and prints a single history entry.
func printHistoryEntry(entry *history.Entry, index int) {
	// Format timestamp
	timestamp := entry.Timestamp.Format(time.RFC3339)

	// Format committed status
	status := "not committed"
	if entry.Committed {
		status = "committed"
	}

	// Print entry header
	fmt.Printf("[%d] %s (%s)\n", index, timestamp, status)

	// Print provider/model info
	if entry.Provider != "" || entry.Model != "" {
		fmt.Printf("    Provider: %s", entry.Provider)
		if entry.Model != "" {
			fmt.Printf(" (%s)", entry.Model)
		}
		fmt.Println()
	}

	// Print message (indent each line)
	fmt.Println("    Message:")
	messageLines := strings.Split(entry.Message, "\n")
	for _, line := range messageLines {
		fmt.Printf("      %s\n", line)
	}

	// Print diff summary if available
	if entry.DiffSummary != "" {
		fmt.Println("    Diff Summary:")
		summaryLines := strings.Split(entry.DiffSummary, "\n")
		for _, line := range summaryLines {
			if line != "" {
				fmt.Printf("      %s\n", line)
			}
		}
	}

	fmt.Println()
}

// newHistoryClearCmd creates the 'history clear' subcommand.
func newHistoryClearCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "clear",
		Short: "Clear all history entries",
		Long: `Delete all entries from the history file.

This action cannot be undone.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Load configuration to get history file path
			configPath, _ := cmd.Flags().GetString("config")
			mgr, err := config.NewManager(configPath)
			if err != nil {
				return fmt.Errorf("failed to create config manager: %w", err)
			}

			cfg, err := mgr.Load()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			// Create history manager
			historyMgr := history.NewFileManager(cfg.History.FilePath, cfg.History.MaxEntries)

			// Clear history
			if err := historyMgr.Clear(); err != nil {
				return fmt.Errorf("failed to clear history: %w", err)
			}

			fmt.Println("History cleared successfully.")
			return nil
		},
	}
}
