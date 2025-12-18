package ai

import (
	"testing"
)

func TestParseCommitMessage_SimpleSubject(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantType    string
		wantScope   string
		wantSubject string
		wantValid   bool
	}{
		{
			name:        "feat with scope",
			input:       "feat(auth): add login functionality",
			wantType:    "feat",
			wantScope:   "auth",
			wantSubject: "add login functionality",
			wantValid:   true,
		},
		{
			name:        "fix without scope",
			input:       "fix: resolve null pointer exception",
			wantType:    "fix",
			wantScope:   "",
			wantSubject: "resolve null pointer exception",
			wantValid:   true,
		},
		{
			name:        "docs type",
			input:       "docs: update README",
			wantType:    "docs",
			wantScope:   "",
			wantSubject: "update README",
			wantValid:   true,
		},
		{
			name:        "chore with scope",
			input:       "chore(deps): update dependencies",
			wantType:    "chore",
			wantScope:   "deps",
			wantSubject: "update dependencies",
			wantValid:   true,
		},
		{
			name:        "invalid type",
			input:       "invalid: some message",
			wantType:    "",
			wantScope:   "",
			wantSubject: "invalid: some message",
			wantValid:   false,
		},
		{
			name:        "no type",
			input:       "just a message",
			wantType:    "",
			wantScope:   "",
			wantSubject: "just a message",
			wantValid:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseCommitMessage(tt.input)

			if result.Type != tt.wantType {
				t.Errorf("Type = %q, want %q", result.Type, tt.wantType)
			}
			if result.Scope != tt.wantScope {
				t.Errorf("Scope = %q, want %q", result.Scope, tt.wantScope)
			}
			if result.Subject != tt.wantSubject {
				t.Errorf("Subject = %q, want %q", result.Subject, tt.wantSubject)
			}
			if result.IsValid != tt.wantValid {
				t.Errorf("IsValid = %v, want %v", result.IsValid, tt.wantValid)
			}
		})
	}
}

func TestParseCommitMessage_MultiLine(t *testing.T) {
	input := `feat(api): add user endpoint

This commit adds a new user endpoint that allows
creating and retrieving user information.

Closes: #123`

	result := ParseCommitMessage(input)

	if result.Type != "feat" {
		t.Errorf("Type = %q, want %q", result.Type, "feat")
	}
	if result.Scope != "api" {
		t.Errorf("Scope = %q, want %q", result.Scope, "api")
	}
	if result.Subject != "add user endpoint" {
		t.Errorf("Subject = %q, want %q", result.Subject, "add user endpoint")
	}
	if result.Body == "" {
		t.Error("Body should not be empty")
	}
	if result.Footer != "Closes: #123" {
		t.Errorf("Footer = %q, want %q", result.Footer, "Closes: #123")
	}
}

func TestParseCommitMessage_BreakingChange(t *testing.T) {
	input := `feat(api)!: change response format

The API response format has been changed to use
a new structure.

BREAKING CHANGE: Response format changed from array to object`

	result := ParseCommitMessage(input)

	if result.Footer == "" {
		t.Error("Footer should contain BREAKING CHANGE")
	}
	if result.Footer != "BREAKING CHANGE: Response format changed from array to object" {
		t.Errorf("Footer = %q", result.Footer)
	}
}

func TestIsValidCommitType(t *testing.T) {
	validTypes := []string{"feat", "fix", "docs", "style", "refactor", "test", "chore", "perf", "ci", "build", "revert"}

	for _, typ := range validTypes {
		if !IsValidCommitType(typ) {
			t.Errorf("IsValidCommitType(%q) = false, want true", typ)
		}
	}

	invalidTypes := []string{"feature", "bugfix", "update", "change", ""}
	for _, typ := range invalidTypes {
		if IsValidCommitType(typ) {
			t.Errorf("IsValidCommitType(%q) = true, want false", typ)
		}
	}
}

func TestValidateCommitMessage(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantIssues int
	}{
		{
			name:       "valid message",
			input:      "feat: add new feature",
			wantIssues: 0,
		},
		{
			name:       "invalid type",
			input:      "invalid: some message",
			wantIssues: 2, // not conventional format, missing type
		},
		{
			name:       "empty message",
			input:      "",
			wantIssues: 3, // not conventional format, missing type, missing subject
		},
		{
			name:       "subject too long",
			input:      "feat: this is a very long subject line that exceeds the recommended 72 character limit for commit messages",
			wantIssues: 1, // subject too long
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issues := ValidateCommitMessage(tt.input)
			if len(issues) != tt.wantIssues {
				t.Errorf("ValidateCommitMessage() returned %d issues, want %d: %v", len(issues), tt.wantIssues, issues)
			}
		})
	}
}

func TestParsedCommitMessage_FormatSubject(t *testing.T) {
	tests := []struct {
		name string
		msg  ParsedCommitMessage
		want string
	}{
		{
			name: "type with scope",
			msg:  ParsedCommitMessage{Type: "feat", Scope: "auth", Subject: "add login"},
			want: "feat(auth): add login",
		},
		{
			name: "type without scope",
			msg:  ParsedCommitMessage{Type: "fix", Subject: "resolve bug"},
			want: "fix: resolve bug",
		},
		{
			name: "no type",
			msg:  ParsedCommitMessage{Subject: "just a message"},
			want: "just a message",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.msg.FormatSubject()
			if got != tt.want {
				t.Errorf("FormatSubject() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParsedCommitMessage_Format(t *testing.T) {
	msg := ParsedCommitMessage{
		Type:    "feat",
		Scope:   "api",
		Subject: "add endpoint",
		Body:    "This adds a new endpoint.",
		Footer:  "Closes: #123",
	}

	expected := `feat(api): add endpoint

This adds a new endpoint.

Closes: #123`

	got := msg.Format()
	if got != expected {
		t.Errorf("Format() = %q, want %q", got, expected)
	}
}
