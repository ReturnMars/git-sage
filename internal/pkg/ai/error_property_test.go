// Package ai provides AI provider interfaces and implementations for GitSage.
package ai

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// Feature: langchaingo-refactor, Property 4: Error Wrapping Consistency
// Validates: Requirements 4.3
//
// For any error returned by the LangChain LLM call, the error wrapping logic
// SHALL produce user-friendly error messages consistent with the original implementation's error handling.

// genErrorMessage generates random error messages for testing.
func genErrorMessage() gopter.Gen {
	return gen.OneConstOf(
		"401 Unauthorized",
		"unauthorized access",
		"Unauthorized request",
		"429 Too Many Requests",
		"rate limit exceeded",
		"too many requests",
		"500 Internal Server Error",
		"502 Bad Gateway",
		"503 Service Unavailable",
		"504 Gateway Timeout",
		"connection refused",
		"connect: connection refused",
		"some random error",
		"network error occurred",
		"timeout error",
	)
}

// genProviderName generates random provider names.
func genProviderName() gopter.Gen {
	return gen.OneConstOf("openai", "deepseek", "ollama", "test-provider")
}

// TestProperty_ErrorWrappingConsistency verifies that error wrapping produces
// user-friendly messages consistently.
//
// Feature: langchaingo-refactor, Property 4: Error Wrapping Consistency
// Validates: Requirements 4.3
func TestProperty_ErrorWrappingConsistency(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	// Property: wrapError never returns nil for non-nil input
	properties.Property("wrapError returns non-nil for non-nil input", prop.ForAll(
		func(errMsg string, providerName string) bool {
			wrapper := NewLangChainWrapper(&MockLLM{}, ProviderConfig{}, providerName)
			err := errors.New(errMsg)
			wrapped := wrapper.wrapError(err)
			return wrapped != nil
		},
		genErrorMessage(),
		genProviderName(),
	))

	// Property: wrapError returns nil for nil input
	properties.Property("wrapError returns nil for nil input", prop.ForAll(
		func(providerName string) bool {
			wrapper := NewLangChainWrapper(&MockLLM{}, ProviderConfig{}, providerName)
			wrapped := wrapper.wrapError(nil)
			return wrapped == nil
		},
		genProviderName(),
	))

	// Property: 401 errors produce authentication-related messages
	properties.Property("401 errors produce authentication messages", prop.ForAll(
		func(providerName string) bool {
			wrapper := NewLangChainWrapper(&MockLLM{}, ProviderConfig{}, providerName)

			authErrors := []string{"401 Unauthorized", "unauthorized", "Unauthorized"}
			for _, errMsg := range authErrors {
				err := errors.New(errMsg)
				wrapped := wrapper.wrapError(err)
				if wrapped == nil {
					return false
				}
				wrappedStr := strings.ToLower(wrapped.Error())
				if !strings.Contains(wrappedStr, "authentication") {
					return false
				}
			}
			return true
		},
		genProviderName(),
	))

	// Property: 429 errors produce rate-limit-related messages
	properties.Property("429 errors produce rate limit messages", prop.ForAll(
		func(providerName string) bool {
			wrapper := NewLangChainWrapper(&MockLLM{}, ProviderConfig{}, providerName)

			rateLimitErrors := []string{"429 Too Many Requests", "rate limit", "too many requests"}
			for _, errMsg := range rateLimitErrors {
				err := errors.New(errMsg)
				wrapped := wrapper.wrapError(err)
				if wrapped == nil {
					return false
				}
				wrappedStr := strings.ToLower(wrapped.Error())
				if !strings.Contains(wrappedStr, "rate limit") {
					return false
				}
			}
			return true
		},
		genProviderName(),
	))

	// Property: Connection refused errors mention the provider
	properties.Property("connection refused errors mention provider", prop.ForAll(
		func(providerName string) bool {
			wrapper := NewLangChainWrapper(&MockLLM{}, ProviderConfig{}, providerName)
			err := errors.New("connection refused")
			wrapped := wrapper.wrapError(err)
			if wrapped == nil {
				return false
			}
			wrappedStr := strings.ToLower(wrapped.Error())
			return strings.Contains(wrappedStr, providerName) || strings.Contains(wrappedStr, "connect")
		},
		genProviderName(),
	))

	// Property: Timeout errors produce timeout-related messages
	properties.Property("timeout errors produce timeout messages", prop.ForAll(
		func(providerName string) bool {
			wrapper := NewLangChainWrapper(&MockLLM{}, ProviderConfig{}, providerName)
			wrapped := wrapper.wrapError(context.DeadlineExceeded)
			if wrapped == nil {
				return false
			}
			wrappedStr := strings.ToLower(wrapped.Error())
			return strings.Contains(wrappedStr, "timeout") || strings.Contains(wrappedStr, "timed out")
		},
		genProviderName(),
	))

	// Property: Generic errors include provider name
	properties.Property("generic errors include provider context", prop.ForAll(
		func(providerName string) bool {
			wrapper := NewLangChainWrapper(&MockLLM{}, ProviderConfig{}, providerName)
			err := errors.New("some random unrecognized error")
			wrapped := wrapper.wrapError(err)
			if wrapped == nil {
				return false
			}
			// Generic errors should mention the provider
			wrappedStr := strings.ToLower(wrapped.Error())
			return strings.Contains(wrappedStr, strings.ToLower(providerName))
		},
		genProviderName(),
	))

	// Property: isRetryableError correctly identifies retryable errors
	properties.Property("retryable errors are correctly identified", prop.ForAll(
		func(providerName string) bool {
			wrapper := NewLangChainWrapper(&MockLLM{}, ProviderConfig{}, providerName)

			// These should be retryable
			retryableErrors := []error{
				errors.New("429 Too Many Requests"),
				errors.New("500 Internal Server Error"),
				errors.New("502 Bad Gateway"),
				errors.New("503 Service Unavailable"),
				errors.New("504 Gateway Timeout"),
				errors.New("rate limit exceeded"),
				context.DeadlineExceeded,
			}

			for _, err := range retryableErrors {
				if !wrapper.isRetryableError(err) {
					return false
				}
			}

			// These should NOT be retryable
			nonRetryableErrors := []error{
				errors.New("401 Unauthorized"),
				errors.New("400 Bad Request"),
				errors.New("404 Not Found"),
				errors.New("some random error"),
				nil,
			}

			for _, err := range nonRetryableErrors {
				if wrapper.isRetryableError(err) {
					return false
				}
			}

			return true
		},
		genProviderName(),
	))

	properties.TestingRun(t)
}
