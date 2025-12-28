// Package ai provides AI provider interfaces and implementations for GitSage.
package ai

import (
	"strings"
	"testing"

	"github.com/gitsage/gitsage/internal/pkg/git"
	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	"github.com/tmc/langchaingo/llms"
)

// Feature: langchaingo-refactor, Property 2: Prompt Rendering Equivalence
// Validates: Requirements 3.4, 3.5
//
// For any PromptData input (including custom prompts), the rendered prompt content
// from the LangChain template system SHALL be semantically equivalent to the
// original Go template rendering.

// genPromptDataForProperty2 generates random PromptData for testing prompt rendering equivalence.
// This generator creates PromptData without CustomPrompt to test template rendering.
func genPromptDataForProperty2() gopter.Gen {
	return gopter.CombineGens(
		genDiffStats(),
		gen.SliceOfN(5, genDiffChunk()),
		gen.Bool(),
		gen.AlphaString(),
	).Map(func(values []interface{}) *PromptData {
		return &PromptData{
			DiffStats:        values[0].(*git.DiffStats),
			Chunks:           values[1].([]git.DiffChunk),
			RequiresChunking: values[2].(bool),
			PreviousAttempt:  values[3].(string),
			CustomPrompt:     "", // Don't use custom prompt for template rendering comparison
		}
	})
}

// genPromptDataWithCustomPrompt generates random PromptData with a custom prompt.
func genPromptDataWithCustomPrompt() gopter.Gen {
	return gopter.CombineGens(
		genDiffStats(),
		gen.SliceOfN(3, genDiffChunk()),
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
	).Map(func(values []interface{}) *PromptData {
		return &PromptData{
			DiffStats:        values[0].(*git.DiffStats),
			Chunks:           values[1].([]git.DiffChunk),
			RequiresChunking: false,
			PreviousAttempt:  "",
			CustomPrompt:     values[2].(string), // Non-empty custom prompt
		}
	})
}

