// Package ai provides AI provider interfaces and implementations for GitSage.
package ai

import (
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/prompts"
)

// LangChainPromptTemplate manages prompt templates using LangChain's prompts package.
// It provides a unified interface for rendering system and user prompts into
// LangChain message format ([]llms.MessageContent).
type LangChainPromptTemplate struct {
	systemPrompt prompts.PromptTemplate
	userPrompt   prompts.PromptTemplate
}

// NewLangChainPromptTemplate creates a new LangChainPromptTemplate with default prompts.
// The default prompts are designed for generating Conventional Commits messages.
func NewLangChainPromptTemplate() *LangChainPromptTemplate {
	return &LangChainPromptTemplate{
		systemPrompt: prompts.NewPromptTemplate(
			DefaultSystemPrompt,
			nil, // System prompt has no input variables
		),
		userPrompt: prompts.NewPromptTemplate(
			DefaultUserPromptTemplate,
			[]string{"DiffStats", "Chunks", "RequiresChunking", "PreviousAttempt", "CustomPrompt"},
		),
	}
}

// NewLangChainPromptTemplateWithCustom creates a new LangChainPromptTemplate with custom prompts.
// If systemPrompt or userPrompt is empty, the default is used.
//
// Parameters:
//   - systemPrompt: Custom system prompt text. If empty, uses DefaultSystemPrompt.
//   - userPrompt: Custom user prompt template. If empty, uses DefaultUserPromptTemplate.
//
// The custom userPrompt should use Go template syntax ({{.VariableName}}) and can reference:
//   - {{.DiffStats}}: Statistics about the diff (files, additions, deletions)
//   - {{.Chunks}}: The diff chunks containing file changes
//   - {{.RequiresChunking}}: Boolean indicating if diff was too large
//   - {{.PreviousAttempt}}: Previous rejected commit message (if regenerating)
//   - {{.CustomPrompt}}: User-provided custom prompt override
func NewLangChainPromptTemplateWithCustom(systemPrompt, userPrompt string) *LangChainPromptTemplate {
	sysPrompt := DefaultSystemPrompt
	if systemPrompt != "" {
		sysPrompt = systemPrompt
	}

	usrPrompt := DefaultUserPromptTemplate
	if userPrompt != "" {
		usrPrompt = userPrompt
	}

	return &LangChainPromptTemplate{
		systemPrompt: prompts.NewPromptTemplate(
			sysPrompt,
			nil,
		),
		userPrompt: prompts.NewPromptTemplate(
			usrPrompt,
			[]string{"DiffStats", "Chunks", "RequiresChunking", "PreviousAttempt", "CustomPrompt"},
		),
	}
}

// RenderMessages renders the prompt templates with the given data and returns
// LangChain message content suitable for llm.GenerateContent().
//
// The returned messages follow the standard chat format:
//   - First message: System message with role instructions
//   - Second message: Human/User message with the actual prompt and diff data
//
// If CustomPrompt is set in the PromptData, it is used directly as the user message
// instead of rendering the user prompt template.
func (pt *LangChainPromptTemplate) RenderMessages(data *PromptData) ([]llms.MessageContent, error) {
	// Render system prompt (no variables needed)
	systemContent, err := pt.systemPrompt.Format(nil)
	if err != nil {
		return nil, err
	}

	// If custom prompt is provided, use it directly
	var userContent string
	if data.CustomPrompt != "" {
		userContent = data.CustomPrompt
	} else {
		// Render user prompt with template variables
		userContent, err = pt.userPrompt.Format(map[string]any{
			"DiffStats":        data.DiffStats,
			"Chunks":           data.Chunks,
			"RequiresChunking": data.RequiresChunking,
			"PreviousAttempt":  data.PreviousAttempt,
			"CustomPrompt":     data.CustomPrompt,
		})
		if err != nil {
			return nil, err
		}
	}

	// Build LangChain message content
	return []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeSystem, systemContent),
		llms.TextParts(llms.ChatMessageTypeHuman, userContent),
	}, nil
}

// GetSystemPrompt returns the raw system prompt template string.
func (pt *LangChainPromptTemplate) GetSystemPrompt() string {
	return pt.systemPrompt.Template
}

// GetUserPrompt returns the raw user prompt template string.
func (pt *LangChainPromptTemplate) GetUserPrompt() string {
	return pt.userPrompt.Template
}
