//go:build !windows

package pathcheck

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// UnixChecker implements Checker for macOS and Linux.
type UnixChecker struct {
	executablePath string
}

// newPlatformChecker creates a Unix-specific checker.
func newPlatformChecker(execPath string) (Checker, error) {
	return &UnixChecker{executablePath: execPath}, nil
}

// IsInPath checks if the executable is accessible via PATH.
func (c *UnixChecker) IsInPath(ctx context.Context) (bool, error) {
	execDir, err := c.GetExecutableDir()
	if err != nil {
		return false, err
	}

	pathEnv := os.Getenv("PATH")
	if pathEnv == "" {
		return false, nil
	}

	paths := filepath.SplitList(pathEnv)
	for _, p := range paths {
		// Normalize paths for comparison
		cleanP := filepath.Clean(p)
		cleanExecDir := filepath.Clean(execDir)
		if cleanP == cleanExecDir {
			return true, nil
		}
	}
	return false, nil
}

// AddToPath adds the executable directory to the system PATH.
// It appends an export statement to the appropriate shell profile file.
func (c *UnixChecker) AddToPath(ctx context.Context) (*PathAddResult, error) {
	execDir, err := c.GetExecutableDir()
	if err != nil {
		return &PathAddResult{
			Success: false,
			Message: fmt.Sprintf("Failed to get executable directory: %v", err),
		}, err
	}

	profilePath, err := c.GetShellProfile()
	if err != nil {
		return &PathAddResult{
			Success:   false,
			AddedPath: execDir,
			Message:   fmt.Sprintf("Failed to get shell profile path: %v", err),
		}, err
	}

	// Generate the appropriate export statement based on shell type
	exportStmt := c.generateExportStatement(execDir)

	// Ensure the profile directory exists (for fish shell)
	profileDir := filepath.Dir(profilePath)
	if err := os.MkdirAll(profileDir, 0755); err != nil {
		return &PathAddResult{
			Success:     false,
			AddedPath:   execDir,
			ProfilePath: profilePath,
			Message:     fmt.Sprintf("Failed to create profile directory: %v", err),
		}, fmt.Errorf("failed to create profile directory %s: %w", profileDir, err)
	}

	// Get existing file permissions if file exists
	var fileMode os.FileMode = 0644
	if info, err := os.Stat(profilePath); err == nil {
		fileMode = info.Mode()
	}

	// Open file for appending (create if not exists)
	f, err := os.OpenFile(profilePath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, fileMode)
	if err != nil {
		return &PathAddResult{
			Success:     false,
			AddedPath:   execDir,
			ProfilePath: profilePath,
			Message:     fmt.Sprintf("Failed to open profile file: %v", err),
		}, fmt.Errorf("failed to open profile file %s: %w", profilePath, err)
	}
	defer f.Close()

	// Write the export statement
	if _, err := f.WriteString(exportStmt); err != nil {
		return &PathAddResult{
			Success:     false,
			AddedPath:   execDir,
			ProfilePath: profilePath,
			Message:     fmt.Sprintf("Failed to write to profile file: %v", err),
		}, fmt.Errorf("failed to write to profile file %s: %w", profilePath, err)
	}

	// Restore original file permissions if they were different
	if info, err := os.Stat(profilePath); err == nil && info.Mode() != fileMode {
		_ = os.Chmod(profilePath, fileMode)
	}

	shellType := c.detectShell()
	reloadCmd := c.getReloadCommand(profilePath, shellType)

	return &PathAddResult{
		Success:     true,
		AddedPath:   execDir,
		ProfilePath: profilePath,
		Message:     fmt.Sprintf("Successfully added %s to PATH in %s. %s", execDir, profilePath, reloadCmd),
		NeedsReload: true,
	}, nil
}

// generateExportStatement generates the appropriate export statement for the current shell.
func (c *UnixChecker) generateExportStatement(execDir string) string {
	shellType := c.detectShell()
	return GenerateExportStatementForShell(shellType, execDir)
}

// GenerateExportStatement is exported for testing purposes.
func (c *UnixChecker) GenerateExportStatement(execDir string) string {
	return c.generateExportStatement(execDir)
}

// getReloadCommand returns the command to reload the shell profile.
func (c *UnixChecker) getReloadCommand(profilePath string, shellType ShellType) string {
	switch shellType {
	case ShellFish:
		return fmt.Sprintf("Please restart your terminal or run: source %s", profilePath)
	default:
		return fmt.Sprintf("Please restart your terminal or run: source %s", profilePath)
	}
}

// GetExecutableDir returns the directory containing the executable.
func (c *UnixChecker) GetExecutableDir() (string, error) {
	// Resolve symlinks to get the real path
	realPath, err := filepath.EvalSymlinks(c.executablePath)
	if err != nil {
		// Fall back to the original path if symlink resolution fails
		realPath = c.executablePath
	}
	return filepath.Dir(realPath), nil
}

// GetShellProfile returns the appropriate shell profile path for the current system.
func (c *UnixChecker) GetShellProfile() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	shellType := c.detectShell()
	profilePath := c.getProfilePathForShell(shellType, homeDir)

	return profilePath, nil
}

// GetOS returns the current operating system.
func (c *UnixChecker) GetOS() string {
	return runtime.GOOS
}

// detectShell detects the current shell type from the SHELL environment variable.
func (c *UnixChecker) detectShell() ShellType {
	shell := os.Getenv("SHELL")
	if shell == "" {
		return ShellUnknown
	}

	shellName := filepath.Base(shell)
	switch {
	case strings.Contains(shellName, "bash"):
		return ShellBash
	case strings.Contains(shellName, "zsh"):
		return ShellZsh
	case strings.Contains(shellName, "fish"):
		return ShellFish
	default:
		return ShellUnknown
	}
}

// getProfilePathForShell returns the profile file path for the given shell type.
func (c *UnixChecker) getProfilePathForShell(shellType ShellType, homeDir string) string {
	switch shellType {
	case ShellBash:
		// Check for .bashrc first, then .bash_profile
		bashrc := filepath.Join(homeDir, ".bashrc")
		if _, err := os.Stat(bashrc); err == nil {
			return bashrc
		}
		bashProfile := filepath.Join(homeDir, ".bash_profile")
		if _, err := os.Stat(bashProfile); err == nil {
			return bashProfile
		}
		// Default to .bashrc if neither exists
		return bashrc
	case ShellZsh:
		return filepath.Join(homeDir, ".zshrc")
	case ShellFish:
		return filepath.Join(homeDir, ".config", "fish", "config.fish")
	default:
		// Default to .profile for unknown shells
		return filepath.Join(homeDir, ".profile")
	}
}

// GetShellType returns the detected shell type.
func (c *UnixChecker) GetShellType() ShellType {
	return c.detectShell()
}
