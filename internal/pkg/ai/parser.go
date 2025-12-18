// Package ai provides AI provider interfaces and implementations for GitSage.
package ai

import (
	"regexp"
	"strings"
)

// ValidCommitTypes contains all valid Conventional Commits types.
var ValidCommitTypes = []string{
	"feat", "fix", "docs", "style", "refactor",
	"test", "chore", "perf", "ci", "build", "revert",
}

// conventionalCommitRegex matches the Conventional Commits format.
// Format: <type>(<scope>): <subject> or <type>: <subject>
var conventionalCommitRegex = regexp.MustCompile(`^(feat|fix|docs|style|refactor|test|chore|perf|ci|build|revert)(\([^)]+\))?:\s*(.+)$`)

// ParsedCommitMessage represents a parsed commit message.
type ParsedCommitMessage struct {
	Type    string
	Scope   string
	Subject string
	Body    string
	Footer  string
	IsValid bool
}

// ParseCommitMessage parses an AI response into a structured commit message.
// It handles multi-line messages with subject, body, and footer sections.
func ParseCommitMessage(rawText string) *ParsedCommitMessage {
	result := &ParsedCommitMessage{
		IsValid: false,
	}

	// Trim whitespace and clean up the response
	rawText = strings.TrimSpace(rawText)
	if rawText == "" {
		return result
	}

	// Split into lines
	lines := strings.Split(rawText, "\n")
	if len(lines) == 0 {
		return result
	}

	// First line is the subject
	subject := strings.TrimSpace(lines[0])

	// Parse the subject line for Conventional Commits format
	matches := conventionalCommitRegex.FindStringSubmatch(subject)
	if matches != nil {
		result.Type = matches[1]
		if matches[2] != "" {
			// Remove parentheses from scope
			result.Scope = strings.Trim(matches[2], "()")
		}
		result.Subject = strings.TrimSpace(matches[3])
		result.IsValid = true
	} else {
		// Not a valid Conventional Commits format, but still parse what we can
		result.Subject = subject
		// Try to extract type if it looks like "type: subject"
		if idx := strings.Index(subject, ":"); idx > 0 {
			potentialType := strings.TrimSpace(subject[:idx])
			// Check if it's a valid type (without scope)
			for _, validType := range ValidCommitTypes {
				if potentialType == validType {
					result.Type = validType
					result.Subject = strings.TrimSpace(subject[idx+1:])
					result.IsValid = true
					break
				}
			}
		}
	}

	// Parse body and footer (if present)
	if len(lines) > 1 {
		bodyLines := []string{}
		footerLines := []string{}
		inFooter := false
		foundBlankAfterSubject := false

		for i := 1; i < len(lines); i++ {
			line := lines[i]
			trimmedLine := strings.TrimSpace(line)

			// Skip the first blank line after subject
			if !foundBlankAfterSubject && trimmedLine == "" {
				foundBlankAfterSubject = true
				continue
			}

			// Check if this line starts a footer section
			// Footer typically starts with "BREAKING CHANGE:", "Refs:", "Closes:", etc.
			if isFooterLine(trimmedLine) {
				inFooter = true
			}

			if inFooter {
				footerLines = append(footerLines, line)
			} else if foundBlankAfterSubject {
				bodyLines = append(bodyLines, line)
			}
		}

		result.Body = strings.TrimSpace(strings.Join(bodyLines, "\n"))
		result.Footer = strings.TrimSpace(strings.Join(footerLines, "\n"))
	}

	return result
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

// ToGenerateResponse converts a ParsedCommitMessage to a GenerateResponse.
func (p *ParsedCommitMessage) ToGenerateResponse(rawText string) *GenerateResponse {
	return &GenerateResponse{
		Subject: p.FormatSubject(),
		Body:    p.Body,
		Footer:  p.Footer,
		RawText: rawText,
	}
}

// FormatSubject formats the subject line in Conventional Commits format.
func (p *ParsedCommitMessage) FormatSubject() string {
	if p.Type == "" {
		return p.Subject
	}

	if p.Scope != "" {
		return p.Type + "(" + p.Scope + "): " + p.Subject
	}
	return p.Type + ": " + p.Subject
}

// Format returns the full formatted commit message.
func (p *ParsedCommitMessage) Format() string {
	var parts []string

	// Subject line
	parts = append(parts, p.FormatSubject())

	// Body (with blank line separator)
	if p.Body != "" {
		parts = append(parts, "")
		parts = append(parts, p.Body)
	}

	// Footer (with blank line separator if no body)
	if p.Footer != "" {
		if p.Body == "" {
			parts = append(parts, "")
		}
		parts = append(parts, "")
		parts = append(parts, p.Footer)
	}

	return strings.Join(parts, "\n")
}

// IsValidCommitType checks if the given type is a valid Conventional Commits type.
func IsValidCommitType(commitType string) bool {
	for _, validType := range ValidCommitTypes {
		if commitType == validType {
			return true
		}
	}
	return false
}

// ValidateCommitMessage validates a commit message against Conventional Commits format.
// Returns a list of validation issues (empty if valid).
func ValidateCommitMessage(rawText string) []string {
	var issues []string

	parsed := ParseCommitMessage(rawText)

	if !parsed.IsValid {
		issues = append(issues, "message does not follow Conventional Commits format")
	}

	if parsed.Type == "" {
		issues = append(issues, "missing commit type")
	} else if !IsValidCommitType(parsed.Type) {
		issues = append(issues, "invalid commit type: "+parsed.Type)
	}

	if parsed.Subject == "" {
		issues = append(issues, "missing commit subject")
	}

	// Check subject length (warning threshold is 100 chars for Chinese support)
	subjectLine := parsed.FormatSubject()
	if len(subjectLine) > 100 {
		issues = append(issues, "subject line exceeds 100 characters")
	}

	return issues
}
