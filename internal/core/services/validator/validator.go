package validator

import (
	"regexp"
	"strings"

	"github.com/gitsage/gitsage/internal/core/domain"
)

// Validator defines the interface for commit message validation.
type Validator interface {
	Validate(msg *domain.CommitMessage) *domain.ValidationResult
	Parse(raw string) *domain.CommitMessage
}

// ConventionalValidator implements Validator for Conventional Commits.
type ConventionalValidator struct{}

func NewConventionalValidator() *ConventionalValidator {
	return &ConventionalValidator{}
}

// ValidCommitTypes contains all valid Conventional Commits types.
var ValidCommitTypes = []string{
	"feat", "fix", "docs", "style", "refactor",
	"test", "chore", "perf", "ci", "build", "revert",
}

var conventionalRegex = regexp.MustCompile(`^(feat|fix|docs|style|refactor|test|chore|perf|ci|build|revert)(\([^)]+\))?:\s*(.+)$`)

func (v *ConventionalValidator) Parse(rawText string) *domain.CommitMessage {
	msg := &domain.CommitMessage{
		Raw: rawText,
	}

	rawText = strings.TrimSpace(rawText)
	if rawText == "" {
		return msg
	}

	lines := strings.Split(rawText, "\n")
	subject := strings.TrimSpace(lines[0])

	// Parse Subject
	matches := conventionalRegex.FindStringSubmatch(subject)
	if matches != nil {
		msg.Type = domain.CommitType(matches[1])
		if matches[2] != "" {
			msg.Scope = strings.Trim(matches[2], "()")
		}
		msg.Subject = strings.TrimSpace(matches[3])
	} else {
		// Fallback: Check if it looks like "type: subject"
		if idx := strings.Index(subject, ":"); idx > 0 {
			potentialType := strings.TrimSpace(subject[:idx])
			for _, t := range ValidCommitTypes {
				if potentialType == t {
					msg.Type = domain.CommitType(t)
					msg.Subject = strings.TrimSpace(subject[idx+1:])
					break
				}
			}
		}
		if msg.Subject == "" {
			msg.Subject = subject
		}
	}

	// Parse Body & Footer
	if len(lines) > 1 {
		var bodyLines, footerLines []string
		inFooter := false

		for i := 1; i < len(lines); i++ {
			line := strings.TrimSpace(lines[i])
			if line == "" && !inFooter {
				continue // Skip leading empty lines in body
			}

			if v.isFooterLine(line) {
				inFooter = true
			}

			if inFooter {
				footerLines = append(footerLines, lines[i])
			} else {
				bodyLines = append(bodyLines, lines[i])
			}
		}
		msg.Body = strings.TrimSpace(strings.Join(bodyLines, "\n"))
		msg.Footer = strings.TrimSpace(strings.Join(footerLines, "\n"))
	}

	return msg
}

func (v *ConventionalValidator) Validate(msg *domain.CommitMessage) *domain.ValidationResult {
	res := &domain.ValidationResult{IsValid: true}

	if msg.Type == "" {
		res.IsValid = false
		res.Issues = append(res.Issues, "missing commit type")
	} else if !v.isValidType(string(msg.Type)) {
		res.IsValid = false
		res.Issues = append(res.Issues, "invalid commit type: "+string(msg.Type))
	}

	if msg.Subject == "" {
		res.IsValid = false
		res.Issues = append(res.Issues, "missing subject")
	}

	// Warning: Check length of full subject line
	fullSubject := string(msg.Type)
	if msg.Scope != "" {
		fullSubject += "(" + msg.Scope + ")"
	}
	fullSubject += ": " + msg.Subject

	if len(fullSubject) > 50 {
		res.Warnings = append(res.Warnings, "subject exceeds 50 characters")
	}
	// Extended limit for Chinese characters consideration
	if len(fullSubject) > 72 {
		res.IsValid = false // Consider strictly invalid if over 72 standard chars
		res.Issues = append(res.Issues, "subject exceeds 72 characters")
	}

	return res
}

func (v *ConventionalValidator) isValidType(t string) bool {
	for _, valid := range ValidCommitTypes {
		if t == valid {
			return true
		}
	}
	return false
}

func (v *ConventionalValidator) isFooterLine(line string) bool {
	upper := strings.ToUpper(line)
	prefixes := []string{
		"BREAKING CHANGE:", "BREAKING-CHANGE:",
		"REFS:", "CLOSES:", "FIXES:", "RESOLVES:", "SEE:",
		"CO-AUTHORED-BY:", "SIGNED-OFF-BY:", "REVIEWED-BY:",
	}
	for _, p := range prefixes {
		if strings.HasPrefix(upper, p) {
			return true
		}
	}
	return strings.HasPrefix(line, "#") // Issue reference
}
