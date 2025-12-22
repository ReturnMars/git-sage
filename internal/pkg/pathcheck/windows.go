//go:build windows

package pathcheck

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// WindowsChecker implements Checker for Windows.
type WindowsChecker struct {
	executablePath string
}

// newPlatformChecker creates a Windows-specific checker.
func newPlatformChecker(execPath string) (Checker, error) {
	return &WindowsChecker{executablePath: execPath}, nil
}

// IsInPath checks if the executable is accessible via PATH.
func (c *WindowsChecker) IsInPath(ctx context.Context) (bool, error) {
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
		// Normalize paths for comparison (case-insensitive on Windows)
		cleanP := strings.ToLower(filepath.Clean(p))
		cleanExecDir := strings.ToLower(filepath.Clean(execDir))
		if cleanP == cleanExecDir {
			return true, nil
		}
	}
	return false, nil
}

// AddToPath adds the executable directory to the user's PATH environment variable.
// It uses the setx command to permanently add the path.
func (c *WindowsChecker) AddToPath(ctx context.Context) (*PathAddResult, error) {
	execDir, err := c.GetExecutableDir()
	if err != nil {
		return &PathAddResult{
			Success: false,
			Message: fmt.Sprintf("Failed to get executable directory: %v", err),
		}, err
	}

	// Get current user PATH
	currentPath := os.Getenv("PATH")

	// Check if already in PATH (shouldn't happen if IsInPath was called first, but be safe)
	paths := filepath.SplitList(currentPath)
	for _, p := range paths {
		cleanP := strings.ToLower(filepath.Clean(p))
		cleanExecDir := strings.ToLower(filepath.Clean(execDir))
		if cleanP == cleanExecDir {
			return &PathAddResult{
				Success:     true,
				AddedPath:   execDir,
				Message:     fmt.Sprintf("%s is already in PATH", execDir),
				NeedsReload: false,
			}, nil
		}
	}

	// Build new PATH value
	// Note: setx has a limit of 1024 characters for the value
	// We need to get the user PATH specifically, not the combined system+user PATH
	userPath := c.getUserPath()
	var newPath string
	if userPath == "" {
		newPath = execDir
	} else {
		newPath = userPath + ";" + execDir
	}

	// Check if the new PATH exceeds setx limit
	if len(newPath) > 1024 {
		return &PathAddResult{
			Success:   false,
			AddedPath: execDir,
			Message:   "PATH value would exceed 1024 character limit for setx command. Please add manually via System Properties.",
		}, fmt.Errorf("PATH value exceeds setx limit of 1024 characters")
	}

	// Use setx to add to user PATH
	cmd := exec.CommandContext(ctx, "setx", "PATH", newPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return &PathAddResult{
			Success:   false,
			AddedPath: execDir,
			Message:   fmt.Sprintf("Failed to execute setx command: %v. Output: %s", err, string(output)),
		}, fmt.Errorf("setx command failed: %w, output: %s", err, string(output))
	}

	return &PathAddResult{
		Success:     true,
		AddedPath:   execDir,
		Message:     fmt.Sprintf("Successfully added %s to PATH. Please restart your terminal for changes to take effect.", execDir),
		NeedsReload: true,
	}, nil
}

// getUserPath attempts to get the user-specific PATH from the registry.
// Falls back to empty string if unable to read.
func (c *WindowsChecker) getUserPath() string {
	// Try to read user PATH from registry using reg query
	cmd := exec.Command("reg", "query", "HKCU\\Environment", "/v", "PATH")
	output, err := cmd.Output()
	if err != nil {
		// User PATH might not exist yet, which is fine
		return ""
	}

	// Parse the output to extract the PATH value
	// Output format: "    PATH    REG_SZ    value" or "    PATH    REG_EXPAND_SZ    value"
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToUpper(line), "PATH") {
			// Find the value after REG_SZ or REG_EXPAND_SZ
			parts := strings.Fields(line)
			if len(parts) >= 3 {
				// The value is everything after the type (REG_SZ or REG_EXPAND_SZ)
				// Join remaining parts in case path contains spaces
				return strings.Join(parts[2:], " ")
			}
		}
	}

	return ""
}

// GetExecutableDir returns the directory containing the executable.
func (c *WindowsChecker) GetExecutableDir() (string, error) {
	// Resolve symlinks to get the real path
	realPath, err := filepath.EvalSymlinks(c.executablePath)
	if err != nil {
		// Fall back to the original path if symlink resolution fails
		realPath = c.executablePath
	}
	return filepath.Dir(realPath), nil
}

// GetShellProfile returns empty string for Windows (not used).
func (c *WindowsChecker) GetShellProfile() (string, error) {
	// Windows doesn't use shell profile files for PATH configuration
	return "", nil
}

// GetOS returns the current operating system.
func (c *WindowsChecker) GetOS() string {
	return runtime.GOOS
}
