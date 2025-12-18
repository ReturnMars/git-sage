// Package ui provides interactive terminal UI components for GitSage.
package ui

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
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

// ProgressSpinner provides loading animation with progress tracking.
type ProgressSpinner interface {
	Spinner
	SetTotal(total int)
	SetCurrent(current int)
	SetCurrentFile(file string)
}

// Manager defines the interface for UI operations.
type Manager interface {
	DisplayMessage(message *ai.GenerateResponse) error
	PromptAction() (Action, error)
	EditMessage(message *ai.GenerateResponse) (*ai.GenerateResponse, error)
	ShowSpinner(text string) Spinner
	ShowProgressSpinner(text string, total int) ProgressSpinner
	ShowError(err error)
	ShowSuccess(message string)
	PromptConfirm(message string) (bool, error)
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
			Padding(1, 2).
			Width(80),
	}
}

// DisplayMessage displays the generated commit message to the user.
func (m *DefaultManager) DisplayMessage(message *ai.GenerateResponse) error {
	if message == nil {
		return fmt.Errorf("message cannot be nil")
	}

	fmt.Println()
	fmt.Println(m.styles.title.Render("Generated Commit Message"))
	fmt.Println(strings.Repeat("-", 50))

	// Subject line
	subject := message.Subject
	if subject == "" && message.RawText != "" {
		// Fall back to raw text if subject is empty
		lines := strings.Split(message.RawText, "\n")
		if len(lines) > 0 {
			subject = lines[0]
		}
	}
	fmt.Println(m.styles.subject.Render(subject))

	// Body
	if message.Body != "" {
		fmt.Println()
		fmt.Println(m.styles.body.Render(message.Body))
	}

	// Footer
	if message.Footer != "" {
		fmt.Println()
		fmt.Println(m.styles.footer.Render(message.Footer))
	}

	fmt.Println(strings.Repeat("-", 50))
	fmt.Println()

	return nil
}

// PromptAction prompts the user to select an action using Bubble Tea.
func (m *DefaultManager) PromptAction() (Action, error) {
	model := newActionSelectModel()
	p := tea.NewProgram(model)

	finalModel, err := p.Run()
	if err != nil {
		return ActionCancel, err
	}

	result := finalModel.(actionSelectModel)
	return result.selected, nil
}

// actionSelectModel is the Bubble Tea model for action selection.
type actionSelectModel struct {
	choices  []actionChoice
	cursor   int
	selected Action
	done     bool
}

type actionChoice struct {
	action Action
	label  string
	icon   string
	desc   string
}

func newActionSelectModel() actionSelectModel {
	return actionSelectModel{
		choices: []actionChoice{
			{ActionAccept, "Accept", "›", "Commit with this message"},
			{ActionEdit, "Edit", "•", "Modify the message"},
			{ActionRegenerate, "Regenerate", "↻", "Generate a new message"},
			{ActionCancel, "Cancel", "×", "Abort without committing"},
		},
		cursor:   0,
		selected: ActionCancel,
	}
}

func (m actionSelectModel) Init() tea.Cmd {
	return nil
}

func (m actionSelectModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.selected = ActionCancel
			m.done = true
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.choices)-1 {
				m.cursor++
			}
		case "enter", " ":
			m.selected = m.choices[m.cursor].action
			m.done = true
			return m, tea.Quit
		case "1":
			m.selected = ActionAccept
			m.done = true
			return m, tea.Quit
		case "2":
			m.selected = ActionEdit
			m.done = true
			return m, tea.Quit
		case "3":
			m.selected = ActionRegenerate
			m.done = true
			return m, tea.Quit
		case "4":
			m.selected = ActionCancel
			m.done = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m actionSelectModel) View() string {
	if m.done {
		return ""
	}

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("39"))

	selectedStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("212"))

	normalStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("252"))

	descStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("245"))

	var sb strings.Builder
	sb.WriteString(titleStyle.Render("What would you like to do?"))
	sb.WriteString("\n\n")

	for i, choice := range m.choices {
		cursor := "  "
		style := normalStyle
		if m.cursor == i {
			cursor = "▸ "
			style = selectedStyle
		}

		line := fmt.Sprintf("%s%s %s", cursor, choice.icon, style.Render(choice.label))
		sb.WriteString(line)
		sb.WriteString(descStyle.Render(fmt.Sprintf(" - %s", choice.desc)))
		sb.WriteString("\n")
	}

	sb.WriteString("\n")
	sb.WriteString(descStyle.Render("↑/↓ or j/k to move • Enter to select • 1-4 quick select • q to cancel"))

	return sb.String()
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
				Description("Edit below. Press Ctrl+D or Tab then Enter to save. Ctrl+C or Esc to cancel.").
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
	return newBubbleSpinner(text)
}

