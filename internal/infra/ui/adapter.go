package ui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
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

// ShowProgress starts a progress indication.
// Returns a function to stop the progress.
// TODO: Task 17 - Implement full progress bar with file count, current file, and percentage
func (u *ConsoleUI) ShowProgress(msg string) func() {
	done := make(chan bool, 1)
	finished := make(chan bool, 1)

	go func() {
		frames := []string{"⣾", "⣽", "⣻", "⢿", "⡿", "⣟", "⣯", "⣷"}
		i := 0
		for {
			select {
			case <-done:
				fmt.Printf("\r%s... Done!                    \n", msg)
				finished <- true
				return
			default:
				fmt.Printf("\r%s %s...   ", frames[i%len(frames)], msg)
				i++
				time.Sleep(120 * time.Millisecond)
			}
		}
	}()

	return func() {
		done <- true
		<-finished
	}
}

// ShowFileProgress displays a progress bar with file information using Bubble Tea.
func (u *ConsoleUI) ShowFileProgress(totalFiles int) (chan<- ports.FileProgress, func()) {
	// Initialize models
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	prog := progress.New(
		progress.WithDefaultGradient(),
		progress.WithWidth(40),
		progress.WithoutPercentage(),
	)

	model := progressModel{
		spinner:    s,
		progress:   prog,
		totalFiles: totalFiles,
		phase:      1,
	}

	// Use os.Stderr to avoid interfering with stdout if strictly needed,
	// but p.Run() usually handles term env well.
	p := tea.NewProgram(model)

	// Channel for external updates
	progressChan := make(chan ports.FileProgress)
	doneChan := make(chan struct{})

	// Run program in background
	// Note: p.Run() is blocking, so we run it in a goroutine?
	// Actually, usually p.Run() blocks until Quit.
	// But we need the workflow to continue.
	// So yes, run UI in goroutine.
	go func() {
		if _, err := p.Run(); err != nil {
			fmt.Printf("Alas, there's been an error: %v", err)
		}
		close(doneChan)
	}()

	// Proxy updates from channel to program
	go func() {
		for pMsg := range progressChan {
			p.Send(fileProgressMsg(pMsg))
		}
	}()

	return progressChan, func() {
		// Signal done
		p.Send(progressDoneMsg{})
		// Wait for program to finish
		<-doneChan
	}
}

// -- Bubble Tea Model for Progress --

type fileProgressMsg ports.FileProgress
type progressDoneMsg struct{}
type switchToSpinnerMsg struct{}

type progressModel struct {
	spinner    spinner.Model
	progress   progress.Model
	totalFiles int
	current    int
	filename   string
	phase      int // 1: Analysis, 2: AI Generation
	done       bool
	quitting   bool
}

func (m progressModel) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m progressModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		return m, nil

	case fileProgressMsg:
		m.current = msg.Current
		m.filename = msg.FileName
		m.phase = 1

		// Calculate percentage
		pct := float64(m.current) / float64(m.totalFiles)
		cmd := m.progress.SetPercent(pct)

		// Check if done analyzing
		if m.current >= m.totalFiles {
			// Auto switch to phase 2 after a brief moment or immediately?
			// Let's switch immediately to spinner
			return m, tea.Batch(cmd, func() tea.Msg {
				return switchToSpinnerMsg{}
			})
		}
		return m, cmd

	case switchToSpinnerMsg:
		m.phase = 2
		return m, m.spinner.Tick

	case spinner.TickMsg:
		var cmd tea.Cmd
		if m.phase == 2 {
			m.spinner, cmd = m.spinner.Update(msg)
		}
		return m, cmd

	case progress.FrameMsg:
		newModel, cmd := m.progress.Update(msg)
		if n, ok := newModel.(progress.Model); ok {
			m.progress = n
		}
		return m, cmd

	case progressDoneMsg:
		m.done = true
		m.quitting = true
		return m, tea.Quit
	}

	return m, nil
}

func (m progressModel) View() string {
	if m.quitting {
		// Final success message
		successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true)
		return fmt.Sprintf("  %s Analyzed %d files\n  %s Commit message generated!\n",
			successStyle.Render("✓"), m.totalFiles,
			successStyle.Render("✓"))
	}

	if m.phase == 1 {
		// Progress Bar View
		// 📁 [████░░] internal/foo.go
		fileStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("39"))

		// Truncate filename
		displayName := m.filename
		if len(displayName) > 30 {
			displayName = "..." + displayName[len(displayName)-27:]
		}

		pad := strings.Repeat(" ", 30-len(displayName)) // simple padding

		return fmt.Sprintf("\r  %s %s%s",
			m.progress.View(),
			fileStyle.Render(displayName),
			pad)
	}

	// Phase 2: Spinner
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	return fmt.Sprintf("\r  %s Generating commit message... %s", m.spinner.View(), dimStyle.Render("(this may take a moment)   "))
}

