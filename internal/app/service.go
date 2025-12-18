// Package app contains the application layer with business orchestration logic.
package app

import (
	"context"
	"fmt"
	"os"
	"strings"
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

// MaxGroupSize is the maximum size (in bytes) for a group of files to be summarized together.
const MaxGroupSize = 4 * 1024 // 4KB per group

// MaxConcurrentGroups is the maximum number of concurrent AI calls.
const MaxConcurrentGroups = 2

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
		// Check if there are unstaged changes that can be added
		hasUnstaged, err := s.gitClient.HasUnstagedChanges(ctx)
		if err != nil {
			return fmt.Errorf("failed to check unstaged changes: %w", err)
		}
		if !hasUnstaged {
			return fmt.Errorf("no changes found. Nothing to commit")
		}

		// Ask user if they want to auto-add all changes
		confirmed, err := s.uiManager.PromptConfirm("No staged changes found. Run 'git add .' to stage all changes?")
		if err != nil {
			return fmt.Errorf("failed to prompt user: %w", err)
		}
		if !confirmed {
			return fmt.Errorf("no staged changes. Use 'git add' to stage changes before generating a commit message")
		}

		// Execute git add .
		spinner := s.uiManager.ShowSpinner("Staging all changes...")
		spinner.Start()
		if err := s.gitClient.AddAll(ctx); err != nil {
			spinner.Stop()
			return fmt.Errorf("failed to stage changes: %w", err)
		}
		spinner.Stop()
		s.uiManager.ShowSuccess("All changes staged")
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
// For large diffs with multiple files, uses two-phase processing for better results.
func (s *CommitService) generateCommitMessage(
	ctx context.Context,
	processedDiff *processor.ProcessedDiff,
	diffStats *git.DiffStats,
	customPrompt string,
	previousAttempt string,
	noCache bool,
) (*ai.GenerateResponse, error) {
	// Generate cache key from diff content
	var diffContent strings.Builder
	for _, chunk := range processedDiff.Chunks {
		diffContent.WriteString(chunk.Content)
	}

	// Check cache if enabled and not bypassed
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

	totalSize := diffContent.Len()
	fileCount := len(processedDiff.Chunks)

	var response *ai.GenerateResponse
	var err error

	// Decision: use two-phase processing for large diffs with multiple files
	if totalSize > 10*1024 && fileCount > 1 {
		// Two-phase processing has its own progress UI
		response, err = s.generateWithTwoPhase(ctx, processedDiff, diffStats, previousAttempt)
	} else {
		// Direct processing: show simple spinner
		spinner := s.uiManager.ShowSpinner("Generating commit message...")
		spinner.Start()
		defer spinner.Stop()

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
		s.cache.Set(cacheKey, response, 0)
	}

	return response, nil
}

// fileGroup represents a group of files to be summarized together.
type fileGroup struct {
	chunks []git.DiffChunk
	files  []string
}

// generateWithTwoPhase implements two-phase processing for large diffs.
// Phase 1: Group small files together, then summarize each group
// Phase 2: Generate final commit message from summaries
func (s *CommitService) generateWithTwoPhase(
	ctx context.Context,
	processedDiff *processor.ProcessedDiff,
	diffStats *git.DiffStats,
	previousAttempt string,
) (*ai.GenerateResponse, error) {
	// Step 1: Group files by size to minimize API calls
	groups := s.groupFilesBySize(processedDiff.Chunks)

	// Create progress spinner
	progress := s.uiManager.ShowProgressSpinner("Analyzing files", len(groups))
	progress.Start()
	defer progress.Stop()

	// Step 2: Process groups in batches (MaxConcurrentGroups at a time)
	summaries := make([]string, len(groups))
	completed := 0

	for batchStart := 0; batchStart < len(groups); batchStart += MaxConcurrentGroups {
		batchEnd := batchStart + MaxConcurrentGroups
		if batchEnd > len(groups) {
			batchEnd = len(groups)
		}

		type result struct {
			index   int
			summary string
			err     error
		}
		batchLen := batchEnd - batchStart
		resultChan := make(chan result, batchLen)

		// Update progress with current files
		var currentFiles []string
		for i := batchStart; i < batchEnd; i++ {
			if len(groups[i].files) > 0 {
				currentFiles = append(currentFiles, groups[i].files[0])
			}
		}
		if len(currentFiles) > 0 {
			progress.SetCurrentFile(strings.Join(currentFiles, ", "))
		}

		// Launch goroutines for this batch
		for i := batchStart; i < batchEnd; i++ {
			idx := i
			group := groups[i]
			go func() {
				summary, err := s.summarizeFileGroup(ctx, group)
				resultChan <- result{index: idx, summary: summary, err: err}
			}()
		}

		// Wait for batch to complete
		for j := 0; j < batchLen; j++ {
			r := <-resultChan
			completed++
			progress.SetCurrent(completed)

			if r.err != nil {
				// Fallback: list files without AI summary
				var files []string
				for _, c := range groups[r.index].chunks {
					files = append(files, fmt.Sprintf("- %s (+%d -%d)", c.FilePath, c.Additions, c.Deletions))
				}
				summaries[r.index] = strings.Join(files, "\n")
			} else {
				summaries[r.index] = r.summary
			}
		}

		// Delay between batches
		if batchEnd < len(groups) {
			time.Sleep(1 * time.Second)
		}
	}

	// Phase 2: Generate final commit message
	progress.Stop()
	finalSpinner := s.uiManager.ShowSpinner("Generating commit message...")
	finalSpinner.Start()
	defer finalSpinner.Stop()

	return s.generateFromSummaries(ctx, summaries, diffStats, previousAttempt)
}

// groupFilesBySize groups files together until each group reaches MaxGroupSize.
func (s *CommitService) groupFilesBySize(chunks []git.DiffChunk) []fileGroup {
	var groups []fileGroup
	var currentGroup fileGroup
	currentSize := 0

	for _, chunk := range chunks {
		chunkSize := len(chunk.Content)

		// If single file is larger than MaxGroupSize, put it in its own group
		if chunkSize >= MaxGroupSize {
			// Save current group if not empty
			if len(currentGroup.chunks) > 0 {
				groups = append(groups, currentGroup)
				currentGroup = fileGroup{}
				currentSize = 0
			}
			// Add large file as its own group
			groups = append(groups, fileGroup{
				chunks: []git.DiffChunk{chunk},
				files:  []string{chunk.FilePath},
			})
			continue
		}

		// If adding this file would exceed MaxGroupSize, start a new group
		if currentSize+chunkSize > MaxGroupSize && len(currentGroup.chunks) > 0 {
			groups = append(groups, currentGroup)
			currentGroup = fileGroup{}
			currentSize = 0
		}

		// Add file to current group
		currentGroup.chunks = append(currentGroup.chunks, chunk)
		currentGroup.files = append(currentGroup.files, chunk.FilePath)
		currentSize += chunkSize
	}

	// Don't forget the last group
	if len(currentGroup.chunks) > 0 {
		groups = append(groups, currentGroup)
	}

	return groups
}

// summarizeFileGroup generates a summary for a group of files.
func (s *CommitService) summarizeFileGroup(ctx context.Context, group fileGroup) (string, error) {
	var sb strings.Builder

	// Build combined diff content
	for _, chunk := range group.chunks {
		content := chunk.Content
		// Truncate individual file if too large
		if len(content) > 2*1024 {
			content = content[:2*1024] + "\n... [truncated]"
		}
		sb.WriteString(fmt.Sprintf("=== %s (%s, +%d -%d) ===\n%s\n\n",
			chunk.FilePath, chunk.ChangeType, chunk.Additions, chunk.Deletions, content))
	}

	prompt := fmt.Sprintf(`简要描述以下文件的改动（每个文件一句话，不超过20字，中文）:

%s

格式:
- 文件名: 改动描述`, sb.String())

	req := &ai.GenerateRequest{
		CustomPrompt: prompt,
	}

	resp, err := s.aiProvider.GenerateCommitMessage(ctx, req)
	if err != nil {
		return "", err
	}

	summary := strings.TrimSpace(resp.RawText)
	if summary == "" {
		summary = resp.Subject
	}

	return summary, nil
}

// generateFromSummaries generates the final commit message from file summaries.
func (s *CommitService) generateFromSummaries(
	ctx context.Context,
	summaries []string,
	diffStats *git.DiffStats,
	previousAttempt string,
) (*ai.GenerateResponse, error) {
	// Filter empty summaries
	var validSummaries []string
	for _, s := range summaries {
		if s != "" {
			validSummaries = append(validSummaries, s)
		}
	}

	prompt := fmt.Sprintf(`根据以下文件改动摘要，生成一个 Conventional Commits 格式的 commit message（中文）:

文件数: %d
总添加: %d 行
总删除: %d 行

各文件改动:
%s

%s

要求:
1. Subject 格式: <type>(<scope>): <简短描述>（不超过50字）
2. Body 必须包含：按模块/目录分组列出主要改动，每个模块一行，格式如：
   - 模块名: 具体功能描述
3. 如果有多个模块，都要列出
4. 只输出 commit message，不要解释`,
		diffStats.TotalFiles,
		diffStats.TotalAdditions,
		diffStats.TotalDeletions,
		strings.Join(validSummaries, "\n"),
		func() string {
			if previousAttempt != "" {
				return fmt.Sprintf("\n上次生成的不满意，请重新生成:\n%s", previousAttempt)
			}
			return ""
		}(),
	)

	req := &ai.GenerateRequest{
		CustomPrompt: prompt,
		DiffStats:    diffStats,
	}

	return s.aiProvider.GenerateCommitMessage(ctx, req)
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