// ShowProgressSpinner creates a spinner with progress tracking.
func (m *DefaultManager) ShowProgressSpinner(text string, total int) ProgressSpinner {
	return newBubbleProgressSpinner(text, total)
}

// ShowError displays an error message to the user.
func (m *DefaultManager) ShowError(err error) {
	if err == nil {
		return
	}
	fmt.Println()
	fmt.Println(m.styles.errorStyle.Render("Error: " + err.Error()))
	fmt.Println()
}

// PromptConfirm prompts the user for a yes/no confirmation using Bubble Tea.
func (m *DefaultManager) PromptConfirm(message string) (bool, error) {
	model := newConfirmModel(message)
	p := tea.NewProgram(model)

	finalModel, err := p.Run()
	if err != nil {
		return false, err
	}

	result := finalModel.(confirmModel)
	return result.confirmed, nil
}

// confirmModel is the Bubble Tea model for yes/no confirmation.
type confirmModel struct {
	message   string
	cursor    int // 0 = Yes, 1 = No
	confirmed bool
	done      bool
}

func newConfirmModel(message string) confirmModel {
	return confirmModel{
		message: message,
		cursor:  0, // Default to Yes
	}
}

func (m confirmModel) Init() tea.Cmd {
	return nil
}

func (m confirmModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "n":
			m.confirmed = false
			m.done = true
			return m, tea.Quit
		case "y", "Y":
			m.confirmed = true
			m.done = true
			return m, tea.Quit
		case "left", "h":
			m.cursor = 0
		case "right", "l":
			m.cursor = 1
		case "enter", " ":
			m.confirmed = m.cursor == 0
			m.done = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m confirmModel) View() string {
	if m.done {
		return ""
	}

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("220"))

	selectedStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("42"))

	normalStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("245"))

	var sb strings.Builder
	sb.WriteString(titleStyle.Render(m.message))
	sb.WriteString(" ")

	yesStyle := normalStyle
	noStyle := normalStyle
	if m.cursor == 0 {
		yesStyle = selectedStyle
	} else {
		noStyle = selectedStyle
	}

	sb.WriteString(yesStyle.Render("[Y]es"))
	sb.WriteString(" / ")
	sb.WriteString(noStyle.Render("[N]o"))

	return sb.String()
}

// ShowSuccess displays a success message to the user.
func (m *DefaultManager) ShowSuccess(message string) {
	fmt.Println()
	fmt.Println(m.styles.success.Render("[OK] " + message))
	fmt.Println()
}

// bubbleSpinner implements Spinner using Bubble Tea.
type bubbleSpinner struct {
	text    string
	program *tea.Program
	model   *spinnerModel
	mu      sync.Mutex
}

// spinnerModel is the Bubble Tea model for simple spinner.
type spinnerModel struct {
	spinner  spinner.Model
	text     string
	quitting bool
}

// spinnerTickMsg is sent to update spinner text from outside.
type spinnerTickMsg struct {
	text string
}

// spinnerQuitMsg signals the spinner to quit.
type spinnerQuitMsg struct{}

func (m spinnerModel) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m spinnerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case spinnerTickMsg:
		m.text = msg.text
		return m, nil
	case spinnerQuitMsg:
		m.quitting = true
		return m, tea.Quit
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m spinnerModel) View() string {
	if m.quitting {
		return ""
	}
	return fmt.Sprintf("%s %s", m.spinner.View(), m.text)
}

func newBubbleSpinner(text string) *bubbleSpinner {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	model := &spinnerModel{
		spinner: s,
		text:    text,
	}

	return &bubbleSpinner{
		text:  text,
		model: model,
	}
}

func (s *bubbleSpinner) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.program = tea.NewProgram(s.model)
	go func() {
		_, _ = s.program.Run()
	}()
}

func (s *bubbleSpinner) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.program != nil {
		s.program.Send(spinnerQuitMsg{})
		time.Sleep(50 * time.Millisecond)
	}
}

func (s *bubbleSpinner) UpdateText(text string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.text = text
	if s.program != nil {
		s.program.Send(spinnerTickMsg{text: text})
	}
}

// bubbleProgressSpinner implements ProgressSpinner using Bubble Tea.
type bubbleProgressSpinner struct {
	text        string
	total       int
	current     int
	currentFile string
	program     *tea.Program
	mu          sync.Mutex
}

