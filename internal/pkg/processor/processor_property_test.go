// Package processor provides diff processing functionality for GitSage.
package processor

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/gitsage/gitsage/internal/pkg/git"
	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// Feature: gitsage, Property 14: Diff chunking threshold
// Validates: Requirements 5.2
//
// Property: For any git diff with total size exceeding the configured threshold,
// the system should split it into per-file chunks.

// Feature: gitsage, Property 13: Lock file exclusion
// Validates: Requirements 5.1
//
// Property: For any git diff containing lock files (package-lock.json, go.sum,
// yarn.lock, Cargo.lock, pnpm-lock.yaml), the processed diff sent to AI should
// not include these files.

// lockFileNames contains all known lock file names for testing.
var lockFileNames = []string{
	"package-lock.json",
	"yarn.lock",
	"pnpm-lock.yaml",
	"go.sum",
	"Cargo.lock",
	"Gemfile.lock",
	"composer.lock",
	"poetry.lock",
	"Pipfile.lock",
}

// regularFileExtensions contains common non-lock file extensions.
var regularFileExtensions = []string{
	".go", ".js", ".ts", ".py", ".java", ".rs", ".rb", ".php",
	".md", ".txt", ".json", ".yaml", ".yml", ".toml", ".xml",
	".css", ".html", ".vue", ".jsx", ".tsx",
}

// genLockFileName generates a lock file name.
func genLockFileName() gopter.Gen {
	return gen.IntRange(0, len(lockFileNames)-1).Map(func(idx int) string {
		return lockFileNames[idx]
	})
}

// genRegularFileName generates a regular (non-lock) file name.
func genRegularFileName() gopter.Gen {
	return gopter.CombineGens(
		gen.IntRange(4, 12),
		gen.IntRange(0, len(regularFileExtensions)-1),
	).Map(func(values []interface{}) string {
		length := values[0].(int)
		extIdx := values[1].(int)

		// Generate a simple alphanumeric name
		name := make([]rune, length)
		for i := range name {
			name[i] = 'a' + rune(i%26)
		}
		return string(name) + regularFileExtensions[extIdx]
	})
}

// genDiffContent generates realistic diff content.
func genDiffContent() gopter.Gen {
	return gen.IntRange(50, 500).Map(func(length int) string {
		var sb strings.Builder
		sb.WriteString("@@ -1,10 +1,15 @@\n")
		for i := 0; i < length/20; i++ {
			sb.WriteString("+added line ")
			sb.WriteString(strings.Repeat("x", 10))
			sb.WriteString("\n")
		}
		return sb.String()
	})
}

// genLockFileChunk generates a DiffChunk for a lock file.
func genLockFileChunk() gopter.Gen {
	return gopter.CombineGens(
		genLockFileName(),
		genDiffContent(),
		gen.IntRange(1, 100),
		gen.IntRange(0, 50),
	).Map(func(values []interface{}) git.DiffChunk {
		return git.DiffChunk{
			FilePath:   values[0].(string),
			Content:    values[1].(string),
			Additions:  values[2].(int),
			Deletions:  values[3].(int),
			ChangeType: git.ChangeTypeModified,
			IsLockFile: true, // Mark as lock file
		}
	})
}

// genRegularFileChunk generates a DiffChunk for a regular file.
func genRegularFileChunk() gopter.Gen {
	return gopter.CombineGens(
		genRegularFileName(),
		genDiffContent(),
		gen.IntRange(1, 100),
		gen.IntRange(0, 50),
	).Map(func(values []interface{}) git.DiffChunk {
		return git.DiffChunk{
			FilePath:   values[0].(string),
			Content:    values[1].(string),
			Additions:  values[2].(int),
			Deletions:  values[3].(int),
			ChangeType: git.ChangeTypeModified,
			IsLockFile: false,
		}
	})
}

