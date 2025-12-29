package ai

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/openai" // For now only OpenAI supported in MVP
)

// Adapter implements the ports.AIModel interface using langchaingo.
type Adapter struct {
	llm      llms.Model
	provider string
}

// Config holds configuration for the AI adapter.
type Config struct {
	ProviderName string
	APIKey       string
	BaseURL      string
	Model        string
	Temperature  float64
	MaxTokens    int
}

// NewAdapter creates a new AI Adapter.
func NewAdapter(cfg *Config) (*Adapter, error) {
	if cfg.APIKey == "" {
		return nil, errors.New("API key is required")
	}

	var llm llms.Model
	var err error

	// Currently supporting OpenAI-compatible providers (DeepSeek, OpenAI, etc.)
	opts := []openai.Option{
		openai.WithToken(cfg.APIKey),
		openai.WithModel(cfg.Model),
	}
	if cfg.BaseURL != "" {
		opts = append(opts, openai.WithBaseURL(cfg.BaseURL))
	}

	llm, err = openai.New(opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize AI provider: %w", err)
	}

	return &Adapter{
		llm:      llm,
		provider: cfg.ProviderName,
	}, nil
}

// GenerateCommitMessage sends the prompts to the LLM and returns the raw response.
func (a *Adapter) GenerateCommitMessage(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	messages := []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeSystem, systemPrompt),
		llms.TextParts(llms.ChatMessageTypeHuman, userPrompt),
	}

	resp, err := a.llm.GenerateContent(ctx, messages,
		llms.WithTemperature(0.2), // Low temp for deterministic code tasks
	)
	if err != nil {
		return "", fmt.Errorf("AI generation failed: %w", sanitizeError(err))
	}

	if len(resp.Choices) == 0 {
		return "", errors.New("empty response from AI model")
	}

	return resp.Choices[0].Content, nil
}

// GenerateCommitMessageStream generates a commit message with streaming support.
// The onChunk callback is called for each chunk of text as it's received.
func (a *Adapter) GenerateCommitMessageStream(ctx context.Context, systemPrompt, userPrompt string, onChunk func(chunk string)) (string, error) {
	messages := []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeSystem, systemPrompt),
		llms.TextParts(llms.ChatMessageTypeHuman, userPrompt),
	}

	var fullResponse strings.Builder

	resp, err := a.llm.GenerateContent(ctx, messages,
		llms.WithTemperature(0.2),
		llms.WithStreamingFunc(func(ctx context.Context, chunk []byte) error {
			chunkStr := string(chunk)
			fullResponse.WriteString(chunkStr)
			if onChunk != nil {
				onChunk(chunkStr)
			}
			return nil
		}),
	)
	if err != nil {
		return "", fmt.Errorf("AI generation failed: %w", sanitizeError(err))
	}

	// If streaming worked, fullResponse has the content
	// Otherwise, fall back to resp.Choices
	result := fullResponse.String()
	if result == "" && len(resp.Choices) > 0 {
		result = resp.Choices[0].Content
	}

	return result, nil
}

// apiKeyPattern matches common API key patterns.
var apiKeyPattern = regexp.MustCompile(`sk-[a-zA-Z0-9]{20,}`)

func sanitizeError(err error) error {
	msg := err.Error()
	sanitized := apiKeyPattern.ReplaceAllStringFunc(msg, func(match string) string {
		if len(match) <= 7 {
			return "sk-****"
		}
		return match[:3] + "..." + match[len(match)-4:]
	})

	if sanitized != msg {
		return errors.New(sanitized)
	}
	return err
}
