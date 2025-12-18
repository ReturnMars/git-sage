// Package git provides Git operations for GitSage.
package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// setupTestRepo creates a temporary git repository for testing.
func setupTestRepo(t *testing.T) string {
	t.Helper()

	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "gitsage-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	// Initialize git repo
	runGit(t, tmpDir, "init")
	runGit(t, tmpDir, "config", "user.email", "test@example.com")
	runGit(t, tmpDir, "config", "user.name", "Test User")

	return tmpDir
}

// runGit runs a git command in the specified directory.
func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\nOutput: %s", args, err, output)
	}
	return string(output)
}

// writeFile creates a file with the given content.
func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	// Create parent directories if needed
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("failed to create directories: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
}

func TestHasStagedChanges_NoChanges(t *testing.T) {
	tmpDir := setupTestRepo(t)
	defer os.RemoveAll(tmpDir)

	// Create initial commit
	writeFile(t, tmpDir, "README.md", "# Test")
	runGit(t, tmpDir, "add", ".")
	runGit(t, tmpDir, "commit", "-m", "initial commit")

	client := NewClientWithWorkDir(tmpDir)
	hasChanges, err := client.HasStagedChanges(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hasChanges {
		t.Error("expected no staged changes")
	}
}

func TestHasStagedChanges_WithChanges(t *testing.T) {
	tmpDir := setupTestRepo(t)
	defer os.RemoveAll(tmpDir)

	// Create initial commit
	writeFile(t, tmpDir, "README.md", "# Test")
	runGit(t, tmpDir, "add", ".")
	runGit(t, tmpDir, "commit", "-m", "initial commit")

	// Make and stage a change
	writeFile(t, tmpDir, "README.md", "# Test\n\nUpdated content")
	runGit(t, tmpDir, "add", ".")

	client := NewClientWithWorkDir(tmpDir)
	hasChanges, err := client.HasStagedChanges(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasChanges {
		t.Error("expected staged changes")
	}
}

func TestGetStagedDiff_ModifiedFile(t *testing.T) {
	tmpDir := setupTestRepo(t)
	defer os.RemoveAll(tmpDir)

	// Create initial commit
	writeFile(t, tmpDir, "main.go", "package main\n\nfunc main() {}\n")
	runGit(t, tmpDir, "add", ".")
	runGit(t, tmpDir, "commit", "-m", "initial commit")

	// Modify and stage
	writeFile(t, tmpDir, "main.go", "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"hello\")\n}\n")
	runGit(t, tmpDir, "add", ".")

	client := NewClientWithWorkDir(tmpDir)
	chunks, err := client.GetStagedDiff(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}

	chunk := chunks[0]
	if chunk.FilePath != "main.go" {
		t.Errorf("expected file path 'main.go', got '%s'", chunk.FilePath)
	}
	if chunk.ChangeType != ChangeTypeModified {
		t.Errorf("expected change type Modified, got %v", chunk.ChangeType)
	}
	if chunk.Additions == 0 {
		t.Error("expected additions > 0")
	}
}

func TestGetStagedDiff_NewFile(t *testing.T) {
	tmpDir := setupTestRepo(t)
	defer os.RemoveAll(tmpDir)

	// Create initial commit
	writeFile(t, tmpDir, "README.md", "# Test")
	runGit(t, tmpDir, "add", ".")
	runGit(t, tmpDir, "commit", "-m", "initial commit")

	// Add new file
	writeFile(t, tmpDir, "new_file.go", "package main\n")
	runGit(t, tmpDir, "add", ".")

	client := NewClientWithWorkDir(tmpDir)
	chunks, err := client.GetStagedDiff(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}

	chunk := chunks[0]
	if chunk.FilePath != "new_file.go" {
		t.Errorf("expected file path 'new_file.go', got '%s'", chunk.FilePath)
	}
	if chunk.ChangeType != ChangeTypeAdded {
		t.Errorf("expected change type Added, got %v", chunk.ChangeType)
	}
}

func TestGetStagedDiff_DeletedFile(t *testing.T) {
	tmpDir := setupTestRepo(t)
	defer os.RemoveAll(tmpDir)

	// Create initial commit with a file
	writeFile(t, tmpDir, "to_delete.txt", "content")
	runGit(t, tmpDir, "add", ".")
	runGit(t, tmpDir, "commit", "-m", "initial commit")

	// Delete and stage
	os.Remove(filepath.Join(tmpDir, "to_delete.txt"))
	runGit(t, tmpDir, "add", ".")

	client := NewClientWithWorkDir(tmpDir)
	chunks, err := client.GetStagedDiff(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}

	chunk := chunks[0]
	if chunk.FilePath != "to_delete.txt" {
		t.Errorf("expected file path 'to_delete.txt', got '%s'", chunk.FilePath)
	}
	if chunk.ChangeType != ChangeTypeDeleted {
		t.Errorf("expected change type Deleted, got %v", chunk.ChangeType)
	}
}

func TestGetStagedDiff_NoStagedChanges(t *testing.T) {
	tmpDir := setupTestRepo(t)
	defer os.RemoveAll(tmpDir)

	// Create initial commit
	writeFile(t, tmpDir, "README.md", "# Test")
	runGit(t, tmpDir, "add", ".")
	runGit(t, tmpDir, "commit", "-m", "initial commit")

	client := NewClientWithWorkDir(tmpDir)
	_, err := client.GetStagedDiff(context.Background())
	if err == nil {
		t.Error("expected error for no staged changes")
	}
}

func TestIsLockFile(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"package-lock.json", true},
		{"yarn.lock", true},
		{"pnpm-lock.yaml", true},
		{"go.sum", true},
		{"Cargo.lock", true},
		{"Gemfile.lock", true},
		{"composer.lock", true},
		{"poetry.lock", true},
		{"Pipfile.lock", true},
		{"some-other.lock", true},
		{"node_modules/package-lock.json", true},
		{"main.go", false},
		{"package.json", false},
		{"go.mod", false},
		{"lockfile.txt", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := isLockFile(tt.path)
			if result != tt.expected {
				t.Errorf("isLockFile(%q) = %v, want %v", tt.path, result, tt.expected)
			}
		})
	}
}

