//go:build !windows

package pathcheck

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUnixChecker_DetectShell tests shell detection logic.
func TestUnixChecker_DetectShell(t *testing.T) {
	tests := []struct {
		name         string
		shellEnv     string
		expectedType ShellType
		expectedName string
	}{
		{
			name:         "bash shell",
			shellEnv:     "/bin/bash",
			expectedType: ShellBash,
			expectedName: "bash",
		},
		{
			name:         "zsh shell",
			shellEnv:     "/usr/bin/zsh",
			expectedType: ShellZsh,
			expectedName: "zsh",
		},
		{
			name:         "fish shell",
			shellEnv:     "/usr/local/bin/fish",
			expectedType: ShellFish,
			expectedName: "fish",
		},
		{
			name:         "unknown shell",
			shellEnv:     "/bin/sh",
			expectedType: ShellUnknown,
			expectedName: "unknown",
		},
		{
			name:         "empty shell",
			shellEnv:     "",
			expectedType: ShellUnknown,
			expectedName: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save original SHELL
			originalShell := os.Getenv("SHELL")
			defer os.Setenv("SHELL", originalShell)

			// Set test SHELL
			os.Setenv("SHELL", tt.shellEnv)

			// Create checker
			checker := &UnixChecker{executablePath: "/test/path"}
			shellType := checker.detectShell()

			assert.Equal(t, tt.expectedType, shellType, "Shell type should match expected")
			assert.Equal(t, tt.expectedName, shellType.String(), "Shell name should match expected")
		})
	}
}

// TestUnixChecker_GetProfilePathForShell tests profile path selection for different shells.
func TestUnixChecker_GetProfilePathForShell(t *testing.T) {
	homeDir := "/home/testuser"

	tests := []struct {
		name         string
		shellType    ShellType
		expectedPath string
	}{
		{
			name:         "bash profile",
			shellType:    ShellBash,
			expectedPath: filepath.Join(homeDir, ".bashrc"),
		},
		{
			name:         "zsh profile",
			shellType:    ShellZsh,
			expectedPath: filepath.Join(homeDir, ".zshrc"),
		},
		{
			name:         "fish profile",
			shellType:    ShellFish,
			expectedPath: filepath.Join(homeDir, ".config", "fish", "config.fish"),
		},
		{
			name:         "unknown shell defaults to .profile",
			shellType:    ShellUnknown,
			expectedPath: filepath.Join(homeDir, ".profile"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := &UnixChecker{executablePath: "/test/path"}
			profilePath := checker.getProfilePathForShell(tt.shellType, homeDir)
			assert.Equal(t, tt.expectedPath, profilePath, "Profile path should match expected")
		})
	}
}

// TestUnixChecker_GetProfilePathForShell_BashPreference tests bash profile file preference.
func TestUnixChecker_GetProfilePathForShell_BashPreference(t *testing.T) {
	// Create a temporary home directory
	tempHome := t.TempDir()

	checker := &UnixChecker{executablePath: "/test/path"}

	// Test 1: Neither .bashrc nor .bash_profile exists - should default to .bashrc
	profilePath := checker.getProfilePathForShell(ShellBash, tempHome)
	expectedPath := filepath.Join(tempHome, ".bashrc")
	assert.Equal(t, expectedPath, profilePath, "Should default to .bashrc when neither exists")

	// Test 2: Only .bashrc exists - should return .bashrc
	bashrcPath := filepath.Join(tempHome, ".bashrc")
	err := os.WriteFile(bashrcPath, []byte("# bashrc"), 0644)
	require.NoError(t, err)

	profilePath = checker.getProfilePathForShell(ShellBash, tempHome)
	assert.Equal(t, bashrcPath, profilePath, "Should return .bashrc when it exists")

	// Test 3: Both .bashrc and .bash_profile exist - should prefer .bashrc
	bashProfilePath := filepath.Join(tempHome, ".bash_profile")
	err = os.WriteFile(bashProfilePath, []byte("# bash_profile"), 0644)
	require.NoError(t, err)

	profilePath = checker.getProfilePathForShell(ShellBash, tempHome)
	assert.Equal(t, bashrcPath, profilePath, "Should prefer .bashrc when both exist")

	// Test 4: Only .bash_profile exists - should return .bash_profile
	os.Remove(bashrcPath)
	profilePath = checker.getProfilePathForShell(ShellBash, tempHome)
	assert.Equal(t, bashProfilePath, profilePath, "Should return .bash_profile when only it exists")
}

