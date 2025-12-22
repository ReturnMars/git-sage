package pathcheck

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewChecker tests the factory function for creating platform-appropriate checkers.
func TestNewChecker(t *testing.T) {
	checker, err := NewChecker()
	require.NoError(t, err, "NewChecker should not return an error")
	require.NotNil(t, checker, "NewChecker should return a non-nil checker")

	// Verify GetOS returns the correct OS
	assert.Equal(t, runtime.GOOS, checker.GetOS(), "GetOS should return the current OS")

	// Verify the checker implements the Checker interface
	var _ Checker = checker
}

// TestGetExecutableDir tests retrieving the executable directory.
func TestGetExecutableDir(t *testing.T) {
	checker, err := NewChecker()
	require.NoError(t, err)

	execDir, err := checker.GetExecutableDir()
	require.NoError(t, err, "GetExecutableDir should not return an error")
	assert.NotEmpty(t, execDir, "GetExecutableDir should return a non-empty path")

	// Verify the directory exists
	info, err := os.Stat(execDir)
	require.NoError(t, err, "Executable directory should exist")
	assert.True(t, info.IsDir(), "Executable path should be a directory")
}

// TestIsInPath_NotInPath tests IsInPath when executable is not in PATH.
func TestIsInPath_NotInPath(t *testing.T) {
	checker, err := NewChecker()
	require.NoError(t, err)

	// Save original PATH
	originalPath := os.Getenv("PATH")
	defer os.Setenv("PATH", originalPath)

	// Set PATH to a directory that definitely doesn't contain the executable
	tempDir := t.TempDir()
	os.Setenv("PATH", tempDir)

	ctx := context.Background()
	inPath, err := checker.IsInPath(ctx)
	require.NoError(t, err)
	assert.False(t, inPath, "Executable should not be in PATH when PATH is set to temp directory")
}

// TestIsInPath_InPath tests IsInPath when executable is in PATH.
func TestIsInPath_InPath(t *testing.T) {
	checker, err := NewChecker()
	require.NoError(t, err)

	execDir, err := checker.GetExecutableDir()
	require.NoError(t, err)

	// Save original PATH
	originalPath := os.Getenv("PATH")
	defer os.Setenv("PATH", originalPath)

	// Add executable directory to PATH
	var newPath string
	if runtime.GOOS == "windows" {
		newPath = execDir + ";" + originalPath
	} else {
		newPath = execDir + ":" + originalPath
	}
	os.Setenv("PATH", newPath)

	ctx := context.Background()
	inPath, err := checker.IsInPath(ctx)
	require.NoError(t, err)
	assert.True(t, inPath, "Executable should be in PATH when its directory is added to PATH")
}

// TestIsInPath_EmptyPath tests IsInPath when PATH is empty.
func TestIsInPath_EmptyPath(t *testing.T) {
	checker, err := NewChecker()
	require.NoError(t, err)

	// Save original PATH
	originalPath := os.Getenv("PATH")
	defer os.Setenv("PATH", originalPath)

	// Set PATH to empty
	os.Setenv("PATH", "")

	ctx := context.Background()
	inPath, err := checker.IsInPath(ctx)
	require.NoError(t, err)
	assert.False(t, inPath, "Executable should not be in PATH when PATH is empty")
}

// TestIsInPath_PathNormalization tests that path comparison handles different path formats.
func TestIsInPath_PathNormalization(t *testing.T) {
	checker, err := NewChecker()
	require.NoError(t, err)

	execDir, err := checker.GetExecutableDir()
	require.NoError(t, err)

	// Save original PATH
	originalPath := os.Getenv("PATH")
	defer os.Setenv("PATH", originalPath)

	// Add executable directory with trailing slash
	execDirWithSlash := execDir + string(filepath.Separator)
	var newPath string
	if runtime.GOOS == "windows" {
		newPath = execDirWithSlash + ";" + originalPath
	} else {
		newPath = execDirWithSlash + ":" + originalPath
	}
	os.Setenv("PATH", newPath)

	ctx := context.Background()
	inPath, err := checker.IsInPath(ctx)
	require.NoError(t, err)
	assert.True(t, inPath, "Path comparison should normalize paths (handle trailing slashes)")
}

// TestGetShellProfile tests retrieving the shell profile path.
func TestGetShellProfile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping shell profile test on Windows")
	}

	checker, err := NewChecker()
	require.NoError(t, err)

	profilePath, err := checker.GetShellProfile()
	require.NoError(t, err, "GetShellProfile should not return an error")
	assert.NotEmpty(t, profilePath, "GetShellProfile should return a non-empty path")

	// Verify the path is absolute
	assert.True(t, filepath.IsAbs(profilePath), "Shell profile path should be absolute")

	// Verify the path contains expected shell profile names
	profileName := filepath.Base(profilePath)
	validProfiles := []string{".bashrc", ".bash_profile", ".zshrc", "config.fish", ".profile"}
	found := false
	for _, valid := range validProfiles {
		if strings.Contains(profilePath, valid) {
			found = true
			break
		}
	}
	assert.True(t, found, "Shell profile path should contain a valid profile name: %s", profileName)
}

// TestGetShellProfile_Windows tests that Windows returns empty string.
func TestGetShellProfile_Windows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Skipping Windows-specific test on non-Windows platform")
	}

	checker, err := NewChecker()
	require.NoError(t, err)

	profilePath, err := checker.GetShellProfile()
	require.NoError(t, err, "GetShellProfile should not return an error on Windows")
	assert.Empty(t, profilePath, "GetShellProfile should return empty string on Windows")
}

// TestShellTypeString tests the String method of ShellType.
func TestShellTypeString(t *testing.T) {
	tests := []struct {
		shellType ShellType
		expected  string
	}{
		{ShellBash, "bash"},
		{ShellZsh, "zsh"},
		{ShellFish, "fish"},
		{ShellPowerShell, "powershell"},
		{ShellCmd, "cmd"},
		{ShellUnknown, "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.shellType.String())
		})
	}
}
