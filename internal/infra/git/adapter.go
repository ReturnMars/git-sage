package git

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/gitsage/gitsage/internal/core/domain"
	apperrors "github.com/gitsage/gitsage/internal/pkg/errors"
)

// GitCommandTimeout is the default timeout for git commands.
const GitCommandTimeout = 10 * time.Second

// Adapter implements the ports.GitProvider interface using os/exec.
type Adapter struct {
	workDir string
}

// NewAdapter creates a new Git Adapter.
func NewAdapter(workDir string) *Adapter {
	return &Adapter{workDir: workDir}
}

// GetStagedChanges returns the diff of currently staged files.
func (a *Adapter) GetStagedChanges(ctx context.Context) (*domain.Diff, error) {
	// First check if there are staged changes
	hasChanges, err := a.HasStagedChanges(ctx)
	if err != nil {
		return nil, err
	}
	if !hasChanges {
		return nil, apperrors.NewNoStagedChangesError() // Assume error package might be reused or we define new one
	}

	ctx, cancel := context.WithTimeout(ctx, GitCommandTimeout)
	defer cancel()

	diffOutput, err := a.runGit(ctx, "diff", "--cached")
	if err != nil {
		return nil, err
	}

	numstatOutput, err := a.runGit(ctx, "diff", "--cached", "--numstat")
	if err != nil {
		return nil, err
	}

	fileStats := a.parseNumstat(numstatOutput)
	files := a.parseDiff(diffOutput, fileStats)

	stats := a.calculateStats(files)

	return &domain.Diff{
		Files: files,
		Stats: stats,
	}, nil
}

// StageAll stages all changes (git add .).
func (a *Adapter) StageAll(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, GitCommandTimeout)
	defer cancel()

	_, err := a.runGit(ctx, "add", ".")
	return err
}

// Commit creates a new commit with the given message.
func (a *Adapter) Commit(ctx context.Context, message *domain.CommitMessage) error {
	ctx, cancel := context.WithTimeout(ctx, GitCommandTimeout)
	defer cancel()

	_, err := a.runGit(ctx, "commit", "-m", message.Raw)
	return err
}

// Push pushes the current branch to the remote.
func (a *Adapter) Push(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second) // Longer timeout for network
	defer cancel()

	// Determine if upstream is set
	hasUpstream, _ := a.HasUpstream(ctx)
	if !hasUpstream {
		branch, err := a.GetCurrentBranch(ctx)
		if err != nil {
			return err
		}
		_, err = a.runGit(ctx, "push", "-u", "origin", branch)
		return err
	}

	_, err := a.runGit(ctx, "push")
	return err
}

// Low-level Helpers

func (a *Adapter) runGit(ctx context.Context, args ...string) ([]byte, error) {
	// Debug log could go here
	cmd := exec.CommandContext(ctx, "git", args...)
	if a.workDir != "" {
		cmd.Dir = a.workDir
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("git command timed out: %w", err)
		}
		return nil, fmt.Errorf("git error: %s (%w)", string(output), err)
	}
	return output, nil
}

func (a *Adapter) HasStagedChanges(ctx context.Context) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, GitCommandTimeout)
	defer cancel()

	err := exec.CommandContext(ctx, "git", "diff", "--cached", "--quiet").Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitErr.ExitCode() == 1 {
				return true, nil
			}
		}
		return false, err
	}
	return false, nil
}

func (a *Adapter) HasUpstream(ctx context.Context) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, GitCommandTimeout)
	defer cancel()
	err := exec.CommandContext(ctx, "git", "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}").Run()
	return err == nil, nil
}

