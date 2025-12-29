package diff

import (
	"strings"

	"github.com/gitsage/gitsage/internal/core/domain"
)

// Default thresholds for diff processing.
const (
	DefaultDiffSizeThreshold = 10 * 1024  // 10KB
	DefaultMaxChunkSize      = 100 * 1024 // 100KB
)

// ProcessingResult contains the result of diff processing.
type ProcessingResult struct {
	FilteredDiff     *domain.Diff
	RequiresChunking bool
	TotalSize        int
}

// Processor defines the interface for diff processing logic.
type Processor interface {
	Process(diff *domain.Diff) *ProcessingResult
	Chunk(diff *domain.Diff, maxSize int) []*domain.Diff
}

// DefaultProcessor implements the Processor interface.
type DefaultProcessor struct {
	threshold int
}

// NewProcessor creates a new DefaultProcessor.
func NewProcessor(threshold int) *DefaultProcessor {
	if threshold <= 0 {
		threshold = DefaultDiffSizeThreshold
	}
	return &DefaultProcessor{threshold: threshold}
}

// Process filters lock files and calculates if chunking is needed.
func (p *DefaultProcessor) Process(diff *domain.Diff) *ProcessingResult {
	filteredFiles, size := p.filterAndCalculate(diff.Files)

	// Recalculate stats based on filtered files
	stats := diff.Stats
	stats.FilesChanged = len(filteredFiles)
	// We might need to recalculate insertions/deletions if precise stats are needed after filtering
	// For now, we keep original stats or we could re-sum them if DiffFile had int stats.
	// domain.DiffFile has Aditions/Deletions, so let's re-sum.
	stats.Insertions = 0
	stats.Deletions = 0
	for _, f := range filteredFiles {
		stats.Insertions += f.Additions
		stats.Deletions += f.Deletions
	}

	filteredDiff := &domain.Diff{
		Files: filteredFiles,
		Stats: stats,
	}

	return &ProcessingResult{
		FilteredDiff:     filteredDiff,
		RequiresChunking: size > p.threshold,
		TotalSize:        size,
	}
}

// Chunk splits the diff into smaller chunks based on maxSize.
// It supports splitting large files by hunks.
func (p *DefaultProcessor) Chunk(diff *domain.Diff, maxSize int) []*domain.Diff {
	var chunks []*domain.Diff
	var currentFiles []domain.DiffFile
	currentSize := 0

	if maxSize <= 0 {
		maxSize = DefaultMaxChunkSize
	}

	for _, file := range diff.Files {
		fileLen := len(file.Content)

		// Case 1: File fits in current chunk
		if currentSize+fileLen <= maxSize {
			currentFiles = append(currentFiles, file)
			currentSize += fileLen
			continue
		}

		// Case 2: File is huge (larger than maxSize itself) -> Needs Hunk splitting
		if fileLen > maxSize {
			// If we have pending files, flush them first
			if len(currentFiles) > 0 {
				chunks = append(chunks, &domain.Diff{Files: currentFiles})
				currentFiles = nil
				currentSize = 0
			}

			// Split this large file
			fileChunks := p.splitFileByHunks(file, maxSize)
			for _, fc := range fileChunks {
				// Each hunk-chunk is wrapped in a Diff object
				chunks = append(chunks, &domain.Diff{Files: []domain.DiffFile{fc}})
			}
			continue
		}

		// Case 3: File fits in a new chunk but not current one
		// Flush current chunk
		if len(currentFiles) > 0 {
			chunks = append(chunks, &domain.Diff{Files: currentFiles})
		}
		// Start new chunk with this file
		currentFiles = []domain.DiffFile{file}
		currentSize = fileLen
	}

	// Add remaining files
	if len(currentFiles) > 0 {
		chunks = append(chunks, &domain.Diff{Files: currentFiles})
	}

	return chunks
}

func (p *DefaultProcessor) splitFileByHunks(file domain.DiffFile, maxSize int) []domain.DiffFile {
	var parts []domain.DiffFile
	lines := strings.Split(file.Content, "\n")

	// If no hunks or weird format, just return original (fallback)
	// Or implement simple line splitting. Let's do simple line splitting for robustness if Hunk detection fails.

	// Header usually is first 2-4 lines (diff --git, index, ---, +++)
	headerEnd := 0
	for i, line := range lines {
		if strings.HasPrefix(line, "@@") {
			headerEnd = i
			break
		}
	}

	header := strings.Join(lines[:headerEnd], "\n")
	if header != "" {
		header += "\n"
	}

	var currentBody strings.Builder
	currentLen := len(header)

	// Iterate through hunks
	for i := headerEnd; i < len(lines); i++ {
		line := lines[i]
		lineLen := len(line) + 1 // +1 for newline

		// check if new hunk starts
		isHunkHeader := strings.HasPrefix(line, "@@")

		if isHunkHeader && currentLen > len(header) && (currentLen+lineLen > maxSize) {
			// Flush current part
			parts = append(parts, domain.DiffFile{
				FilePath:    file.FilePath,
				OldFilePath: file.OldFilePath,
				ChangeType:  file.ChangeType,
				Content:     header + currentBody.String(),
				Additions:   file.Additions, // Note: stats are duplicated, acceptable for prompt context
				Deletions:   file.Deletions,
			})
			currentBody.Reset()
			currentLen = len(header)
		}

		currentBody.WriteString(line + "\n")
		currentLen += lineLen
	}

	// Flush last part
	if currentBody.Len() > 0 {
		parts = append(parts, domain.DiffFile{
			FilePath:    file.FilePath,
			OldFilePath: file.OldFilePath,
			ChangeType:  file.ChangeType,
			Content:     header + currentBody.String(),
			Additions:   file.Additions,
			Deletions:   file.Deletions,
		})
	}

	if len(parts) == 0 {
		return []domain.DiffFile{file}
	}

	return parts
}

func (p *DefaultProcessor) filterAndCalculate(files []domain.DiffFile) ([]domain.DiffFile, int) {
	var filtered []domain.DiffFile
	totalSize := 0

	for _, file := range files {
		if p.isLockFile(file.FilePath) {
			continue
		}
		filtered = append(filtered, file)
		totalSize += len(file.Content)
	}
	return filtered, totalSize
}

func (p *DefaultProcessor) isLockFile(path string) bool {
	lower := strings.ToLower(path)
	return strings.HasSuffix(lower, "package-lock.json") ||
		strings.HasSuffix(lower, "yarn.lock") ||
		strings.HasSuffix(lower, "pnpm-lock.yaml") ||
		strings.HasSuffix(lower, "go.sum") ||
		strings.HasSuffix(lower, "cargo.lock") ||
		strings.HasSuffix(lower, "gemfile.lock") ||
		strings.HasSuffix(lower, "composer.lock") ||
		strings.HasSuffix(lower, "mix.lock") ||
		strings.HasSuffix(lower, "poetry.lock") ||
		strings.HasSuffix(lower, "flute.lock") // Just in case
}
