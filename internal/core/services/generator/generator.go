package generator

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gitsage/gitsage/internal/core/domain"
	"github.com/gitsage/gitsage/internal/core/ports"
	"github.com/gitsage/gitsage/internal/core/services/cache"
	"github.com/gitsage/gitsage/internal/core/services/diff"
	"github.com/gitsage/gitsage/internal/core/services/prompt"
	"github.com/gitsage/gitsage/internal/core/services/retry"
)

// Service defines the interface for the commit message generator.
type Service interface {
	Generate(ctx context.Context, diff *domain.Diff, hint string) (*domain.CommitMessage, error)
	GenerateWithStream(ctx context.Context, diff *domain.Diff, hint string, onChunk func(chunk string)) (*domain.CommitMessage, error)
}
type SmartGenerator struct {
	aiModel       ports.AIModel
	promptBuilder *prompt.Builder
	diffProcessor diff.Processor
	retryStrategy retry.BackoffStrategy
	cache         *cache.LRUCache
	maxConcurrent int
	maxChunkSize  int
}

// NewSmartGenerator creates a new generator instance.
func NewSmartGenerator(ai ports.AIModel, pb *prompt.Builder, dp diff.Processor, rs retry.BackoffStrategy, c *cache.LRUCache) *SmartGenerator {
	return &SmartGenerator{
		aiModel:       ai,
		promptBuilder: pb,
		diffProcessor: dp,
		retryStrategy: rs,
		cache:         c,
		maxConcurrent: 3,          // Derived from requirements
		maxChunkSize:  100 * 1024, // 100KB, default chunk size
	}
}

// Generate orchestrates the commit message generation.
func (g *SmartGenerator) Generate(ctx context.Context, d *domain.Diff, hint string) (*domain.CommitMessage, error) {
	// Try cache first
	if g.cache != nil {
		// We need a stable representation of Diff for cache key.
		// For MVP, we use total content concatenation.
		// Note: This might be expensive for huge diffs, but usually acceptable.
		// Optimize later if needed.
		var contentBuilder strings.Builder
		for _, f := range d.Files {
			contentBuilder.WriteString(f.FilePath)
			contentBuilder.WriteString(f.Content)
		}
		key := cache.GenerateKey(contentBuilder.String(), hint)
		if cached, found := g.cache.Get(key); found {
			return cached, nil
		}
	}

	totalSize := g.calculateSize(d)
	requiresChunking := totalSize > 10*1024 // 10KB threshold

	var result *domain.CommitMessage
	var err error

	if !requiresChunking {
		// Use callWithRetry for direct generation
		result, err = g.callWithRetry(ctx, func() (*domain.CommitMessage, error) {
			return g.generateDirect(ctx, d, hint)
		})
	} else {
		result, err = g.generateTwoPhase(ctx, d, hint)
	}

	if err != nil {
		return nil, err
	}

	// Cache successful result
	if g.cache != nil && result != nil {
		var contentBuilder strings.Builder
		for _, f := range d.Files {
			contentBuilder.WriteString(f.FilePath)
			contentBuilder.WriteString(f.Content)
		}
		key := cache.GenerateKey(contentBuilder.String(), hint)
		g.cache.Set(key, result)
	}

	return result, nil
}

func (g *SmartGenerator) callWithRetry(ctx context.Context, op func() (*domain.CommitMessage, error)) (*domain.CommitMessage, error) {
	var err error
	for attempt := 0; attempt < 3; attempt++ {
		// Check context before trying
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		res, attemptErr := op()
		if attemptErr == nil {
			return res, nil
		}

		err = attemptErr
		// If error is not retryable (e.g. context canceled), stop
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		// Wait backoff
		if g.retryStrategy != nil {
			delay := g.retryStrategy.CalculateBackoff(attempt)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}
	}
	return nil, fmt.Errorf("max retries exceeded: %w", err)
}

func (g *SmartGenerator) generateDirect(ctx context.Context, d *domain.Diff, hint string) (*domain.CommitMessage, error) {
	// Build Prompts
	userPrompt, err := g.promptBuilder.BuildUserPrompt(d, hint)
	if err != nil {
		return nil, fmt.Errorf("failed to build prompt: %w", err)
	}
	systemPrompt := g.promptBuilder.GetSystemPrompt()

	// Call AI
	rawResponse, err := g.aiModel.GenerateCommitMessage(ctx, systemPrompt, userPrompt)
	if err != nil {
		return nil, err
	}

	return g.parseResponse(rawResponse), nil
}

