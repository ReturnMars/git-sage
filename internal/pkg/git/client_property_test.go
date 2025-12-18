// Package git provides Git operations for GitSage.
package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// Feature: gitsage, Property 1: Staged changes retrieval
// Validates: Requirements 1.1
//
// Property: For any git repository with staged changes, retrieving the diff
// should return all staged file modifications.

// StagedFile represents a file to be staged for testing.
type StagedFile struct {
	Name    string
	Content string
	IsNew   bool // true = new file, false = modify existing
}

// windowsReservedNames contains Windows reserved device names that cannot be used as file names.
var windowsReservedNames = map[string]bool{
	"con": true, "prn": true, "aux": true, "nul": true,
	"com1": true, "com2": true, "com3": true, "com4": true,
	"com5": true, "com6": true, "com7": true, "com8": true, "com9": true,
	"lpt1": true, "lpt2": true, "lpt3": true, "lpt4": true,
	"lpt5": true, "lpt6": true, "lpt7": true, "lpt8": true, "lpt9": true,
}

// isWindowsReservedName checks if a name (without extension) is a Windows reserved name.
func isWindowsReservedName(name string) bool {
	// Extract base name without extension
	baseName := strings.ToLower(name)
	if idx := strings.LastIndex(baseName, "."); idx > 0 {
		baseName = baseName[:idx]
	}
	return windowsReservedNames[baseName]
}

// genValidFileName generates valid file names for testing.
// Avoids special characters, Windows reserved names, and ensures reasonable length.
func genValidFileName() gopter.Gen {
	return gen.IntRange(4, 15).FlatMap(func(length interface{}) gopter.Gen {
		n := length.(int)
		return gen.SliceOfN(n, gen.Rune()).Map(func(runes []rune) string {
			for i := range runes {
				// Map to lowercase letters a-z
				runes[i] = 'a' + (runes[i] % 26)
			}
			name := string(runes)
			// Avoid Windows reserved names by prefixing with "file_"
			if isWindowsReservedName(name) {
				name = "file_" + name
			}
			return name + ".txt"
		})
	}, reflect.TypeOf(""))
}

// genFileContent generates valid file content for testing.
func genFileContent() gopter.Gen {
	return gen.IntRange(10, 100).FlatMap(func(length interface{}) gopter.Gen {
		n := length.(int)
		return gen.SliceOfN(n, gen.AlphaNumChar()).Map(func(chars []rune) string {
			// Add some newlines to make it more realistic
			result := string(chars)
			// Insert newlines every ~20 chars
			var sb strings.Builder
			for i, c := range result {
				sb.WriteRune(c)
				if i > 0 && i%20 == 0 {
					sb.WriteRune('\n')
				}
			}
			return sb.String()
		})
	}, reflect.TypeOf(""))
}

// genStagedFile generates a StagedFile for testing.
func genStagedFile() gopter.Gen {
	return gopter.CombineGens(
		genValidFileName(),
		genFileContent(),
		gen.Bool(),
	).Map(func(values []interface{}) StagedFile {
		return StagedFile{
			Name:    values[0].(string),
			Content: values[1].(string),
			IsNew:   values[2].(bool),
		}
	})
}

// genStagedFiles generates a slice of 1-5 staged files.
func genStagedFiles() gopter.Gen {
	return gen.IntRange(1, 5).FlatMap(func(count interface{}) gopter.Gen {
		n := count.(int)
		return gen.SliceOfN(n, genStagedFile()).Map(func(files []StagedFile) []StagedFile {
			// Ensure unique file names
			seen := make(map[string]bool)
			unique := make([]StagedFile, 0, len(files))
			for _, f := range files {
				if !seen[f.Name] {
					seen[f.Name] = true
					unique = append(unique, f)
				}
			}
			return unique
		})
	}, reflect.TypeOf([]StagedFile{}))
}