// genMixedChunks generates a slice containing both lock files and regular files.
func genMixedChunks() gopter.Gen {
	return gopter.CombineGens(
		gen.IntRange(1, 3), // Number of lock files
		gen.IntRange(1, 5), // Number of regular files
	).FlatMap(func(values interface{}) gopter.Gen {
		counts := values.([]interface{})
		numLockFiles := counts[0].(int)
		numRegularFiles := counts[1].(int)

		return gopter.CombineGens(
			gen.SliceOfN(numLockFiles, genLockFileChunk()),
			gen.SliceOfN(numRegularFiles, genRegularFileChunk()),
		).Map(func(chunks []interface{}) []git.DiffChunk {
			lockChunks := chunks[0].([]git.DiffChunk)
			regularChunks := chunks[1].([]git.DiffChunk)

			// Combine and interleave
			result := make([]git.DiffChunk, 0, len(lockChunks)+len(regularChunks))
			result = append(result, lockChunks...)
			result = append(result, regularChunks...)
			return result
		})
	}, reflect.TypeOf([]git.DiffChunk{}))
}

// genOnlyLockFileChunks generates a slice containing only lock files.
func genOnlyLockFileChunks() gopter.Gen {
	return gen.IntRange(1, 5).FlatMap(func(count interface{}) gopter.Gen {
		n := count.(int)
		return gen.SliceOfN(n, genLockFileChunk())
	}, reflect.TypeOf([]git.DiffChunk{}))
}

// genOnlyRegularFileChunks generates a slice containing only regular files.
func genOnlyRegularFileChunks() gopter.Gen {
	return gen.IntRange(1, 5).FlatMap(func(count interface{}) gopter.Gen {
		n := count.(int)
		return gen.SliceOfN(n, genRegularFileChunk())
	}, reflect.TypeOf([]git.DiffChunk{}))
}

func TestLockFileExclusion_Property(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	parameters.Rng.Seed(42)

	properties := gopter.NewProperties(parameters)

	processor := NewProcessor()

	// Property 1: Lock files are always excluded from processed output
	// Feature: gitsage, Property 13: Lock file exclusion
	// Validates: Requirements 5.1
	properties.Property("lock files are excluded from processed diff", prop.ForAll(
		func(chunks []git.DiffChunk) bool {
			if len(chunks) == 0 {
				return true
			}

			result, err := processor.Process(context.Background(), chunks)
			if err != nil {
				t.Logf("Process error: %v", err)
				return false
			}

			// Verify no lock files in the result
			for _, chunk := range result.Chunks {
				if chunk.IsLockFile {
					t.Logf("Lock file %s found in processed result", chunk.FilePath)
					return false
				}
			}

			return true
		},
		genMixedChunks(),
	))

	// Property 2: All regular files are preserved after filtering
	properties.Property("regular files are preserved after filtering", prop.ForAll(
		func(chunks []git.DiffChunk) bool {
			if len(chunks) == 0 {
				return true
			}

			// Count regular files in input
			regularCount := 0
			for _, chunk := range chunks {
				if !chunk.IsLockFile {
					regularCount++
				}
			}

			result, err := processor.Process(context.Background(), chunks)
			if err != nil {
				t.Logf("Process error: %v", err)
				return false
			}

			// Result should have exactly the same number of regular files
			if len(result.Chunks) != regularCount {
				t.Logf("Expected %d regular files, got %d", regularCount, len(result.Chunks))
				return false
			}

			return true
		},
		genMixedChunks(),
	))

	// Property 3: Processing only lock files results in empty output
	properties.Property("only lock files results in empty processed diff", prop.ForAll(
		func(chunks []git.DiffChunk) bool {
			if len(chunks) == 0 {
				return true
			}

			result, err := processor.Process(context.Background(), chunks)
			if err != nil {
				t.Logf("Process error: %v", err)
				return false
			}

			// All lock files should be filtered out
			if len(result.Chunks) != 0 {
				t.Logf("Expected 0 chunks after filtering lock files, got %d", len(result.Chunks))
				return false
			}

			return true
		},
		genOnlyLockFileChunks(),
	))

	// Property 4: Processing only regular files preserves all files
	properties.Property("only regular files are all preserved", prop.ForAll(
		func(chunks []git.DiffChunk) bool {
			if len(chunks) == 0 {
				return true
			}

			result, err := processor.Process(context.Background(), chunks)
			if err != nil {
				t.Logf("Process error: %v", err)
				return false
			}

			// All regular files should be preserved
			if len(result.Chunks) != len(chunks) {
				t.Logf("Expected %d chunks, got %d", len(chunks), len(result.Chunks))
				return false
			}

			return true
		},
		genOnlyRegularFileChunks(),
	))

	// Property 5: File paths of lock files never appear in result
	properties.Property("lock file paths never appear in result", prop.ForAll(
		func(chunks []git.DiffChunk) bool {
			if len(chunks) == 0 {
				return true
			}

			// Collect lock file paths from input
			lockFilePaths := make(map[string]bool)
			for _, chunk := range chunks {
				if chunk.IsLockFile {
					lockFilePaths[chunk.FilePath] = true
				}
			}

			result, err := processor.Process(context.Background(), chunks)
			if err != nil {
				t.Logf("Process error: %v", err)
				return false
			}

			// Verify none of the lock file paths appear in result
			for _, chunk := range result.Chunks {
				if lockFilePaths[chunk.FilePath] {
					t.Logf("Lock file path %s found in result", chunk.FilePath)
					return false
				}
			}

			return true
		},
		genMixedChunks(),
	))

	properties.TestingRun(t)
}