func TestGetStagedDiff_LockFileDetection(t *testing.T) {
	tmpDir := setupTestRepo(t)
	defer os.RemoveAll(tmpDir)

	// Create initial commit
	writeFile(t, tmpDir, "README.md", "# Test")
	runGit(t, tmpDir, "add", ".")
	runGit(t, tmpDir, "commit", "-m", "initial commit")

	// Add lock file and regular file
	writeFile(t, tmpDir, "package-lock.json", `{"name": "test"}`)
	writeFile(t, tmpDir, "main.js", "console.log('hello');")
	runGit(t, tmpDir, "add", ".")

	client := NewClientWithWorkDir(tmpDir)
	chunks, err := client.GetStagedDiff(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks, got %d", len(chunks))
	}

	// Find the lock file chunk
	var lockChunk, regularChunk *DiffChunk
	for i := range chunks {
		if chunks[i].FilePath == "package-lock.json" {
			lockChunk = &chunks[i]
		} else if chunks[i].FilePath == "main.js" {
			regularChunk = &chunks[i]
		}
	}

	if lockChunk == nil {
		t.Fatal("lock file chunk not found")
	}
	if !lockChunk.IsLockFile {
		t.Error("expected package-lock.json to be marked as lock file")
	}

	if regularChunk == nil {
		t.Fatal("regular file chunk not found")
	}
	if regularChunk.IsLockFile {
		t.Error("expected main.js to not be marked as lock file")
	}
}

func TestCommit(t *testing.T) {
	tmpDir := setupTestRepo(t)
	defer os.RemoveAll(tmpDir)

	// Create initial commit
	writeFile(t, tmpDir, "README.md", "# Test")
	runGit(t, tmpDir, "add", ".")
	runGit(t, tmpDir, "commit", "-m", "initial commit")

	// Make and stage a change
	writeFile(t, tmpDir, "README.md", "# Test\n\nUpdated")
	runGit(t, tmpDir, "add", ".")

	client := NewClientWithWorkDir(tmpDir)
	err := client.Commit(context.Background(), "feat: update readme")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify commit was made
	output := runGit(t, tmpDir, "log", "--oneline", "-1")
	if !contains(output, "feat: update readme") {
		t.Errorf("commit message not found in log: %s", output)
	}
}

func TestGetDiffStats(t *testing.T) {
	tmpDir := setupTestRepo(t)
	defer os.RemoveAll(tmpDir)

	// Create initial commit
	writeFile(t, tmpDir, "README.md", "# Test")
	runGit(t, tmpDir, "add", ".")
	runGit(t, tmpDir, "commit", "-m", "initial commit")

	// Add multiple files
	writeFile(t, tmpDir, "file1.go", "package main\n\nfunc one() {}\n")
	writeFile(t, tmpDir, "file2.go", "package main\n\nfunc two() {}\n")
	runGit(t, tmpDir, "add", ".")

	client := NewClientWithWorkDir(tmpDir)
	stats, err := client.GetDiffStats(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if stats.TotalFiles != 2 {
		t.Errorf("expected 2 files, got %d", stats.TotalFiles)
	}
	if stats.TotalAdditions == 0 {
		t.Error("expected additions > 0")
	}
	if len(stats.Chunks) != 2 {
		t.Errorf("expected 2 chunks, got %d", len(stats.Chunks))
	}
}

func TestExtractNewPath(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"old.txt => new.txt", "new.txt"},
		{"{old => new}/file.txt", "new/file.txt"},
		{"dir/{old.txt => new.txt}", "dir/new.txt"},
		{"src/{old => new}/main.go", "src/new/main.go"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := extractNewPath(tt.input)
			if result != tt.expected {
				t.Errorf("extractNewPath(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
