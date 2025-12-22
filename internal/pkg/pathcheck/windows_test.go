//go:build windows

package pathcheck

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWindowsChecker_IsInPath(t *testing.T) {
	checker := &WindowsChecker{executablePath: "C:\\test\\gitsage.exe"}
	originalPath := os.Getenv("PATH")
	defer os.Setenv("PATH", originalPath)
	ctx := context.Background()

	os.Setenv("PATH", "C:\\Windows\\System32;C:\\Windows")
	inPath, err := checker.IsInPath(ctx)
	require.NoError(t, err)
	assert.False(t, inPath)

	os.Setenv("PATH", "C:\\test;C:\\Windows\\System32")
	inPath, err = checker.IsInPath(ctx)
	require.NoError(t, err)
	assert.True(t, inPath)
}

func TestWindowsChecker_GetExecutableDir(t *testing.T) {
	checker := &WindowsChecker{executablePath: "C:\\Program Files\\GitSage\\gitsage.exe"}
	dir, err := checker.GetExecutableDir()
	require.NoError(t, err)
	assert.Equal(t, "C:\\Program Files\\GitSage", dir)
}

func TestWindowsChecker_GetShellProfile(t *testing.T) {
	checker := &WindowsChecker{executablePath: "C:\\test\\gitsage.exe"}
	profile, err := checker.GetShellProfile()
	require.NoError(t, err)
	assert.Empty(t, profile)
}

func TestWindowsChecker_GetOS(t *testing.T) {
	checker := &WindowsChecker{executablePath: "C:\\test\\gitsage.exe"}
	osName := checker.GetOS()
	assert.Equal(t, "windows", osName)
}

func TestWindowsChecker_IsInPath_CaseInsensitive(t *testing.T) {
	checker := &WindowsChecker{executablePath: "C:\\Test\\gitsage.exe"}
	originalPath := os.Getenv("PATH")
	defer os.Setenv("PATH", originalPath)
	ctx := context.Background()

	os.Setenv("PATH", "c:\\test;C:\\Windows")
	inPath, err := checker.IsInPath(ctx)
	require.NoError(t, err)
	assert.True(t, inPath, "Should match case-insensitively")

	os.Setenv("PATH", "C:\\TEST;C:\\Windows")
	inPath, err = checker.IsInPath(ctx)
	require.NoError(t, err)
	assert.True(t, inPath, "Should match case-insensitively")
}

func TestWindowsChecker_IsInPath_EmptyPath(t *testing.T) {
	checker := &WindowsChecker{executablePath: "C:\\test\\gitsage.exe"}
	originalPath := os.Getenv("PATH")
	defer os.Setenv("PATH", originalPath)
	ctx := context.Background()

	os.Setenv("PATH", "")
	inPath, err := checker.IsInPath(ctx)
	require.NoError(t, err)
	assert.False(t, inPath, "Should return false for empty PATH")
}

func TestWindowsChecker_AddToPath_AlreadyInPath(t *testing.T) {
	checker := &WindowsChecker{executablePath: "C:\\test\\gitsage.exe"}
	originalPath := os.Getenv("PATH")
	defer os.Setenv("PATH", originalPath)

	os.Setenv("PATH", "C:\\test;C:\\Windows\\System32")

	ctx := context.Background()
	result, err := checker.AddToPath(ctx)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Success)
	assert.Equal(t, "C:\\test", result.AddedPath)
	assert.False(t, result.NeedsReload)
}
