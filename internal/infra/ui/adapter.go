package ui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/gitsage/gitsage/internal/core/domain"
	"github.com/gitsage/gitsage/internal/core/ports"
)

// ConsoleUI implements ports.UserInterface using Bubble Tea.
type ConsoleUI struct {
	styles *Styles
	editor string // Editor command to use for editing
}

// NewConsoleUI creates a new console UI adapter.
func NewConsoleUI() *ConsoleUI {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = os.Getenv("VISUAL")
	}
	if editor == "" {
		editor = "vim" // Default fallback
	}
	return &ConsoleUI{
		styles: NewStyles(),
		editor: editor,
	}
}

// ShowProgress starts a progress indication (e.g. spinner).
// Returns a function to stop the progress.
func (u *ConsoleUI) ShowProgress(msg string) func() {
	done := make(chan bool)
	go func() {
		frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
		i := 0
		for {
			select {
			case <-done:
				fmt.Printf("\r%s... Done!\n", msg)
				return
			default:
				fmt.Printf("\r%s %s...   ", frames[i%len(frames)], msg)
				i++
				time.Sleep(80 * time.Millisecond)
			}
		}
	}()
	return func() {
		done <- true
	}
}

// ReviewMessage presents the commit message to the user for review.
func (u *ConsoleUI) ReviewMessage(ctx context.Context, msg *domain.CommitMessage) (ports.UserAction, *domain.CommitMessage, error) {
	fmt.Println(u.styles.Title.Render("Generated Commit Message"))
	fmt.Println(u.styles.Border.Render(
		fmt.Sprintf("%s\n\n%s",
			u.styles.Subject.Render(msg.Subject),
			u.styles.Body.Render(msg.Body),
		),
	))

	model := newReviewModel()
	p := tea.NewProgram(model)
	finalModel, err := p.Run()
	if err != nil {
		return ports.ActionCancel, nil, err
	}

	m := finalModel.(reviewModel)
	if m.action == ports.ActionEdit {
		// Launch external editor
		editedMsg, err := u.editMessageWithEditor(msg)
		if err != nil {
			u.ShowError(err)
			return ports.ActionEdit, msg, nil // Return original on error
		}
		return ports.ActionEdit, editedMsg, nil
	}

	return m.action, msg, nil
}

// editMessageWithEditor opens an external editor for the user to edit the commit message.
func (u *ConsoleUI) editMessageWithEditor(msg *domain.CommitMessage) (*domain.CommitMessage, error) {
	// Create temp file with current message
	tmpFile, err := os.CreateTemp("", "gitsage-commit-*.txt")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	// Write current message to temp file
	content := msg.Subject
	if msg.Body != "" {
		content += "\n\n" + msg.Body
	}
	if _, err := tmpFile.WriteString(content); err != nil {
		return nil, fmt.Errorf("failed to write to temp file: %w", err)
	}
	tmpFile.Close()

	// Launch editor
	cmd := exec.Command(u.editor, tmpFile.Name())
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("editor failed: %w", err)
	}

	// Read edited content
	editedContent, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		return nil, fmt.Errorf("failed to read edited file: %w", err)
	}

	// Parse edited content
	lines := strings.Split(strings.TrimSpace(string(editedContent)), "\n")
	newMsg := &domain.CommitMessage{
		Raw: string(editedContent),
	}
	if len(lines) > 0 {
		newMsg.Subject = lines[0]
		if len(lines) > 1 {
			// Skip empty line after subject
			bodyStart := 1
			if len(lines) > 1 && strings.TrimSpace(lines[1]) == "" {
				bodyStart = 2
			}
			if bodyStart < len(lines) {
				newMsg.Body = strings.Join(lines[bodyStart:], "\n")
			}
		}
	}

	return newMsg, nil
}

// ShowError displays an error.
func (u *ConsoleUI) ShowError(err error) {
	fmt.Println(u.styles.Error.Render(fmt.Sprintf("Error: %v", err)))
}

// ShowSuccess displays a success message.
func (u *ConsoleUI) ShowSuccess(msg string) {
	fmt.Println(lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true).Render(fmt.Sprintf("Success: %s", msg)))
}

// PromptConfirm prompts the user for confirmation (y/n).
func (u *ConsoleUI) PromptConfirm(msg string) (bool, error) {
	fmt.Printf("%s [y/N]: ", u.styles.Title.Render(msg))

	// Simple scanner for now
	var response string
	_, err := fmt.Scanln(&response)
	if err != nil && err.Error() != "unexpected newline" {
		return false, nil // Treat empty enter as No
	}

	response = strings.ToLower(strings.TrimSpace(response))
	return response == "y" || response == "yes", nil
}

// Styles definition
type Styles struct {
	Title   lipgloss.Style
	Subject lipgloss.Style
	Body    lipgloss.Style
	Error   lipgloss.Style
	Border  lipgloss.Style
}

func NewStyles() *Styles {
	return &Styles{
		Title:   lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39")).MarginBottom(1),
		Subject: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("220")),
		Body:    lipgloss.NewStyle().Foreground(lipgloss.Color("252")),
		Error:   lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("196")),
		Border:  lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(1, 2),
	}
}

// Internal BubbleTea Model for Review
type reviewModel struct {
	action ports.UserAction
	done   bool
}

func newReviewModel() reviewModel {
	return reviewModel{action: ports.ActionCancel}
}

func (m reviewModel) Init() tea.Cmd { return nil }

func (m reviewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "a", "enter":
			m.action = ports.ActionAccept
			return m, tea.Quit
		case "e":
			m.action = ports.ActionEdit
			return m, tea.Quit
		case "r":
			m.action = ports.ActionRegenerate
			return m, tea.Quit
		case "q", "ctrl+c":
			m.action = ports.ActionCancel
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m reviewModel) View() string {
	return "\n[a] Accept  [e] Edit  [r] Regenerate  [q] Cancel\n"
}