// Feature: gitsage, Property 14: Diff chunking threshold
// Validates: Requirements 5.2
//
// Property: For any git diff with total size exceeding the configured threshold,
// the system should split it into per-file chunks.

// genContentOfSize generates content of approximately the specified size in bytes.
func genContentOfSize(minSize, maxSize int) gopter.Gen {
	return gen.IntRange(minSize, maxSize).Map(func(size int) string {
		var sb strings.Builder
		sb.WriteString("@@ -1,10 +1,15 @@\n")
		// Each line is approximately 20 chars
		lineCount := size / 20
		for i := 0; i < lineCount; i++ {
			sb.WriteString("+added line content\n")
		}
		return sb.String()
	})
}

// genChunkWithSize generates a DiffChunk with content of approximately the specified size.
func genChunkWithSize(minSize, maxSize int) gopter.Gen {
	return gopter.CombineGens(
		genRegularFileName(),
		genContentOfSize(minSize, maxSize),
		gen.IntRange(1, 100),
		gen.IntRange(0, 50),
	).Map(func(values []interface{}) git.DiffChunk {
		return git.DiffChunk{
			FilePath:   values[0].(string),
			Content:    values[1].(string),
			Additions:  values[2].(int),
			Deletions:  values[3].(int),
			ChangeType: git.ChangeTypeModified,
			IsLockFile: false,
		}
	})
}

// genChunksExceedingThreshold generates chunks whose total size exceeds the threshold.
func genChunksExceedingThreshold(threshold int) gopter.Gen {
	// Generate 2-5 chunks, each with size that ensures total exceeds threshold
	return gen.IntRange(2, 5).FlatMap(func(count interface{}) gopter.Gen {
		n := count.(int)
		// Each chunk should have at least threshold/n + some extra to ensure we exceed
		minPerChunk := (threshold / n) + 100
		maxPerChunk := minPerChunk + 500
		return gen.SliceOfN(n, genChunkWithSize(minPerChunk, maxPerChunk))
	}, reflect.TypeOf([]git.DiffChunk{}))
}

// genChunksBelowThreshold generates chunks whose total size is below the threshold.
func genChunksBelowThreshold(threshold int) gopter.Gen {
	// Generate 1-3 small chunks that stay well below threshold
	return gen.IntRange(1, 3).FlatMap(func(count interface{}) gopter.Gen {
		n := count.(int)
		// Each chunk should be small enough that total stays below threshold
		maxPerChunk := (threshold / (n + 1)) - 100
		if maxPerChunk < 50 {
			maxPerChunk = 50
		}
		minPerChunk := 50
		return gen.SliceOfN(n, genChunkWithSize(minPerChunk, maxPerChunk))
	}, reflect.TypeOf([]git.DiffChunk{}))
}

