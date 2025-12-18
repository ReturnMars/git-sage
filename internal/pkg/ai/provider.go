// Package ai provides AI provider interfaces and implementations for GitSage.
package ai

import (
	"context"

	"github.com/gitsage/gitsage/internal/pkg/git"
)

// GenerateRequest contains the data needed to generate a commit message.
type GenerateRequest struct {
	DiffChunks      []git.DiffChunk
	DiffStats       *git.DiffStats
	CustomPrompt    string
	PreviousAttempt string
}

// GenerateResponse contains the generated commit message.
type GenerateResponse struct {
	Subject string
	Body    string
	Footer  string
	RawText string
}

// ProviderConfig contains configuration for an AI provider.
type ProviderConfig struct {
	APIKey      string
	Model       string
	Endpoint    string
	Temperature float32
	MaxTokens   int
}

// Provider defines the interface for AI providers.
type Provider interface {
	GenerateCommitMessage(ctx context.Context, req *GenerateRequest) (*GenerateResponse, error)
	Name() string
	ValidateConfig(config ProviderConfig) error
}
