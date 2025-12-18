// Package app contains the application layer with business orchestration logic.
package app

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gitsage/gitsage/internal/pkg/ai"
	"github.com/gitsage/gitsage/internal/pkg/cache"
	"github.com/gitsage/gitsage/internal/pkg/config"
	"github.com/gitsage/gitsage/internal/pkg/git"
	"github.com/gitsage/gitsage/internal/pkg/history"
	"github.com/gitsage/gitsage/internal/pkg/message"
	"github.com/gitsage/gitsage/internal/pkg/processor"
	"github.com/gitsage/gitsage/internal/pkg/ui"
)

// writeFile is a variable to allow mocking in tests.
var writeFile = os.WriteFile

// MaxRegenerationAttempts is the maximum number of times a user can regenerate a commit message.
const MaxRegenerationAttempts = 5

// MaxConcurrentAICalls is the maximum number of concurrent AI calls for chunk processing.
const MaxConcurrentAICalls = 3

// CommitOptions contains options for the commit workflow.
type CommitOptions struct {
	DryRun       bool
	OutputFile   string
	SkipConfirm  bool
	CustomPrompt string
	NoCache      bool
}

// CommitService orchestrates the commit message generation workflow.
type CommitService struct {
	gitClient     git.Client
	aiProvider    ai.Provider
	diffProcessor processor.DiffProcessor
	uiManager     ui.Manager
	historyMgr    history.Manager
	config        *config.Config
	cache         cache.Manager
}

// NewCommitService creates a new CommitService with the given dependencies.
func NewCommitService(
	gitClient git.Client,
	aiProvider ai.Provider,
	diffProcessor processor.DiffProcessor,
	uiManager ui.Manager,
	historyMgr history.Manager,
	cfg *config.Config,
) *CommitService {
	// Initialize cache if enabled
	var cacheManager cache.Manager
	if cfg != nil && cfg.Cache.Enabled {
		ttl := time.Duration(cfg.Cache.TTLMinutes) * time.Minute
		if ttl <= 0 {
			ttl = cache.DefaultTTL
		}
		maxEntries := cfg.Cache.MaxEntries
		if maxEntries <= 0 {
			maxEntries = cache.DefaultMaxEntries
		}
		cacheManager = cache.NewLRUCache(maxEntries, ttl)
	}

	return &CommitService{
		gitClient:     gitClient,
		aiProvider:    aiProvider,
		diffProcessor: diffProcessor,
		uiManager:     uiManager,
		historyMgr:    historyMgr,
		config:        cfg,
		cache:         cacheManager,
	}
}

// GenerateAndCommit orchestrates the complete commit message generation workflow.
// Workflow: check staged → get diff → process → generate → display → handle action → commit/save
func (s *CommitService) GenerateAndCommit(ctx context.Context, opts *CommitOptions) error {
	if opts == nil {
		opts = &CommitOptions{}
	}

	// Step 1: Check for staged changes
	hasChanges, err := s.gitClient.HasStagedChanges(ctx)
	if err != nil {
		return fmt.Errorf("failed to check staged changes: %w", err)
	}
	if !hasChanges {
		return fmt.Errorf("no staged changes found. Use 'git add' to stage changes before generating a commit message")
	}

	// Step 2: Get diff and stats
	spinner := s.uiManager.ShowSpinner("Retrieving staged changes...")
	spinner.Start()

	diffChunks, err := s.gitClient.GetStagedDiff(ctx)
	if err != nil {
		spinner.Stop()
		return fmt.Errorf("failed to get staged diff: %w", err)
	}

	diffStats, err := s.gitClient.GetDiffStats(ctx)
	if err != nil {
		spinner.Stop()
		return fmt.Errorf("failed to get diff stats: %w", err)
	}

	spinner.Stop()

	// Step 3: Process diff (filter lock files, chunk if needed)
	spinner = s.uiManager.ShowSpinner("Processing diff...")
	spinner.Start()

	processedDiff, err := s.diffProcessor.Process(ctx, diffChunks)
	if err != nil {
		spinner.Stop()
		return fmt.Errorf("failed to process diff: %w", err)
	}

	spinner.Stop()

	// Check if there are any changes left after filtering
	if len(processedDiff.Chunks) == 0 {
		return fmt.Errorf("no changes to commit after filtering lock files")
	}

	// Step 4-7: Generate, display, handle action loop with regeneration support
	return s.generateAndHandleLoop(ctx, opts, processedDiff, diffStats)
}

