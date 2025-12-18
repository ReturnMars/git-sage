package ai

import (
	"strings"
	"testing"

	"github.com/gitsage/gitsage/internal/pkg/git"
)

func TestNewPromptTemplate(t *testing.T) {
	pt := NewPromptTemplate()

	if pt.SystemPrompt == "" {
		t.Error("SystemPrompt should not be empty")
	}
	if pt.UserPrompt == "" {
		t.Error("UserPrompt should not be empty")
	}
}

func TestNewPromptTemplateWithCustom(t *testing.T) {
	customSystem := "Custom system prompt"
	customUser := "Custom user prompt"

	pt := NewPromptTemplateWithCustom(customSystem, customUser)

	if pt.SystemPrompt != customSystem {
		t.Errorf("SystemPrompt = %q, want %q", pt.SystemPrompt, customSystem)
	}
	if pt.UserPrompt != customUser {
		t.Errorf("UserPrompt = %q, want %q", pt.UserPrompt, customUser)
	}
}

func TestNewPromptTemplateWithCustom_EmptyFallsBackToDefault(t *testing.T) {
	pt := NewPromptTemplateWithCustom("", "")

	if pt.SystemPrompt != DefaultSystemPrompt {
		t.Error("Empty system prompt should fall back to default")
	}
	if pt.UserPrompt != DefaultUserPromptTemplate {
		t.Error("Empty user prompt should fall back to default")
	}
}

func TestPromptTemplate_RenderUserPrompt(t *testing.T) {
	pt := NewPromptTemplate()

	data := &PromptData{
		DiffStats: &git.DiffStats{
			TotalFiles:     2,
			TotalAdditions: 10,
			TotalDeletions: 5,
		},
		Chunks: []git.DiffChunk{
			{
				FilePath:   "main.go",
				ChangeType: git.ChangeTypeModified,
				Additions:  5,
				Deletions:  2,
				Content:    "diff content here",
			},
		},
		RequiresChunking: false,
	}

	result, err := pt.RenderUserPrompt(data)
	if err != nil {
		t.Fatalf("RenderUserPrompt() error = %v", err)
	}

	// Check that the result contains expected content
	if !strings.Contains(result, "Files changed: 2") {
		t.Error("Result should contain file count")
	}
	if !strings.Contains(result, "Additions: 10") {
		t.Error("Result should contain additions count")
	}
	if !strings.Contains(result, "diff content here") {
		t.Error("Result should contain diff content")
	}
}

func TestPromptTemplate_RenderUserPrompt_WithChunking(t *testing.T) {
	pt := NewPromptTemplate()

	data := &PromptData{
		DiffStats: &git.DiffStats{
			TotalFiles:     2,
			TotalAdditions: 100,
			TotalDeletions: 50,
		},
		Chunks: []git.DiffChunk{
			{
				FilePath:   "main.go",
				ChangeType: git.ChangeTypeModified,
				Additions:  50,
				Deletions:  25,
			},
			{
				FilePath:   "util.go",
				ChangeType: git.ChangeTypeAdded,
				Additions:  50,
				Deletions:  25,
			},
		},
		RequiresChunking: true,
	}

	result, err := pt.RenderUserPrompt(data)
	if err != nil {
		t.Fatalf("RenderUserPrompt() error = %v", err)
	}

	// When chunking is required, should show summary instead of full diff
	if !strings.Contains(result, "Summary of changes") {
		t.Error("Result should contain summary header when chunking")
	}
	if !strings.Contains(result, "main.go") {
		t.Error("Result should contain file paths")
	}
}

func TestPromptTemplate_RenderUserPrompt_CustomPrompt(t *testing.T) {
	pt := NewPromptTemplate()

	customPrompt := "Generate a commit message for: test changes"
	data := &PromptData{
		CustomPrompt: customPrompt,
	}

	result, err := pt.RenderUserPrompt(data)
	if err != nil {
		t.Fatalf("RenderUserPrompt() error = %v", err)
	}

	if result != customPrompt {
		t.Errorf("Result = %q, want %q", result, customPrompt)
	}
}

func TestPromptTemplate_RenderUserPrompt_WithPreviousAttempt(t *testing.T) {
	pt := NewPromptTemplate()

	data := &PromptData{
		DiffStats: &git.DiffStats{
			TotalFiles: 1,
		},
		Chunks: []git.DiffChunk{
			{
				FilePath: "test.go",
				Content:  "test diff",
			},
		},
		PreviousAttempt: "feat: previous attempt message",
	}

	result, err := pt.RenderUserPrompt(data)
	if err != nil {
		t.Fatalf("RenderUserPrompt() error = %v", err)
	}

	if !strings.Contains(result, "Previous attempt") {
		t.Error("Result should contain previous attempt section")
	}
	if !strings.Contains(result, "feat: previous attempt message") {
		t.Error("Result should contain the previous attempt message")
	}
}

func TestBuildPromptData(t *testing.T) {
	req := &GenerateRequest{
		DiffChunks: []git.DiffChunk{
			{FilePath: "test.go"},
		},
		DiffStats: &git.DiffStats{
			TotalFiles: 1,
		},
		CustomPrompt:    "custom",
		PreviousAttempt: "previous",
	}

	data := BuildPromptData(req, true)

	if data.DiffStats != req.DiffStats {
		t.Error("DiffStats should match")
	}
	if len(data.Chunks) != len(req.DiffChunks) {
		t.Error("Chunks should match")
	}
	if !data.RequiresChunking {
		t.Error("RequiresChunking should be true")
	}
	if data.CustomPrompt != "custom" {
		t.Error("CustomPrompt should match")
	}
	if data.PreviousAttempt != "previous" {
		t.Error("PreviousAttempt should match")
	}
}

func TestDefaultSystemPrompt_ContainsConventionalCommitsInstructions(t *testing.T) {
	// Verify that the system prompt contains instructions for Conventional Commits
	// This validates Requirements 4.3
	if !strings.Contains(DefaultSystemPrompt, "Conventional Commits") {
		t.Error("System prompt should mention Conventional Commits format")
	}
	if !strings.Contains(DefaultSystemPrompt, "feat") {
		t.Error("System prompt should list valid commit types")
	}
	if !strings.Contains(DefaultSystemPrompt, "fix") {
		t.Error("System prompt should list valid commit types")
	}
	if !strings.Contains(DefaultSystemPrompt, "<type>") {
		t.Error("System prompt should describe the format")
	}
}
