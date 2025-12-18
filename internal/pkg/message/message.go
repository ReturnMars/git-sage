// Package message provides commit message validation and formatting for GitSage.
package message

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
)

// ValidCommitTypes contains all valid Conventional Commits types.
var ValidCommitTypes = []string{
	"feat", "fix", "docs", "style", "refactor",
	"test", "chore", "perf", "ci", "build", "revert",
}

// MaxSubjectLength is the recommended maximum length for commit subject lines.
const MaxSubjectLength = 72

// conventionalCommitRegex matches the Conventional Commits format.
// Format: <type>(<scope>): <subject> or <type>: <subject>
var conventionalCommitRegex = regexp.MustCompile(`^(feat|fix|docs|style|refactor|test|chore|perf|ci|build|revert)(\([^)]+\))?:\s*(.+)$`)

// ValidationError represents a commit message validation error.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// ValidationResult contains the result of commit message validation.
type ValidationResult struct {
	IsValid  bool
	Errors   []ValidationError
	Warnings []string
}

// CommitMessage represents a structured commit message following Conventional Commits format.
type CommitMessage struct {
	Type    string // feat, fix, docs, etc.
	Scope   string // Optional scope
	Subject string // Short description (max 72 chars recommended)
	Body    string // Optional detailed description
	Footer  string // Optional footer (breaking changes, refs)
}

// NewCommitMessage creates a new CommitMessage from raw text.
func NewCommitMessage(rawText string) *CommitMessage {
	cm := &CommitMessage{}
	cm.Parse(rawText)
	return cm
}

// Parse parses raw text into the CommitMessage structure.
func (cm *CommitMessage) Parse(rawText string) {
	rawText = strings.TrimSpace(rawText)
	if rawText == "" {
		return
	}

	lines := strings.Split(rawText, "\n")
	if len(lines) == 0 {
		return
	}

	// Parse subject line
	subject := strings.TrimSpace(lines[0])
	cm.parseSubject(subject)

	// Parse body and footer
	if len(lines) > 1 {
		cm.parseBodyAndFooter(lines[1:])
	}
}

// parseSubject parses the subject line for Conventional Commits format.
func (cm *CommitMessage) parseSubject(subject string) {
	matches := conventionalCommitRegex.FindStringSubmatch(subject)
	if matches != nil {
		cm.Type = matches[1]
		if matches[2] != "" {
			cm.Scope = strings.Trim(matches[2], "()")
		}
		cm.Subject = strings.TrimSpace(matches[3])
	} else {
		// Try to extract type if it looks like "type: subject"
		if idx := strings.Index(subject, ":"); idx > 0 {
			potentialType := strings.TrimSpace(subject[:idx])
			if IsValidCommitType(potentialType) {
				cm.Type = potentialType
				cm.Subject = strings.TrimSpace(subject[idx+1:])
				return
			}
		}
		// Not a valid format, store as subject only
		cm.Subject = subject
	}
}

// parseBodyAndFooter parses the body and footer sections.
func (cm *CommitMessage) parseBodyAndFooter(lines []string) {
	bodyLines := []string{}
	footerLines := []string{}
	inFooter := false
	foundBlankAfterSubject := false

	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)

		// Skip the first blank line after subject
		if !foundBlankAfterSubject && trimmedLine == "" {
			foundBlankAfterSubject = true
			continue
		}

		// Check if this line starts a footer section
		if isFooterLine(trimmedLine) {
			inFooter = true
		}

		if inFooter {
			footerLines = append(footerLines, line)
		} else if foundBlankAfterSubject {
			bodyLines = append(bodyLines, line)
		}
	}

	cm.Body = strings.TrimSpace(strings.Join(bodyLines, "\n"))
	cm.Footer = strings.TrimSpace(strings.Join(footerLines, "\n"))
}