// setupPropertyTestRepo creates a temporary git repository for property testing.
func setupPropertyTestRepo(t *testing.T) (string, func()) {
	t.Helper()

	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "gitsage-property-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	cleanup := func() {
		os.RemoveAll(tmpDir)
	}

	// Initialize git repo
	if err := runGitCmd(tmpDir, "init"); err != nil {
		cleanup()
		t.Fatalf("failed to init git repo: %v", err)
	}
	if err := runGitCmd(tmpDir, "config", "user.email", "test@example.com"); err != nil {
		cleanup()
		t.Fatalf("failed to set git email: %v", err)
	}
	if err := runGitCmd(tmpDir, "config", "user.name", "Test User"); err != nil {
		cleanup()
		t.Fatalf("failed to set git name: %v", err)
	}

	// Create initial commit
	initialFile := filepath.Join(tmpDir, "README.md")
	if err := os.WriteFile(initialFile, []byte("# Test Repository\n"), 0644); err != nil {
		cleanup()
		t.Fatalf("failed to write initial file: %v", err)
	}
	if err := runGitCmd(tmpDir, "add", "."); err != nil {
		cleanup()
		t.Fatalf("failed to add initial file: %v", err)
	}
	if err := runGitCmd(tmpDir, "commit", "-m", "initial commit"); err != nil {
		cleanup()
		t.Fatalf("failed to create initial commit: %v", err)
	}

	return tmpDir, cleanup
}

// runGitCmd runs a git command in the specified directory.
func runGitCmd(dir string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return &exec.ExitError{Stderr: output}
	}
	return nil
}

// writeTestFile creates a file with the given content.
func writeTestFile(dir, name, content string) error {
	path := filepath.Join(dir, name)
	// Create parent directories if needed
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0644)
}

