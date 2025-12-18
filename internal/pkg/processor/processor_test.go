// Package processor provides diff processing functionality for GitSage.
package processor

import (
	"context"
	"strings"
	"testing"

	"github.com/gitsage/gitsage/internal/pkg/git"
)

func TestFilterLockFiles(t *testing.T) {
	p := NewProcessor()

	chunks := []git.DiffChunk{
		{FilePath: "main.go", IsLockFile: false},
		{FilePath: "go.sum", IsLockFile: true},
		{FilePath: "package-lock.json", IsLockFile: true},
		{FilePath: "src/app.ts", IsLockFile: false},
	}

	ctx := context.Background()
	result, err := p.Process(ctx, chunks)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	// Should have filtered out lock files
	if len(result.Chunks) != 2 {
		t.Errorf("Expected 2 chunks after filtering, got %d", len(result.Chunks))
	}

	// Verify no lock files remain
	for _, chunk := range result.Chunks {
		if chunk.IsLockFile {
			t.Errorf("Lock file %s should have been filtered", chunk.FilePath)
		}
	}
}

func TestCalculateTotalSize(t *testing.T) {
	p := NewProcessor()

	chunks := []git.DiffChunk{
		{FilePath: "file1.go", Content: "content1"},  // 8 bytes
		{FilePath: "file2.go", Content: "content22"}, // 9 bytes
	}

	ctx := context.Background()
	result, err := p.Process(ctx, chunks)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	expectedSize := 17
	if result.TotalSize != expectedSize {
		t.Errorf("Expected total size %d, got %d", expectedSize, result.TotalSize)
	}
}

func TestChunkingThreshold(t *testing.T) {
	// Test with small diff - should not require chunking
	t.Run("small diff no chunking", func(t *testing.T) {
		p := NewProcessor()
		chunks := []git.DiffChunk{
			{FilePath: "small.go", Content: "small content"},
		}

		ctx := context.Background()
		result, err := p.Process(ctx, chunks)
		if err != nil {
			t.Fatalf("Process failed: %v", err)
		}

		if result.RequiresChunking {
			t.Error("Small diff should not require chunking")
		}
	})

	// Test with large diff - should require chunking
	t.Run("large diff requires chunking", func(t *testing.T) {
		config := ProcessorConfig{
			DiffSizeThreshold: 100, // 100 bytes threshold for testing
			MaxChunkSize:      50,
			MaxConcurrent:     3,
		}
		p := NewProcessorWithConfig(config)

		// Create content larger than threshold
		largeContent := strings.Repeat("x", 150)
		chunks := []git.DiffChunk{
			{FilePath: "large.go", Content: largeContent},
		}

		ctx := context.Background()
		result, err := p.Process(ctx, chunks)
		if err != nil {
			t.Fatalf("Process failed: %v", err)
		}

		if !result.RequiresChunking {
			t.Error("Large diff should require chunking")
		}
	})
}

func TestGroupChunks(t *testing.T) {
	config := ProcessorConfig{
		DiffSizeThreshold: 10, // Low threshold to trigger chunking
		MaxChunkSize:      1000,
		MaxConcurrent:     2,
	}
	p := NewProcessorWithConfig(config)

	chunks := []git.DiffChunk{
		{FilePath: "file1.go", Content: strings.Repeat("a", 20)},
		{FilePath: "file2.go", Content: strings.Repeat("b", 20)},
		{FilePath: "file3.go", Content: strings.Repeat("c", 20)},
		{FilePath: "file4.go", Content: strings.Repeat("d", 20)},
	}

	ctx := context.Background()
	result, err := p.Process(ctx, chunks)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	// Should have 2 groups (MaxConcurrent = 2)
	if len(result.ChunkGroups) != 2 {
		t.Errorf("Expected 2 chunk groups, got %d", len(result.ChunkGroups))
	}

	// Each group should have 2 chunks (4 chunks / 2 groups)
	for i, group := range result.ChunkGroups {
		if len(group.Chunks) != 2 {
			t.Errorf("Group %d: expected 2 chunks, got %d", i, len(group.Chunks))
		}
	}
}