// isFooterLine checks if a line is a footer line.
func isFooterLine(line string) bool {
	footerPrefixes := []string{
		"BREAKING CHANGE:",
		"BREAKING-CHANGE:",
		"Refs:",
		"Closes:",
		"Fixes:",
		"Resolves:",
		"See:",
		"Co-authored-by:",
		"Signed-off-by:",
		"Reviewed-by:",
		"Acked-by:",
	}

	upperLine := strings.ToUpper(line)
	for _, prefix := range footerPrefixes {
		if strings.HasPrefix(upperLine, strings.ToUpper(prefix)) {
			return true
		}
	}

	// Also check for issue references like "#123" at the start
	if strings.HasPrefix(line, "#") {
		return true
	}

	return false
}

// Format returns the full formatted commit message following Conventional Commits.
func (cm *CommitMessage) Format() string {
	var parts []string

	// Subject line
	parts = append(parts, cm.FormatSubject())

	// Body (with blank line separator)
	if cm.Body != "" {
		parts = append(parts, "")
		parts = append(parts, cm.Body)
	}

	// Footer (with blank line separator)
	if cm.Footer != "" {
		parts = append(parts, "")
		parts = append(parts, cm.Footer)
	}

	return strings.Join(parts, "\n")
}

// FormatSubject formats the subject line in Conventional Commits format.
func (cm *CommitMessage) FormatSubject() string {
	if cm.Type == "" {
		return cm.Subject
	}

	if cm.Scope != "" {
		return cm.Type + "(" + cm.Scope + "): " + cm.Subject
	}
	return cm.Type + ": " + cm.Subject
}

// Validate validates the commit message against Conventional Commits format.
// Returns an error if the message is invalid.
func (cm *CommitMessage) Validate() error {
	result := cm.ValidateWithWarnings()
	if !result.IsValid {
		var errMsgs []string
		for _, e := range result.Errors {
			errMsgs = append(errMsgs, e.Error())
		}
		return errors.New(strings.Join(errMsgs, "; "))
	}
	return nil
}

// ValidateWithWarnings validates the commit message and returns detailed results.
// This includes both errors (invalid format) and warnings (e.g., subject too long).
func (cm *CommitMessage) ValidateWithWarnings() *ValidationResult {
	result := &ValidationResult{
		IsValid:  true,
		Errors:   []ValidationError{},
		Warnings: []string{},
	}

	// Check for missing type
	if cm.Type == "" {
		result.IsValid = false
		result.Errors = append(result.Errors, ValidationError{
			Field:   "type",
			Message: "missing commit type",
		})
	} else if !IsValidCommitType(cm.Type) {
		result.IsValid = false
		result.Errors = append(result.Errors, ValidationError{
			Field:   "type",
			Message: fmt.Sprintf("invalid commit type: %s (valid types: %s)", cm.Type, strings.Join(ValidCommitTypes, ", ")),
		})
	}

	// Check for missing subject
	if cm.Subject == "" {
		result.IsValid = false
		result.Errors = append(result.Errors, ValidationError{
			Field:   "subject",
			Message: "missing commit subject",
		})
	}

	// Check subject length (warning, not error)
	subjectLine := cm.FormatSubject()
	if len(subjectLine) > MaxSubjectLength {
		result.Warnings = append(result.Warnings, fmt.Sprintf(
			"subject line exceeds %d characters (%d chars)",
			MaxSubjectLength, len(subjectLine),
		))
	}

	return result
}

// IsValidCommitType checks if the given type is a valid Conventional Commits type.
func IsValidCommitType(commitType string) bool {
	return slices.Contains(ValidCommitTypes, commitType)
}

// SubjectExceedsLength checks if the formatted subject line exceeds the max length.
func (cm *CommitMessage) SubjectExceedsLength() bool {
	return len(cm.FormatSubject()) > MaxSubjectLength
}

// HasBody returns true if the commit message has a body section.
func (cm *CommitMessage) HasBody() bool {
	return cm.Body != ""
}

// HasFooter returns true if the commit message has a footer section.
func (cm *CommitMessage) HasFooter() bool {
	return cm.Footer != ""
}

// IsMultiLine returns true if the commit message has body or footer sections.
func (cm *CommitMessage) IsMultiLine() bool {
	return cm.HasBody() || cm.HasFooter()
}
