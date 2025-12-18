// Package git provides Git operations for GitSage.
package git

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	apperrors "github.com/gitsage/gitsage/internal/pkg/errors"
)

const (
	// GitCommandTimeout is the default timeout for git commands.
	GitCommandTimeout = 10 * time.Second
)

// ChangeType represents the type of change in a diff.
type ChangeType int

const (
	ChangeTypeAdded ChangeType = iota
	ChangeTypeModified
	ChangeTypeDeleted
	ChangeTypeRenamed
)

// String returns the string representation of ChangeType.
func (c ChangeType) String() string {
	switch c {
	case ChangeTypeAdded:
		return "added"
	case ChangeTypeModified:
		return "modified"
	case ChangeTypeDeleted:
		return "deleted"
	case ChangeTypeRenamed:
		return "renamed"
	default:
		return "unknown"
	}
}

// DiffChunk represents a segment of git diff output.
type DiffChunk struct {
	FilePath   string
	ChangeType ChangeType
	Additions  int
	Deletions  int
	Content    string
	IsLockFile bool
	IsBinary   bool
	OldPath    string // For renames, the original file path
}

// DiffStats contains statistics about the diff.
type DiffStats struct {
	TotalFiles     int
	TotalAdditions int
	TotalDeletions int
	Chunks         []DiffChunk
}

// Client defines the interface for Git operations.
type Client interface {
	GetStagedDiff(ctx context.Context) ([]DiffChunk, error)
	GetDiffStats(ctx context.Context) (*DiffStats, error)
	Commit(ctx context.Context, message string) error
	HasStagedChanges(ctx context.Context) (bool, error)
	HasUnstagedChanges(ctx context.Context) (bool, error)
	AddAll(ctx context.Context) error
	Pull(ctx context.Context) (*PullResult, error)
	Push(ctx context.Context) error
	PushWithUpstream(ctx context.Context) error
	HasRemote(ctx context.Context) (bool, error)
	HasUpstream(ctx context.Context) (bool, error)
	GetCurrentBranch(ctx context.Context) (string, error)
}

// DefaultClient implements the Client interface using exec.CommandContext.
type DefaultClient struct {
	// workDir is the working directory for git commands.
	// If empty, uses the current directory.
	workDir string
}

// NewClient creates a new DefaultClient.
func NewClient() *DefaultClient {
	return &DefaultClient{}
}

// NewClientWithWorkDir creates a new DefaultClient with a specific working directory.
func NewClientWithWorkDir(workDir string) *DefaultClient {
	return &DefaultClient{workDir: workDir}
}

// lockFilePatterns contains patterns for lock files that should be excluded.
var lockFilePatterns = []string{
	"package-lock.json",
	"yarn.lock",
	"pnpm-lock.yaml",
	"go.sum",
	"Cargo.lock",
	"Gemfile.lock",
	"composer.lock",
	"poetry.lock",
	"Pipfile.lock",
}

// isLockFile checks if a file path matches any lock file pattern.
func isLockFile(filePath string) bool {
	baseName := filepath.Base(filePath)
	for _, pattern := range lockFilePatterns {
		if baseName == pattern {
			return true
		}
	}
	// Also check for generic .lock extension
	if strings.HasSuffix(baseName, ".lock") {
		return true
	}
	return false
}

// HasStagedChanges checks if there are any staged changes in the repository.
func (c *DefaultClient) HasStagedChanges(ctx context.Context) (bool, error) {
	// Apply timeout to context
	ctx, cancel := context.WithTimeout(ctx, GitCommandTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "diff", "--cached", "--quiet")
	if c.workDir != "" {
		cmd.Dir = c.workDir
	}

	err := cmd.Run()
	if err != nil {
		// Check for context timeout
		if ctx.Err() == context.DeadlineExceeded {
			return false, apperrors.NewTimeoutError(ctx.Err())
		}
		// Exit code 1 means there are differences (staged changes exist)
		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitErr.ExitCode() == 1 {
				return true, nil
			}
		}
		return false, apperrors.NewGitError(err, "")
	}
	// Exit code 0 means no differences (no staged changes)
	return false, nil
}

