# Design Document

## Overview

GitSage is a CLI tool built in Go that bridges Git version control with AI-powered commit message generation. The architecture follows Clean Architecture principles with clear separation between CLI, business logic, and infrastructure layers. The system supports multiple AI providers through a pluggable interface pattern, enabling users to choose between cloud-based services (OpenAI, DeepSeek) and local models (Ollama) based on their needs for cost, privacy, and availability.

The core workflow involves: (1) extracting staged git changes, (2) intelligently processing and chunking diffs, (3) generating commit messages via AI, (4) presenting an interactive UI for user review, and (5) executing the commit operation.

## Architecture

### High-Level Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                         CLI Layer                            │
│                    (Cobra Commands)                          │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐   │
│  │  commit  │  │ generate │  │  config  │  │ history  │   │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘   │
└─────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│                    Application Layer                         │
│                  (Business Orchestration)                    │
│  ┌────────────────────────────────────────────────────────┐ │
│  │           CommitMessageGenerator Service               │ │
│  │  • Orchestrates workflow                               │ │
│  │  • Coordinates Git, AI, and UI components              │ │
│  └────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────┘
                            │
        ┌───────────────────┼───────────────────┐
        ▼                   ▼                   ▼
┌──────────────┐   ┌──────────────┐   ┌──────────────┐
│ Git Client   │   │ AI Provider  │   │ UI Manager   │
│              │   │  Interface   │   │              │
│ • GetDiff    │   │              │   │ • Prompt     │
│ • Commit     │   │ ┌──────────┐ │   │ • Edit       │
│ • Status     │   │ │ OpenAI   │ │   │ • Confirm    │
│              │   │ └──────────┘ │   │              │
│              │   │ ┌──────────┐ │   │              │
│              │   │ │DeepSeek  │ │   │              │
│              │   │ └──────────┘ │   │              │
│              │   │ ┌──────────┐ │   │              │
│              │   │ │ Ollama   │ │   │              │
│              │   │ └──────────┘ │   │              │
└──────────────┘   └──────────────┘   └──────────────┘
```

### Layer Responsibilities

**CLI Layer (`cmd/gitsage`)**
- Parse command-line arguments and flags using Cobra
- Validate user input
- Delegate to application layer services
- Handle process exit codes

**Application Layer (`internal/app`)**
- Implement core business logic
- Orchestrate interactions between infrastructure components
- Manage workflow state and error handling
- Enforce business rules (e.g., Conventional Commits format)

**Infrastructure Layer (`internal/pkg`)**
- Git operations (via os/exec)
- AI provider integrations (via HTTP clients)
- Configuration management (via Viper)
- Terminal UI (via charmbracelet libraries)
- File system operations (history storage)

## Components and Interfaces

### 1. Git Client (`internal/pkg/git`)

```go
type Client interface {
    GetStagedDiff(ctx context.Context) ([]DiffChunk, error)
    GetDiffStats(ctx context.Context) (*DiffStats, error)
    Commit(ctx context.Context, message string) error
    HasStagedChanges(ctx context.Context) (bool, error)
}

type DiffChunk struct {
    FilePath    string
    ChangeType  ChangeType // Added, Modified, Deleted
    Additions   int
    Deletions   int
    Content     string
    IsLockFile  bool
}

type DiffStats struct {
    TotalFiles    int
    TotalAdditions int
    TotalDeletions int
    Chunks        []DiffChunk
}

type ChangeType int

const (
    ChangeTypeAdded ChangeType = iota
    ChangeTypeModified
    ChangeTypeDeleted
)
```

**Implementation Notes:**
- Use `exec.CommandContext` to invoke git commands
- Parse `git diff --cached --numstat` for statistics
- Parse `git diff --cached` for full content
- Filter lock files based on filename patterns
- Implement chunking logic to split large diffs

### 2. AI Provider (`internal/pkg/ai`)

```go
type Provider interface {
    GenerateCommitMessage(ctx context.Context, req *GenerateRequest) (*GenerateResponse, error)
    Name() string
    ValidateConfig(config ProviderConfig) error
}

type GenerateRequest struct {
    DiffChunks      []git.DiffChunk
    DiffStats       *git.DiffStats
    CustomPrompt    string
    PreviousAttempt string // For regeneration context
}

