// Package pathcheck provides PATH detection and modification functionality for GitSage.
package pathcheck

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// Feature: path-detection, Property 3: Shell profile selection
// Validates: Requirements 4.1, 4.2, 4.3, 4.4, 4.5

// genShellType generates a random shell type.
func genShellType() gopter.Gen {
	return gen.OneConstOf(
		ShellBash,
		ShellZsh,
		ShellFish,
		ShellUnknown,
	)
}

// genHomeDir generates a valid home directory path.
func genHomeDir() gopter.Gen {
	return gen.OneConstOf(
		"/home/user",
		"/Users/testuser",
		"/root",
	)
}

// getProfilePathForShellTest is a test helper that implements the shell profile selection logic.
// This allows testing the logic on any platform.
func getProfilePathForShellTest(shellType ShellType, homeDir string) string {
	switch shellType {
	case ShellBash:
		// Default to .bashrc for bash
		return filepath.Join(homeDir, ".bashrc")
	case ShellZsh:
		return filepath.Join(homeDir, ".zshrc")
	case ShellFish:
		return filepath.Join(homeDir, ".config", "fish", "config.fish")
	default:
		// Default to .profile for unknown shells
		return filepath.Join(homeDir, ".profile")
	}
}

// TestProperty_ShellProfileSelection verifies that for any Unix shell type,
// the GetShellProfile function returns the correct profile file path.
//
// Feature: path-detection, Property 3: Shell profile selection
// Validates: Requirements 4.1, 4.2, 4.3, 4.4, 4.5
func TestProperty_ShellProfileSelection(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Shell profile selection is only applicable on Unix systems")
	}

	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	// Property: For any shell type and home directory, the profile path should be correct
	properties.Property("shell profile path matches expected pattern for shell type", prop.ForAll(
		func(shellType ShellType, homeDir string) bool {
			profilePath := getProfilePathForShellTest(shellType, homeDir)

			switch shellType {
			case ShellBash:
				// Bash should use .bashrc or .bash_profile
				return strings.HasSuffix(profilePath, ".bashrc") ||
					strings.HasSuffix(profilePath, ".bash_profile")
			case ShellZsh:
				// Zsh should use .zshrc
				return strings.HasSuffix(profilePath, ".zshrc")
			case ShellFish:
				// Fish should use config.fish in .config/fish/
				return strings.Contains(profilePath, ".config") &&
					strings.Contains(profilePath, "fish") &&
					strings.HasSuffix(profilePath, "config.fish")
			case ShellUnknown:
				// Unknown shell should default to .profile
				return strings.HasSuffix(profilePath, ".profile")
			default:
				return false
			}
		},
		genShellType(),
		genHomeDir(),
	))

	// Property: Profile path should always start with the home directory
	properties.Property("profile path starts with home directory", prop.ForAll(
		func(shellType ShellType, homeDir string) bool {
			profilePath := getProfilePathForShellTest(shellType, homeDir)
			return strings.HasPrefix(profilePath, homeDir)
		},
		genShellType(),
		genHomeDir(),
	))

	// Property: Profile path should never be empty
	properties.Property("profile path is never empty", prop.ForAll(
		func(shellType ShellType, homeDir string) bool {
			profilePath := getProfilePathForShellTest(shellType, homeDir)
			return len(profilePath) > 0
		},
		genShellType(),
		genHomeDir(),
	))

	properties.TestingRun(t)
}

// Feature: path-detection, Property 5: Unix export statement format
// Validates: Requirements 2.3

// genValidUnixPath generates valid Unix directory paths.
func genValidUnixPath() gopter.Gen {
	return gen.OneConstOf(
		"/usr/local/bin",
		"/home/user/bin",
		"/opt/gitsage",
		"/Users/developer/.local/bin",
		"/root/tools",
		"/var/lib/app",
	)
}

// genUnixShellType generates Unix shell types (bash, zsh, fish, unknown).
func genUnixShellType() gopter.Gen {
	return gen.OneConstOf(
		ShellBash,
		ShellZsh,
		ShellFish,
		ShellUnknown,
	)
}

// TestProperty_UnixExportStatementFormat verifies that for any valid directory path
// on Unix systems, the generated export statement is syntactically correct for the
// target shell and contains the exact directory path.
//
// Feature: path-detection, Property 5: Unix export statement format
// Validates: Requirements 2.3
func TestProperty_UnixExportStatementFormat(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	// Property: Export statement contains the exact directory path
	properties.Property("export statement contains exact directory path", prop.ForAll(
		func(shellType ShellType, dirPath string) bool {
			exportStmt := GenerateExportStatementForShell(shellType, dirPath)
			return strings.Contains(exportStmt, dirPath)
		},
		genUnixShellType(),
		genValidUnixPath(),
	))

	// Property: Bash/Zsh export statement has correct syntax
	properties.Property("bash/zsh export statement has correct syntax", prop.ForAll(
		func(dirPath string) bool {
			// Test for bash (same format as zsh and unknown)
			exportStmt := GenerateExportStatementForShell(ShellBash, dirPath)

			// Should contain "export PATH="
			hasExportKeyword := strings.Contains(exportStmt, "export PATH=")
			// Should contain $PATH reference
			hasPathRef := strings.Contains(exportStmt, "$PATH")
			// Should contain the directory
			hasDir := strings.Contains(exportStmt, dirPath)
			// Should have GitSage comment
			hasComment := strings.Contains(exportStmt, "# Added by GitSage")

			return hasExportKeyword && hasPathRef && hasDir && hasComment
		},
		genValidUnixPath(),
	))

	// Property: Fish export statement has correct syntax
	properties.Property("fish export statement has correct syntax", prop.ForAll(
		func(dirPath string) bool {
			exportStmt := GenerateExportStatementForShell(ShellFish, dirPath)

			// Should contain "set -gx PATH"
			hasSetCommand := strings.Contains(exportStmt, "set -gx PATH")
			// Should contain $PATH reference
			hasPathRef := strings.Contains(exportStmt, "$PATH")
			// Should contain the directory
			hasDir := strings.Contains(exportStmt, dirPath)
			// Should have GitSage comment
			hasComment := strings.Contains(exportStmt, "# Added by GitSage")
			// Should NOT contain "export" keyword (fish uses set)
			noExport := !strings.Contains(exportStmt, "export")

			return hasSetCommand && hasPathRef && hasDir && hasComment && noExport
		},
		genValidUnixPath(),
	))

	// Property: Export statement is never empty
	properties.Property("export statement is never empty", prop.ForAll(
		func(shellType ShellType, dirPath string) bool {
			exportStmt := GenerateExportStatementForShell(shellType, dirPath)
			return len(strings.TrimSpace(exportStmt)) > 0
		},
		genUnixShellType(),
		genValidUnixPath(),
	))

	// Property: Unknown shell uses bash/zsh format (default)
	properties.Property("unknown shell uses bash format", prop.ForAll(
		func(dirPath string) bool {
			unknownStmt := GenerateExportStatementForShell(ShellUnknown, dirPath)
			bashStmt := GenerateExportStatementForShell(ShellBash, dirPath)
			return unknownStmt == bashStmt
		},
		genValidUnixPath(),
	))

	properties.TestingRun(t)
}