// generateAndHandleLoop handles the generate → display → action loop with regeneration support.
func (s *CommitService) generateAndHandleLoop(
	ctx context.Context,
	opts *CommitOptions,
	processedDiff *processor.ProcessedDiff,
	diffStats *git.DiffStats,
) error {
	var previousAttempt string
	regenerationCount := 0

	for {
		// Step 4: Generate commit message via AI
		response, err := s.generateCommitMessage(ctx, processedDiff, diffStats, opts.CustomPrompt, previousAttempt, opts.NoCache)
		if err != nil {
			return fmt.Errorf("failed to generate commit message: %w", err)
		}

		// Step 5: Display in interactive UI
		if err := s.uiManager.DisplayMessage(response); err != nil {
			return fmt.Errorf("failed to display message: %w", err)
		}

		// Validate and show warnings
		s.validateAndWarn(response)

		// Step 6: Handle user action
		action, err := s.uiManager.PromptAction()
		if err != nil {
			return fmt.Errorf("failed to get user action: %w", err)
		}

		switch action {
		case ui.ActionAccept:
			// Step 7: Execute commit or save to file
			return s.handleAccept(ctx, opts, response, processedDiff)

		case ui.ActionEdit:
			editedResponse, err := s.uiManager.EditMessage(response)
			if err != nil {
				s.uiManager.ShowError(fmt.Errorf("failed to edit message: %w", err))
				continue
			}
			return s.handleAccept(ctx, opts, editedResponse, processedDiff)

		case ui.ActionRegenerate:
			regenerationCount++
			if regenerationCount >= MaxRegenerationAttempts {
				s.uiManager.ShowError(fmt.Errorf("maximum regeneration attempts (%d) reached", MaxRegenerationAttempts))
				return fmt.Errorf("maximum regeneration attempts reached")
			}
			// Track previous attempt for context
			previousAttempt = s.formatResponseForContext(response)
			continue

		case ui.ActionCancel:
			s.uiManager.ShowSuccess("Commit cancelled")
			return nil
		}
	}
}

// generateCommitMessage generates a commit message using the AI provider.
// Handles both single-request and chunked diff scenarios.
// Uses cache if enabled and noCache is false.
func (s *CommitService) generateCommitMessage(
	ctx context.Context,
	processedDiff *processor.ProcessedDiff,
	diffStats *git.DiffStats,
	customPrompt string,
	previousAttempt string,
	noCache bool,
) (*ai.GenerateResponse, error) {
	spinner := s.uiManager.ShowSpinner("Generating commit message...")
	spinner.Start()
	defer spinner.Stop()

	// Generate cache key from diff content
	var diffContent strings.Builder
	for _, chunk := range processedDiff.Chunks {
		diffContent.WriteString(chunk.Content)
	}

	// Check cache if enabled and not bypassed
	// Don't use cache for regeneration (previousAttempt is set)
	cacheKey := ""
	if s.cache != nil && !noCache && previousAttempt == "" {
		cacheKey = cache.GenerateCacheKey(
			diffContent.String(),
			s.aiProvider.Name(),
			s.config.Provider.Model,
			customPrompt,
		)

		if cached, ok := s.cache.Get(cacheKey); ok {
			if response, ok := cached.(*ai.GenerateResponse); ok {
				return response, nil
			}
		}
	}

	// If chunking is required, process chunks concurrently
	var response *ai.GenerateResponse
	var err error

	if processedDiff.RequiresChunking && len(processedDiff.ChunkGroups) > 1 {
		response, err = s.generateFromChunks(ctx, processedDiff, diffStats, customPrompt, previousAttempt)
	} else {
		// Single request for non-chunked diffs
		req := &ai.GenerateRequest{
			DiffChunks:      processedDiff.Chunks,
			DiffStats:       diffStats,
			CustomPrompt:    customPrompt,
			PreviousAttempt: previousAttempt,
		}
		response, err = s.aiProvider.GenerateCommitMessage(ctx, req)
	}

	if err != nil {
		return nil, err
	}

	// Store in cache if enabled
	if s.cache != nil && cacheKey != "" && response != nil {
		s.cache.Set(cacheKey, response, 0) // Use default TTL
	}

	return response, nil
}

// chunkResult holds the result of processing a single chunk group.
type chunkResult struct {
	index    int
	response *ai.GenerateResponse
	err      error
}