type GenerateResponse struct {
    Subject  string
    Body     string
    Footer   string
    RawText  string
}

type ProviderConfig struct {
    APIKey      string
    Model       string
    Endpoint    string
    Temperature float32
    MaxTokens   int
}
```

**Provider Implementations:**

**OpenAI Provider (`internal/pkg/ai/openai.go`)**
- Use `github.com/sashabaranov/go-openai` library
- Default model: `gpt-4o-mini`
- Support custom endpoints for OpenAI-compatible APIs

**DeepSeek Provider (`internal/pkg/ai/deepseek.go`)**
- Use OpenAI-compatible client with DeepSeek endpoint
- Default model: `deepseek-chat`
- Endpoint: `https://api.deepseek.com/v1`

**Ollama Provider (`internal/pkg/ai/ollama.go`)**
- Use HTTP client to call local Ollama API
- Default endpoint: `http://localhost:11434`
- Default model: `codellama` or user-configured

### 3. Diff Processor (`internal/pkg/processor`)

```go
type DiffProcessor interface {
    Process(ctx context.Context, chunks []git.DiffChunk) (*ProcessedDiff, error)
}

type ProcessedDiff struct {
    Chunks           []git.DiffChunk
    Summary          string
    TotalSize        int
    RequiresChunking bool
    ChunkGroups      []ChunkGroup  // For parallel processing
}

type ChunkGroup struct {
    Chunks    []git.DiffChunk
    TotalSize int
}

type ChunkingStrategy int

const (
    ChunkingByFile ChunkingStrategy = iota
    ChunkingBySize
    ChunkingBySummary
)
```

**Processing Rules:**
- Filter out lock files (package-lock.json, go.sum, yarn.lock, Cargo.lock, pnpm-lock.yaml)
- Calculate total diff size in bytes
- If size > threshold (default 10KB), enable chunking
- For chunked diffs, generate per-file summaries
- Group chunks for potential parallel processing (max 3 concurrent AI calls)
- Aggregate summaries into a coherent overview

**Optimization:**
- Use streaming parser to avoid loading entire diff into memory
- Implement chunk grouping to balance between parallelism and API rate limits
- Cache processed diffs by hash to avoid reprocessing identical changes

### 4. Configuration Manager (`internal/pkg/config`)

```go
type Config struct {
    Provider    ProviderConfig
    Git         GitConfig
    UI          UIConfig
    History     HistoryConfig
}

type ProviderConfig struct {
    Name        string  // "openai", "deepseek", "ollama"
    APIKey      string
    Model       string
    Endpoint    string
    Temperature float32
    MaxTokens   int
}

type GitConfig struct {
    DiffSizeThreshold int
    ExcludePatterns   []string
}

type UIConfig struct {
    Editor          string
    ColorEnabled    bool
    SpinnerStyle    string
}

type HistoryConfig struct {
    Enabled     bool
    MaxEntries  int
    FilePath    string
}

type Manager interface {
    Load() (*Config, error)
    Save(config *Config) error
    Set(key string, value string) error
    Get(key string) (string, error)
    Init() error
}
```

**Configuration Priority:**
1. Command-line flags (highest)
2. Environment variables (e.g., `GITSAGE_API_KEY`)
3. Config file (`~/.gitsage/config.yaml`)
4. Default values (lowest)

**Default Configuration:**
```yaml
provider:
  name: openai
  model: gpt-4o-mini
  temperature: 0.2
  max_tokens: 500

git:
  diff_size_threshold: 10240  # 10KB
  exclude_patterns:
    - "*.lock"
    - "go.sum"
    - "package-lock.json"
    - "yarn.lock"
    - "pnpm-lock.yaml"
    - "Cargo.lock"

ui:
  editor: ""  # Use $EDITOR or $VISUAL
  color_enabled: true
  spinner_style: "dots"

history:
  enabled: true
  max_entries: 1000
  file_path: "~/.gitsage/history.json"
```

### 5. Interactive UI (`internal/pkg/ui`)

```go
type Manager interface {
    DisplayMessage(message *ai.GenerateResponse) error
    PromptAction() (Action, error)
    EditMessage(message *ai.GenerateResponse) (*ai.GenerateResponse, error)
    ShowSpinner(text string) Spinner
    ShowError(err error)
    ShowSuccess(message string)
}

type Action int

const (
    ActionAccept Action = iota
    ActionEdit
    ActionRegenerate
    ActionCancel
)

type Spinner interface {
    Start()
    Stop()
    UpdateText(text string)
}
```