// GenerateWithStream generates a commit message using streaming API.
// The onChunk callback is called for each chunk of text as it's received from AI.
func (g *SmartGenerator) GenerateWithStream(ctx context.Context, d *domain.Diff, hint string, onChunk func(chunk string)) (*domain.CommitMessage, error) {
	// Build Prompts
	userPrompt, err := g.promptBuilder.BuildUserPrompt(d, hint)
	if err != nil {
		return nil, fmt.Errorf("failed to build prompt: %w", err)
	}
	systemPrompt := g.promptBuilder.GetSystemPrompt()

	// Call AI with streaming
	rawResponse, err := g.aiModel.GenerateCommitMessageStream(ctx, systemPrompt, userPrompt, onChunk)
	if err != nil {
		return nil, err
	}

	return g.parseResponse(rawResponse), nil
}

func (g *SmartGenerator) generateTwoPhase(ctx context.Context, d *domain.Diff, hint string) (*domain.CommitMessage, error) {
	// Use DiffProcessor to chunk logic, which now supports hunk splitting
	chunks := g.diffProcessor.Chunk(d, g.maxChunkSize)

	summaries, err := g.generateSummaries(ctx, chunks)
	if err != nil {
		return nil, err
	}

	return g.generateFinalFromSummaries(ctx, summaries, hint)
}

func (g *SmartGenerator) generateSummaries(ctx context.Context, chunks []*domain.Diff) ([]string, error) {
	var (
		wg        sync.WaitGroup
		mu        sync.Mutex
		summaries = make([]string, len(chunks))
		errs      []error
		sem       = make(chan struct{}, g.maxConcurrent)
	)

	for i, chunk := range chunks {
		i, chunk := i, chunk
		wg.Add(1)

		go func() {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}

			// Build Summary Prompt
			summaryPrompt := g.promptBuilder.BuildSummaryPrompt(chunk)
			systemPrompt := "You are a helpful assistant that summarizes code changes concisely."

			rawResp, err := g.aiModel.GenerateCommitMessage(ctx, systemPrompt, summaryPrompt)

			mu.Lock()
			defer mu.Unlock()

			if err != nil {
				errs = append(errs, fmt.Errorf("chunk %d failed: %w", i, err))
				return
			}
			summaries[i] = rawResp
		}()
	}

	wg.Wait()

	if len(errs) > 0 {
		return nil, fmt.Errorf("summary generation failed: %v", errs[0])
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	return summaries, nil
}

func (g *SmartGenerator) generateFinalFromSummaries(ctx context.Context, summaries []string, hint string) (*domain.CommitMessage, error) {
	combinedSummary := strings.Join(summaries, "\n\n")

	aggregationPrompt := fmt.Sprintf(`Here are summaries of changes from multiple files:
	
%s

Based on these summaries, generate a single Conventional Commit message.`, combinedSummary)

	if hint != "" {
		aggregationPrompt += "\nHint: " + hint
	}

	systemPrompt := g.promptBuilder.GetSystemPrompt()

	rawResp, err := g.aiModel.GenerateCommitMessage(ctx, systemPrompt, aggregationPrompt)
	if err != nil {
		return nil, err
	}

	return g.parseResponse(rawResp), nil
}

func (g *SmartGenerator) parseResponse(raw string) *domain.CommitMessage {
	// Clean the response using our parser logic
	cleaned := cleanResponse(raw)

	msg := &domain.CommitMessage{
		Raw:     cleaned,
		Subject: "Generated Message",
		Body:    cleaned,
	}

	lines := strings.Split(cleaned, "\n")
	if len(lines) > 0 {
		msg.Subject = strings.TrimSpace(lines[0])
		if len(lines) > 1 {
			msg.Body = strings.TrimSpace(strings.Join(lines[1:], "\n"))
		}
	}
	// Fallback: if subject is empty but body isn't (rare), swap?
	// With Conventional Commits, usually first line is type(scope): ...

	// Basic validation/fixup could go here (e.g. remove trailing periods from subject)
	msg.Subject = strings.TrimSuffix(msg.Subject, ".")

	return msg
}

func (g *SmartGenerator) calculateSize(d *domain.Diff) int {
	total := 0
	for _, f := range d.Files {
		total += len(f.Content)
	}
	return total
}
