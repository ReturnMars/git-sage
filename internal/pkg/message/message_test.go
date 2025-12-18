package message

import (
	"testing"
)

func TestNewCommitMessage(t *testing.T) {
	tests := []struct {
		name     string
		rawText  string
		wantType string
		wantScope string
		wantSubject string
		wantBody string
		wantFooter string
	}{
		{
			name:        "simple feat commit",
			rawText:     "feat: add new feature",
			wantType:    "feat",
			wantScope:   "",
			wantSubject: "add new feature",
		},
		{
			name:        "feat with scope",
			rawText:     "feat(auth): add login functionality",
			wantType:    "feat",
			wantScope:   "auth",
			wantSubject: "add login functionality",
		},
		{
			name:        "fix commit",
			rawText:     "fix: resolve null pointer exception",
			wantType:    "fix",
			wantScope:   "",
			wantSubject: "resolve null pointer exception",
		},
		{
			name:        "commit with body",
			rawText:     "feat: add feature\n\nThis is the body of the commit message.",
			wantType:    "feat",
			wantSubject: "add feature",
			wantBody:    "This is the body of the commit message.",
		},
		{
			name:        "commit with footer",
			rawText:     "fix: fix bug\n\nBREAKING CHANGE: API changed",
			wantType:    "fix",
			wantSubject: "fix bug",
			wantFooter:  "BREAKING CHANGE: API changed",
		},
		{
			name:        "commit with body and footer",
			rawText:     "feat(api): add endpoint\n\nAdded new REST endpoint.\n\nCloses: #123",
			wantType:    "feat",
			wantScope:   "api",
			wantSubject: "add endpoint",
			wantBody:    "Added new REST endpoint.",
			wantFooter:  "Closes: #123",
		},
		{
			name:        "empty input",
			rawText:     "",
			wantType:    "",
			wantSubject: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cm := NewCommitMessage(tt.rawText)
			if cm.Type != tt.wantType {
				t.Errorf("Type = %q, want %q", cm.Type, tt.wantType)
			}
			if cm.Scope != tt.wantScope {
				t.Errorf("Scope = %q, want %q", cm.Scope, tt.wantScope)
			}
			if cm.Subject != tt.wantSubject {
				t.Errorf("Subject = %q, want %q", cm.Subject, tt.wantSubject)
			}
			if cm.Body != tt.wantBody {
				t.Errorf("Body = %q, want %q", cm.Body, tt.wantBody)
			}
			if cm.Footer != tt.wantFooter {
				t.Errorf("Footer = %q, want %q", cm.Footer, tt.wantFooter)
			}
		})
	}
}