func (a *Adapter) GetCurrentBranch(ctx context.Context) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, GitCommandTimeout)
	defer cancel()
	out, err := a.runGit(ctx, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// Parsing Logic (Migrated from old client.go but adapted for domain types)

type fileStat struct {
	additions int
	deletions int
}

// unquotePath handles git quoted paths (octal escapes, etc.)
func unquotePath(path string) string {
	path = strings.TrimSpace(path)
	if len(path) == 0 {
		return ""
	}

	// If quoted, try to unquote
	if path[0] == '"' {
		if unquoted, err := strconv.Unquote(path); err == nil {
			return unquoted
		}
	}
	return path
}

func (a *Adapter) parseNumstat(output []byte) map[string]fileStat {
	stats := make(map[string]fileStat)
	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		parts := strings.Split(scanner.Text(), "\t")
		if len(parts) < 3 {
			continue
		}

		// Handle potential renames in numstat output?
		// Git numstat usually just outputs "path" or "old => new" if -M used without --summary?
		// Actually, numstat with -M puts content in 3rd column.
		// For MVP we just take the last part which is usually the current filepath
		rawPath := parts[2]
		filePath := unquotePath(rawPath)

		stat := fileStat{}
		if parts[0] != "-" {
			stat.additions, _ = strconv.Atoi(parts[0])
			stat.deletions, _ = strconv.Atoi(parts[1])
		}
		stats[filePath] = stat
	}
	return stats
}

func (a *Adapter) parseDiff(diffOutput []byte, fileStats map[string]fileStat) []domain.DiffFile {
	var files []domain.DiffFile
	diffStr := string(diffOutput)

	// Split mainly by "diff --git"
	parts := strings.Split(diffStr, "diff --git ")
	for _, part := range parts {
		if strings.TrimSpace(part) == "" {
			continue
		}
		part = "diff --git " + part // Restore prefix for parsing

		file := a.parseFileDiff(part, fileStats)
		if file != nil {
			files = append(files, *file)
		}
	}
	return files
}

func (a *Adapter) parseFileDiff(fileDiff string, fileStats map[string]fileStat) *domain.DiffFile {
	lines := strings.Split(fileDiff, "\n")
	if len(lines) == 0 {
		return nil
	}

	file := &domain.DiffFile{
		Content:    fileDiff,
		ChangeType: domain.ChangeTypeModified,
	}

	headerLine := lines[0]
	// diff --git a/path b/path
	// Parsing this is tricky with spaces.
	// We iterate to find " b/" separator that makes sense?
	// Or relies on "--- a/" and "+++ b/" lines which are more reliable.

	// Try finding metadata lines
	for _, line := range lines {
		if strings.HasPrefix(line, "--- a/") {
			file.OldFilePath = unquotePath(strings.TrimPrefix(line, "--- a/"))
		}
		if strings.HasPrefix(line, "+++ b/") {
			file.FilePath = unquotePath(strings.TrimPrefix(line, "+++ b/"))
		}

		if strings.HasPrefix(line, "new file mode") {
			file.ChangeType = domain.ChangeTypeAdded
		} else if strings.HasPrefix(line, "deleted file mode") {
			file.ChangeType = domain.ChangeTypeDeleted
		} else if strings.HasPrefix(line, "rename from") {
			file.ChangeType = domain.ChangeTypeRenamed
			file.OldFilePath = unquotePath(strings.TrimPrefix(line, "rename from "))
		} else if strings.HasPrefix(line, "rename to") {
			file.FilePath = unquotePath(strings.TrimPrefix(line, "rename to "))
		}
	}

	// Fallback if FilePath is empty (e.g. only header lines found and no +++)
	if file.FilePath == "" && file.ChangeType != domain.ChangeTypeDeleted {
		// Use header parsing fallback
		parts := strings.Split(headerLine, " b/")
		if len(parts) >= 2 {
			file.FilePath = unquotePath(parts[1])
		} else {
			file.FilePath = unquotePath(strings.TrimPrefix(headerLine, "diff --git a/"))
		}
	}
	if file.FilePath == "" && file.ChangeType == domain.ChangeTypeDeleted {
		file.FilePath = file.OldFilePath
	}

	// Attach stats using direct lookup
	if stat, ok := fileStats[file.FilePath]; ok {
		file.Additions = stat.additions
		file.Deletions = stat.deletions
	}

	return file
}

func (a *Adapter) calculateStats(files []domain.DiffFile) domain.DiffStats {
	stats := domain.DiffStats{
		FilesChanged: len(files),
	}
	for _, f := range files {
		stats.Insertions += f.Additions
		stats.Deletions += f.Deletions
	}
	return stats
}