// generateFromChunks processes diff chunks concurrently and aggregates results.
func (s *CommitService) generateFromChunks(
	ctx context.Context,
	processedDiff *processor.ProcessedDiff,
	diffStats *git.DiffStats,
	customPrompt string,
	previousAttempt string,
) (*ai.GenerateResponse, error) {
	chunkGroups := processedDiff.ChunkGroups
	numGroups := len(chunkGroups)

	// Limit concurrent calls
	maxConcurrent := MaxConcurrentAICalls
	if numGroups < maxConcurrent {
		maxConcurrent = numGroups
	}

	// Create channels for results and semaphore for concurrency control
	results := make(chan chunkResult, numGroups)
	semaphore := make(chan struct{}, maxConcurrent)

	var wg sync.WaitGroup

	// Process each chunk group concurrently
	for i, group := range chunkGroups {
		wg.Add(1)
		go func(idx int, chunkGroup processor.ChunkGroup) {
			defer wg.Done()

			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			// Check context cancellation
			select {
			case <-ctx.Done():
				results <- chunkResult{index: idx, err: ctx.Err()}
				return
			default:
			}

			// Generate commit message for this chunk group
			req := &ai.GenerateRequest{
				DiffChunks:      chunkGroup.Chunks,
				DiffStats:       diffStats,
				CustomPrompt:    customPrompt,
				PreviousAttempt: previousAttempt,
			}

			resp, err := s.aiProvider.GenerateCommitMessage(ctx, req)
			results <- chunkResult{index: idx, response: resp, err: err}
		}(i, group)
	}

	// Wait for all goroutines to complete
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results
	responses := make([]*ai.GenerateResponse, numGroups)
	var errors []error

	for result := range results {
		if result.err != nil {
			errors = append(errors, fmt.Errorf("chunk %d: %w", result.index, result.err))
		} else {
			responses[result.index] = result.response
		}
	}

	// Handle partial failures gracefully
	if len(errors) > 0 {
		// If all chunks failed, return the first error
		if len(errors) == numGroups {
			return nil, fmt.Errorf("all chunk processing failed: %w", errors[0])
		}
		// Log partial failures but continue with successful results
		// In a real implementation, we might want to log these warnings
	}

	// Aggregate responses into a single commit message
	return s.aggregateResponses(responses, processedDiff.Summary), nil
}

// aggregateResponses combines multiple AI responses into a single commit message.
func (s *CommitService) aggregateResponses(responses []*ai.GenerateResponse, summary string) *ai.GenerateResponse {
	var subjects []string
	var bodies []string
	var footers []string

	for _, resp := range responses {
		if resp == nil {
			continue
		}
		if resp.Subject != "" {
			subjects = append(subjects, resp.Subject)
		}
		if resp.Body != "" {
			bodies = append(bodies, resp.Body)
		}
		if resp.Footer != "" {
			footers = append(footers, resp.Footer)
		}
	}

	// If we have multiple subjects, try to create a combined subject
	var finalSubject string
	if len(subjects) == 1 {
		finalSubject = subjects[0]
	} else if len(subjects) > 1 {
		// Try to extract a common type and create a combined subject
		finalSubject = s.combineSubjects(subjects)
	}

	// Combine bodies
	var finalBody string
	if len(bodies) == 1 {
		finalBody = bodies[0]
	} else if len(bodies) > 1 {
		finalBody = strings.Join(bodies, "\n\n")
	}

	// Combine footers (deduplicate)
	var finalFooter string
	if len(footers) > 0 {
		finalFooter = s.deduplicateFooters(footers)
	}

	// Build raw text
	var rawParts []string
	if finalSubject != "" {
		rawParts = append(rawParts, finalSubject)
	}
	if finalBody != "" {
		rawParts = append(rawParts, "", finalBody)
	}
	if finalFooter != "" {
		rawParts = append(rawParts, "", finalFooter)
	}

	return &ai.GenerateResponse{
		Subject: finalSubject,
		Body:    finalBody,
		Footer:  finalFooter,
		RawText: strings.Join(rawParts, "\n"),
	}
}

// typePriority defines the priority order for commit types when counts are equal.
// Higher priority types are preferred when aggregating multiple commits.
var typePriority = map[string]int{
	"feat":     10,
	"fix":      9,
	"perf":     8,
	"refactor": 7,
	"docs":     6,
	"test":     5,
	"style":    4,
	"ci":       3,
	"build":    2,
	"chore":    1,
	"revert":   0,
}