**Implementation:**
- Use `github.com/charmbracelet/huh` for forms and prompts
- Use `github.com/charmbracelet/lipgloss` for styling
- Use `github.com/briandowns/spinner` for loading animations
- Support both interactive and non-interactive modes (for `--yes` flag)

### 6. History Manager (`internal/pkg/history`)

```go
type Manager interface {
    Save(entry *Entry) error
    List(limit int) ([]*Entry, error)
    Clear() error
}

type Entry struct {
    ID          string
    Timestamp   time.Time
    Message     string
    DiffSummary string
    Provider    string
    Model       string
    Committed   bool
}
```

**Storage Format:**
- JSON file at `~/.gitsage/history.json`
- Each entry is a JSON object
- Automatic rotation when exceeding max entries

### 7. Application Service (`internal/app`)

```go
type CommitService struct {
    gitClient     git.Client
    aiProvider    ai.Provider
    diffProcessor processor.DiffProcessor
    uiManager     ui.Manager
    historyMgr    history.Manager
    config        *config.Config
}

func (s *CommitService) GenerateAndCommit(ctx context.Context, opts *CommitOptions) error {
    // 1. Check for staged changes
    // 2. Get diff and stats
    // 3. Process diff (filter, chunk)
    // 4. Generate commit message via AI
    // 5. Display in interactive UI
    // 6. Handle user action (accept/edit/regenerate/cancel)
    // 7. Execute commit or save to file
    // 8. Save to history
}

type CommitOptions struct {
    DryRun       bool
    OutputFile   string
    SkipConfirm  bool
    CustomPrompt string
}
```

## Data Models

### Commit Message Structure

```go
type CommitMessage struct {
    Type    string   // feat, fix, docs, etc.
    Scope   string   // Optional scope
    Subject string   // Short description (max 72 chars)
    Body    string   // Optional detailed description
    Footer  string   // Optional footer (breaking changes, refs)
}

func (cm *CommitMessage) Format() string {
    // Returns formatted string following Conventional Commits
}

func (cm *CommitMessage) Validate() error {
    // Validates format compliance
}
```

### Prompt Template

```go
type PromptTemplate struct {
    SystemPrompt string
    UserPrompt   string
}

const DefaultSystemPrompt = `You are an expert at writing semantic git commit messages.

Format Requirements:
- Use Conventional Commits format: <type>(<scope>): <subject>
- Types: feat, fix, docs, style, refactor, test, chore, perf, ci, build, revert
- Subject: imperative mood, no period, max 72 characters
- Body: optional, explain what and why (not how)
- Footer: optional, reference issues or breaking changes

Rules:
1. Be concise and specific
2. Focus on the "what" and "why", not the "how"
3. Use present tense ("add" not "added")
4. First line should be standalone summary
5. Separate subject from body with blank line

Output only the commit message, no explanations.`

const DefaultUserPromptTemplate = `Generate a commit message for these changes:

{{if .DiffStats}}
Files changed: {{.DiffStats.TotalFiles}}
Additions: {{.DiffStats.TotalAdditions}}
Deletions: {{.DiffStats.TotalDeletions}}
{{end}}

{{if .RequiresChunking}}
Summary of changes:
{{range .Chunks}}
- {{.FilePath}}: {{.ChangeType}} (+{{.Additions}} -{{.Deletions}})
{{end}}
{{else}}
Diff:
{{range .Chunks}}
{{.Content}}
{{end}}
{{end}}

{{if .PreviousAttempt}}
Previous attempt (user requested regeneration):
{{.PreviousAttempt}}
{{end}}`
```

## Correctness Properties

*A property is a characteristic or behavior that should hold true across all valid executions of a system—essentially, a formal statement about what the system should do. Properties serve as the bridge between human-readable specifications and machine-verifiable correctness guarantees.*


### Property Reflection

After analyzing all acceptance criteria, I've identified several areas where properties can be consolidated:

**Redundancy Analysis:**
- Properties 2.2, 2.3, 2.4 (provider initialization examples) can be combined into a single property about provider instantiation
- Properties 6.2, 6.3, 6.4, 6.5 (user action handling) can be combined into a single property about action-response mapping
- Properties 7.1, 7.2, 7.5 (dry-run behavior) overlap and can be consolidated
- Properties 10.1, 10.2, 10.3, 10.4 (flag overrides) all test the same precedence mechanism

**Consolidated Properties:**
The following properties represent unique validation value after removing redundancy:

### Core Workflow Properties

Property 1: Staged changes retrieval
*For any* git repository with staged changes, retrieving the diff should return all staged file modifications
**Validates: Requirements 1.1**

Property 2: AI provider invocation
*For any* retrieved diff, the system should send the diff data to the configured AI provider
**Validates: Requirements 1.2**

Property 3: Message display
*For any* AI-generated commit message, the system should display it in the interactive interface
**Validates: Requirements 1.3**

Property 4: Commit execution on confirmation
*For any* confirmed commit message, the system should execute git commit with that exact message
**Validates: Requirements 1.4**

### Provider Management Properties

Property 5: Provider instantiation
*For any* valid provider configuration (openai, deepseek, ollama), the system should instantiate the corresponding provider client with the configured credentials
**Validates: Requirements 2.2, 2.3, 2.4**

### Configuration Properties

Property 6: Configuration persistence
*For any* key-value pair set via `config set`, the value should be retrievable via `config list` and persisted to the config file
**Validates: Requirements 3.2**

Property 7: Configuration precedence
*For any* configuration key with values at multiple levels (flag, env, file, default), the system should use the value from the highest priority source
**Validates: Requirements 3.4**

### Commit Message Format Properties

Property 8: Conventional Commits format validation
*For any* generated commit message, the subject line should match the pattern `<type>(<scope>): <subject>` or `<type>: <subject>`
**Validates: Requirements 4.1**

Property 9: Multi-line message support
*For any* commit message with body or footer sections, the system should preserve all sections with proper formatting
**Validates: Requirements 4.2**

Property 10: Prompt instruction inclusion
*For any* prompt sent to an AI provider, the prompt should contain instructions for Conventional Commits format
**Validates: Requirements 4.3**

Property 11: Subject length warning
*For any* generated commit message with subject exceeding 72 characters, the system should display a warning
**Validates: Requirements 4.4**

Property 12: Valid commit type recognition
*For any* commit message with type in {feat, fix, docs, style, refactor, test, chore, perf, ci, build, revert}, the system should accept it as valid
**Validates: Requirements 4.5**

### Diff Processing Properties

Property 13: Lock file exclusion
*For any* git diff containing lock files (package-lock.json, go.sum, yarn.lock, Cargo.lock, pnpm-lock.yaml), the processed diff sent to AI should not include these files
**Validates: Requirements 5.1**

Property 14: Diff chunking threshold
*For any* git diff with total size exceeding the configured threshold, the system should split it into per-file chunks
**Validates: Requirements 5.2**

Property 15: Chunk processing
*For any* chunked diff, the system should send each chunk to the AI provider separately
**Validates: Requirements 5.3**

### Interactive UI Properties

Property 16: User action handling
*For any* user action (accept, edit, regenerate, cancel), the system should execute the corresponding operation: accept→commit, edit→open editor, regenerate→new AI call, cancel→exit
**Validates: Requirements 6.2, 6.3, 6.4, 6.5**

### Dry-run Mode Properties

Property 17: Dry-run no-commit guarantee
*For any* execution in dry-run mode (via `generate` command or `--dry-run` flag), the system should not execute git commit and should not modify repository state
**Validates: Requirements 7.1, 7.5**

Property 18: Dry-run output
*For any* dry-run execution, the generated message should be written to stdout or the specified output file
**Validates: Requirements 7.2, 7.3**

Property 19: Dry-run exit status
*For any* successful dry-run execution, the system should exit with status code 0
**Validates: Requirements 7.4**

### History Properties

Property 20: History persistence
*For any* generated commit message, an entry with message, timestamp, and diff summary should be appended to the history file
**Validates: Requirements 8.1**

Property 21: History limit parameter
*For any* `history --limit n` command, the system should display exactly n entries (or fewer if total entries < n)
**Validates: Requirements 8.3**

Property 22: History rotation
*For any* history file exceeding 1000 entries, the system should automatically remove the oldest entries to maintain the limit
**Validates: Requirements 8.5**