func TestStagedChangesRetrieval_Property(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	parameters.Rng.Seed(42)

	properties := gopter.NewProperties(parameters)

	// Property 1: All staged files should be returned in the diff
	properties.Property("all staged files are returned in diff", prop.ForAll(
		func(files []StagedFile) bool {
			if len(files) == 0 {
				return true // Skip empty file lists
			}

			// Setup test repo
			tmpDir, cleanup := setupPropertyTestRepo(t)
			defer cleanup()

			// Stage the files
			stagedFileNames := make(map[string]bool)
			for _, f := range files {
				if f.IsNew {
					// Create new file
					if err := writeTestFile(tmpDir, f.Name, f.Content); err != nil {
						t.Logf("Failed to write file %s: %v", f.Name, err)
						return false
					}
				} else {
					// Create file, commit it, then modify
					if err := writeTestFile(tmpDir, f.Name, "original content\n"); err != nil {
						t.Logf("Failed to write original file %s: %v", f.Name, err)
						return false
					}
					if err := runGitCmd(tmpDir, "add", f.Name); err != nil {
						t.Logf("Failed to add original file %s: %v", f.Name, err)
						return false
					}
					if err := runGitCmd(tmpDir, "commit", "-m", "add "+f.Name); err != nil {
						t.Logf("Failed to commit original file %s: %v", f.Name, err)
						return false
					}
					// Now modify
					if err := writeTestFile(tmpDir, f.Name, f.Content); err != nil {
						t.Logf("Failed to modify file %s: %v", f.Name, err)
						return false
					}
				}
				stagedFileNames[f.Name] = true
			}

			// Stage all changes
			if err := runGitCmd(tmpDir, "add", "."); err != nil {
				t.Logf("Failed to stage changes: %v", err)
				return false
			}

			// Get staged diff using our client
			client := NewClientWithWorkDir(tmpDir)
			chunks, err := client.GetStagedDiff(context.Background())
			if err != nil {
				t.Logf("Failed to get staged diff: %v", err)
				return false
			}

			// Verify all staged files are in the diff
			retrievedFiles := make(map[string]bool)
			for _, chunk := range chunks {
				retrievedFiles[chunk.FilePath] = true
			}

			// Check that all staged files are retrieved
			for name := range stagedFileNames {
				if !retrievedFiles[name] {
					t.Logf("Staged file %s not found in diff", name)
					return false
				}
			}

			return true
		},
		genStagedFiles(),
	))

	// Property 2: Number of chunks equals number of staged files
	properties.Property("chunk count equals staged file count", prop.ForAll(
		func(files []StagedFile) bool {
			if len(files) == 0 {
				return true // Skip empty file lists
			}

			// Setup test repo
			tmpDir, cleanup := setupPropertyTestRepo(t)
			defer cleanup()

			// Create and stage new files only (simpler case)
			for _, f := range files {
				if err := writeTestFile(tmpDir, f.Name, f.Content); err != nil {
					t.Logf("Failed to write file %s: %v", f.Name, err)
					return false
				}
			}

			// Stage all changes
			if err := runGitCmd(tmpDir, "add", "."); err != nil {
				t.Logf("Failed to stage changes: %v", err)
				return false
			}

			// Get staged diff
			client := NewClientWithWorkDir(tmpDir)
			chunks, err := client.GetStagedDiff(context.Background())
			if err != nil {
				t.Logf("Failed to get staged diff: %v", err)
				return false
			}

			// Number of chunks should equal number of staged files
			if len(chunks) != len(files) {
				t.Logf("Expected %d chunks, got %d", len(files), len(chunks))
				return false
			}

			return true
		},
		genStagedFiles(),
	))

	// Property 3: Each chunk has correct change type for new files
	properties.Property("new files have ChangeTypeAdded", prop.ForAll(
		func(files []StagedFile) bool {
			if len(files) == 0 {
				return true
			}

			// Setup test repo
			tmpDir, cleanup := setupPropertyTestRepo(t)
			defer cleanup()

			// Create and stage new files
			for _, f := range files {
				if err := writeTestFile(tmpDir, f.Name, f.Content); err != nil {
					t.Logf("Failed to write file %s: %v", f.Name, err)
					return false
				}
			}

			// Stage all changes
			if err := runGitCmd(tmpDir, "add", "."); err != nil {
				t.Logf("Failed to stage changes: %v", err)
				return false
			}

			// Get staged diff
			client := NewClientWithWorkDir(tmpDir)
			chunks, err := client.GetStagedDiff(context.Background())
			if err != nil {
				t.Logf("Failed to get staged diff: %v", err)
				return false
			}

			// All chunks should be ChangeTypeAdded
			for _, chunk := range chunks {
				if chunk.ChangeType != ChangeTypeAdded {
					t.Logf("Expected ChangeTypeAdded for %s, got %v", chunk.FilePath, chunk.ChangeType)
					return false
				}
			}

			return true
		},
		genStagedFiles(),
	))

	// Property 4: Each chunk contains non-empty content
	properties.Property("chunks contain non-empty content", prop.ForAll(
		func(files []StagedFile) bool {
			if len(files) == 0 {
				return true
			}

			// Setup test repo
			tmpDir, cleanup := setupPropertyTestRepo(t)
			defer cleanup()

			// Create and stage new files
			for _, f := range files {
				if err := writeTestFile(tmpDir, f.Name, f.Content); err != nil {
					t.Logf("Failed to write file %s: %v", f.Name, err)
					return false
				}
			}

			// Stage all changes
			if err := runGitCmd(tmpDir, "add", "."); err != nil {
				t.Logf("Failed to stage changes: %v", err)
				return false
			}

			// Get staged diff
			client := NewClientWithWorkDir(tmpDir)
			chunks, err := client.GetStagedDiff(context.Background())
			if err != nil {
				t.Logf("Failed to get staged diff: %v", err)
				return false
			}

			// All chunks should have non-empty content
			for _, chunk := range chunks {
				if chunk.Content == "" {
					t.Logf("Chunk for %s has empty content", chunk.FilePath)
					return false
				}
			}

			return true
		},
		genStagedFiles(),
	))

	// Property 5: Additions count is positive for new files with content
	properties.Property("new files have positive additions count", prop.ForAll(
		func(files []StagedFile) bool {
			if len(files) == 0 {
				return true
			}

			// Setup test repo
			tmpDir, cleanup := setupPropertyTestRepo(t)
			defer cleanup()

			// Create and stage new files
			for _, f := range files {
				if err := writeTestFile(tmpDir, f.Name, f.Content); err != nil {
					t.Logf("Failed to write file %s: %v", f.Name, err)
					return false
				}
			}

			// Stage all changes
			if err := runGitCmd(tmpDir, "add", "."); err != nil {
				t.Logf("Failed to stage changes: %v", err)
				return false
			}

			// Get staged diff
			client := NewClientWithWorkDir(tmpDir)
			chunks, err := client.GetStagedDiff(context.Background())
			if err != nil {
				t.Logf("Failed to get staged diff: %v", err)
				return false
			}

			// All chunks should have positive additions
			for _, chunk := range chunks {
				if chunk.Additions <= 0 {
					t.Logf("Chunk for %s has non-positive additions: %d", chunk.FilePath, chunk.Additions)
					return false
				}
			}

			return true
		},
		genStagedFiles(),
	))

	properties.TestingRun(t)
}
