package history

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFileManager_Save(t *testing.T) {
	// Create temp directory for test
	tmpDir := t.TempDir()
	historyFile := filepath.Join(tmpDir, "history.json")

	mgr := NewFileManager(historyFile, 1000)

	entry := &Entry{
		Message:     "feat: add new feature",
		DiffSummary: "2 files changed",
		Provider:    "openai",
		Model:       "gpt-4o-mini",
		Committed:   true,
	}

	err := mgr.Save(entry)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify entry was saved with generated ID and timestamp
	if entry.ID == "" {
		t.Error("Expected ID to be generated")
	}
	if entry.Timestamp.IsZero() {
		t.Error("Expected Timestamp to be set")
	}

	// Verify file exists
	if _, err := os.Stat(historyFile); os.IsNotExist(err) {
		t.Error("History file was not created")
	}
}

func TestFileManager_List(t *testing.T) {
	tmpDir := t.TempDir()
	historyFile := filepath.Join(tmpDir, "history.json")

	mgr := NewFileManager(historyFile, 1000)

	// Save multiple entries
	for i := 0; i < 5; i++ {
		entry := &Entry{
			Message:     "feat: feature " + string(rune('A'+i)),
			DiffSummary: "1 file changed",
			Provider:    "openai",
			Model:       "gpt-4o-mini",
			Committed:   true,
		}
		if err := mgr.Save(entry); err != nil {
			t.Fatalf("Save failed: %v", err)
		}
	}

	// List all entries
	entries, err := mgr.List(0)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(entries) != 5 {
		t.Errorf("Expected 5 entries, got %d", len(entries))
	}

	// List with limit
	entries, err = mgr.List(3)
	if err != nil {
		t.Fatalf("List with limit failed: %v", err)
	}
	if len(entries) != 3 {
		t.Errorf("Expected 3 entries, got %d", len(entries))
	}
}

func TestFileManager_List_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	historyFile := filepath.Join(tmpDir, "nonexistent", "history.json")

	mgr := NewFileManager(historyFile, 1000)

	// List from non-existent file should return empty slice
	entries, err := mgr.List(10)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("Expected 0 entries, got %d", len(entries))
	}
}

func TestFileManager_Clear(t *testing.T) {
	tmpDir := t.TempDir()
	historyFile := filepath.Join(tmpDir, "history.json")

	mgr := NewFileManager(historyFile, 1000)

	// Save an entry
	entry := &Entry{
		Message:     "feat: test",
		DiffSummary: "1 file changed",
		Provider:    "openai",
		Model:       "gpt-4o-mini",
		Committed:   true,
	}
	if err := mgr.Save(entry); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Clear history
	if err := mgr.Clear(); err != nil {
		t.Fatalf("Clear failed: %v", err)
	}

	// Verify history is empty
	entries, err := mgr.List(0)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("Expected 0 entries after clear, got %d", len(entries))
	}
}

func TestFileManager_Rotation(t *testing.T) {
	tmpDir := t.TempDir()
	historyFile := filepath.Join(tmpDir, "history.json")

	// Set max entries to 5 for testing
	mgr := NewFileManager(historyFile, 5)

	// Save 10 entries
	for i := 0; i < 10; i++ {
		entry := &Entry{
			Message:     "feat: feature " + string(rune('0'+i)),
			DiffSummary: "1 file changed",
			Provider:    "openai",
			Model:       "gpt-4o-mini",
			Committed:   true,
		}
		if err := mgr.Save(entry); err != nil {
			t.Fatalf("Save failed: %v", err)
		}
	}

	// Should only have 5 entries (the most recent ones)
	entries, err := mgr.List(0)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(entries) != 5 {
		t.Errorf("Expected 5 entries after rotation, got %d", len(entries))
	}

	// Verify we have the most recent entries (5-9)
	for i, entry := range entries {
		expected := "feat: feature " + string(rune('0'+5+i))
		if entry.Message != expected {
			t.Errorf("Entry %d: expected message %q, got %q", i, expected, entry.Message)
		}
	}
}

func TestFileManager_PreservesExistingData(t *testing.T) {
	tmpDir := t.TempDir()
	historyFile := filepath.Join(tmpDir, "history.json")

	mgr := NewFileManager(historyFile, 1000)

	// Save first entry with specific data
	entry1 := &Entry{
		ID:          "test-id-1",
		Timestamp:   time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
		Message:     "feat: first feature",
		DiffSummary: "1 file changed",
		Provider:    "openai",
		Model:       "gpt-4o-mini",
		Committed:   true,
	}
	if err := mgr.Save(entry1); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Save second entry
	entry2 := &Entry{
		Message:     "fix: bug fix",
		DiffSummary: "2 files changed",
		Provider:    "deepseek",
		Model:       "deepseek-chat",
		Committed:   false,
	}
	if err := mgr.Save(entry2); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// List and verify both entries are preserved
	entries, err := mgr.List(0)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("Expected 2 entries, got %d", len(entries))
	}

	// Verify first entry data is preserved
	if entries[0].ID != "test-id-1" {
		t.Errorf("Expected ID 'test-id-1', got %q", entries[0].ID)
	}
	if entries[0].Message != "feat: first feature" {
		t.Errorf("Expected message 'feat: first feature', got %q", entries[0].Message)
	}
	if entries[0].Provider != "openai" {
		t.Errorf("Expected provider 'openai', got %q", entries[0].Provider)
	}
}

func TestFileManager_FilePermissions(t *testing.T) {
	// Skip on Windows as file permissions work differently
	if os.PathSeparator == '\\' {
		t.Skip("Skipping file permissions test on Windows")
	}

	tmpDir := t.TempDir()
	historyFile := filepath.Join(tmpDir, "history.json")

	mgr := NewFileManager(historyFile, 1000)

	entry := &Entry{
		Message:     "feat: test",
		DiffSummary: "1 file changed",
		Provider:    "openai",
		Model:       "gpt-4o-mini",
		Committed:   true,
	}
	if err := mgr.Save(entry); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Check file permissions (should be 0600)
	info, err := os.Stat(historyFile)
	if err != nil {
		t.Fatalf("Stat failed: %v", err)
	}

	perm := info.Mode().Perm()
	if perm != 0600 {
		t.Errorf("Expected file permissions 0600, got %o", perm)
	}
}

func TestNewFileManager_DefaultMaxEntries(t *testing.T) {
	mgr := NewFileManager("/tmp/test.json", 0)
	if mgr.maxEntries != DefaultMaxEntries {
		t.Errorf("Expected default max entries %d, got %d", DefaultMaxEntries, mgr.maxEntries)
	}

	mgr = NewFileManager("/tmp/test.json", -1)
	if mgr.maxEntries != DefaultMaxEntries {
		t.Errorf("Expected default max entries %d, got %d", DefaultMaxEntries, mgr.maxEntries)
	}
}
