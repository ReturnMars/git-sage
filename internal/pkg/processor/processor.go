// Package processor provides diff processing functionality for GitSage.
package processor

import (
	"context"
	"fmt"
	"strings"

	"github.com/gitsage/gitsage/internal/pkg/git"
)

// Default thresholds for diff processing.
const (
	DefaultDiffSizeThreshold = 10 * 1024  // 10KB - triggers chunking
	DefaultMaxChunkSize      = 100 * 1024 // 100KB - max size per chunk
	DefaultMaxConcurrent     = 3          // Max concurrent AI calls
)

// ChunkingStrategy defines how diffs should be chunked.
type ChunkingStrategy int

const (
	ChunkingByFile ChunkingStrategy = iota
	ChunkingBySize
	ChunkingBySummary
)

// ChunkGroup represents a group of chunks for parallel processing.
type ChunkGroup struct {
	Chunks    []git.DiffChunk
	TotalSize int
}

// ProcessedDiff contains the result of diff processing.
type ProcessedDiff struct {
	Chunks           []git.DiffChunk
	Summary          string
	TotalSize        int
	RequiresChunking bool
	ChunkGroups      []ChunkGroup
}

// DiffProcessor defines the interface for diff processing.
type DiffProcessor interface {
	Process(ctx context.Context, chunks []git.DiffChunk) (*ProcessedDiff, error)
}

// ProcessorConfig holds configuration for the diff processor.
type ProcessorConfig struct {
	DiffSizeThreshold int // Size in bytes that triggers chunking
	MaxChunkSize      int // Maximum size per chunk in bytes
	MaxConcurrent     int // Maximum concurrent AI calls for chunk processing
}

// DefaultProcessor implements the DiffProcessor interface.
type DefaultProcessor struct {
	config ProcessorConfig
}

// NewProcessor creates a new DefaultProcessor with default configuration.
func NewProcessor() *DefaultProcessor {
	return &DefaultProcessor{
		config: ProcessorConfig{
			DiffSizeThreshold: DefaultDiffSizeThreshold,
			MaxChunkSize:      DefaultMaxChunkSize,
			MaxConcurrent:     DefaultMaxConcurrent,
		},
	}
}

// NewProcessorWithConfig creates a new DefaultProcessor with custom configuration.
func NewProcessorWithConfig(config ProcessorConfig) *DefaultProcessor {
	// Apply defaults for zero values
	if config.DiffSizeThreshold <= 0 {
		config.DiffSizeThreshold = DefaultDiffSizeThreshold
	}
	if config.MaxChunkSize <= 0 {
		config.MaxChunkSize = DefaultMaxChunkSize
	}
	if config.MaxConcurrent <= 0 {
		config.MaxConcurrent = DefaultMaxConcurrent
	}
	return &DefaultProcessor{config: config}
}

// Process processes the diff chunks by filtering lock files, calculating size,
// and applying chunking strategy if needed.
func (p *DefaultProcessor) Process(ctx context.Context, chunks []git.DiffChunk) (*ProcessedDiff, error) {
	// Step 1: Filter out lock files
	filteredChunks := p.filterLockFiles(chunks)

	// Step 2: Calculate total size
	totalSize := p.calculateTotalSize(filteredChunks)

	// Step 3: Determine if chunking is required
	requiresChunking := totalSize > p.config.DiffSizeThreshold

	result := &ProcessedDiff{
		Chunks:           filteredChunks,
		TotalSize:        totalSize,
		RequiresChunking: requiresChunking,
	}

	// Step 4: Apply chunking strategy if needed
	if requiresChunking {
		// Process large files - replace content with summary for files exceeding max chunk size
		result.Chunks = p.processLargeFiles(filteredChunks)

		// Group chunks for parallel processing
		result.ChunkGroups = p.groupChunks(result.Chunks)

		// Generate overall summary
		result.Summary = p.generateSummary(result.Chunks)
	}

	return result, nil
}

// filterLockFiles removes lock files from the chunks.
func (p *DefaultProcessor) filterLockFiles(chunks []git.DiffChunk) []git.DiffChunk {
	filtered := make([]git.DiffChunk, 0, len(chunks))
	for _, chunk := range chunks {
		if !chunk.IsLockFile {
			filtered = append(filtered, chunk)
		}
	}
	return filtered
}