// progressModel is the Bubble Tea model for progress spinner.
type progressModel struct {
	spinner     spinner.Model
	progress    progress.Model
	text        string
	total       int
	current     int
	currentFile string
	quitting    bool
}

// progressUpdateMsg updates progress state.
type progressUpdateMsg struct {
	current     int
	total       int
	text        string
	currentFile string
}

// progressQuitMsg signals quit.
type progressQuitMsg struct{}

func (m progressModel) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m progressModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case progressUpdateMsg:
		m.current = msg.current
		m.total = msg.total
		if msg.text != "" {
			m.text = msg.text
		}
		m.currentFile = msg.currentFile
		return m, nil
	case progressQuitMsg:
		m.quitting = true
		return m, tea.Quit
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case progress.FrameMsg:
		progressModel, cmd := m.progress.Update(msg)
		m.progress = progressModel.(progress.Model)
		return m, cmd
	}
	return m, nil
}

func (m progressModel) View() string {
	if m.quitting {
		return ""
	}

	// Calculate percentage
	percent := 0.0
	if m.total > 0 {
		percent = float64(m.current) / float64(m.total)
	}

	// Build the view
	var sb strings.Builder
	sb.WriteString(m.spinner.View())
	sb.WriteString(" ")
	sb.WriteString(m.progress.ViewAs(percent))
	sb.WriteString(fmt.Sprintf(" %d/%d ", m.current, m.total))
	sb.WriteString(m.text)

	if m.currentFile != "" {
		file := m.currentFile
		if len(file) > 25 {
			file = "..." + file[len(file)-22:]
		}
		sb.WriteString(" → ")
		sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Render(file))
	}

	return sb.String()
}

func newBubbleProgressSpinner(text string, total int) *bubbleProgressSpinner {
	return &bubbleProgressSpinner{
		text:  text,
		total: total,
	}
}

func (s *bubbleProgressSpinner) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	prog := progress.New(
		progress.WithDefaultGradient(),
		progress.WithWidth(20),
		progress.WithoutPercentage(),
	)

	model := progressModel{
		spinner:  sp,
		progress: prog,
		text:     s.text,
		total:    s.total,
		current:  0,
	}

	s.program = tea.NewProgram(model)
	go func() {
		_, _ = s.program.Run()
	}()
}

func (s *bubbleProgressSpinner) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.program != nil {
		s.program.Send(progressQuitMsg{})
		time.Sleep(50 * time.Millisecond)
	}
}

func (s *bubbleProgressSpinner) UpdateText(text string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.text = text
	if s.program != nil {
		s.program.Send(progressUpdateMsg{
			current:     s.current,
			total:       s.total,
			text:        text,
			currentFile: s.currentFile,
		})
	}
}

func (s *bubbleProgressSpinner) SetTotal(total int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.total = total
	if s.program != nil {
		s.program.Send(progressUpdateMsg{
			current:     s.current,
			total:       total,
			text:        s.text,
			currentFile: s.currentFile,
		})
	}
}

func (s *bubbleProgressSpinner) SetCurrent(current int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.current = current
	if s.program != nil {
		s.program.Send(progressUpdateMsg{
			current:     current,
			total:       s.total,
			text:        s.text,
			currentFile: s.currentFile,
		})
	}
}

func (s *bubbleProgressSpinner) SetCurrentFile(file string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.currentFile = file
	if s.program != nil {
		s.program.Send(progressUpdateMsg{
			current:     s.current,
			total:       s.total,
			text:        s.text,
			currentFile: file,
		})
	}
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

// ShowProgressSpinner returns a no-op progress spinner in non-interactive mode.
func (m *NonInteractiveManager) ShowProgressSpinner(text string, total int) ProgressSpinner {
	return &noopProgressSpinner{}
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

// PromptConfirm always returns true in non-interactive mode.
func (m *NonInteractiveManager) PromptConfirm(message string) (bool, error) {
	return true, nil
}

// noopSpinner is a no-op implementation of Spinner.
type noopSpinner struct{}

func (s *noopSpinner) Start()            {}
func (s *noopSpinner) Stop()             {}
func (s *noopSpinner) UpdateText(string) {}

// noopProgressSpinner is a no-op implementation of ProgressSpinner.
type noopProgressSpinner struct{}

func (s *noopProgressSpinner) Start()                {}
func (s *noopProgressSpinner) Stop()                 {}
func (s *noopProgressSpinner) UpdateText(string)     {}
func (s *noopProgressSpinner) SetTotal(int)          {}
func (s *noopProgressSpinner) SetCurrent(int)        {}
func (s *noopProgressSpinner) SetCurrentFile(string) {}