### Logging Properties

Property 23: Verbose logging
*For any* execution with `--verbose` flag, the system should output detailed logs including API requests and responses
**Validates: Requirements 9.2**

Property 24: Git error handling
*For any* failed git command, the system should display the git error output and exit with a non-zero status code
**Validates: Requirements 9.4**

Property 25: Error stack traces
*For any* unexpected error in verbose mode, the system should log the full stack trace
**Validates: Requirements 9.5**

### Flag Override Properties

Property 26: Command-line flag precedence
*For any* configuration key with both a command-line flag and config file value, the system should use the flag value for that execution only
**Validates: Requirements 10.1, 10.2, 10.3, 10.4**

Property 27: Non-interactive mode
*For any* execution with `--yes` flag, the system should skip all interactive prompts and commit immediately
**Validates: Requirements 10.5**

## Error Handling

### Error Categories

**User Errors (Exit Code 1)**
- No staged changes
- Invalid configuration
- Missing API key
- Invalid command-line arguments

**System Errors (Exit Code 2)**
- Git command failures
- File system errors
- Configuration file corruption

**External Errors (Exit Code 3)**
- AI provider API failures
- Network connectivity issues
- API rate limiting

### Error Handling Strategy

```go
type ErrorHandler interface {
    Handle(err error) error
    Wrap(err error, context string) error
    IsRetryable(err error) bool
}

type AppError struct {
    Code    ErrorCode
    Message string
    Cause   error
    Context map[string]interface{}
}

type ErrorCode int

const (
    ErrNoStagedChanges ErrorCode = iota
    ErrGitCommandFailed
    ErrAIProviderFailed
    ErrConfigInvalid
    ErrFileSystemError
)
```

**Error Handling Rules:**
1. All errors should be wrapped with context using `fmt.Errorf` with `%w`
2. User-facing errors should be clear and actionable
3. Internal errors should be logged with full context in verbose mode
4. Retryable errors (network, rate limit) should be handled with exponential backoff
5. Non-retryable errors should fail fast with clear messages

### Retry Strategy

```go
type RetryConfig struct {
    MaxAttempts  int
    InitialDelay time.Duration
    MaxDelay     time.Duration
    Multiplier   float64
}

// Default: 3 attempts, 1s initial, 10s max, 2x multiplier

type RetryableError interface {
    error
    IsRetryable() bool
    RetryAfter() time.Duration  // For rate limit errors
}
```

**Retry Rules:**
1. Network errors: Retry with exponential backoff
2. Rate limit errors (429): Retry after the duration specified in response headers
3. Server errors (5xx): Retry with exponential backoff
4. Client errors (4xx except 429): Do not retry
5. Timeout errors: Retry once with increased timeout

**Circuit Breaker:**
- After 5 consecutive failures, enter "open" state for 60 seconds
- In "open" state, fail fast without attempting requests
- After 60 seconds, enter "half-open" state and allow one test request
- If test succeeds, return to "closed" state; if fails, return to "open"

## Testing Strategy

### Unit Testing

**Framework:** Go standard `testing` package with `testify` for assertions

**Coverage Targets:**
- Git client: Test diff parsing, command execution, error handling
- AI providers: Test request formatting, response parsing, error handling (with mocked HTTP)
- Config manager: Test loading, saving, precedence rules
- Diff processor: Test filtering, chunking, size calculations
- Commit message parser: Test format validation, multi-line parsing

**Key Unit Tests:**
- Lock file filtering with various file patterns
- Configuration precedence with different value sources
- Commit message format validation with valid and invalid inputs
- Diff chunking with various size thresholds
- Error wrapping and unwrapping

### Property-Based Testing

**Framework:** `github.com/leanovate/gopter` (Go property testing library)

**Configuration:** Each property test should run a minimum of 100 iterations

**Test Tagging:** Each property-based test must include a comment with the format:
```go
// Feature: gitsage, Property X: <property description>
// Validates: Requirements X.Y
```

**Property Test Coverage:**

1. **Diff Retrieval Property Test**
   - Generate random git repositories with random staged changes
   - Verify all staged changes are retrieved
   - **Feature: gitsage, Property 1: Staged changes retrieval**
   - **Validates: Requirements 1.1**