func TestCommitMessage_Format(t *testing.T) {
	tests := []struct {
		name string
		cm   *CommitMessage
		want string
	}{
		{
			name: "simple message",
			cm: &CommitMessage{
				Type:    "feat",
				Subject: "add feature",
			},
			want: "feat: add feature",
		},
		{
			name: "message with scope",
			cm: &CommitMessage{
				Type:    "fix",
				Scope:   "auth",
				Subject: "fix login",
			},
			want: "fix(auth): fix login",
		},
		{
			name: "message with body",
			cm: &CommitMessage{
				Type:    "feat",
				Subject: "add feature",
				Body:    "This is the body.",
			},
			want: "feat: add feature\n\nThis is the body.",
		},
		{
			name: "message with footer",
			cm: &CommitMessage{
				Type:    "fix",
				Subject: "fix bug",
				Footer:  "Closes: #123",
			},
			want: "fix: fix bug\n\nCloses: #123",
		},
		{
			name: "full message",
			cm: &CommitMessage{
				Type:    "feat",
				Scope:   "api",
				Subject: "add endpoint",
				Body:    "Added new REST endpoint.",
				Footer:  "BREAKING CHANGE: API changed",
			},
			want: "feat(api): add endpoint\n\nAdded new REST endpoint.\n\nBREAKING CHANGE: API changed",
		},
		{
			name: "no type",
			cm: &CommitMessage{
				Subject: "just a subject",
			},
			want: "just a subject",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.cm.Format()
			if got != tt.want {
				t.Errorf("Format() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCommitMessage_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cm      *CommitMessage
		wantErr bool
	}{
		{
			name: "valid feat commit",
			cm: &CommitMessage{
				Type:    "feat",
				Subject: "add feature",
			},
			wantErr: false,
		},
		{
			name: "valid fix commit with scope",
			cm: &CommitMessage{
				Type:    "fix",
				Scope:   "auth",
				Subject: "fix login",
			},
			wantErr: false,
		},
		{
			name: "missing type",
			cm: &CommitMessage{
				Subject: "add feature",
			},
			wantErr: true,
		},
		{
			name: "invalid type",
			cm: &CommitMessage{
				Type:    "invalid",
				Subject: "add feature",
			},
			wantErr: true,
		},
		{
			name: "missing subject",
			cm: &CommitMessage{
				Type: "feat",
			},
			wantErr: true,
		},
		{
			name:    "empty message",
			cm:      &CommitMessage{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cm.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCommitMessage_ValidateWithWarnings(t *testing.T) {
	tests := []struct {
		name         string
		cm           *CommitMessage
		wantValid    bool
		wantErrors   int
		wantWarnings int
	}{
		{
			name: "valid short subject",
			cm: &CommitMessage{
				Type:    "feat",
				Subject: "add feature",
			},
			wantValid:    true,
			wantErrors:   0,
			wantWarnings: 0,
		},
		{
			name: "valid but long subject",
			cm: &CommitMessage{
				Type:    "feat",
				Subject: "this is a very long subject line that exceeds the recommended 72 character limit",
			},
			wantValid:    true,
			wantErrors:   0,
			wantWarnings: 1,
		},
		{
			name: "invalid type",
			cm: &CommitMessage{
				Type:    "invalid",
				Subject: "add feature",
			},
			wantValid:  false,
			wantErrors: 1,
		},
		{
			name: "missing type and subject",
			cm:   &CommitMessage{},
			wantValid:  false,
			wantErrors: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.cm.ValidateWithWarnings()
			if result.IsValid != tt.wantValid {
				t.Errorf("IsValid = %v, want %v", result.IsValid, tt.wantValid)
			}
			if len(result.Errors) != tt.wantErrors {
				t.Errorf("Errors count = %d, want %d", len(result.Errors), tt.wantErrors)
			}
			if len(result.Warnings) != tt.wantWarnings {
				t.Errorf("Warnings count = %d, want %d", len(result.Warnings), tt.wantWarnings)
			}
		})
	}
}

func TestIsValidCommitType(t *testing.T) {
	validTypes := []string{"feat", "fix", "docs", "style", "refactor", "test", "chore", "perf", "ci", "build", "revert"}
	for _, typ := range validTypes {
		if !IsValidCommitType(typ) {
			t.Errorf("IsValidCommitType(%q) = false, want true", typ)
		}
	}

	invalidTypes := []string{"feature", "bugfix", "invalid", "", "FEAT", "Fix"}
	for _, typ := range invalidTypes {
		if IsValidCommitType(typ) {
			t.Errorf("IsValidCommitType(%q) = true, want false", typ)
		}
	}
}

func TestCommitMessage_SubjectExceedsLength(t *testing.T) {
	tests := []struct {
		name string
		cm   *CommitMessage
		want bool
	}{
		{
			name: "short subject",
			cm: &CommitMessage{
				Type:    "feat",
				Subject: "add feature",
			},
			want: false,
		},
		{
			name: "exactly 72 chars",
			cm: &CommitMessage{
				Type:    "feat",
				Subject: "this subject is exactly sixty-six characters long ok",
			},
			want: false,
		},
		{
			name: "exceeds 72 chars",
			cm: &CommitMessage{
				Type:    "feat",
				Subject: "this is a very long subject line that definitely exceeds the recommended limit",
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cm.SubjectExceedsLength(); got != tt.want {
				formatted := tt.cm.FormatSubject()
				t.Errorf("SubjectExceedsLength() = %v, want %v (length: %d)", got, tt.want, len(formatted))
			}
		})
	}
}

func TestCommitMessage_IsMultiLine(t *testing.T) {
	tests := []struct {
		name string
		cm   *CommitMessage
		want bool
	}{
		{
			name: "subject only",
			cm: &CommitMessage{
				Type:    "feat",
				Subject: "add feature",
			},
			want: false,
		},
		{
			name: "with body",
			cm: &CommitMessage{
				Type:    "feat",
				Subject: "add feature",
				Body:    "This is the body.",
			},
			want: true,
		},
		{
			name: "with footer",
			cm: &CommitMessage{
				Type:    "feat",
				Subject: "add feature",
				Footer:  "Closes: #123",
			},
			want: true,
		},
		{
			name: "with body and footer",
			cm: &CommitMessage{
				Type:    "feat",
				Subject: "add feature",
				Body:    "Body text.",
				Footer:  "Closes: #123",
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cm.IsMultiLine(); got != tt.want {
				t.Errorf("IsMultiLine() = %v, want %v", got, tt.want)
			}
		})
	}
}
