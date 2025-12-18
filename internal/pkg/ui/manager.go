// Package ui provides interactive terminal UI components for GitSage.
package ui

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"

	"github.com/gitsage/gitsage/internal/pkg/ai"
)

// Action represents a user action in the interactive UI.
type Action int

const (
	ActionAccept Action = iota
	ActionEdit
	ActionRegenerate
	ActionCancel
)

// String returns the string representation of an Action.
func (a Action) String() string {
	switch a {
	case ActionAccept:
		return "accept"
	case ActionEdit:
		return "edit"
	case ActionRegenerate:
		return "regenerate"
	case ActionCancel:
		return "cancel"
	default:
		return "unknown"
	}
}

// Spinner provides loading animation functionality.
type Spinner interface {
	Start()
	Stop()
	UpdateText(text string)
}

// Manager defines the interface for UI operations.
type Manager interface {
	DisplayMessage(message *ai.GenerateResponse) error
	PromptAction() (Action, error)
	EditMessage(message *ai.GenerateResponse) (*ai.GenerateResponse, error)
	ShowSpinner(text string) Spinner
	ShowError(err error)
	ShowSuccess(message string)
}

// DefaultManager implements the Manager interface using charmbracelet libraries.
type DefaultManager struct {
	colorEnabled bool
	editor       string
	styles       *styles
}

// styles holds the lipgloss styles for UI rendering.
type styles struct {
	title      lipgloss.Style
	subject    lipgloss.Style
	body       lipgloss.Style
	footer     lipgloss.Style
	success    lipgloss.Style
	errorStyle lipgloss.Style
	info       lipgloss.Style
	border     lipgloss.Style
}

// NewDefaultManager creates a new DefaultManager with the specified options.
func NewDefaultManager(colorEnabled bool, editor string) *DefaultManager {
	m := &DefaultManager{
		colorEnabled: colorEnabled,
		editor:       editor,
	}
	m.initStyles()
	return m
}

// initStyles initializes the lipgloss styles.
func (m *DefaultManager) initStyles() {
	if !m.colorEnabled {
		m.styles = &styles{
			title:      lipgloss.NewStyle(),
			subject:    lipgloss.NewStyle(),
			body:       lipgloss.NewStyle(),
			footer:     lipgloss.NewStyle(),
			success:    lipgloss.NewStyle(),
			errorStyle: lipgloss.NewStyle(),
			info:       lipgloss.NewStyle(),
			border:     lipgloss.NewStyle(),
		}
		return
	}

	m.styles = &styles{
		title: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("39")).
			MarginBottom(1),
		subject: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("220")),
		body: lipgloss.NewStyle().
			Foreground(lipgloss.Color("252")),
		footer: lipgloss.NewStyle().
			Foreground(lipgloss.Color("245")).
			Italic(true),
		success: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("42")),
		errorStyle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("196")),
		info: lipgloss.NewStyle().
			Foreground(lipgloss.Color("39")),
		border: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(1, 2),
	}
}

// DisplayMessage displays the generated commit message to the user.
func (m *DefaultManager) DisplayMessage(message *ai.GenerateResponse) error {
	if message == nil {
		return fmt.Errorf("message cannot be nil")
	}

	fmt.Println()
	fmt.Println(m.styles.title.Render("📝 Generated Commit Message"))
	fmt.Println()

	// Build the message display
	var msgBuilder strings.Builder

	// Subject line
	subject := message.Subject
	if subject == "" && message.RawText != "" {
		// Fall back to raw text if subject is empty
		lines := strings.Split(message.RawText, "\n")
		if len(lines) > 0 {
			subject = lines[0]
		}
	}
	msgBuilder.WriteString(m.styles.subject.Render(subject))

	// Body
	if message.Body != "" {
		msgBuilder.WriteString("\n\n")
		msgBuilder.WriteString(m.styles.body.Render(message.Body))
	}

	// Footer
	if message.Footer != "" {
		msgBuilder.WriteString("\n\n")
		msgBuilder.WriteString(m.styles.footer.Render(message.Footer))
	}

	// Display in a bordered box
	fmt.Println(m.styles.border.Render(msgBuilder.String()))
	fmt.Println()

	return nil
}

// PromptAction prompts the user to select an action.
func (m *DefaultManager) PromptAction() (Action, error) {
	var selected string

	options := []huh.Option[string]{
		huh.NewOption("✅ Accept - Commit with this message", "accept"),
		huh.NewOption("✏️  Edit - Modify the message", "edit"),
		huh.NewOption("🔄 Regenerate - Generate a new message", "regenerate"),
		huh.NewOption("❌ Cancel - Abort without committing", "cancel"),
	}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("What would you like to do?").
				Options(options...).
				Value(&selected),
		),
	)

	err := form.Run()
	if err != nil {
		// If user pressed Ctrl+C or form was interrupted
		return ActionCancel, nil
	}

	return m.mapSelectionToAction(selected), nil
}

