package ports

import (
	"context"

	"github.com/gitsage/gitsage/internal/core/domain"
)

// GitProvider defines the interface for Git operations.
type GitProvider interface {
	// GetStagedChanges returns the diff of currently staged files.
	GetStagedChanges(ctx context.Context) (*domain.Diff, error)

	// StageAll stages all changes (git add .).
	StageAll(ctx context.Context) error

	// Commit creates a new commit with the given message.
	Commit(ctx context.Context, message *domain.CommitMessage) error

	// Push pushes the current branch to the remote.
	Push(ctx context.Context) error
}

// AIModel defines the interface for AI generation.
type AIModel interface {
	// GenerateCommitMessage sends the prompts to the LLM and returns the raw response.
	// Parsing the response into a domain object is the responsibility of the Core layer.
	GenerateCommitMessage(ctx context.Context, systemPrompt, userPrompt string) (string, error)

	// GenerateCommitMessageStream sends the prompts to the LLM and streams the response.
	// The onChunk callback is called for each chunk of text as it's received.
	// Returns the complete response string when done.
	GenerateCommitMessageStream(ctx context.Context, systemPrompt, userPrompt string, onChunk func(chunk string)) (string, error)
}

// UserAction represents the action taken by the user during review.
type UserAction string

const (
	ActionAccept     UserAction = "accept"
	ActionEdit       UserAction = "edit"
	ActionRegenerate UserAction = "regenerate"
	ActionCancel     UserAction = "cancel"
)

// UserInterface defines the interface for user interaction.
type UserInterface interface {
	// ShowProgress displays a loading spinner or progress bar.
	ShowProgress(msg string) func()

	// ShowFileProgress displays a progress bar with file information.
	// Returns a channel to send progress updates and a function to stop.
	// Usage: progressChan, stop := ui.ShowFileProgress(totalFiles)
	//        progressChan <- FileProgress{Current: 1, FileName: "file.go"}
	//        stop()
	ShowFileProgress(totalFiles int) (chan<- FileProgress, func())

	// ShowStreamingText displays streaming text from AI.
	// Returns a channel to send text chunks and a function to finish.
	// Usage: textChan, finish := ui.ShowStreamingText("AI is generating...")
	//        textChan <- "chunk1"
	//        finish()
	ShowStreamingText(title string) (chan<- string, func())

	// ReviewMessage presents the generated message to the user for review.
	ReviewMessage(ctx context.Context, msg *domain.CommitMessage) (UserAction, *domain.CommitMessage, error)

	// ShowError displays an error message to the user.
	ShowError(err error)

	// ShowSuccess displays a success message.
	ShowSuccess(msg string)

	// PromptConfirm handles a yes/no confirmation.
	PromptConfirm(msg string) (bool, error)
}

// FileProgress represents progress information for file processing.
type FileProgress struct {
	Current  int
	FileName string
}

// ConfigStore defines the interface for configuration management.
type ConfigStore interface {
	// GetProviderConfig returns the configuration for the AI provider.
	GetProviderConfig() map[string]string
}
