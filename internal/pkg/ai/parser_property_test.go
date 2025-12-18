// Package ai provides AI provider interfaces and implementations for GitSage.
package ai

import (
	"regexp"
	"strings"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// Feature: gitsage, Property 8: Conventional Commits format validation
// Validates: Requirements 4.1

// conventionalCommitPattern matches the Conventional Commits format.
// Format: <type>(<scope>): <subject> or <type>: <subject>
var conventionalCommitPattern = regexp.MustCompile(`^(feat|fix|docs|style|refactor|test|chore|perf|ci|build|revert)(\([^)]+\))?:\s*.+$`)

// genValidCommitType generates a random valid commit type
func genValidCommitType() gopter.Gen {
	return gen.OneConstOf(
		"feat", "fix", "docs", "style", "refactor",
		"test", "chore", "perf", "ci", "build", "revert",
	)
}

// genOptionalScope generates an optional scope (empty string or alphanumeric)
func genOptionalScope() gopter.Gen {
	return gen.OneGenOf(
		gen.Const(""),
		gen.Identifier().Map(func(s string) string {
			// Limit scope length to reasonable size
			if len(s) > 20 {
				return s[:20]
			}
			return s
		}),
	)
}

// genNonEmptySubject generates a non-empty subject string
func genNonEmptySubject() gopter.Gen {
	return gen.Identifier().SuchThat(func(s string) bool {
		return len(s) > 0
	}).Map(func(s string) string {
		// Limit subject length to reasonable size
		if len(s) > 50 {
			return s[:50]
		}
		return s
	})
}

// genValidConventionalCommit generates a valid Conventional Commits message
func genValidConventionalCommit() gopter.Gen {
	return gopter.CombineGens(
		genValidCommitType(),
		genOptionalScope(),
		genNonEmptySubject(),
	).Map(func(values []any) string {
		commitType := values[0].(string)
		scope := values[1].(string)
		subject := values[2].(string)

		if scope != "" {
			return commitType + "(" + scope + "): " + subject
		}
		return commitType + ": " + subject
	})
}

// TestProperty_ConventionalCommitsFormatValidation verifies that for any generated commit message,
// the subject line should match the pattern `<type>(<scope>): <subject>` or `<type>: <subject>`.
//
// Feature: gitsage, Property 8: Conventional Commits format validation
// Validates: Requirements 4.1
func TestProperty_ConventionalCommitsFormatValidation(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	// Property 1: Any valid Conventional Commits message should be parsed correctly
	// and marked as valid
	properties.Property("valid conventional commits are recognized as valid", prop.ForAll(
		func(message string) bool {
			parsed := ParseCommitMessage(message)
			return parsed.IsValid
		},
		genValidConventionalCommit(),
	))

	// Property 2: For any valid Conventional Commits message, the parsed type
	// should be one of the valid commit types
	properties.Property("parsed type is always a valid commit type", prop.ForAll(
		func(message string) bool {
			parsed := ParseCommitMessage(message)
			if !parsed.IsValid {
				return true // Skip invalid messages
			}
			return IsValidCommitType(parsed.Type)
		},
		genValidConventionalCommit(),
	))

	// Property 3: For any valid Conventional Commits message, the formatted subject
	// should match the Conventional Commits pattern
	properties.Property("formatted subject matches conventional commits pattern", prop.ForAll(
		func(message string) bool {
			parsed := ParseCommitMessage(message)
			if !parsed.IsValid {
				return true // Skip invalid messages
			}
			formattedSubject := parsed.FormatSubject()
			return conventionalCommitPattern.MatchString(formattedSubject)
		},
		genValidConventionalCommit(),
	))

	// Property 4: Round-trip property - parsing then formatting should preserve
	// the essential structure (type, scope, subject)
	properties.Property("parse then format preserves structure", prop.ForAll(
		func(commitType string, scope string, subject string) bool {
			// Build original message
			var original string
			if scope != "" {
				original = commitType + "(" + scope + "): " + subject
			} else {
				original = commitType + ": " + subject
			}

			// Parse and format
			parsed := ParseCommitMessage(original)
			formatted := parsed.FormatSubject()

			// The formatted version should match the original
			return formatted == original
		},
		genValidCommitType(),
		genOptionalScope(),
		genNonEmptySubject(),
	))

	// Property 5: Messages without valid type should be marked as invalid
	properties.Property("messages without valid type are invalid", prop.ForAll(
		func(invalidType string, subject string) bool {
			// Skip if the random string happens to be a valid type
			if IsValidCommitType(invalidType) {
				return true
			}
			message := invalidType + ": " + subject
			parsed := ParseCommitMessage(message)
			return !parsed.IsValid
		},
		gen.Identifier().SuchThat(func(s string) bool {
			return len(s) > 0 && !IsValidCommitType(s)
		}),
		genNonEmptySubject(),
	))

	// Property 6: ValidateCommitMessage returns no issues for valid messages
	properties.Property("validation returns no issues for valid messages", prop.ForAll(
		func(message string) bool {
			issues := ValidateCommitMessage(message)
			// Valid messages should have no issues (except possibly length warning)
			for _, issue := range issues {
				if !strings.Contains(issue, "exceeds") {
					return false
				}
			}
			return true
		},
		genValidConventionalCommit(),
	))

	properties.TestingRun(t)
}