// GetStagedDiff retrieves all staged changes as DiffChunks.
func (c *DefaultClient) GetStagedDiff(ctx context.Context) ([]DiffChunk, error) {
	// First check if there are staged changes
	hasChanges, err := c.HasStagedChanges(ctx)
	if err != nil {
		return nil, err
	}
	if !hasChanges {
		return nil, apperrors.NewNoStagedChangesError()
	}

	// Apply timeout to context for remaining operations
	ctx, cancel := context.WithTimeout(ctx, GitCommandTimeout)
	defer cancel()

	// Get the full diff content
	diffCmd := exec.CommandContext(ctx, "git", "diff", "--cached")
	if c.workDir != "" {
		diffCmd.Dir = c.workDir
	}

	diffOutput, err := diffCmd.Output()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, apperrors.NewTimeoutError(ctx.Err())
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, apperrors.NewGitError(err, string(exitErr.Stderr))
		}
		return nil, apperrors.NewGitError(err, "")
	}

	// Get numstat for additions/deletions count
	numstatCmd := exec.CommandContext(ctx, "git", "diff", "--cached", "--numstat")
	if c.workDir != "" {
		numstatCmd.Dir = c.workDir
	}

	numstatOutput, err := numstatCmd.Output()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, apperrors.NewTimeoutError(ctx.Err())
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, apperrors.NewGitError(err, string(exitErr.Stderr))
		}
		return nil, apperrors.NewGitError(err, "")
	}

	// Parse numstat to get file statistics
	fileStats := parseNumstat(numstatOutput)

	// Parse the diff output into chunks
	chunks := parseDiff(diffOutput, fileStats)

	return chunks, nil
}

// GetDiffStats retrieves statistics about staged changes.
func (c *DefaultClient) GetDiffStats(ctx context.Context) (*DiffStats, error) {
	chunks, err := c.GetStagedDiff(ctx)
	if err != nil {
		return nil, err
	}

	stats := &DiffStats{
		TotalFiles: len(chunks),
		Chunks:     chunks,
	}

	for _, chunk := range chunks {
		stats.TotalAdditions += chunk.Additions
		stats.TotalDeletions += chunk.Deletions
	}

	return stats, nil
}

// Commit executes a git commit with the given message.
func (c *DefaultClient) Commit(ctx context.Context, message string) error {
	// Apply timeout to context
	ctx, cancel := context.WithTimeout(ctx, GitCommandTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "commit", "-m", message)
	if c.workDir != "" {
		cmd.Dir = c.workDir
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return apperrors.NewTimeoutError(ctx.Err())
		}
		return apperrors.NewGitError(err, string(output))
	}
	return nil
}

// HasUnstagedChanges checks if there are any unstaged changes (modified/untracked files).
func (c *DefaultClient) HasUnstagedChanges(ctx context.Context) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, GitCommandTimeout)
	defer cancel()

	// Check for modified files (not staged)
	cmd := exec.CommandContext(ctx, "git", "status", "--porcelain")
	if c.workDir != "" {
		cmd.Dir = c.workDir
	}

	output, err := cmd.Output()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return false, apperrors.NewTimeoutError(ctx.Err())
		}
		return false, apperrors.NewGitError(err, "")
	}

	// If there's any output, there are changes
	return len(strings.TrimSpace(string(output))) > 0, nil
}

// AddAll stages all changes (git add .).
func (c *DefaultClient) AddAll(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, GitCommandTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "add", ".")
	if c.workDir != "" {
		cmd.Dir = c.workDir
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return apperrors.NewTimeoutError(ctx.Err())
		}
		return apperrors.NewGitError(err, string(output))
	}
	return nil
}

// Push pushes commits to the remote repository.
// If setUpstream is true and there's no upstream, it will set the upstream to origin/<branch>.
func (c *DefaultClient) Push(ctx context.Context) error {
	return c.pushInternal(ctx, false)
}

// PushWithUpstream pushes commits and sets the upstream tracking branch.
func (c *DefaultClient) PushWithUpstream(ctx context.Context) error {
	return c.pushInternal(ctx, true)
}

// pushInternal handles the actual push logic.
func (c *DefaultClient) pushInternal(ctx context.Context, setUpstream bool) error {
	// Use longer timeout for push (network operation)
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	args := []string{"push"}
	if setUpstream {
		branch, err := c.GetCurrentBranch(ctx)
		if err != nil {
			return fmt.Errorf("failed to get current branch: %w", err)
		}
		args = append(args, "-u", "origin", branch)
	}

	cmd := exec.CommandContext(ctx, "git", args...)
	if c.workDir != "" {
		cmd.Dir = c.workDir
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return apperrors.NewTimeoutError(ctx.Err())
		}
		return apperrors.NewGitError(err, string(output))
	}
	return nil
}