// TestUnixChecker_GenerateExportStatement tests export statement generation.
func TestUnixChecker_GenerateExportStatement(t *testing.T) {
	tests := []struct {
		name          string
		shellEnv      string
		execDir       string
		expectedParts []string
	}{
		{
			name:     "bash export statement",
			shellEnv: "/bin/bash",
			execDir:  "/usr/local/bin",
			expectedParts: []string{
				"# Added by GitSage",
				"export PATH=\"$PATH:/usr/local/bin\"",
			},
		},
		{
			name:     "zsh export statement",
			shellEnv: "/usr/bin/zsh",
			execDir:  "/opt/gitsage",
			expectedParts: []string{
				"# Added by GitSage",
				"export PATH=\"$PATH:/opt/gitsage\"",
			},
		},
		{
			name:     "fish export statement",
			shellEnv: "/usr/local/bin/fish",
			execDir:  "/home/user/bin",
			expectedParts: []string{
				"# Added by GitSage",
				"set -gx PATH $PATH /home/user/bin",
			},
		},
		{
			name:     "unknown shell uses bash format",
			shellEnv: "/bin/sh",
			execDir:  "/usr/bin",
			expectedParts: []string{
				"# Added by GitSage",
				"export PATH=\"$PATH:/usr/bin\"",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save original SHELL
			originalShell := os.Getenv("SHELL")
			defer os.Setenv("SHELL", originalShell)

			// Set test SHELL
			os.Setenv("SHELL", tt.shellEnv)

			checker := &UnixChecker{executablePath: "/test/path"}
			exportStmt := checker.GenerateExportStatement(tt.execDir)

			// Verify all expected parts are present
			for _, part := range tt.expectedParts {
				assert.Contains(t, exportStmt, part, "Export statement should contain: %s", part)
			}

			// Verify the statement starts with a newline (for proper formatting)
			assert.True(t, strings.HasPrefix(exportStmt, "\n"), "Export statement should start with newline")
		})
	}
}

// TestUnixChecker_AddToPath tests the full AddToPath flow.
func TestUnixChecker_AddToPath(t *testing.T) {
	// Create a temporary home directory
	tempHome := t.TempDir()

	// Save original HOME and SHELL
	originalHome := os.Getenv("HOME")
	originalShell := os.Getenv("SHELL")
	defer func() {
		os.Setenv("HOME", originalHome)
		os.Setenv("SHELL", originalShell)
	}()

	// Set test environment
	os.Setenv("HOME", tempHome)
	os.Setenv("SHELL", "/bin/bash")

	// Create a test executable path
	execPath := filepath.Join(tempHome, "test-bin", "gitsage")
	checker := &UnixChecker{executablePath: execPath}

	ctx := context.Background()
	result, err := checker.AddToPath(ctx)

	require.NoError(t, err, "AddToPath should not return an error")
	require.NotNil(t, result, "AddToPath should return a result")
	assert.True(t, result.Success, "AddToPath should succeed")
	assert.Equal(t, filepath.Dir(execPath), result.AddedPath, "AddedPath should match executable directory")
	assert.NotEmpty(t, result.ProfilePath, "ProfilePath should not be empty")
	assert.True(t, result.NeedsReload, "NeedsReload should be true")

	// Verify the profile file was created and contains the export statement
	profileContent, err := os.ReadFile(result.ProfilePath)
	require.NoError(t, err, "Should be able to read profile file")

	profileStr := string(profileContent)
	assert.Contains(t, profileStr, "# Added by GitSage", "Profile should contain GitSage comment")
	assert.Contains(t, profileStr, "export PATH=", "Profile should contain export statement")
	assert.Contains(t, profileStr, filepath.Dir(execPath), "Profile should contain executable directory")
}