2. **Configuration Precedence Property Test**
   - Generate random configuration keys and values at different levels
   - Verify correct precedence order is followed
   - **Feature: gitsage, Property 7: Configuration precedence**
   - **Validates: Requirements 3.4**

3. **Commit Format Validation Property Test**
   - Generate random commit messages with various formats
   - Verify Conventional Commits format validation
   - **Feature: gitsage, Property 8: Conventional Commits format validation**
   - **Validates: Requirements 4.1**

4. **Lock File Filtering Property Test**
   - Generate random diffs with and without lock files
   - Verify lock files are always excluded
   - **Feature: gitsage, Property 13: Lock file exclusion**
   - **Validates: Requirements 5.1**

5. **Diff Chunking Property Test**
   - Generate random diffs of various sizes
   - Verify chunking occurs when threshold is exceeded
   - **Feature: gitsage, Property 14: Diff chunking threshold**
   - **Validates: Requirements 5.2**

6. **Dry-run No-Commit Property Test**
   - Generate random diffs and run in dry-run mode
   - Verify git repository state is never modified
   - **Feature: gitsage, Property 17: Dry-run no-commit guarantee**
   - **Validates: Requirements 7.1, 7.5**

7. **History Rotation Property Test**
   - Generate more than 1000 history entries
   - Verify oldest entries are removed
   - **Feature: gitsage, Property 22: History rotation**
   - **Validates: Requirements 8.5**

8. **Flag Precedence Property Test**
   - Generate random configurations with conflicting values
   - Verify command-line flags always take precedence
   - **Feature: gitsage, Property 26: Command-line flag precedence**
   - **Validates: Requirements 10.1, 10.2, 10.3, 10.4**

### Integration Testing

**Scope:** Test interactions between components with real dependencies

**Key Integration Tests:**
- End-to-end workflow: staged changes → AI generation → commit
- Configuration loading from file → provider initialization → API call
- Diff processing → chunking → multiple AI calls → aggregation
- Interactive UI flow with simulated user input
- History persistence across multiple executions

**Test Environment:**
- Use temporary git repositories for each test
- Mock AI provider responses to avoid external dependencies
- Use temporary config files to avoid affecting user configuration

### Manual Testing Checklist

**Pre-release validation:**
- [ ] Test with real OpenAI API
- [ ] Test with DeepSeek API
- [ ] Test with local Ollama instance
- [ ] Test on Windows, macOS, Linux
- [ ] Test with various git repository sizes
- [ ] Test with non-ASCII characters in diffs
- [ ] Test with very large diffs (>100KB)
- [ ] Test configuration migration from old versions
- [ ] Test error messages are user-friendly
- [ ] Test spinner and UI rendering in different terminals

## Performance Considerations

### Optimization Targets

**Diff Processing:**
- Parse diffs incrementally to avoid loading entire diff into memory
- Use streaming for large file diffs
- Cache diff statistics to avoid re-parsing

**AI Provider Calls:**
- Implement request timeout (default 30s)
- Use connection pooling for HTTP clients
- Implement response streaming for large responses

**Configuration Loading:**
- Load config file once at startup
- Cache parsed configuration in memory
- Use lazy loading for optional components

### Resource Limits

```go
const (
    MaxDiffSize        = 1 * 1024 * 1024  // 1MB
    MaxChunkSize       = 100 * 1024       // 100KB
    MaxHistoryEntries  = 1000
    MaxHistoryFileSize = 10 * 1024 * 1024 // 10MB
    APITimeout         = 30 * time.Second
    MaxRetries         = 3
    MaxConcurrentAICalls = 3              // Parallel chunk processing
    ConfigLoadTimeout  = 100 * time.Millisecond
    GitCommandTimeout  = 10 * time.Second
)
```

### Caching Strategy

```go
type CacheManager interface {
    Get(key string) (*ai.GenerateResponse, bool)
    Set(key string, response *ai.GenerateResponse, ttl time.Duration)
    Clear()
}

// Cache key: SHA256(diff content + provider + model + prompt)
// TTL: 1 hour (configurable)
// Storage: In-memory LRU cache with max 100 entries
```

**Caching Rules:**
1. Cache AI responses by diff content hash
2. Invalidate cache when provider/model/prompt changes
3. Use LRU eviction when cache is full
4. Provide `--no-cache` flag to bypass cache
5. Clear cache on `gitsage config set` commands that affect AI behavior

