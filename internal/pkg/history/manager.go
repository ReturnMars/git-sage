// Package history provides commit message history management for GitSage.
package history

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	// DefaultMaxEntries is the default maximum number of history entries.
	DefaultMaxEntries = 1000
)

// Entry represents a single history entry.
type Entry struct {
	ID          string    `json:"id"`
	Timestamp   time.Time `json:"timestamp"`
	Message     string    `json:"message"`
	DiffSummary string    `json:"diff_summary"`
	Provider    string    `json:"provider"`
	Model       string    `json:"model"`
	Committed   bool      `json:"committed"`
}

// Manager defines the interface for history management.
type Manager interface {
	Save(entry *Entry) error
	List(limit int) ([]*Entry, error)
	Clear() error
}

// FileManager implements Manager using a JSON file for storage.
type FileManager struct {
	filePath   string
	maxEntries int
	mu         sync.Mutex
}

// NewFileManager creates a new FileManager with the specified file path and max entries.
func NewFileManager(filePath string, maxEntries int) *FileManager {
	if maxEntries <= 0 {
		maxEntries = DefaultMaxEntries
	}
	return &FileManager{
		filePath:   filePath,
		maxEntries: maxEntries,
	}
}

// Save appends a new entry to the history file.
// If the entry has no ID, a new UUID is generated.
// If the entry has no timestamp, the current time is used.
// Automatic rotation is performed if entries exceed maxEntries.
func (m *FileManager) Save(entry *Entry) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Generate ID if not provided
	if entry.ID == "" {
		entry.ID = uuid.New().String()
	}

	// Set timestamp if not provided
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}

	// Load existing entries
	entries, err := m.loadEntries()
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to load history: %w", err)
	}

	// Append new entry
	entries = append(entries, entry)

	// Rotate if exceeding max entries
	if len(entries) > m.maxEntries {
		// Remove oldest entries to maintain the limit
		entries = entries[len(entries)-m.maxEntries:]
	}

	// Save entries
	if err := m.saveEntries(entries); err != nil {
		return fmt.Errorf("failed to save history: %w", err)
	}

	return nil
}

// List returns the most recent entries up to the specified limit.
// If limit is 0 or negative, returns all entries.
func (m *FileManager) List(limit int) ([]*Entry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	entries, err := m.loadEntries()
	if err != nil {
		if os.IsNotExist(err) {
			return []*Entry{}, nil
		}
		return nil, fmt.Errorf("failed to load history: %w", err)
	}

	// Return all entries if limit is not specified
	if limit <= 0 {
		return entries, nil
	}

	// Return the most recent entries (from the end of the list)
	if len(entries) <= limit {
		return entries, nil
	}

	return entries[len(entries)-limit:], nil
}

// Clear removes all entries from the history file.
func (m *FileManager) Clear() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Ensure directory exists
	dir := filepath.Dir(m.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create history directory: %w", err)
	}

	// Write empty array to file
	if err := os.WriteFile(m.filePath, []byte("[]"), 0600); err != nil {
		return fmt.Errorf("failed to clear history: %w", err)
	}

	return nil
}

// loadEntries reads all entries from the history file.
func (m *FileManager) loadEntries() ([]*Entry, error) {
	data, err := os.ReadFile(m.filePath)
	if err != nil {
		return nil, err
	}

	var entries []*Entry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("failed to parse history file: %w", err)
	}

	return entries, nil
}

// saveEntries writes all entries to the history file.
func (m *FileManager) saveEntries(entries []*Entry) error {
	// Ensure directory exists
	dir := filepath.Dir(m.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create history directory: %w", err)
	}

	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal history: %w", err)
	}

	// Write with secure permissions (user read/write only)
	if err := os.WriteFile(m.filePath, data, 0600); err != nil {
		return fmt.Errorf("failed to write history file: %w", err)
	}

	return nil
}