// GetCurrentBranch returns the name of the current branch.
func (c *DefaultClient) GetCurrentBranch(ctx context.Context) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, GitCommandTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--abbrev-ref", "HEAD")
	if c.workDir != "" {
		cmd.Dir = c.workDir
	}

	output, err := cmd.Output()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "", apperrors.NewTimeoutError(ctx.Err())
		}
		return "", apperrors.NewGitError(err, "")
	}

	return strings.TrimSpace(string(output)), nil
}

// HasRemote checks if the repository has a remote configured.
func (c *DefaultClient) HasRemote(ctx context.Context) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, GitCommandTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "remote")
	if c.workDir != "" {
		cmd.Dir = c.workDir
	}

	output, err := cmd.Output()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return false, apperrors.NewTimeoutError(ctx.Err())
		}
		return false, apperrors.NewGitError(err, "")
	}

	return len(strings.TrimSpace(string(output))) > 0, nil
}

// PullResult contains the result of a git pull operation.
type PullResult struct {
	Updated      bool   // Whether there were updates from remote
	UpdatedFiles int    // Number of files updated
	Message      string // Summary message
	Skipped      bool   // Whether pull was skipped (no upstream)
}

// HasUpstream checks if the current branch has an upstream tracking branch.
func (c *DefaultClient) HasUpstream(ctx context.Context) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, GitCommandTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")
	if c.workDir != "" {
		cmd.Dir = c.workDir
	}

	err := cmd.Run()
	if err != nil {
		// Exit code 128 means no upstream configured
		return false, nil
	}
	return true, nil
}

// Pull pulls changes from the remote repository.
func (c *DefaultClient) Pull(ctx context.Context) (*PullResult, error) {
	// First check if there's an upstream branch
	hasUpstream, _ := c.HasUpstream(ctx)
	if !hasUpstream {
		return &PullResult{
			Skipped: true,
			Message: "No upstream branch configured",
		}, nil
	}

	// Use longer timeout for pull (network operation)
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "pull", "--rebase")
	if c.workDir != "" {
		cmd.Dir = c.workDir
	}

	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, apperrors.NewTimeoutError(ctx.Err())
		}
		return nil, apperrors.NewGitError(err, outputStr)
	}

	result := &PullResult{
		Message: strings.TrimSpace(outputStr),
	}

	// Check if there were updates
	if strings.Contains(outputStr, "Already up to date") ||
		strings.Contains(outputStr, "Already up-to-date") {
		result.Updated = false
	} else {
		result.Updated = true
		// Try to count updated files from output
		lines := strings.Split(outputStr, "\n")
		for _, line := range lines {
			if strings.Contains(line, "file changed") || strings.Contains(line, "files changed") {
				result.UpdatedFiles = countFilesChanged(line)
				break
			}
		}
	}

	return result, nil
}

// countFilesChanged extracts the number of files changed from git output.
func countFilesChanged(line string) int {
	// Format: "X file(s) changed, Y insertions(+), Z deletions(-)"
	parts := strings.Fields(line)
	if len(parts) > 0 {
		var count int
		fmt.Sscanf(parts[0], "%d", &count)
		return count
	}
	return 0
}

// fileStat holds statistics for a single file from numstat.
type fileStat struct {
	additions int
	deletions int
	isBinary  bool
}

// parseNumstat parses the output of git diff --numstat.
// Format: additions<TAB>deletions<TAB>filepath
// Binary files show as: -<TAB>-<TAB>filepath
func parseNumstat(output []byte) map[string]fileStat {
	stats := make(map[string]fileStat)
	scanner := bufio.NewScanner(bytes.NewReader(output))

	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Split(line, "\t")
		if len(parts) < 3 {
			continue
		}

		addStr, delStr, filePath := parts[0], parts[1], parts[2]

		// Handle renamed files: old path => new path
		if strings.Contains(filePath, " => ") {
			// Extract the new path from rename notation
			filePath = extractNewPath(filePath)
		}

		stat := fileStat{}
		if addStr == "-" && delStr == "-" {
			// Binary file
			stat.isBinary = true
		} else {
			stat.additions, _ = strconv.Atoi(addStr)
			stat.deletions, _ = strconv.Atoi(delStr)
		}

		stats[filePath] = stat
	}

	return stats
}

