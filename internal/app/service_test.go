// Package app contains the application layer with business orchestration logic.
package app

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/gitsage/gitsage/internal/pkg/ai"
	"github.com/gitsage/gitsage/internal/pkg/config"
	"github.com/gitsage/gitsage/internal/pkg/git"
	"github.com/gitsage/gitsage/internal/pkg/history"
	"github.com/gitsage/gitsage/internal/pkg/processor"
	"github.com/gitsage/gitsage/internal/pkg/ui"
)

// MockGitClient is a mock implementation of git.Client
type MockGitClient struct {
	mock.Mock
}

func (m *MockGitClient) GetStagedDiff(ctx context.Context) ([]git.DiffChunk, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]git.DiffChunk), args.Error(1)
}

func (m *MockGitClient) GetDiffStats(ctx context.Context) (*git.DiffStats, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*git.DiffStats), args.Error(1)
}

func (m *MockGitClient) Commit(ctx context.Context, message string) error {
	args := m.Called(ctx, message)
	return args.Error(0)
}

func (m *MockGitClient) HasStagedChanges(ctx context.Context) (bool, error) {
	args := m.Called(ctx)
	return args.Bool(0), args.Error(1)
}

func (m *MockGitClient) HasUnstagedChanges(ctx context.Context) (bool, error) {
	args := m.Called(ctx)
	return args.Bool(0), args.Error(1)
}

func (m *MockGitClient) AddAll(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

// MockAIProvider is a mock implementation of ai.Provider
type MockAIProvider struct {
	mock.Mock
}

func (m *MockAIProvider) GenerateCommitMessage(ctx context.Context, req *ai.GenerateRequest) (*ai.GenerateResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ai.GenerateResponse), args.Error(1)
}

func (m *MockAIProvider) Name() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockAIProvider) ValidateConfig(config ai.ProviderConfig) error {
	args := m.Called(config)
	return args.Error(0)
}

// MockDiffProcessor is a mock implementation of processor.DiffProcessor
type MockDiffProcessor struct {
	mock.Mock
}

func (m *MockDiffProcessor) Process(ctx context.Context, chunks []git.DiffChunk) (*processor.ProcessedDiff, error) {
	args := m.Called(ctx, chunks)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*processor.ProcessedDiff), args.Error(1)
}

// MockUIManager is a mock implementation of ui.Manager
type MockUIManager struct {
	mock.Mock
}

func (m *MockUIManager) DisplayMessage(message *ai.GenerateResponse) error {
	args := m.Called(message)
	return args.Error(0)
}

func (m *MockUIManager) PromptAction() (ui.Action, error) {
	args := m.Called()
	return args.Get(0).(ui.Action), args.Error(1)
}

func (m *MockUIManager) EditMessage(message *ai.GenerateResponse) (*ai.GenerateResponse, error) {
	args := m.Called(message)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ai.GenerateResponse), args.Error(1)
}

func (m *MockUIManager) ShowSpinner(text string) ui.Spinner {
	args := m.Called(text)
	return args.Get(0).(ui.Spinner)
}

func (m *MockUIManager) ShowProgressSpinner(text string, total int) ui.ProgressSpinner {
	args := m.Called(text, total)
	return args.Get(0).(ui.ProgressSpinner)
}

func (m *MockUIManager) ShowError(err error) {
	m.Called(err)
}

func (m *MockUIManager) PromptConfirm(message string) (bool, error) {
	args := m.Called(message)
	return args.Bool(0), args.Error(1)
}

func (m *MockUIManager) ShowSuccess(message string) {
	m.Called(message)
}

// MockSpinner is a mock implementation of ui.Spinner
type MockSpinner struct {
	mock.Mock
}

func (m *MockSpinner) Start() {
	m.Called()
}

func (m *MockSpinner) Stop() {
	m.Called()
}

func (m *MockSpinner) UpdateText(text string) {
	m.Called(text)
}

// MockProgressSpinner is a mock implementation of ui.ProgressSpinner
type MockProgressSpinner struct {
	mock.Mock
}