// combineSubjects attempts to create a combined subject from multiple subjects.
func (s *CommitService) combineSubjects(subjects []string) string {
	if len(subjects) == 0 {
		return ""
	}
	if len(subjects) == 1 {
		return subjects[0]
	}

	// Try to find a common commit type
	typeCount := make(map[string]int)
	for _, subj := range subjects {
		cm := message.NewCommitMessage(subj)
		if cm.Type != "" {
			typeCount[cm.Type]++
		}
	}

	// Find the most common type, using priority as tiebreaker
	var mostCommonType string
	maxCount := 0
	maxPriority := -1
	for t, count := range typeCount {
		priority := typePriority[t]
		if count > maxCount || (count == maxCount && priority > maxPriority) {
			mostCommonType = t
			maxCount = count
			maxPriority = priority
		}
	}

	// If we found a common type, use it
	if mostCommonType != "" {
		return fmt.Sprintf("%s: multiple changes across %d files", mostCommonType, len(subjects))
	}

	// Default to chore for mixed changes
	return fmt.Sprintf("chore: multiple changes across %d files", len(subjects))
}

// deduplicateFooters removes duplicate footer lines.
func (s *CommitService) deduplicateFooters(footers []string) string {
	seen := make(map[string]bool)
	var unique []string

	for _, footer := range footers {
		lines := strings.Split(footer, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line != "" && !seen[line] {
				seen[line] = true
				unique = append(unique, line)
			}
		}
	}

	return strings.Join(unique, "\n")
}

// validateAndWarn validates the commit message and shows warnings if needed.
func (s *CommitService) validateAndWarn(response *ai.GenerateResponse) {
	if response == nil {
		return
	}

	// Parse the response into a CommitMessage for validation
	rawText := response.RawText
	if rawText == "" {
		rawText = response.Subject
		if response.Body != "" {
			rawText += "\n\n" + response.Body
		}
		if response.Footer != "" {
			rawText += "\n\n" + response.Footer
		}
	}

	cm := message.NewCommitMessage(rawText)
	result := cm.ValidateWithWarnings()

	// Show warnings (but not errors - those would prevent commit)
	for _, warning := range result.Warnings {
		s.uiManager.ShowError(fmt.Errorf("warning: %s", warning))
	}
}

// handleAccept handles the accept action - commits or saves to file based on options.
func (s *CommitService) handleAccept(
	ctx context.Context,
	opts *CommitOptions,
	response *ai.GenerateResponse,
	processedDiff *processor.ProcessedDiff,
) error {
	// Format the commit message
	commitMsg := s.formatCommitMessage(response)

	// Save to history if enabled
	if s.historyMgr != nil && s.config != nil && s.config.History.Enabled {
		entry := &history.Entry{
			Message:     commitMsg,
			DiffSummary: processedDiff.Summary,
			Provider:    s.aiProvider.Name(),
			Model:       s.config.Provider.Model,
			Committed:   !opts.DryRun,
		}
		if err := s.historyMgr.Save(entry); err != nil {
			// Log but don't fail the commit
			s.uiManager.ShowError(fmt.Errorf("warning: failed to save to history: %w", err))
		}
	}

	// Dry-run mode: output message without committing
	if opts.DryRun {
		if opts.OutputFile != "" {
			return s.writeToFile(opts.OutputFile, commitMsg)
		}
		// Message already displayed, just return success
		s.uiManager.ShowSuccess("Dry-run complete - message generated but not committed")
		return nil
	}

	// Execute git commit
	spinner := s.uiManager.ShowSpinner("Committing changes...")
	spinner.Start()

	err := s.gitClient.Commit(ctx, commitMsg)
	spinner.Stop()

	if err != nil {
		return fmt.Errorf("failed to commit: %w", err)
	}

	s.uiManager.ShowSuccess("Successfully committed!")
	return nil
}

// formatCommitMessage formats the AI response into a proper commit message string.
func (s *CommitService) formatCommitMessage(response *ai.GenerateResponse) string {
	if response == nil {
		return ""
	}

	// If we have structured parts, format them properly
	if response.Subject != "" {
		var parts []string
		parts = append(parts, response.Subject)

		if response.Body != "" {
			parts = append(parts, "")
			parts = append(parts, response.Body)
		}

		if response.Footer != "" {
			parts = append(parts, "")
			parts = append(parts, response.Footer)
		}

		return strings.Join(parts, "\n")
	}

	// Fall back to raw text
	return strings.TrimSpace(response.RawText)
}

// formatResponseForContext formats the response for use as previous attempt context.
func (s *CommitService) formatResponseForContext(response *ai.GenerateResponse) string {
	if response == nil {
		return ""
	}

	if response.RawText != "" {
		return response.RawText
	}

	return s.formatCommitMessage(response)
}

// writeToFile writes the commit message to a file.
func (s *CommitService) writeToFile(filePath, content string) error {
	if err := writeFile(filePath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write to file %s: %w", filePath, err)
	}

	s.uiManager.ShowSuccess(fmt.Sprintf("Message written to %s", filePath))
	return nil
}
