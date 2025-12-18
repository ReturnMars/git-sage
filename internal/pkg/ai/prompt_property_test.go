// Package ai provides AI provider interfaces and implementations for GitSage.
package ai

import (
	"strings"
	"testing"

	"github.com/gitsage/gitsage/internal/pkg/git"
	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// Feature: gitsage, Property 10: Prompt instruction inclusion
// Validates: Requirements 4.3

var validCommitTypes = []string{
	"feat", "fix", "docs", "style", "refactor", "test", "chore", "perf", "ci", "build", "revert",
}

func genDiffChunk() gopter.Gen {
	return gopter.CombineGens(
		gen.Identifier(), // Generates valid identifiers (non-empty alphanumeric strings)
		gen.IntRange(0, 2),
		gen.IntRange(0, 100),
		gen.IntRange(0, 100),
		gen.AnyString(),
	).Map(func(values []interface{}) git.DiffChunk {
		filePath := values[0].(string) + ".go"
		changeTypeInt := values[1].(int)
		var changeType git.ChangeType
		switch changeTypeInt {
		case 0:
			changeType = git.ChangeTypeAdded
		case 1:
			changeType = git.ChangeTypeModified
		default:
			changeType = git.ChangeTypeDeleted
		}
		return git.DiffChunk{
			FilePath:   filePath,
			ChangeType: changeType,
			Additions:  values[2].(int),
			Deletions:  values[3].(int),
			Content:    values[4].(string),
			IsLockFile: false,
		}
	})
}

// genDiffStats generates random DiffStats for testing
func genDiffStats() gopter.Gen {
	return gopter.CombineGens(
		gen.IntRange(1, 50),
		gen.IntRange(0, 1000),
		gen.IntRange(0, 1000),
	).Map(func(values []interface{}) *git.DiffStats {
		return &git.DiffStats{
			TotalFiles:     values[0].(int),
			TotalAdditions: values[1].(int),
			TotalDeletions: values[2].(int),
		}
	})
}

// genPromptData generates random PromptData for testing
func genPromptData() gopter.Gen {
	return gopter.CombineGens(
		genDiffStats(),
		gen.SliceOfN(5, genDiffChunk()),
		gen.Bool(),
		gen.AlphaString(),
	).Map(func(values []interface{}) *PromptData {
		return &PromptData{
			DiffStats:        values[0].(*git.DiffStats),
			Chunks:           values[1].([]git.DiffChunk),
			RequiresChunking: values[2].(bool),
			PreviousAttempt:  values[3].(string),
			CustomPrompt:     "", // Don't use custom prompt for this test
		}
	})
}

// TestProperty_PromptInstructionInclusion verifies that for any prompt sent to an AI provider,
// the prompt should contain instructions for Conventional Commits format.
//
// Feature: gitsage, Property 10: Prompt instruction inclusion
// Validates: Requirements 4.3
func TestProperty_PromptInstructionInclusion(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	// Property: For any PromptTemplate (default or custom with empty system prompt),
	// the system prompt should contain Conventional Commits instructions
	properties.Property("system prompt contains Conventional Commits instructions", prop.ForAll(
		func(data *PromptData) bool {
			// Create a new prompt template (uses default system prompt)
			pt := NewPromptTemplate()
			systemPrompt := pt.GetSystemPrompt()

			// The system prompt must contain "Conventional Commits" format instruction
			hasConventionalCommits := strings.Contains(systemPrompt, "Conventional Commits")

			// The system prompt must contain the format pattern <type>(<scope>): or similar
			hasFormatPattern := strings.Contains(systemPrompt, "<type>") ||
				strings.Contains(systemPrompt, "type(scope)")

			// The system prompt must list at least some valid commit types
			hasCommitTypes := false
			for _, commitType := range validCommitTypes {
				if strings.Contains(systemPrompt, commitType) {
					hasCommitTypes = true
					break
				}
			}

			return hasConventionalCommits && hasFormatPattern && hasCommitTypes
		},
		genPromptData(),
	))

	// Property: For any custom prompt template with empty system prompt,
	// it should fall back to default which contains Conventional Commits instructions
	properties.Property("empty custom system prompt falls back to default with CC instructions", prop.ForAll(
		func(customUserPrompt string) bool {
			// Create template with empty system prompt (should fall back to default)
			pt := NewPromptTemplateWithCustom("", customUserPrompt)
			systemPrompt := pt.GetSystemPrompt()

			// Should still contain Conventional Commits instructions
			return strings.Contains(systemPrompt, "Conventional Commits")
		},
		gen.AlphaString(),
	))

	// Property: The rendered user prompt should be valid (no template errors)
	// for any valid input data
	properties.Property("user prompt renders without error for any valid input", prop.ForAll(
		func(data *PromptData) bool {
			pt := NewPromptTemplate()
			_, err := pt.RenderUserPrompt(data)
			return err == nil
		},
		genPromptData(),
	))

	properties.TestingRun(t)
}