// TestUnixChecker_AddToPath_FishShell tests AddToPath with Fish shell.
func TestUnixChecker_AddToPath_FishShell(t *testing.T) {
	// Create a temporary home directory
	tempHome := t.TempDir()

	// Save original HOME and SHELL
	originalHome := os.Getenv("HOME")
	originalShell := os.Getenv("SHELL")
	defer func() {
		os.Setenv("HOME", originalHome)
		os.Setenv("SHELL", originalShell)
	}()

	// Set test environment for Fish
	os.Setenv("HOME", tempHome)
	os.Setenv("SHELL", "/usr/local/bin/fish")

	// Create a test executable path
	execPath := filepath.Join(tempHome, "test-bin", "gitsage")
	checker := &UnixChecker{executablePath: execPath}

	ctx := context.Background()
	result, err := checker.AddToPath(ctx)

	require.NoError(t, err, "AddToPath should not return an error")
	require.NotNil(t, result, "AddToPath should return a result")
	assert.True(t, result.Success, "AddToPath should succeed")

	// Verify the Fish config directory was created
	fishConfigDir := filepath.Join(tempHome, ".config", "fish")
	info, err := os.Stat(fishConfigDir)
	require.NoError(t, err, "Fish config directory should exist")
	assert.True(t, info.IsDir(), "Fish config path should be a directory")

	// Verify the profile file contains Fish-specific syntax
	profileContent, err := os.ReadFile(result.ProfilePath)
	require.NoError(t, err, "Should be able to read Fish config file")

	profileStr := string(profileContent)
	assert.Contains(t, profileStr, "# Added by GitSage", "Fish config should contain GitSage comment")
	assert.Contains(t, profileStr, "set -gx PATH", "Fish config should contain set -gx PATH statement")
	assert.Contains(t, profileStr, filepath.Dir(execPath), "Fish config should contain executable directory")
}

// TestUnixChecker_AddToPath_PreservesPermissions tests that file permissions are preserved.
func TestUnixChecker_AddToPath_PreservesPermissions(t *testing.T) {
	// Create a temporary home directory
	tempHome := t.TempDir()

	// Save original HOME and SHELL
	originalHome := os.Getenv("HOME")
	originalShell := os.Getenv("SHELL")
	defer func() {
		os.Setenv("HOME", originalHome)
		os.Setenv("SHELL", originalShell)
	}()

	// Set test environment
	os.Setenv("HOME", tempHome)
	os.Setenv("SHELL", "/bin/bash")

	// Create a .bashrc file with specific permissions
	bashrcPath := filepath.Join(tempHome, ".bashrc")
	err := os.WriteFile(bashrcPath, []byte("# existing content\n"), 0600)
	require.NoError(t, err)

	// Verify initial permissions
	initialInfo, err := os.Stat(bashrcPath)
	require.NoError(t, err)
	initialMode := initialInfo.Mode()

	// Create a test executable path
	execPath := filepath.Join(tempHome, "test-bin", "gitsage")
	checker := &UnixChecker{executablePath: execPath}

	ctx := context.Background()
	result, err := checker.AddToPath(ctx)

	require.NoError(t, err, "AddToPath should not return an error")
	assert.True(t, result.Success, "AddToPath should succeed")

	// Verify permissions are preserved
	finalInfo, err := os.Stat(bashrcPath)
	require.NoError(t, err)
	finalMode := finalInfo.Mode()

	assert.Equal(t, initialMode, finalMode, "File permissions should be preserved")
}

// TestUnixChecker_GetShellType tests the exported GetShellType method.
func TestUnixChecker_GetShellType(t *testing.T) {
	// Save original SHELL
	originalShell := os.Getenv("SHELL")
	defer os.Setenv("SHELL", originalShell)

	tests := []struct {
		name         string
		shellEnv     string
		expectedType ShellType
	}{
		{"bash", "/bin/bash", ShellBash},
		{"zsh", "/usr/bin/zsh", ShellZsh},
		{"fish", "/usr/local/bin/fish", ShellFish},
		{"unknown", "/bin/sh", ShellUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("SHELL", tt.shellEnv)
			checker := &UnixChecker{executablePath: "/test/path"}
			shellType := checker.GetShellType()
			assert.Equal(t, tt.expectedType, shellType)
		})
	}
}