func (m *MockProgressSpinner) Start() {
	m.Called()
}

func (m *MockProgressSpinner) Stop() {
	m.Called()
}

func (m *MockProgressSpinner) UpdateText(text string) {
	m.Called(text)
}

func (m *MockProgressSpinner) SetTotal(total int) {
	m.Called(total)
}

func (m *MockProgressSpinner) SetCurrent(current int) {
	m.Called(current)
}

func (m *MockProgressSpinner) SetCurrentFile(file string) {
	m.Called(file)
}

// MockHistoryManager is a mock implementation of history.Manager
type MockHistoryManager struct {
	mock.Mock
}

func (m *MockHistoryManager) Save(entry *history.Entry) error {
	args := m.Called(entry)
	return args.Error(0)
}

func (m *MockHistoryManager) List(limit int) ([]*history.Entry, error) {
	args := m.Called(limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*history.Entry), args.Error(1)
}

func (m *MockHistoryManager) Clear() error {
	args := m.Called()
	return args.Error(0)
}

func TestNewCommitService(t *testing.T) {
	gitClient := &MockGitClient{}
	aiProvider := &MockAIProvider{}
	diffProcessor := &MockDiffProcessor{}
	uiManager := &MockUIManager{}
	historyMgr := &MockHistoryManager{}
	cfg := &config.Config{}

	service := NewCommitService(gitClient, aiProvider, diffProcessor, uiManager, historyMgr, cfg)

	assert.NotNil(t, service)
	assert.Equal(t, gitClient, service.gitClient)
	assert.Equal(t, aiProvider, service.aiProvider)
	assert.Equal(t, diffProcessor, service.diffProcessor)
	assert.Equal(t, uiManager, service.uiManager)
	assert.Equal(t, historyMgr, service.historyMgr)
	assert.Equal(t, cfg, service.config)
}

