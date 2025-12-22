// Package pathcheck provides PATH detection and modification functionality for GitSage.
package pathcheck

import (
	"context"
	"fmt"
	"os"
)

// ShellType represents the type of shell.
type ShellType int

const (
	// ShellUnknown represents an unknown shell type.
	ShellUnknown ShellType = iota
	// ShellBash represents the Bash shell.
	ShellBash
	// ShellZsh represents the Zsh shell.
	ShellZsh
	// ShellFish represents the Fish shell.
	ShellFish
	// ShellPowerShell represents PowerShell.
	ShellPowerShell
	// ShellCmd represents Windows Command Prompt.
	ShellCmd
)

// String returns the string representation of the shell type.
func (s ShellType) String() string {
	switch s {
	case ShellBash:
		return "bash"
	case ShellZsh:
		return "zsh"
	case ShellFish:
		return "fish"
	case ShellPowerShell:
		return "powershell"
	case ShellCmd:
		return "cmd"
	default:
		return "unknown"
	}
}

// PathAddResult contains the result of adding to PATH.
type PathAddResult struct {
	// Success indicates if the PATH addition was successful.
	Success bool
	// AddedPath is the directory path that was added to PATH.
	AddedPath string
	// ProfilePath is the shell profile file that was modified (Unix only).
	ProfilePath string
	// Message contains a human-readable result message.
	Message string
	// NeedsReload indicates if the user needs to reload their shell/terminal.
	NeedsReload bool
}

// Checker provides PATH detection and modification functionality.
type Checker interface {
	// IsInPath checks if the executable is accessible via PATH.
	IsInPath(ctx context.Context) (bool, error)

	// AddToPath adds the executable directory to the system PATH.
	// Returns the result of the operation.
	AddToPath(ctx context.Context) (*PathAddResult, error)

	// GetExecutableDir returns the directory containing the executable.
	GetExecutableDir() (string, error)

	// GetShellProfile returns the appropriate shell profile path for the current system.
	// Returns empty string for Windows.
	GetShellProfile() (string, error)

	// GetOS returns the current operating system.
	GetOS() string
}

// NewChecker creates a platform-appropriate Checker.
// The implementation is provided by platform-specific files (unix.go, windows.go).
func NewChecker() (Checker, error) {
	execPath, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("failed to get executable path: %w", err)
	}
	return newPlatformChecker(execPath)
}