// TestProperty_PromptRenderingEquivalence verifies that the LangChain prompt template
// renders content equivalent to the original Go template implementation.
//
// Feature: langchaingo-refactor, Property 2: Prompt Rendering Equivalence
// Validates: Requirements 3.4, 3.5
func TestProperty_PromptRenderingEquivalence(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	// Property: LangChain system prompt equals original system prompt
	properties.Property("LangChain system prompt equals original", prop.ForAll(
		func(data *PromptData) bool {
			originalPT := NewPromptTemplate()
			langchainPT := NewLangChainPromptTemplate()

			originalSystem := originalPT.GetSystemPrompt()
			langchainSystem := langchainPT.GetSystemPrompt()

			return originalSystem == langchainSystem
		},
		genPromptDataForProperty2(),
	))

	// Property: LangChain user prompt rendering is equivalent to original for template-based prompts
	properties.Property("LangChain user prompt equals original for template rendering", prop.ForAll(
		func(data *PromptData) bool {
			originalPT := NewPromptTemplate()
			langchainPT := NewLangChainPromptTemplate()

			// Render with original template
			originalUserPrompt, err := originalPT.RenderUserPrompt(data)
			if err != nil {
				t.Logf("Original template render error: %v", err)
				return false
			}

			// Render with LangChain template
			messages, err := langchainPT.RenderMessages(data)
			if err != nil {
				t.Logf("LangChain template render error: %v", err)
				return false
			}

			// Extract user message content
			langchainUserPrompt := extractUserMessageContent(messages)

			// Compare (they should be equal)
			return originalUserPrompt == langchainUserPrompt
		},
		genPromptDataForProperty2(),
	))

	// Property: Custom prompts bypass template rendering in both implementations
	properties.Property("custom prompt directly used as user message", prop.ForAll(
		func(data *PromptData) bool {
			originalPT := NewPromptTemplate()
			langchainPT := NewLangChainPromptTemplate()

			// Render with original template
			originalUserPrompt, err := originalPT.RenderUserPrompt(data)
			if err != nil {
				return false
			}

			// Render with LangChain template
			messages, err := langchainPT.RenderMessages(data)
			if err != nil {
				return false
			}

			langchainUserPrompt := extractUserMessageContent(messages)

			// Both should return the custom prompt directly
			return originalUserPrompt == data.CustomPrompt &&
				langchainUserPrompt == data.CustomPrompt
		},
		genPromptDataWithCustomPrompt(),
	))

	// Property: RenderMessages always returns exactly 2 messages (system + user)
	properties.Property("RenderMessages returns exactly 2 messages", prop.ForAll(
		func(data *PromptData) bool {
			langchainPT := NewLangChainPromptTemplate()
			messages, err := langchainPT.RenderMessages(data)
			if err != nil {
				return false
			}
			return len(messages) == 2
		},
		genPromptDataForProperty2(),
	))

	// Property: First message is always system type, second is always human type
	properties.Property("message types are system then human", prop.ForAll(
		func(data *PromptData) bool {
			langchainPT := NewLangChainPromptTemplate()
			messages, err := langchainPT.RenderMessages(data)
			if err != nil {
				return false
			}
			if len(messages) != 2 {
				return false
			}
			return messages[0].Role == llms.ChatMessageTypeSystem &&
				messages[1].Role == llms.ChatMessageTypeHuman
		},
		genPromptDataForProperty2(),
	))

	// Property: Custom templates maintain equivalence between implementations
	properties.Property("custom templates are equivalent between implementations", prop.ForAll(
		func(customSystem, customUser string) bool {
			// Skip empty strings as they fall back to defaults
			if customSystem == "" && customUser == "" {
				return true
			}

			originalPT := NewPromptTemplateWithCustom(customSystem, customUser)
			langchainPT := NewLangChainPromptTemplateWithCustom(customSystem, customUser)

			// System prompts should match
			if originalPT.GetSystemPrompt() != langchainPT.GetSystemPrompt() {
				return false
			}

			// User template strings should match
			return originalPT.UserPrompt == langchainPT.GetUserPrompt()
		},
		gen.AlphaString(),
		gen.AlphaString(),
	))

	// Property: Rendered user prompt contains file paths when RequiresChunking is false
	properties.Property("rendered prompt contains file paths when not chunking", prop.ForAll(
		func(stats *git.DiffStats, chunks []git.DiffChunk) bool {
			if len(chunks) == 0 {
				return true // Skip empty chunks
			}

			data := &PromptData{
				DiffStats:        stats,
				Chunks:           chunks,
				RequiresChunking: false,
				CustomPrompt:     "",
			}

			langchainPT := NewLangChainPromptTemplate()
			messages, err := langchainPT.RenderMessages(data)
			if err != nil {
				return false
			}

			userContent := extractUserMessageContent(messages)

			// At least one file path should be in the rendered content
			for _, chunk := range chunks {
				if strings.Contains(userContent, chunk.FilePath) {
					return true
				}
			}
			return false
		},
		genDiffStats(),
		gen.SliceOfN(3, genDiffChunk()).SuchThat(func(chunks []git.DiffChunk) bool {
			return len(chunks) > 0
		}),
	))

	// Property: Rendered user prompt contains previous attempt when provided
	properties.Property("rendered prompt contains previous attempt", prop.ForAll(
		func(data *PromptData, previousAttempt string) bool {
			if previousAttempt == "" {
				return true // Skip empty previous attempts
			}

			data.PreviousAttempt = previousAttempt
			data.CustomPrompt = "" // Ensure we use template rendering

			langchainPT := NewLangChainPromptTemplate()
			messages, err := langchainPT.RenderMessages(data)
			if err != nil {
				return false
			}

			userContent := extractUserMessageContent(messages)
			return strings.Contains(userContent, previousAttempt)
		},
		genPromptDataForProperty2(),
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
	))

	properties.TestingRun(t)
}

// extractUserMessageContent extracts the text content from the user (human) message.
func extractUserMessageContent(messages []llms.MessageContent) string {
	if len(messages) < 2 {
		return ""
	}

	for _, part := range messages[1].Parts {
		if textPart, ok := part.(llms.TextContent); ok {
			return textPart.Text
		}
	}
	return ""
}
