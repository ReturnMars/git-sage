// Package ai provides AI provider interfaces and implementations for GitSage.
package ai

import (
	"strings"
	"testing"

	"github.com/gitsage/gitsage/internal/pkg/git"
	"github.com/tmc/langchaingo/llms"
)

func TestNewLangChainPromptTemplate(t *testing.T) {
	pt := NewLangChainPromptTemplate()

	if pt == nil {
		t.Fatal("NewLangChainPromptTemplate returned nil")
	}

	// Verify default prompts are set
	if pt.GetSystemPrompt() != DefaultSystemPrompt {
		t.Errorf("Expected default system prompt, got different value")
	}
	if pt.GetUserPrompt() != DefaultUserPromptTemplate {
		t.Errorf("Expected default user prompt template, got different value")
	}
}

func TestNewLangChainPromptTemplateWithCustom(t *testing.T) {
	customSystem := "Custom system prompt"
	customUser := "Custom user prompt {{.DiffStats}}"

	pt := NewLangChainPromptTemplateWithCustom(customSystem, customUser)

	if pt == nil {
		t.Fatal("NewLangChainPromptTemplateWithCustom returned nil")
	}

	if pt.GetSystemPrompt() != customSystem {
		t.Errorf("Expected custom system prompt %q, got %q", customSystem, pt.GetSystemPrompt())
	}
	if pt.GetUserPrompt() != customUser {
		t.Errorf("Expected custom user prompt %q, got %q", customUser, pt.GetUserPrompt())
	}
}

func TestNewLangChainPromptTemplateWithCustom_EmptyFallsBackToDefault(t *testing.T) {
	// Empty strings should fall back to defaults
	pt := NewLangChainPromptTemplateWithCustom("", "")

	if pt.GetSystemPrompt() != DefaultSystemPrompt {
		t.Errorf("Empty system prompt should fall back to default")
	}
	if pt.GetUserPrompt() != DefaultUserPromptTemplate {
		t.Errorf("Empty user prompt should fall back to default")
	}
}

func TestLangChainPromptTemplate_RenderMessages(t *testing.T) {
	pt := NewLangChainPromptTemplate()

	data := &PromptData{
		DiffStats: &git.DiffStats{
			TotalFiles:     2,
			TotalAdditions: 10,
			TotalDeletions: 5,
		},
		Chunks: []git.DiffChunk{
			{
				FilePath:   "test.go",
				ChangeType: git.ChangeTypeModified,
				Content:    "some diff content",
			},
		},
		RequiresChunking: false,
		PreviousAttempt:  "",
		CustomPrompt:     "",
	}

	messages, err := pt.RenderMessages(data)
	if err != nil {
		t.Fatalf("RenderMessages failed: %v", err)
	}

	// Should have exactly 2 messages: system and user
	if len(messages) != 2 {
		t.Fatalf("Expected 2 messages, got %d", len(messages))
	}

	// First message should be system
	if messages[0].Role != llms.ChatMessageTypeSystem {
		t.Errorf("First message should be system, got %v", messages[0].Role)
	}

	// Second message should be human/user
	if messages[1].Role != llms.ChatMessageTypeHuman {
		t.Errorf("Second message should be human, got %v", messages[1].Role)
	}

	// Verify system message contains expected content
	systemContent := getTextFromMessage(messages[0])
	if systemContent == "" {
		t.Error("System message should have content")
	}
	if !strings.Contains(systemContent, "Conventional Commits") {
		t.Error("System message should contain Conventional Commits instructions")
	}

	// Verify user message contains diff info
	userContent := getTextFromMessage(messages[1])
	if userContent == "" {
		t.Error("User message should have content")
	}
	if !strings.Contains(userContent, "test.go") {
		t.Error("User message should contain file path")
	}
}

func TestLangChainPromptTemplate_RenderMessages_WithCustomPrompt(t *testing.T) {
	pt := NewLangChainPromptTemplate()

	customPrompt := "My custom prompt for the AI"
	data := &PromptData{
		DiffStats: &git.DiffStats{
			TotalFiles:     1,
			TotalAdditions: 5,
			TotalDeletions: 2,
		},
		Chunks:       []git.DiffChunk{},
		CustomPrompt: customPrompt,
	}

	messages, err := pt.RenderMessages(data)
	if err != nil {
		t.Fatalf("RenderMessages failed: %v", err)
	}

	// User message should be the custom prompt
	userContent := getTextFromMessage(messages[1])
	if userContent != customPrompt {
		t.Errorf("Expected custom prompt %q, got %q", customPrompt, userContent)
	}
}

func TestLangChainPromptTemplate_RenderMessages_WithPreviousAttempt(t *testing.T) {
	pt := NewLangChainPromptTemplate()

	previousAttempt := "feat(test): previous attempt"
	data := &PromptData{
		DiffStats: &git.DiffStats{
			TotalFiles:     1,
			TotalAdditions: 5,
			TotalDeletions: 2,
		},
		Chunks: []git.DiffChunk{
			{FilePath: "test.go", ChangeType: git.ChangeTypeModified, Content: "diff"},
		},
		PreviousAttempt: previousAttempt,
	}

	messages, err := pt.RenderMessages(data)
	if err != nil {
		t.Fatalf("RenderMessages failed: %v", err)
	}

	// User message should contain previous attempt info
	userContent := getTextFromMessage(messages[1])
	if !strings.Contains(userContent, previousAttempt) {
		t.Errorf("User message should contain previous attempt: %s", userContent)
	}
}

func TestLangChainPromptTemplate_RenderMessages_WithChunking(t *testing.T) {
	pt := NewLangChainPromptTemplate()

	data := &PromptData{
		DiffStats: &git.DiffStats{
			TotalFiles:     10,
			TotalAdditions: 1000,
			TotalDeletions: 500,
		},
		Chunks: []git.DiffChunk{
			{FilePath: "file1.go", ChangeType: git.ChangeTypeAdded},
			{FilePath: "file2.go", ChangeType: git.ChangeTypeModified},
			{FilePath: "file3.go", ChangeType: git.ChangeTypeDeleted},
		},
		RequiresChunking: true,
	}

	messages, err := pt.RenderMessages(data)
	if err != nil {
		t.Fatalf("RenderMessages failed: %v", err)
	}

	// User message should indicate chunking was required
	userContent := getTextFromMessage(messages[1])
	if !strings.Contains(userContent, "Summary of changes") {
		t.Error("User message should indicate summary mode when chunking is required")
	}
}

// Helper function to extract text content from a message
func getTextFromMessage(msg llms.MessageContent) string {
	for _, part := range msg.Parts {
		if textPart, ok := part.(llms.TextContent); ok {
			return textPart.Text
		}
	}
	return ""
}
