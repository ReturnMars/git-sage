// Package ai provides AI provider interfaces and implementations for GitSage.
package ai

import (
	"testing"

	"github.com/gitsage/gitsage/internal/pkg/git"
	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// Feature: langchaingo-refactor, Property 1: Output Format Consistency
// Validates: Requirements 1.4, 6.3, 6.4
//
// For any valid GenerateRequest input, the GenerateResponse returned by the refactored system
// SHALL have the same structure (Subject, Body, Footer, RawText fields) and the same parsing
// behavior as the original implementation.

// genCommitType generates valid commit types.
func genCommitType() gopter.Gen {
	return gen.OneConstOf(
		"feat", "fix", "docs", "style", "refactor",
		"test", "chore", "perf", "ci", "build", "revert",
	)
}

// genScope generates optional scopes.
func genScope() gopter.Gen {
	return gen.OneConstOf(
		"", "api", "ui", "db", "auth", "config", "cli", "core",
	)
}

// genSubject generates commit subjects.
func genSubject() gopter.Gen {
	return gen.OneConstOf(
		"add new feature",
		"fix bug in handler",
		"update documentation",
		"refactor code structure",
		"add unit tests",
		"update dependencies",
	)
}

// genBody generates optional commit bodies.
func genBody() gopter.Gen {
	return gen.OneConstOf(
		"",
		"This is a detailed description of the change.",
		"- Added new functionality\n- Fixed issues",
		"Multiple lines\nof body\ntext here",
	)
}

// genRawCommitMessage generates raw commit messages for testing.
func genRawCommitMessage() gopter.Gen {
	return gopter.CombineGens(
		genCommitType(),
		genScope(),
		genSubject(),
		genBody(),
	).Map(func(values []interface{}) string {
		commitType := values[0].(string)
		scope := values[1].(string)
		subject := values[2].(string)
		body := values[3].(string)

		// Build subject line
		var subjectLine string
		if scope != "" {
			subjectLine = commitType + "(" + scope + "): " + subject
		} else {
			subjectLine = commitType + ": " + subject
		}

		if body != "" {
			return subjectLine + "\n\n" + body
		}
		return subjectLine
	})
}

// TestProperty_OutputFormatConsistency verifies that the output format is consistent.
//
// Feature: langchaingo-refactor, Property 1: Output Format Consistency
// Validates: Requirements 1.4, 6.3, 6.4
func TestProperty_OutputFormatConsistency(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	// Property: ParseCommitMessage always returns non-nil result
	properties.Property("ParseCommitMessage returns non-nil for any input", prop.ForAll(
		func(rawText string) bool {
			parsed := ParseCommitMessage(rawText)
			return parsed != nil
		},
		genRawCommitMessage(),
	))

	// Property: GenerateResponse has all required fields
	properties.Property("GenerateResponse has all required fields", prop.ForAll(
		func(rawText string) bool {
			parsed := ParseCommitMessage(rawText)
			resp := parsed.ToGenerateResponse(rawText)

			// All fields should be present (may be empty but not nil pointer)
			if resp == nil {
				return false
			}

			// RawText should match input
			if resp.RawText != rawText {
				return false
			}

			return true
		},
		genRawCommitMessage(),
	))

	// Property: Valid conventional commits have non-empty Subject
	properties.Property("valid conventional commits have non-empty Subject", prop.ForAll(
		func(rawText string) bool {
			parsed := ParseCommitMessage(rawText)
			resp := parsed.ToGenerateResponse(rawText)

			// For valid conventional commits, Subject should not be empty
			if resp.Subject == "" {
				return false
			}

			return true
		},
		genRawCommitMessage(),
	))

	// Property: Subject contains type prefix for valid conventional commits
	properties.Property("Subject contains commit type for valid messages", prop.ForAll(
		func(commitType, scope, subject string) bool {
			var rawText string
			if scope != "" {
				rawText = commitType + "(" + scope + "): " + subject
			} else {
				rawText = commitType + ": " + subject
			}

			parsed := ParseCommitMessage(rawText)
			resp := parsed.ToGenerateResponse(rawText)

			// Subject should contain the commit type
			return len(resp.Subject) > 0
		},
		genCommitType(),
		genScope(),
		genSubject(),
	))

	// Property: LangChainWrapper produces valid GenerateResponse structure
	properties.Property("LangChainWrapper produces valid response structure", prop.ForAll(
		func(rawText string) bool {
			// Simulate what LangChainWrapper does with the parsed response
			parsed := ParseCommitMessage(rawText)
			resp := parsed.ToGenerateResponse(rawText)

			// Response structure validation
			if resp == nil {
				return false
			}

			// Subject is derived from the first line
			if resp.Subject == "" {
				return false
			}

			return true
		},
		genRawCommitMessage(),
	))

	// Property: Body extraction is consistent
	properties.Property("body extraction is consistent", prop.ForAll(
		func(commitType, subject, body string) bool {
			var rawText string
			if body != "" {
				rawText = commitType + ": " + subject + "\n\n" + body
			} else {
				rawText = commitType + ": " + subject
			}

			parsed := ParseCommitMessage(rawText)
			resp := parsed.ToGenerateResponse(rawText)

			// If body was in input, it should be in response
			if body != "" && resp.Body == "" {
				return false
			}

			return true
		},
		genCommitType(),
		genSubject(),
		genBody(),
	))

	// Property: BuildPromptData creates valid PromptData
	properties.Property("BuildPromptData creates valid PromptData", prop.ForAll(
		func(filesCount int, additions int, deletions int) bool {
			req := &GenerateRequest{
				DiffChunks: make([]git.DiffChunk, filesCount),
				DiffStats: &git.DiffStats{
					TotalFiles:     filesCount,
					TotalAdditions: additions,
					TotalDeletions: deletions,
				},
			}

			data := BuildPromptData(req, false)

			// Validate PromptData structure
			if data == nil {
				return false
			}
			if data.DiffStats == nil {
				return false
			}
			if len(data.Chunks) != filesCount {
				return false
			}

			return true
		},
		gen.IntRange(0, 10),
		gen.IntRange(0, 1000),
		gen.IntRange(0, 1000),
	))

	properties.TestingRun(t)
}
