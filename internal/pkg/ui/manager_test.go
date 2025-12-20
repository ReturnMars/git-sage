// Package ui provides interactive terminal UI components for GitSage.
package ui

import (
	"testing"

	"github.com/gitsage/gitsage/internal/pkg/ai"
)

func TestActionString(t *testing.T) {
	tests := []struct {
		action   Action
		expected string
	}{
		{ActionAccept, "accept"},
		{ActionEdit, "edit"},
		{ActionRegenerate, "regenerate"},
		{ActionCancel, "cancel"},
		{Action(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.action.String(); got != tt.expected {
				t.Errorf("Action.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestFormatMessageForEdit(t *testing.T) {
	m := NewDefaultManager(true, "")

	tests := []struct {
		name     string
		message  *ai.GenerateResponse
		expected string
	}{
		{
			name: "subject only",
			message: &ai.GenerateResponse{
				Subject: "feat: add new feature",
			},
			expected: "feat: add new feature",
		},
		{
			name: "subject and body",
			message: &ai.GenerateResponse{
				Subject: "feat: add new feature",
				Body:    "This is the body",
			},
			expected: "feat: add new feature\n\nThis is the body",
		},
		{
			name: "subject, body, and footer",
			message: &ai.GenerateResponse{
				Subject: "feat: add new feature",
				Body:    "This is the body",
				Footer:  "Closes #123",
			},
			expected: "feat: add new feature\n\nThis is the body\n\nCloses #123",
		},
		{
			name: "fallback to raw text",
			message: &ai.GenerateResponse{
				RawText: "fix: bug fix\n\nDetails here",
			},
			expected: "fix: bug fix",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := m.formatMessageForEdit(tt.message)
			if got != tt.expected {
				t.Errorf("formatMessageForEdit() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestParseEditedMessage(t *testing.T) {
	m := NewDefaultManager(true, "")

	tests := []struct {
		name            string
		edited          string
		expectedSubject string
		expectedBody    string
		expectedFooter  string
	}{
		{
			name:            "subject only",
			edited:          "feat: add new feature",
			expectedSubject: "feat: add new feature",
			expectedBody:    "",
			expectedFooter:  "",
		},
		{
			name:            "subject and body",
			edited:          "feat: add new feature\n\nThis is the body",
			expectedSubject: "feat: add new feature",
			expectedBody:    "This is the body",
			expectedFooter:  "",
		},
		{
			name:            "subject, body, and footer",
			edited:          "feat: add new feature\n\nThis is the body\n\nCloses #123",
			expectedSubject: "feat: add new feature",
			expectedBody:    "This is the body",
			expectedFooter:  "Closes #123",
		},
		{
			name:            "empty string",
			edited:          "",
			expectedSubject: "",
			expectedBody:    "",
			expectedFooter:  "",
		},
		{
			name:            "whitespace only",
			edited:          "   \n\n   ",
			expectedSubject: "",
			expectedBody:    "",
			expectedFooter:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := m.parseEditedMessage(tt.edited)
			if got.Subject != tt.expectedSubject {
				t.Errorf("parseEditedMessage().Subject = %q, want %q", got.Subject, tt.expectedSubject)
			}
			if got.Body != tt.expectedBody {
				t.Errorf("parseEditedMessage().Body = %q, want %q", got.Body, tt.expectedBody)
			}
			if got.Footer != tt.expectedFooter {
				t.Errorf("parseEditedMessage().Footer = %q, want %q", got.Footer, tt.expectedFooter)
			}
		})
	}
}

func TestGetEditor(t *testing.T) {
	tests := []struct {
		name           string
		configEditor   string
		expectedEditor string
	}{
		{
			name:           "config editor set",
			configEditor:   "vim",
			expectedEditor: "vim",
		},
		{
			name:           "empty config editor",
			configEditor:   "",
			expectedEditor: "", // Will fall back to env vars
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewDefaultManager(true, tt.configEditor)
			got := m.getEditor()
			if tt.configEditor != "" && got != tt.expectedEditor {
				t.Errorf("getEditor() = %q, want %q", got, tt.expectedEditor)
			}
		})
	}
}

func TestNewDefaultManager(t *testing.T) {
	t.Run("with colors enabled", func(t *testing.T) {
		m := NewDefaultManager(true, "vim")
		if m == nil {
			t.Fatal("NewDefaultManager returned nil")
		}
		if !m.colorEnabled {
			t.Error("colorEnabled should be true")
		}
		if m.editor != "vim" {
			t.Errorf("editor = %q, want %q", m.editor, "vim")
		}
		if m.styles == nil {
			t.Error("styles should not be nil")
		}
	})

	t.Run("with colors disabled", func(t *testing.T) {
		m := NewDefaultManager(false, "")
		if m == nil {
			t.Fatal("NewDefaultManager returned nil")
		}
		if m.colorEnabled {
			t.Error("colorEnabled should be false")
		}
	})
}

func TestNonInteractiveManager(t *testing.T) {
	t.Run("PromptAction always returns Accept", func(t *testing.T) {
		m := NewNonInteractiveManager(true)
		action, err := m.PromptAction()
		if err != nil {
			t.Errorf("PromptAction() error = %v", err)
		}
		if action != ActionAccept {
			t.Errorf("PromptAction() = %v, want %v", action, ActionAccept)
		}
	})

	t.Run("EditMessage returns original message", func(t *testing.T) {
		m := NewNonInteractiveManager(true)
		original := &ai.GenerateResponse{
			Subject: "test subject",
			Body:    "test body",
		}
		edited, err := m.EditMessage(original)
		if err != nil {
			t.Errorf("EditMessage() error = %v", err)
		}
		if edited != original {
			t.Error("EditMessage() should return the original message")
		}
	})

	t.Run("ShowSpinner returns animated spinner", func(t *testing.T) {
		m := NewNonInteractiveManager(true)
		spinner := m.ShowSpinner("test")
		if spinner == nil {
			t.Error("ShowSpinner() returned nil")
		}
		// Verify it's an animated spinner (bubbleSpinner), not a noop
		if _, ok := spinner.(*bubbleSpinner); !ok {
			t.Errorf("ShowSpinner() should return *bubbleSpinner, got %T", spinner)
		}
		// These should not panic
		spinner.Start()
		spinner.UpdateText("new text")
		spinner.Stop()
	})

	t.Run("ShowProgressSpinner returns animated progress spinner", func(t *testing.T) {
		m := NewNonInteractiveManager(true)
		spinner := m.ShowProgressSpinner("test", 10)
		if spinner == nil {
			t.Error("ShowProgressSpinner() returned nil")
		}
		// Verify it's an animated progress spinner
		if _, ok := spinner.(*bubbleProgressSpinner); !ok {
			t.Errorf("ShowProgressSpinner() should return *bubbleProgressSpinner, got %T", spinner)
		}
		// These should not panic
		spinner.Start()
		spinner.SetTotal(20)
		spinner.SetCurrent(5)
		spinner.SetCurrentFile("test.go")
		spinner.UpdateText("new text")
		spinner.Stop()
	})

	t.Run("PromptConfirm always returns true", func(t *testing.T) {
		m := NewNonInteractiveManager(true)
		confirmed, err := m.PromptConfirm("Are you sure?")
		if err != nil {
			t.Errorf("PromptConfirm() error = %v", err)
		}
		if !confirmed {
			t.Error("PromptConfirm() should always return true in non-interactive mode")
		}
	})
}

func TestDefaultSpinner(t *testing.T) {
	t.Run("Start and Stop", func(t *testing.T) {
		m := NewDefaultManager(true, "")
		spinner := m.ShowSpinner("Loading...")

		// Start spinner
		spinner.Start()

		// Update text
		spinner.UpdateText("Still loading...")

		// Stop spinner
		spinner.Stop()
	})

	t.Run("Double Start should not panic", func(t *testing.T) {
		m := NewDefaultManager(true, "")
		spinner := m.ShowSpinner("Loading...")
		spinner.Start()
		spinner.Start() // Should not panic
		spinner.Stop()
	})

	t.Run("Double Stop should not panic", func(t *testing.T) {
		m := NewDefaultManager(true, "")
		spinner := m.ShowSpinner("Loading...")
		spinner.Start()
		spinner.Stop()
		spinner.Stop() // Should not panic
	})
}

func TestDisplayMessageNilError(t *testing.T) {
	m := NewDefaultManager(true, "")
	err := m.DisplayMessage(nil)
	if err == nil {
		t.Error("DisplayMessage(nil) should return an error")
	}
}

func TestEditMessageNilError(t *testing.T) {
	m := NewDefaultManager(true, "")
	_, err := m.EditMessage(nil)
	if err == nil {
		t.Error("EditMessage(nil) should return an error")
	}
}

func TestShowErrorNil(t *testing.T) {
	m := NewDefaultManager(true, "")
	// Should not panic
	m.ShowError(nil)
}
