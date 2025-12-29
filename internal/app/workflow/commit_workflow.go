package workflow

import (
	"context"
	"errors"
	"strings"

	"github.com/gitsage/gitsage/internal/core/domain"
	"github.com/gitsage/gitsage/internal/core/ports"
	"github.com/gitsage/gitsage/internal/core/services/diff"
	"github.com/gitsage/gitsage/internal/core/services/generator"
	"github.com/gitsage/gitsage/internal/core/services/validator"
)

// CommitWorkflow orchestrates the commit generation process.
type CommitWorkflow struct {
	git       ports.GitProvider
	ui        ports.UserInterface
	diff      diff.Processor
	generator generator.Service
	validator validator.Validator
}

// NewCommitWorkflow creates a new workflow instance.
func NewCommitWorkflow(
	git ports.GitProvider,
	ui ports.UserInterface,
	diff diff.Processor,
	gen generator.Service,
	val validator.Validator,
) *CommitWorkflow {
	return &CommitWorkflow{
		git:       git,
		ui:        ui,
		diff:      diff,
		generator: gen,
		validator: val,
	}
}

// Run executes the workflow.
// If autoAccept is true, the workflow will skip interactive review and commit immediately.
func (w *CommitWorkflow) Run(ctx context.Context, hint string, autoAccept bool) error {
	// 1. Get Staged Changes
	stopSpinner := w.ui.ShowProgress("Fetching staged changes...")
	stagedDiff, err := w.git.GetStagedChanges(ctx)
	stopSpinner()
	if err != nil {
		// Check if it's "no staged changes" error
		if isNoStagedChangesError(err) {
			var confirmed bool
			if autoAccept {
				// Auto-accept mode: auto-stage
				confirmed = true
			} else {
				// Prompt user to stage all changes
				var promptErr error
				confirmed, promptErr = w.ui.PromptConfirm("No staged changes found. Would you like to stage all changes (git add .)?")
				if promptErr != nil {
					return promptErr
				}
			}
			if confirmed {
				// Stage all changes
				stageSpinner := w.ui.ShowProgress("Staging all changes...")
				if stageErr := w.git.StageAll(ctx); stageErr != nil {
					stageSpinner()
					w.ui.ShowError(stageErr)
					return stageErr
				}
				stageSpinner()

				// Try getting staged changes again
				stopSpinner = w.ui.ShowProgress("Fetching staged changes...")
				stagedDiff, err = w.git.GetStagedChanges(ctx)
				stopSpinner()
				if err != nil {
					w.ui.ShowError(err)
					return err
				}
			} else {
				return errors.New("no staged changes - operation cancelled")
			}
		} else {
			w.ui.ShowError(err)
			return err
		}
	}

	if len(stagedDiff.Files) == 0 {
		err := errors.New("no staged changes found")
		w.ui.ShowError(err)
		return err
	}

	// 2. Generate Loop
	for {
		// Start Generation
		stopSpinner = w.ui.ShowProgress("Generating commit message...")
		msg, err := w.generator.Generate(ctx, stagedDiff, hint)
		stopSpinner()
		if err != nil {
			w.ui.ShowError(err)
			return err
		}

		// Auto-accept mode: skip review, commit directly
		if autoAccept {
			stopSpinner = w.ui.ShowProgress("Committing...")
			err := w.git.Commit(ctx, msg)
			stopSpinner()
			if err != nil {
				w.ui.ShowError(err)
				return err
			}
			w.ui.ShowSuccess("Changes committed successfully!")
			return nil
		}

		// 3. Review Loop (Edit/Regenerate cycle)
		action, finalMsg, err := w.runReviewLoop(ctx, msg)
		if err != nil {
			w.ui.ShowError(err)
			return err
		}

		switch action {
		case ports.ActionRegenerate:
			continue // Go back to generation
		case ports.ActionCancel:
			return nil // Exit without error
		case ports.ActionAccept:
			// 4. Commit
			if finalMsg == nil {
				// Should not happen if Accept
				return errors.New("accepted message is nil")
			}
			stopSpinner = w.ui.ShowProgress("Committing...")
			err := w.git.Commit(ctx, finalMsg)
			stopSpinner()
			if err != nil {
				w.ui.ShowError(err)
				return err
			}
			w.ui.ShowSuccess("Changes committed successfully!")
			return nil
		}
	}
}

func (w *CommitWorkflow) runReviewLoop(ctx context.Context, msg *domain.CommitMessage) (ports.UserAction, *domain.CommitMessage, error) {
	currentMsg := msg

	for {
		// Validate (Optional: Attach warnings if any)
		res := w.validator.Validate(currentMsg)
		if !res.IsValid {
			// For now, validation issues are not blocking, just maybe visible if UI supports it.
			// Ideally we modify currentMsg or UI handles display.
		}

		// Show UI
		action, editedMsg, err := w.ui.ReviewMessage(ctx, currentMsg)
		if err != nil {
			return ports.ActionCancel, nil, err
		}

		switch action {
		case ports.ActionCancel:
			return ports.ActionCancel, nil, nil

		case ports.ActionRegenerate:
			return ports.ActionRegenerate, nil, nil

		case ports.ActionEdit:
			// Update current message with edited content and loop back to review it
			currentMsg = editedMsg
			continue

		case ports.ActionAccept:
			return ports.ActionAccept, editedMsg, nil

		default:
			return ports.ActionCancel, nil, nil
		}
	}
}

// isNoStagedChangesError checks if the error indicates no staged changes.
func isNoStagedChangesError(err error) bool {
	if err == nil {
		return false
	}
	errMsg := err.Error()
	return strings.Contains(errMsg, "no staged changes") ||
		strings.Contains(errMsg, "NoStagedChanges")
}