func TestProcessLargeFiles(t *testing.T) {
	config := ProcessorConfig{
		DiffSizeThreshold: 10,
		MaxChunkSize:      50, // Small max chunk size for testing
		MaxConcurrent:     3,
	}
	p := NewProcessorWithConfig(config)

	largeContent := strings.Repeat("x", 100) // Exceeds MaxChunkSize
	chunks := []git.DiffChunk{
		{
			FilePath:   "large.go",
			Content:    largeContent,
			ChangeType: git.ChangeTypeModified,
			Additions:  50,
			Deletions:  10,
		},
	}

	ctx := context.Background()
	result, err := p.Process(ctx, chunks)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	// Content should be replaced with summary
	if len(result.Chunks) != 1 {
		t.Fatalf("Expected 1 chunk, got %d", len(result.Chunks))
	}

	// Summary should contain file info
	content := result.Chunks[0].Content
	if !strings.Contains(content, "large.go") {
		t.Error("Summary should contain file path")
	}
	if !strings.Contains(content, "+50") {
		t.Error("Summary should contain additions count")
	}
	if !strings.Contains(content, "-10") {
		t.Error("Summary should contain deletions count")
	}
}

func TestGenerateSummary(t *testing.T) {
	config := ProcessorConfig{
		DiffSizeThreshold: 10, // Low threshold to trigger summary generation
		MaxChunkSize:      1000,
		MaxConcurrent:     3,
	}
	p := NewProcessorWithConfig(config)

	chunks := []git.DiffChunk{
		{
			FilePath:   "added.go",
			ChangeType: git.ChangeTypeAdded,
			Additions:  100,
			Deletions:  0,
			Content:    strings.Repeat("a", 20),
		},
		{
			FilePath:   "modified.go",
			ChangeType: git.ChangeTypeModified,
			Additions:  50,
			Deletions:  30,
			Content:    strings.Repeat("m", 20),
		},
		{
			FilePath:   "deleted.go",
			ChangeType: git.ChangeTypeDeleted,
			Additions:  0,
			Deletions:  75,
			Content:    strings.Repeat("d", 20),
		},
	}

	ctx := context.Background()
	result, err := p.Process(ctx, chunks)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	// Summary should be generated for large diffs
	if result.Summary == "" {
		t.Error("Summary should be generated for large diffs")
	}

	// Summary should contain change indicators
	if !strings.Contains(result.Summary, "[A]") {
		t.Error("Summary should contain [A] for added files")
	}
	if !strings.Contains(result.Summary, "[M]") {
		t.Error("Summary should contain [M] for modified files")
	}
	if !strings.Contains(result.Summary, "[D]") {
		t.Error("Summary should contain [D] for deleted files")
	}

	// Summary should contain totals
	if !strings.Contains(result.Summary, "3 files") {
		t.Error("Summary should contain total file count")
	}
}

func TestEmptyChunks(t *testing.T) {
	p := NewProcessor()

	ctx := context.Background()
	result, err := p.Process(ctx, []git.DiffChunk{})
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	if len(result.Chunks) != 0 {
		t.Errorf("Expected 0 chunks, got %d", len(result.Chunks))
	}

	if result.TotalSize != 0 {
		t.Errorf("Expected total size 0, got %d", result.TotalSize)
	}

	if result.RequiresChunking {
		t.Error("Empty diff should not require chunking")
	}
}

func TestRenamedFileInSummary(t *testing.T) {
	config := ProcessorConfig{
		DiffSizeThreshold: 10,
		MaxChunkSize:      1000,
		MaxConcurrent:     3,
	}
	p := NewProcessorWithConfig(config)

	chunks := []git.DiffChunk{
		{
			FilePath:   "new_name.go",
			OldPath:    "old_name.go",
			ChangeType: git.ChangeTypeRenamed,
			Additions:  5,
			Deletions:  2,
			Content:    strings.Repeat("r", 20),
		},
	}

	ctx := context.Background()
	result, err := p.Process(ctx, chunks)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	// Summary should contain rename indicator
	if !strings.Contains(result.Summary, "[R]") {
		t.Error("Summary should contain [R] for renamed files")
	}

	// Summary should mention old path
	if !strings.Contains(result.Summary, "old_name.go") {
		t.Error("Summary should contain old file path for renames")
	}
}

func TestBinaryFileInSummary(t *testing.T) {
	config := ProcessorConfig{
		DiffSizeThreshold: 10,
		MaxChunkSize:      20, // Small to trigger summary
		MaxConcurrent:     3,
	}
	p := NewProcessorWithConfig(config)

	chunks := []git.DiffChunk{
		{
			FilePath:   "image.png",
			ChangeType: git.ChangeTypeAdded,
			IsBinary:   true,
			Content:    strings.Repeat("b", 50), // Exceeds MaxChunkSize
		},
	}

	ctx := context.Background()
	result, err := p.Process(ctx, chunks)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	// Summary should mention binary file
	if !strings.Contains(result.Chunks[0].Content, "Binary file") {
		t.Error("Summary should indicate binary file")
	}
}