## Security Considerations

### API Key Management

**Storage:**
- Config file should have permissions 0600 (user read/write only)
- Never log API keys, even in verbose mode
- Mask API keys in error messages (show only last 4 characters)

**Validation:**
- Validate API key format before making requests
- Fail fast with clear error if API key is invalid
- Support environment variable override for CI/CD

### Diff Content

**Privacy:**
- Warn users that diffs are sent to external AI providers
- Provide option to review diff before sending
- Support local-only mode with Ollama

**Sanitization:**
- Remove sensitive patterns (API keys, passwords) from diffs before sending
- Provide configurable regex patterns for sanitization
- Log sanitization actions in verbose mode

## Deployment and Distribution

### Build Process

**Build Targets:**
- Linux: amd64, arm64
- macOS: amd64 (Intel), arm64 (Apple Silicon)
- Windows: amd64

**Build Command:**
```bash
CGO_ENABLED=0 go build -ldflags="-s -w" -o bin/gitsage ./cmd/gitsage
```

### Installation Methods

**Homebrew (macOS/Linux):**
```bash
brew install gitsage
```

**Go Install:**
```bash
go install github.com/yourusername/gitsage@latest
```

**Binary Download:**
- GitHub Releases with pre-built binaries
- Checksums for verification

### Release Process

**Using GoReleaser:**
```yaml
# .goreleaser.yaml
project_name: gitsage

before:
  hooks:
    - go mod tidy
    - go test ./...

builds:
  - env:
      - CGO_ENABLED=0
    goos:
      - linux
      - windows
      - darwin
    goarch:
      - amd64
      - arm64
    ldflags:
      - -s -w
      - -X main.version={{.Version}}
      - -X main.commit={{.Commit}}
      - -X main.date={{.Date}}

archives:
  - format: tar.gz
    name_template: >-
      {{ .ProjectName }}_
      {{- .Version }}_
      {{- .Os }}_
      {{- .Arch }}
    format_overrides:
      - goos: windows
        format: zip

checksum:
  name_template: 'checksums.txt'

changelog:
  sort: asc
  filters:
    exclude:
      - '^docs:'
      - '^test:'
```

## Future Enhancements

### Phase 2 Features

1. **Git Hook Integration**
   - Generate `prepare-commit-msg` hook
   - Auto-trigger on `git commit`
   - Support hook configuration

2. **Custom Prompt Templates**
   - User-defined prompt templates
   - Template variables and functions
   - Team-specific commit conventions

3. **Commit Message Templates**
   - Pre-defined templates for common changes
   - Template selection in interactive UI
   - Custom template creation

4. **Advanced Diff Analysis**
   - Semantic diff analysis (function/class level)
   - Language-specific parsing
   - Change impact assessment

### Phase 3 Features

1. **Team Collaboration**
   - Shared configuration profiles
   - Team-wide prompt templates
   - Commit message style enforcement

2. **Analytics and Insights**
   - Commit message quality metrics
   - Usage statistics
   - Cost tracking for API calls

3. **IDE Integration**
   - VS Code extension
   - JetBrains plugin
   - Git GUI integration

## Appendix

### Dependencies

**Core:**
- `github.com/spf13/cobra` - CLI framework
- `github.com/spf13/viper` - Configuration management
- `github.com/sashabaranov/go-openai` - OpenAI client

**UI:**
- `github.com/charmbracelet/huh` - Interactive forms
- `github.com/charmbracelet/lipgloss` - Terminal styling
- `github.com/briandowns/spinner` - Loading animations

**Testing:**
- `github.com/stretchr/testify` - Test assertions
- `github.com/leanovate/gopter` - Property-based testing

**Utilities:**
- `gopkg.in/yaml.v3` - YAML parsing
- `github.com/google/uuid` - UUID generation

### References

- [Conventional Commits](https://www.conventionalcommits.org/)
- [Go Project Layout](https://github.com/golang-standards/project-layout)
- [Clean Architecture](https://blog.cleancoder.com/uncle-bob/2012/08/13/the-clean-architecture.html)
- [OpenAI API Documentation](https://platform.openai.com/docs/api-reference)
- [Ollama API Documentation](https://github.com/ollama/ollama/blob/main/docs/api.md)