func TestDiffChunkingThreshold_Property(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	parameters.Rng.Seed(42)

	properties := gopter.NewProperties(parameters)

	// Use a smaller threshold for testing to make generation easier
	testThreshold := 1024 // 1KB for testing
	processor := NewProcessorWithConfig(ProcessorConfig{
		DiffSizeThreshold: testThreshold,
		MaxChunkSize:      DefaultMaxChunkSize,
		MaxConcurrent:     DefaultMaxConcurrent,
	})

	// Property 1: Diffs exceeding threshold trigger chunking
	// Feature: gitsage, Property 14: Diff chunking threshold
	// Validates: Requirements 5.2
	properties.Property("diffs exceeding threshold trigger chunking", prop.ForAll(
		func(chunks []git.DiffChunk) bool {
			if len(chunks) == 0 {
				return true
			}

			// Calculate total size
			totalSize := 0
			for _, chunk := range chunks {
				totalSize += len(chunk.Content)
			}

			// Skip if total size doesn't actually exceed threshold
			if totalSize <= testThreshold {
				return true
			}

			result, err := processor.Process(context.Background(), chunks)
			if err != nil {
				t.Logf("Process error: %v", err)
				return false
			}

			// Verify chunking is required
			if !result.RequiresChunking {
				t.Logf("Expected RequiresChunking=true for total size %d (threshold %d)", totalSize, testThreshold)
				return false
			}

			// Verify chunk groups are created
			if len(result.ChunkGroups) == 0 {
				t.Logf("Expected ChunkGroups to be created when chunking is required")
				return false
			}

			return true
		},
		genChunksExceedingThreshold(testThreshold),
	))

	// Property 2: Diffs below threshold do not trigger chunking
	properties.Property("diffs below threshold do not trigger chunking", prop.ForAll(
		func(chunks []git.DiffChunk) bool {
			if len(chunks) == 0 {
				return true
			}

			// Calculate total size
			totalSize := 0
			for _, chunk := range chunks {
				totalSize += len(chunk.Content)
			}

			// Skip if total size exceeds threshold
			if totalSize > testThreshold {
				return true
			}

			result, err := processor.Process(context.Background(), chunks)
			if err != nil {
				t.Logf("Process error: %v", err)
				return false
			}

			// Verify chunking is NOT required
			if result.RequiresChunking {
				t.Logf("Expected RequiresChunking=false for total size %d (threshold %d)", totalSize, testThreshold)
				return false
			}

			// Verify no chunk groups are created
			if len(result.ChunkGroups) != 0 {
				t.Logf("Expected no ChunkGroups when chunking is not required")
				return false
			}

			return true
		},
		genChunksBelowThreshold(testThreshold),
	))

	// Property 3: Chunk groups contain all original chunks
	properties.Property("chunk groups contain all original chunks", prop.ForAll(
		func(chunks []git.DiffChunk) bool {
			if len(chunks) == 0 {
				return true
			}

			result, err := processor.Process(context.Background(), chunks)
			if err != nil {
				t.Logf("Process error: %v", err)
				return false
			}

			// Only check when chunking is required
			if !result.RequiresChunking {
				return true
			}

			// Count total chunks in all groups
			totalInGroups := 0
			for _, group := range result.ChunkGroups {
				totalInGroups += len(group.Chunks)
			}

			// Should equal the number of processed chunks
			if totalInGroups != len(result.Chunks) {
				t.Logf("Expected %d chunks in groups, got %d", len(result.Chunks), totalInGroups)
				return false
			}

			return true
		},
		genChunksExceedingThreshold(testThreshold),
	))

	// Property 4: Number of chunk groups respects max concurrent limit
	properties.Property("chunk groups respect max concurrent limit", prop.ForAll(
		func(chunks []git.DiffChunk) bool {
			if len(chunks) == 0 {
				return true
			}

			result, err := processor.Process(context.Background(), chunks)
			if err != nil {
				t.Logf("Process error: %v", err)
				return false
			}

			// Only check when chunking is required
			if !result.RequiresChunking {
				return true
			}

			// Number of groups should not exceed max concurrent
			if len(result.ChunkGroups) > DefaultMaxConcurrent {
				t.Logf("Expected at most %d chunk groups, got %d", DefaultMaxConcurrent, len(result.ChunkGroups))
				return false
			}

			return true
		},
		genChunksExceedingThreshold(testThreshold),
	))

	// Property 5: Summary is generated when chunking is required
	properties.Property("summary is generated when chunking is required", prop.ForAll(
		func(chunks []git.DiffChunk) bool {
			if len(chunks) == 0 {
				return true
			}

			result, err := processor.Process(context.Background(), chunks)
			if err != nil {
				t.Logf("Process error: %v", err)
				return false
			}

			// Only check when chunking is required
			if !result.RequiresChunking {
				return true
			}

			// Summary should be non-empty
			if result.Summary == "" {
				t.Logf("Expected non-empty summary when chunking is required")
				return false
			}

			return true
		},
		genChunksExceedingThreshold(testThreshold),
	))

	properties.TestingRun(t)
}