// ShowStreamingText displays streaming text from AI in real-time using Bubble Tea.
func (u *ConsoleUI) ShowStreamingText(title string) (chan<- string, func()) {
	// Initialize spinner
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	model := streamingModel{
		spinner: s,
		title:   title,
	}

	p := tea.NewProgram(model)

	textChan := make(chan string)
	doneChan := make(chan struct{})

	go func() {
		if _, err := p.Run(); err != nil {
			fmt.Printf("Error: %v", err)
		}
		close(doneChan)
	}()

	go func() {
		for chunk := range textChan {
			p.Send(streamMsg(chunk))
		}
	}()

	return textChan, func() {
		p.Send(streamDoneMsg{})
		<-doneChan
	}
}

// -- Bubble Tea Model for Streaming --

type streamMsg string
type streamDoneMsg struct{}

type streamingModel struct {
	spinner spinner.Model
	title   string
	content strings.Builder
	done    bool
}

func (m streamingModel) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m streamingModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	case streamMsg:
		m.content.WriteString(string(msg))
		return m, nil
	case streamDoneMsg:
		m.done = true
		return m, tea.Quit
	case spinner.TickMsg:
		var cmd tea.Cmd
		if !m.done {
			m.spinner, cmd = m.spinner.Update(msg)
		}
		return m, cmd
	}
	return m, nil
}

func (m streamingModel) View() string {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

	header := fmt.Sprintf("\n  %s %s\n%s\n",
		titleStyle.Render("🤖"),
		titleStyle.Render(m.title),
		dimStyle.Render("  ─────────────────────────────────────────"))

	body := m.content.String()

	footer := ""
	if !m.done {
		footer = fmt.Sprintf("\n  %s", m.spinner.View())
	} else {
		footer = fmt.Sprintf("\n%s\n", dimStyle.Render("  ─────────────────────────────────────────"))
	}

	return header + body + footer
}

// ReviewMessage presents the commit message to the user for review.
func (u *ConsoleUI) ReviewMessage(ctx context.Context, msg *domain.CommitMessage) (ports.UserAction, *domain.CommitMessage, error) {
	// Prepare content
	var content string
	if msg.Body != "" {
		content = fmt.Sprintf("%s\n\n%s",
			u.styles.Subject.Render(msg.Subject),
			u.styles.Body.Render(msg.Body),
		)
	} else {
		content = u.styles.Subject.Render(msg.Subject)
	}

	// Calculate content lines to adjust viewport height
	// We count newlines. Soft-wrapping isn't accounted for unless using lipgloss.Width.
	// For better accuracy with long lines, we could improve this, but for now strict line count is safer.
	lines := strings.Count(content, "\n") + 1

	// Dynamic height: min 5 (to accommodate subject + spacing + minimal body or padding), max 15
	const minHeight = 5
	const maxHeight = 15

	height := lines
	// Since we wrap border OUTSIDE, the viewport height is purely for content.

	if height < minHeight {
		height = minHeight
	}
	if height > maxHeight {
		height = maxHeight
	}

	const width = 80

	vp := viewport.New(width, height)
	vp.SetContent(content)
	// Do NOT set vp.Style = border here, as it eats into the height layout.
	// We will render border in the View() method.

	model := reviewModel{
		viewport:    vp,
		title:       u.styles.Title.Render("Generated Commit Message"),
		borderStyle: u.styles.Border,
		action:      ports.ActionCancel,
	}

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

// PromptConfirm prompts the user for confirmation (Y/n).
func (u *ConsoleUI) PromptConfirm(msg string) (bool, error) {
	// Use a style without margin to keep prompt on the same line
	promptStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
	fmt.Printf("%s [Y/n]: ", promptStyle.Render(msg))

	// Simple scanner
	var response string
	_, err := fmt.Scanln(&response)

	// Handle empty input (Enter key) as Yes
	if err != nil {
		if err.Error() == "unexpected newline" {
			return true, nil
		}
		// logic for other errors? usually treat as no or error, let's treat as no for safety unless it's EOF
		return false, nil
	}

	response = strings.ToLower(strings.TrimSpace(response))
	// Default is Yes, so check for explicit No
	if response == "n" || response == "no" {
		return false, nil
	}
	// Everything else (y, yes, etc.) is Yes
	return true, nil
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
	viewport    viewport.Model
	title       string
	borderStyle lipgloss.Style
	action      ports.UserAction
	done        bool
}

func (m reviewModel) Init() tea.Cmd { return nil }

func (m reviewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
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

	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m reviewModel) View() string {
	view := m.borderStyle.Render(m.viewport.View())
	return fmt.Sprintf("%s\n%s\n%s",
		m.title,
		view,
		"\n[a] Accept  [e] Edit  [r] Regenerate  [q] Cancel\n",
	)
}