// extractNewPath extracts the new file path from git rename notation.
// Examples:
//   - "old.txt => new.txt" -> "new.txt"
//   - "{old => new}/file.txt" -> "new/file.txt"
//   - "dir/{old.txt => new.txt}" -> "dir/new.txt"
func extractNewPath(renamePath string) string {
	// Handle simple rename: "old.txt => new.txt"
	if strings.Contains(renamePath, " => ") && !strings.Contains(renamePath, "{") {
		parts := strings.Split(renamePath, " => ")
		if len(parts) == 2 {
			return strings.TrimSpace(parts[1])
		}
	}

	// Handle brace notation: "{old => new}/file.txt" or "dir/{old.txt => new.txt}"
	re := regexp.MustCompile(`\{([^}]*) => ([^}]*)\}`)
	result := re.ReplaceAllString(renamePath, "$2")
	return result
}

// parseDiff parses the full diff output into DiffChunks.
func parseDiff(diffOutput []byte, fileStats map[string]fileStat) []DiffChunk {
	var chunks []DiffChunk

	// Split diff by file headers
	// Each file diff starts with "diff --git a/... b/..."
	diffStr := string(diffOutput)
	fileDiffs := splitByFileDiff(diffStr)

	for _, fileDiff := range fileDiffs {
		if fileDiff == "" {
			continue
		}

		chunk := parseFileDiff(fileDiff, fileStats)
		if chunk != nil {
			chunks = append(chunks, *chunk)
		}
	}

	return chunks
}

// splitByFileDiff splits the diff output by file boundaries.
func splitByFileDiff(diffStr string) []string {
	// Split on "diff --git" but keep the delimiter
	parts := strings.Split(diffStr, "diff --git ")
	var result []string
	for i, part := range parts {
		if part == "" {
			continue
		}
		if i > 0 {
			part = "diff --git " + part
		}
		result = append(result, part)
	}
	return result
}

// parseFileDiff parses a single file's diff into a DiffChunk.
func parseFileDiff(fileDiff string, fileStats map[string]fileStat) *DiffChunk {
	lines := strings.Split(fileDiff, "\n")
	if len(lines) == 0 {
		return nil
	}

	chunk := &DiffChunk{
		Content: fileDiff,
	}

	// Parse the diff header to extract file path and change type
	for _, line := range lines {
		// Parse "diff --git a/path b/path"
		if strings.HasPrefix(line, "diff --git ") {
			chunk.FilePath = extractFilePath(line)
			chunk.ChangeType = ChangeTypeModified // Default
		}

		// Detect new file
		if strings.HasPrefix(line, "new file mode") {
			chunk.ChangeType = ChangeTypeAdded
		}

		// Detect deleted file
		if strings.HasPrefix(line, "deleted file mode") {
			chunk.ChangeType = ChangeTypeDeleted
		}

		// Detect renamed file
		if strings.HasPrefix(line, "rename from ") {
			chunk.OldPath = strings.TrimPrefix(line, "rename from ")
			chunk.ChangeType = ChangeTypeRenamed
		}
		if strings.HasPrefix(line, "rename to ") {
			chunk.FilePath = strings.TrimPrefix(line, "rename to ")
		}

		// Detect binary file
		if strings.HasPrefix(line, "Binary files") {
			chunk.IsBinary = true
		}
	}

	// Get statistics from numstat
	if stat, ok := fileStats[chunk.FilePath]; ok {
		chunk.Additions = stat.additions
		chunk.Deletions = stat.deletions
		chunk.IsBinary = stat.isBinary
	}

	// Check if it's a lock file
	chunk.IsLockFile = isLockFile(chunk.FilePath)

	return chunk
}

// extractFilePath extracts the file path from a diff header line.
// Format: "diff --git a/path/to/file b/path/to/file"
func extractFilePath(line string) string {
	// Remove "diff --git " prefix
	line = strings.TrimPrefix(line, "diff --git ")

	// Split by " b/"
	parts := strings.Split(line, " b/")
	if len(parts) >= 2 {
		return parts[1]
	}

	// Fallback: try to extract from "a/path"
	if strings.HasPrefix(line, "a/") {
		parts = strings.SplitN(line, " ", 2)
		if len(parts) > 0 {
			return strings.TrimPrefix(parts[0], "a/")
		}
	}

	return line
}