// mapSelectionToAction maps a string selection to an Action enum.
func (m *DefaultManager) mapSelectionToAction(selection string) Action {
	switch selection {
	case "accept":
		return ActionAccept
	case "edit":
		return ActionEdit
	case "regenerate":
		return ActionRegenerate
	case "cancel":
		return ActionCancel
	default:
		return ActionCancel
	}
}

// EditMessage opens an editor for the user to modify the commit message.
func (m *DefaultManager) EditMessage(message *ai.GenerateResponse) (*ai.GenerateResponse, error) {
	if message == nil {
		return nil, fmt.Errorf("message cannot be nil")
	}

	// Format the message for editing
	editContent := m.formatMessageForEdit(message)

	// Try to use external editor first
	editor := m.getEditor()
	if editor != "" {
		edited, err := m.editWithExternalEditor(editor, editContent)
		if err == nil {
			return m.parseEditedMessage(edited), nil
		}
		// Fall back to inline editor if external editor fails
		fmt.Println(m.styles.info.Render("External editor not available, using inline editor..."))
	}

	// Use huh text area for inline editing
	edited, err := m.editWithInlineEditor(editContent)
	if err != nil {
		return nil, fmt.Errorf("failed to edit message: %w", err)
	}

	return m.parseEditedMessage(edited), nil
}

// formatMessageForEdit formats the message for editing.
func (m *DefaultManager) formatMessageForEdit(message *ai.GenerateResponse) string {
	var builder strings.Builder

	// Subject
	subject := message.Subject
	if subject == "" && message.RawText != "" {
		lines := strings.Split(message.RawText, "\n")
		if len(lines) > 0 {
			subject = lines[0]
		}
	}
	builder.WriteString(subject)

	// Body
	if message.Body != "" {
		builder.WriteString("\n\n")
		builder.WriteString(message.Body)
	}

	// Footer
	if message.Footer != "" {
		builder.WriteString("\n\n")
		builder.WriteString(message.Footer)
	}

	return builder.String()
}

// getEditor returns the editor to use for editing messages.
func (m *DefaultManager) getEditor() string {
	if m.editor != "" {
		return m.editor
	}

	// Check environment variables
	if editor := os.Getenv("EDITOR"); editor != "" {
		return editor
	}
	if editor := os.Getenv("VISUAL"); editor != "" {
		return editor
	}

	return ""
}

// editWithExternalEditor opens an external editor for editing.
func (m *DefaultManager) editWithExternalEditor(editor, content string) (string, error) {
	// Create a temporary file
	tmpFile, err := os.CreateTemp("", "gitsage-commit-*.txt")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	// Write content to temp file
	if _, err := tmpFile.WriteString(content); err != nil {
		tmpFile.Close()
		return "", fmt.Errorf("failed to write to temp file: %w", err)
	}
	tmpFile.Close()

	// Open editor
	cmd := exec.Command(editor, tmpPath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("editor failed: %w", err)
	}

	// Read edited content
	edited, err := os.ReadFile(tmpPath)
	if err != nil {
		return "", fmt.Errorf("failed to read edited file: %w", err)
	}

	return string(edited), nil
}

// editWithInlineEditor uses huh text area for inline editing.
func (m *DefaultManager) editWithInlineEditor(content string) (string, error) {
	edited := content

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewText().
				Title("Edit Commit Message").
				Description("Modify the message below. First line is the subject.").
				Value(&edited).
				CharLimit(0), // No limit
		),
	)

	if err := form.Run(); err != nil {
		return "", err
	}

	return edited, nil
}

// parseEditedMessage parses the edited text back into a GenerateResponse.
func (m *DefaultManager) parseEditedMessage(edited string) *ai.GenerateResponse {
	edited = strings.TrimSpace(edited)
	if edited == "" {
		return &ai.GenerateResponse{}
	}

	// Split by double newlines to separate sections
	parts := strings.SplitN(edited, "\n\n", 3)

	response := &ai.GenerateResponse{
		RawText: edited,
	}

	// First part is always the subject
	if len(parts) > 0 {
		response.Subject = strings.TrimSpace(parts[0])
	}

	// Second part is the body
	if len(parts) > 1 {
		response.Body = strings.TrimSpace(parts[1])
	}

	// Third part is the footer
	if len(parts) > 2 {
		response.Footer = strings.TrimSpace(parts[2])
	}

	return response
}

// ShowSpinner creates and returns a spinner for loading states.
func (m *DefaultManager) ShowSpinner(text string) Spinner {
	return &defaultSpinner{
		text:    text,
		running: false,
		done:    make(chan struct{}),
	}
}