func TestGenerateAndCommit_NoStagedChanges(t *testing.T) {
	gitClient := &MockGitClient{}
	aiProvider := &MockAIProvider{}
	diffProcessor := &MockDiffProcessor{}
	uiManager := &MockUIManager{}
	historyMgr := &MockHistoryManager{}
	cfg := &config.Config{}

	service := NewCommitService(gitClient, aiProvider, diffProcessor, uiManager, historyMgr, cfg)

	// Setup mock expectations
	gitClient.On("HasStagedChanges", mock.Anything).Return(false, nil)

	err := service.GenerateAndCommit(context.Background(), nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no staged changes found")
	gitClient.AssertExpectations(t)
}

func TestGenerateAndCommit_HasStagedChangesError(t *testing.T) {
	gitClient := &MockGitClient{}
	aiProvider := &MockAIProvider{}
	diffProcessor := &MockDiffProcessor{}
	uiManager := &MockUIManager{}
	historyMgr := &MockHistoryManager{}
	cfg := &config.Config{}

	service := NewCommitService(gitClient, aiProvider, diffProcessor, uiManager, historyMgr, cfg)

	// Setup mock expectations
	gitClient.On("HasStagedChanges", mock.Anything).Return(false, errors.New("git error"))

	err := service.GenerateAndCommit(context.Background(), nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to check staged changes")
	gitClient.AssertExpectations(t)
}

func TestGenerateAndCommit_SuccessfulCommit(t *testing.T) {
	gitClient := &MockGitClient{}
	aiProvider := &MockAIProvider{}
	diffProcessor := &MockDiffProcessor{}
	uiManager := &MockUIManager{}
	historyMgr := &MockHistoryManager{}
	spinner := &MockSpinner{}
	cfg := &config.Config{
		History:  config.HistoryConfig{Enabled: true},
		Provider: config.ProviderConfig{Model: "test-model"},
	}

	service := NewCommitService(gitClient, aiProvider, diffProcessor, uiManager, historyMgr, cfg)

	// Setup mock expectations
	chunks := []git.DiffChunk{
		{FilePath: "test.go", ChangeType: git.ChangeTypeModified, Content: "test content"},
	}
	stats := &git.DiffStats{TotalFiles: 1, TotalAdditions: 10, TotalDeletions: 5, Chunks: chunks}
	processedDiff := &processor.ProcessedDiff{
		Chunks:           chunks,
		TotalSize:        100,
		RequiresChunking: false,
	}
	response := &ai.GenerateResponse{
		Subject: "feat: add new feature",
		Body:    "This is the body",
		RawText: "feat: add new feature\n\nThis is the body",
	}

	gitClient.On("HasStagedChanges", mock.Anything).Return(true, nil)
	gitClient.On("GetStagedDiff", mock.Anything).Return(chunks, nil)
	gitClient.On("GetDiffStats", mock.Anything).Return(stats, nil)
	gitClient.On("Commit", mock.Anything, mock.Anything).Return(nil)

	diffProcessor.On("Process", mock.Anything, chunks).Return(processedDiff, nil)

	aiProvider.On("GenerateCommitMessage", mock.Anything, mock.Anything).Return(response, nil)
	aiProvider.On("Name").Return("test-provider")

	uiManager.On("ShowSpinner", mock.Anything).Return(spinner)
	uiManager.On("DisplayMessage", response).Return(nil)
	uiManager.On("PromptAction").Return(ui.ActionAccept, nil)
	uiManager.On("ShowSuccess", mock.Anything).Return()
	uiManager.On("ShowError", mock.Anything).Maybe() // May or may not be called for warnings

	historyMgr.On("Save", mock.Anything).Return(nil)

	spinner.On("Start").Return()
	spinner.On("Stop").Return()

	err := service.GenerateAndCommit(context.Background(), &CommitOptions{})

	assert.NoError(t, err)
	gitClient.AssertExpectations(t)
	aiProvider.AssertExpectations(t)
	diffProcessor.AssertExpectations(t)
}

func TestGenerateAndCommit_DryRun(t *testing.T) {
	gitClient := &MockGitClient{}
	aiProvider := &MockAIProvider{}
	diffProcessor := &MockDiffProcessor{}
	uiManager := &MockUIManager{}
	historyMgr := &MockHistoryManager{}
	spinner := &MockSpinner{}
	cfg := &config.Config{
		History:  config.HistoryConfig{Enabled: true},
		Provider: config.ProviderConfig{Model: "test-model"},
	}

	service := NewCommitService(gitClient, aiProvider, diffProcessor, uiManager, historyMgr, cfg)

	// Setup mock expectations
	chunks := []git.DiffChunk{
		{FilePath: "test.go", ChangeType: git.ChangeTypeModified, Content: "test content"},
	}
	stats := &git.DiffStats{TotalFiles: 1, TotalAdditions: 10, TotalDeletions: 5, Chunks: chunks}
	processedDiff := &processor.ProcessedDiff{
		Chunks:           chunks,
		TotalSize:        100,
		RequiresChunking: false,
	}
	response := &ai.GenerateResponse{
		Subject: "feat: add new feature",
		RawText: "feat: add new feature",
	}

	gitClient.On("HasStagedChanges", mock.Anything).Return(true, nil)
	gitClient.On("GetStagedDiff", mock.Anything).Return(chunks, nil)
	gitClient.On("GetDiffStats", mock.Anything).Return(stats, nil)
	// Note: Commit should NOT be called in dry-run mode

	diffProcessor.On("Process", mock.Anything, chunks).Return(processedDiff, nil)

	aiProvider.On("GenerateCommitMessage", mock.Anything, mock.Anything).Return(response, nil)
	aiProvider.On("Name").Return("test-provider")

	uiManager.On("ShowSpinner", mock.Anything).Return(spinner)
	uiManager.On("DisplayMessage", response).Return(nil)
	uiManager.On("PromptAction").Return(ui.ActionAccept, nil)
	uiManager.On("ShowSuccess", mock.Anything).Return()
	uiManager.On("ShowError", mock.Anything).Return()

	historyMgr.On("Save", mock.Anything).Return(nil)

	spinner.On("Start").Return()
	spinner.On("Stop").Return()

	err := service.GenerateAndCommit(context.Background(), &CommitOptions{DryRun: true})

	assert.NoError(t, err)
	// Verify Commit was NOT called
	gitClient.AssertNotCalled(t, "Commit", mock.Anything, mock.Anything)
}

func TestGenerateAndCommit_Cancel(t *testing.T) {
	gitClient := &MockGitClient{}
	aiProvider := &MockAIProvider{}
	diffProcessor := &MockDiffProcessor{}
	uiManager := &MockUIManager{}
	historyMgr := &MockHistoryManager{}
	spinner := &MockSpinner{}
	cfg := &config.Config{}

	service := NewCommitService(gitClient, aiProvider, diffProcessor, uiManager, historyMgr, cfg)

	// Setup mock expectations
	chunks := []git.DiffChunk{
		{FilePath: "test.go", ChangeType: git.ChangeTypeModified, Content: "test content"},
	}
	stats := &git.DiffStats{TotalFiles: 1, Chunks: chunks}
	processedDiff := &processor.ProcessedDiff{
		Chunks:           chunks,
		TotalSize:        100,
		RequiresChunking: false,
	}
	response := &ai.GenerateResponse{
		Subject: "feat: add new feature",
		RawText: "feat: add new feature",
	}

	gitClient.On("HasStagedChanges", mock.Anything).Return(true, nil)
	gitClient.On("GetStagedDiff", mock.Anything).Return(chunks, nil)
	gitClient.On("GetDiffStats", mock.Anything).Return(stats, nil)

	diffProcessor.On("Process", mock.Anything, chunks).Return(processedDiff, nil)

	aiProvider.On("GenerateCommitMessage", mock.Anything, mock.Anything).Return(response, nil)

	uiManager.On("ShowSpinner", mock.Anything).Return(spinner)
	uiManager.On("DisplayMessage", response).Return(nil)
	uiManager.On("PromptAction").Return(ui.ActionCancel, nil)
	uiManager.On("ShowSuccess", "Commit cancelled").Return()
	uiManager.On("ShowError", mock.Anything).Return()

	spinner.On("Start").Return()
	spinner.On("Stop").Return()

	err := service.GenerateAndCommit(context.Background(), &CommitOptions{})

	assert.NoError(t, err)
	// Verify Commit was NOT called
	gitClient.AssertNotCalled(t, "Commit", mock.Anything, mock.Anything)
}

func TestGenerateAndCommit_Edit(t *testing.T) {
	gitClient := &MockGitClient{}
	aiProvider := &MockAIProvider{}
	diffProcessor := &MockDiffProcessor{}
	uiManager := &MockUIManager{}
	historyMgr := &MockHistoryManager{}
	spinner := &MockSpinner{}
	cfg := &config.Config{
		History: config.HistoryConfig{Enabled: false},
	}

	service := NewCommitService(gitClient, aiProvider, diffProcessor, uiManager, historyMgr, cfg)

	// Setup mock expectations
	chunks := []git.DiffChunk{
		{FilePath: "test.go", ChangeType: git.ChangeTypeModified, Content: "test content"},
	}
	stats := &git.DiffStats{TotalFiles: 1, Chunks: chunks}
	processedDiff := &processor.ProcessedDiff{
		Chunks:           chunks,
		TotalSize:        100,
		RequiresChunking: false,
	}
	response := &ai.GenerateResponse{
		Subject: "feat: add new feature",
		RawText: "feat: add new feature",
	}
	editedResponse := &ai.GenerateResponse{
		Subject: "fix: edited message",
		RawText: "fix: edited message",
	}

	gitClient.On("HasStagedChanges", mock.Anything).Return(true, nil)
	gitClient.On("GetStagedDiff", mock.Anything).Return(chunks, nil)
	gitClient.On("GetDiffStats", mock.Anything).Return(stats, nil)
	gitClient.On("Commit", mock.Anything, "fix: edited message").Return(nil)

	diffProcessor.On("Process", mock.Anything, chunks).Return(processedDiff, nil)

	aiProvider.On("GenerateCommitMessage", mock.Anything, mock.Anything).Return(response, nil)

	uiManager.On("ShowSpinner", mock.Anything).Return(spinner)
	uiManager.On("DisplayMessage", response).Return(nil)
	uiManager.On("PromptAction").Return(ui.ActionEdit, nil)
	uiManager.On("EditMessage", response).Return(editedResponse, nil)
	uiManager.On("ShowSuccess", mock.Anything).Return()
	uiManager.On("ShowError", mock.Anything).Return()

	spinner.On("Start").Return()
	spinner.On("Stop").Return()

	err := service.GenerateAndCommit(context.Background(), &CommitOptions{})

	assert.NoError(t, err)
	gitClient.AssertCalled(t, "Commit", mock.Anything, "fix: edited message")
}

func TestGenerateAndCommit_Regenerate(t *testing.T) {
	gitClient := &MockGitClient{}
	aiProvider := &MockAIProvider{}
	diffProcessor := &MockDiffProcessor{}
	uiManager := &MockUIManager{}
	historyMgr := &MockHistoryManager{}
	spinner := &MockSpinner{}
	cfg := &config.Config{
		History: config.HistoryConfig{Enabled: false},
	}

	service := NewCommitService(gitClient, aiProvider, diffProcessor, uiManager, historyMgr, cfg)

	// Setup mock expectations
	chunks := []git.DiffChunk{
		{FilePath: "test.go", ChangeType: git.ChangeTypeModified, Content: "test content"},
	}
	stats := &git.DiffStats{TotalFiles: 1, Chunks: chunks}
	processedDiff := &processor.ProcessedDiff{
		Chunks:           chunks,
		TotalSize:        100,
		RequiresChunking: false,
	}
	response1 := &ai.GenerateResponse{
		Subject: "feat: first attempt",
		RawText: "feat: first attempt",
	}
	response2 := &ai.GenerateResponse{
		Subject: "feat: second attempt",
		RawText: "feat: second attempt",
	}

	gitClient.On("HasStagedChanges", mock.Anything).Return(true, nil)
	gitClient.On("GetStagedDiff", mock.Anything).Return(chunks, nil)
	gitClient.On("GetDiffStats", mock.Anything).Return(stats, nil)
	gitClient.On("Commit", mock.Anything, "feat: second attempt").Return(nil)

	diffProcessor.On("Process", mock.Anything, chunks).Return(processedDiff, nil)

	// First call returns response1, second call returns response2
	aiProvider.On("GenerateCommitMessage", mock.Anything, mock.MatchedBy(func(req *ai.GenerateRequest) bool {
		return req.PreviousAttempt == ""
	})).Return(response1, nil).Once()
	aiProvider.On("GenerateCommitMessage", mock.Anything, mock.MatchedBy(func(req *ai.GenerateRequest) bool {
		return req.PreviousAttempt != ""
	})).Return(response2, nil).Once()

	uiManager.On("ShowSpinner", mock.Anything).Return(spinner)
	uiManager.On("DisplayMessage", response1).Return(nil).Once()
	uiManager.On("DisplayMessage", response2).Return(nil).Once()
	// First prompt returns regenerate, second returns accept
	uiManager.On("PromptAction").Return(ui.ActionRegenerate, nil).Once()
	uiManager.On("PromptAction").Return(ui.ActionAccept, nil).Once()
	uiManager.On("ShowSuccess", mock.Anything).Return()
	uiManager.On("ShowError", mock.Anything).Return()

	spinner.On("Start").Return()
	spinner.On("Stop").Return()

	err := service.GenerateAndCommit(context.Background(), &CommitOptions{})

	assert.NoError(t, err)
	// Verify AI was called twice
	aiProvider.AssertNumberOfCalls(t, "GenerateCommitMessage", 2)
}

func TestGenerateAndCommit_MaxRegenerationAttempts(t *testing.T) {
	gitClient := &MockGitClient{}
	aiProvider := &MockAIProvider{}
	diffProcessor := &MockDiffProcessor{}
	uiManager := &MockUIManager{}
	historyMgr := &MockHistoryManager{}
	spinner := &MockSpinner{}
	cfg := &config.Config{}

	service := NewCommitService(gitClient, aiProvider, diffProcessor, uiManager, historyMgr, cfg)

	// Setup mock expectations
	chunks := []git.DiffChunk{
		{FilePath: "test.go", ChangeType: git.ChangeTypeModified, Content: "test content"},
	}
	stats := &git.DiffStats{TotalFiles: 1, Chunks: chunks}
	processedDiff := &processor.ProcessedDiff{
		Chunks:           chunks,
		TotalSize:        100,
		RequiresChunking: false,
	}
	response := &ai.GenerateResponse{
		Subject: "feat: attempt",
		RawText: "feat: attempt",
	}

	gitClient.On("HasStagedChanges", mock.Anything).Return(true, nil)
	gitClient.On("GetStagedDiff", mock.Anything).Return(chunks, nil)
	gitClient.On("GetDiffStats", mock.Anything).Return(stats, nil)

	diffProcessor.On("Process", mock.Anything, chunks).Return(processedDiff, nil)

	aiProvider.On("GenerateCommitMessage", mock.Anything, mock.Anything).Return(response, nil)

	uiManager.On("ShowSpinner", mock.Anything).Return(spinner)
	uiManager.On("DisplayMessage", mock.Anything).Return(nil)
	// Always return regenerate to hit the limit
	uiManager.On("PromptAction").Return(ui.ActionRegenerate, nil)
	uiManager.On("ShowError", mock.Anything).Return()

	spinner.On("Start").Return()
	spinner.On("Stop").Return()

	err := service.GenerateAndCommit(context.Background(), &CommitOptions{})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "maximum regeneration attempts reached")
	// Should be called MaxRegenerationAttempts times (initial + 4 regenerations, then error on 5th)
	aiProvider.AssertNumberOfCalls(t, "GenerateCommitMessage", MaxRegenerationAttempts)
}

func TestGenerateAndCommit_ChunkedDiff(t *testing.T) {
	gitClient := &MockGitClient{}
	aiProvider := &MockAIProvider{}
	diffProcessor := &MockDiffProcessor{}
	uiManager := &MockUIManager{}
	historyMgr := &MockHistoryManager{}
	spinner := &MockSpinner{}
	cfg := &config.Config{
		History: config.HistoryConfig{Enabled: false},
	}

	service := NewCommitService(gitClient, aiProvider, diffProcessor, uiManager, historyMgr, cfg)

	// Setup mock expectations with chunked diff
	chunks := []git.DiffChunk{
		{FilePath: "file1.go", ChangeType: git.ChangeTypeModified, Content: "content1"},
		{FilePath: "file2.go", ChangeType: git.ChangeTypeAdded, Content: "content2"},
		{FilePath: "file3.go", ChangeType: git.ChangeTypeModified, Content: "content3"},
	}
	stats := &git.DiffStats{TotalFiles: 3, Chunks: chunks}
	processedDiff := &processor.ProcessedDiff{
		Chunks:           chunks,
		TotalSize:        15000, // Over threshold
		RequiresChunking: true,
		ChunkGroups: []processor.ChunkGroup{
			{Chunks: []git.DiffChunk{chunks[0]}, TotalSize: 5000},
			{Chunks: []git.DiffChunk{chunks[1]}, TotalSize: 5000},
			{Chunks: []git.DiffChunk{chunks[2]}, TotalSize: 5000},
		},
		Summary: "Summary of changes",
	}

	gitClient.On("HasStagedChanges", mock.Anything).Return(true, nil)
	gitClient.On("GetStagedDiff", mock.Anything).Return(chunks, nil)
	gitClient.On("GetDiffStats", mock.Anything).Return(stats, nil)
	gitClient.On("Commit", mock.Anything, mock.Anything).Return(nil)

	diffProcessor.On("Process", mock.Anything, chunks).Return(processedDiff, nil)

	// Each chunk group gets its own AI call
	aiProvider.On("GenerateCommitMessage", mock.Anything, mock.Anything).Return(&ai.GenerateResponse{
		Subject: "feat: chunk change",
		RawText: "feat: chunk change",
	}, nil)

	uiManager.On("ShowSpinner", mock.Anything).Return(spinner)
	uiManager.On("DisplayMessage", mock.Anything).Return(nil)
	uiManager.On("PromptAction").Return(ui.ActionAccept, nil)
	uiManager.On("ShowSuccess", mock.Anything).Return()
	uiManager.On("ShowError", mock.Anything).Return()

	spinner.On("Start").Return()
	spinner.On("Stop").Return()

	err := service.GenerateAndCommit(context.Background(), &CommitOptions{})

	assert.NoError(t, err)
	// Should be called 3 times for 3 chunk groups
	aiProvider.AssertNumberOfCalls(t, "GenerateCommitMessage", 3)
}

func TestGenerateAndCommit_NoChangesAfterFiltering(t *testing.T) {
	gitClient := &MockGitClient{}
	aiProvider := &MockAIProvider{}
	diffProcessor := &MockDiffProcessor{}
	uiManager := &MockUIManager{}
	historyMgr := &MockHistoryManager{}
	spinner := &MockSpinner{}
	cfg := &config.Config{}

	service := NewCommitService(gitClient, aiProvider, diffProcessor, uiManager, historyMgr, cfg)

	// Setup mock expectations - all files are lock files
	chunks := []git.DiffChunk{
		{FilePath: "package-lock.json", IsLockFile: true, Content: "lock content"},
	}
	stats := &git.DiffStats{TotalFiles: 1, Chunks: chunks}
	processedDiff := &processor.ProcessedDiff{
		Chunks:           []git.DiffChunk{}, // Empty after filtering
		TotalSize:        0,
		RequiresChunking: false,
	}

	gitClient.On("HasStagedChanges", mock.Anything).Return(true, nil)
	gitClient.On("GetStagedDiff", mock.Anything).Return(chunks, nil)
	gitClient.On("GetDiffStats", mock.Anything).Return(stats, nil)

	diffProcessor.On("Process", mock.Anything, chunks).Return(processedDiff, nil)

	uiManager.On("ShowSpinner", mock.Anything).Return(spinner)

	spinner.On("Start").Return()
	spinner.On("Stop").Return()

	err := service.GenerateAndCommit(context.Background(), &CommitOptions{})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no changes to commit after filtering")
}
func TestFormatCommitMessage(t *testing.T) {
	service := &CommitService{}

	tests := []struct {
		name     string
		response *ai.GenerateResponse
		expected string
	}{
		{
			name:     "nil response",
			response: nil,
			expected: "",
		},
		{
			name: "subject only",
			response: &ai.GenerateResponse{
				Subject: "feat: add feature",
			},
			expected: "feat: add feature",
		},
		{
			name: "subject and body",
			response: &ai.GenerateResponse{
				Subject: "feat: add feature",
				Body:    "This is the body",
			},
			expected: "feat: add feature\n\nThis is the body",
		},
		{
			name: "full message",
			response: &ai.GenerateResponse{
				Subject: "feat: add feature",
				Body:    "This is the body",
				Footer:  "Refs: #123",
			},
			expected: "feat: add feature\n\nThis is the body\n\nRefs: #123",
		},
		{
			name: "raw text fallback",
			response: &ai.GenerateResponse{
				RawText: "raw commit message",
			},
			expected: "raw commit message",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.formatCommitMessage(tt.response)
			assert.Equal(t, tt.expected, result)
		})
	}
}
