// Package security provides security utilities for GitSage.
package security

import (
	"fmt"
	"regexp"
	"strings"
)

// APIKeyFormat defines the expected format patterns for different providers.
var APIKeyFormat = map[string]*regexp.Regexp{
	"openai":   regexp.MustCompile(`^sk-[a-zA-Z0-9]{20,}$`),
	"deepseek": regexp.MustCompile(`^sk-[a-zA-Z0-9]{20,}$`),
	"ollama":   nil, // Ollama doesn't require API key
}

// MaskAPIKey masks an API key, showing only the last 4 characters.
// This should be used when logging or displaying API keys.
func MaskAPIKey(key string) string {
	if len(key) <= 4 {
		return "****"
	}
	return strings.Repeat("*", len(key)-4) + key[len(key)-4:]
}

// ValidateAPIKeyFormat validates the format of an API key for a given provider.
// Returns nil if the key format is valid, or an error describing the issue.
func ValidateAPIKeyFormat(provider, apiKey string) error {
	// Ollama doesn't require API key
	if provider == "ollama" {
		return nil
	}

	if apiKey == "" {
		return fmt.Errorf("API key is required for %s provider", provider)
	}

	// Check minimum length
	if len(apiKey) < 20 {
		return fmt.Errorf("API key appears to be invalid (too short)")
	}

	// Check format if we have a pattern for this provider
	pattern, exists := APIKeyFormat[provider]
	if exists && pattern != nil {
		if !pattern.MatchString(apiKey) {
			return fmt.Errorf("API key format appears invalid for %s provider (expected format: sk-...)", provider)
		}
	}

	return nil
}

// SanitizeForLogging sanitizes a string for safe logging by masking potential secrets.
// It looks for common patterns like API keys, passwords, and tokens.
func SanitizeForLogging(s string) string {
	// Patterns to mask
	patterns := []struct {
		regex       *regexp.Regexp
		replacement string
	}{
		// API keys (sk-...)
		{regexp.MustCompile(`sk-[a-zA-Z0-9]{20,}`), "sk-****"},
		// Bearer tokens
		{regexp.MustCompile(`Bearer\s+[a-zA-Z0-9._-]+`), "Bearer ****"},
		// Generic API key patterns
		{regexp.MustCompile(`(?i)(api[_-]?key|apikey|api_secret|secret[_-]?key)\s*[:=]\s*["']?[a-zA-Z0-9._-]+["']?`), "$1=****"},
		// Password patterns
		{regexp.MustCompile(`(?i)(password|passwd|pwd)\s*[:=]\s*["']?[^\s"']+["']?`), "$1=****"},
	}

	result := s
	for _, p := range patterns {
		result = p.regex.ReplaceAllString(result, p.replacement)
	}

	return result
}

// FirstUseWarning is the warning message displayed on first use.
const FirstUseWarning = `
⚠️  IMPORTANT SECURITY NOTICE ⚠️

GitSage sends your staged git diff content to external AI services
(OpenAI, DeepSeek, or other configured providers) to generate commit messages.

This means your code changes will be transmitted over the internet to third-party
servers. Please ensure you:

1. Do not stage sensitive information (API keys, passwords, secrets)
2. Review your staged changes before running GitSage
3. Consider using a local AI provider (Ollama) for sensitive projects

For more information, see: https://github.com/gitsage/gitsage#security

`

// FirstUseAcknowledgment is the message shown after user acknowledges the warning.
const FirstUseAcknowledgment = "Thank you for acknowledging the security notice. This warning will not be shown again."