// ShowError displays an error message to the user.
func (m *DefaultManager) ShowError(err error) {
	if err == nil {
		return
	}
	fmt.Println()
	fmt.Println(m.styles.errorStyle.Render("❌ Error: " + err.Error()))
	fmt.Println()
}

// ShowSuccess displays a success message to the user.
func (m *DefaultManager) ShowSuccess(message string) {
	fmt.Println()
	fmt.Println(m.styles.success.Render("✅ " + message))
	fmt.Println()
}

// defaultSpinner implements the Spinner interface.
type defaultSpinner struct {
	text    string
	running bool
	done    chan struct{}
}

// spinnerFrames contains the animation frames for the spinner.
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// Start begins the spinner animation.
func (s *defaultSpinner) Start() {
	if s.running {
		return
	}
	s.running = true
	s.done = make(chan struct{})

	go func() {
		frameIdx := 0
		ticker := time.NewTicker(80 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-s.done:
				// Clear the spinner line
				fmt.Print("\r\033[K")
				return
			case <-ticker.C:
				frame := spinnerFrames[frameIdx%len(spinnerFrames)]
				fmt.Printf("\r%s %s", frame, s.text)
				frameIdx++
			}
		}
	}()
}

// Stop stops the spinner animation.
func (s *defaultSpinner) Stop() {
	if !s.running {
		return
	}
	s.running = false
	close(s.done)
	// Give the goroutine time to clean up
	time.Sleep(100 * time.Millisecond)
}

// UpdateText updates the spinner text.
func (s *defaultSpinner) UpdateText(text string) {
	s.text = text
}

// NonInteractiveManager implements Manager for non-interactive mode (--yes flag).
type NonInteractiveManager struct {
	colorEnabled bool
	styles       *styles
}

// NewNonInteractiveManager creates a new NonInteractiveManager.
func NewNonInteractiveManager(colorEnabled bool) *NonInteractiveManager {
	m := &NonInteractiveManager{
		colorEnabled: colorEnabled,
	}
	m.initStyles()
	return m
}

// initStyles initializes the lipgloss styles.
func (m *NonInteractiveManager) initStyles() {
	if !m.colorEnabled {
		m.styles = &styles{
			title:      lipgloss.NewStyle(),
			subject:    lipgloss.NewStyle(),
			body:       lipgloss.NewStyle(),
			footer:     lipgloss.NewStyle(),
			success:    lipgloss.NewStyle(),
			errorStyle: lipgloss.NewStyle(),
			info:       lipgloss.NewStyle(),
			border:     lipgloss.NewStyle(),
		}
		return
	}

	m.styles = &styles{
		title: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("39")),
		subject: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("220")),
		body: lipgloss.NewStyle().
			Foreground(lipgloss.Color("252")),
		footer: lipgloss.NewStyle().
			Foreground(lipgloss.Color("245")).
			Italic(true),
		success: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("42")),
		errorStyle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("196")),
		info: lipgloss.NewStyle().
			Foreground(lipgloss.Color("39")),
		border: lipgloss.NewStyle(),
	}
}

// DisplayMessage displays the generated commit message.
func (m *NonInteractiveManager) DisplayMessage(message *ai.GenerateResponse) error {
	if message == nil {
		return fmt.Errorf("message cannot be nil")
	}

	subject := message.Subject
	if subject == "" && message.RawText != "" {
		lines := strings.Split(message.RawText, "\n")
		if len(lines) > 0 {
			subject = lines[0]
		}
	}

	fmt.Println(subject)
	if message.Body != "" {
		fmt.Println()
		fmt.Println(message.Body)
	}
	if message.Footer != "" {
		fmt.Println()
		fmt.Println(message.Footer)
	}

	return nil
}

// PromptAction always returns ActionAccept in non-interactive mode.
func (m *NonInteractiveManager) PromptAction() (Action, error) {
	return ActionAccept, nil
}

// EditMessage returns the original message unchanged in non-interactive mode.
func (m *NonInteractiveManager) EditMessage(message *ai.GenerateResponse) (*ai.GenerateResponse, error) {
	return message, nil
}

// ShowSpinner returns a no-op spinner in non-interactive mode.
func (m *NonInteractiveManager) ShowSpinner(text string) Spinner {
	return &noopSpinner{}
}

// ShowError displays an error message.
func (m *NonInteractiveManager) ShowError(err error) {
	if err == nil {
		return
	}
	fmt.Fprintf(os.Stderr, "Error: %s\n", err.Error())
}

// ShowSuccess displays a success message.
func (m *NonInteractiveManager) ShowSuccess(message string) {
	fmt.Println(message)
}

// noopSpinner is a no-op implementation of Spinner.
type noopSpinner struct{}

func (s *noopSpinner) Start()            {}
func (s *noopSpinner) Stop()             {}
func (s *noopSpinner) UpdateText(string) {}