// calculateTotalSize calculates the total size of all chunk contents in bytes.
func (p *DefaultProcessor) calculateTotalSize(chunks []git.DiffChunk) int {
	total := 0
	for _, chunk := range chunks {
		total += len(chunk.Content)
	}
	return total
}

// processLargeFiles replaces content with summary for files exceeding max chunk size.
func (p *DefaultProcessor) processLargeFiles(chunks []git.DiffChunk) []git.DiffChunk {
	processed := make([]git.DiffChunk, len(chunks))
	for i, chunk := range chunks {
		processed[i] = chunk
		if len(chunk.Content) > p.config.MaxChunkSize {
			// Replace content with a summary for very large files
			processed[i].Content = p.generateFileSummary(&chunk)
		}
	}
	return processed
}

// generateFileSummary creates a summary for a single file when it's too large.
func (p *DefaultProcessor) generateFileSummary(chunk *git.DiffChunk) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("File: %s\n", chunk.FilePath))
	sb.WriteString(fmt.Sprintf("Change Type: %s\n", chunk.ChangeType.String()))
	sb.WriteString(fmt.Sprintf("Additions: +%d\n", chunk.Additions))
	sb.WriteString(fmt.Sprintf("Deletions: -%d\n", chunk.Deletions))

	if chunk.IsBinary {
		sb.WriteString("Note: Binary file (content not shown)\n")
	} else {
		sb.WriteString(fmt.Sprintf("Note: Large file (%d bytes) - showing statistics only\n", len(chunk.Content)))
	}

	if chunk.OldPath != "" {
		sb.WriteString(fmt.Sprintf("Renamed from: %s\n", chunk.OldPath))
	}

	return sb.String()
}

// groupChunks groups chunks for parallel processing, respecting max concurrent limit.
func (p *DefaultProcessor) groupChunks(chunks []git.DiffChunk) []ChunkGroup {
	if len(chunks) == 0 {
		return nil
	}

	// Simple strategy: distribute chunks evenly across groups
	numGroups := p.config.MaxConcurrent
	if len(chunks) < numGroups {
		numGroups = len(chunks)
	}

	groups := make([]ChunkGroup, numGroups)
	for i := range groups {
		groups[i] = ChunkGroup{
			Chunks: make([]git.DiffChunk, 0),
		}
	}

	// Round-robin distribution
	for i, chunk := range chunks {
		groupIdx := i % numGroups
		groups[groupIdx].Chunks = append(groups[groupIdx].Chunks, chunk)
		groups[groupIdx].TotalSize += len(chunk.Content)
	}

	// Remove empty groups
	result := make([]ChunkGroup, 0, numGroups)
	for _, group := range groups {
		if len(group.Chunks) > 0 {
			result = append(result, group)
		}
	}

	return result
}

// generateSummary creates an overall summary of all changes.
func (p *DefaultProcessor) generateSummary(chunks []git.DiffChunk) string {
	if len(chunks) == 0 {
		return "No changes"
	}

	var sb strings.Builder
	sb.WriteString("Summary of changes:\n")

	totalAdditions := 0
	totalDeletions := 0

	for _, chunk := range chunks {
		changeSymbol := "M" // Modified
		switch chunk.ChangeType {
		case git.ChangeTypeAdded:
			changeSymbol = "A"
		case git.ChangeTypeDeleted:
			changeSymbol = "D"
		case git.ChangeTypeRenamed:
			changeSymbol = "R"
		}

		sb.WriteString(fmt.Sprintf("  [%s] %s (+%d/-%d)\n",
			changeSymbol, chunk.FilePath, chunk.Additions, chunk.Deletions))

		if chunk.OldPath != "" {
			sb.WriteString(fmt.Sprintf("      (renamed from %s)\n", chunk.OldPath))
		}

		totalAdditions += chunk.Additions
		totalDeletions += chunk.Deletions
	}

	sb.WriteString(fmt.Sprintf("\nTotal: %d files, +%d additions, -%d deletions\n",
		len(chunks), totalAdditions, totalDeletions))

	return sb.String()
}
