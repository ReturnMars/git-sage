# Implementation Plan

- [x] 1. Set up project structure and dependencies





  - Initialize Go module with `go mod init`
  - Create directory structure following golang-standards layout
  - Add core dependencies: cobra, viper, go-openai, charmbracelet libraries
  - Create Makefile with build, test, and install targets
  - Set up .gitignore for Go projects
  - _Requirements: All_

- [x] 2. Implement configuration management





  - Create config package with Config struct and Manager interface
  - Implement YAML config file loading with Viper
  - Implement config precedence: flags > env > file > defaults
  - Create default configuration template
  - _Requirements: 3.1, 3.2, 3.3, 3.4, 3.5_

- [x] 2.1 Implement config init command


  - Create `config init` command that generates ~/.gitsage.yaml
  - Populate with default values from design document
  - Set file permissions to 0600 for security
  - _Requirements: 3.1_


- [x] 2.2 Implement config set command

  - Create `config set <key> <value>` command
  - Parse nested keys (e.g., "provider.name")
  - Update YAML file preserving structure and comments
  - _Requirements: 3.2_

- [x] 2.3 Write property test for configuration precedence






  - **Property 7: Configuration precedence**
  - **Validates: Requirements 3.4**



- [x] 2.4 Implement config list command





  - Create `config list` command
  - Display all configuration values in readable format
  - Mask sensitive values (API keys)
  - _Requirements: 3.3_

- [x] 3. Implement Git client





  - Create git package with Client interface
  - Implement GetStagedDiff using exec.CommandContext
  - Parse git diff output into DiffChunk structs
  - Implement GetDiffStats for file statistics
  - Implement HasStagedChanges check
  - Implement Commit method
  - _Requirements: 1.1, 1.4, 1.5_

- [x] 3.1 Implement diff parsing logic


  - Parse `git diff --cached` output
  - Extract file paths, change types, additions, deletions
  - Handle binary files and renames
  - Detect lock files by filename patterns
  - _Requirements: 1.1, 5.1_

- [x] 3.2 Write property test for staged changes retrieval







  - **Property 1: Staged changes retrieval**
  - **Validates: Requirements 1.1**

- [ ] 3.3 Write property test for lock file exclusion




  - **Property 13: Lock file exclusion**
  - **Validates: Requirements 5.1**

- [x] 4. Implement diff processor





  - Create processor package with DiffProcessor interface
  - Implement lock file filtering logic
  - Implement diff size calculation
  - Implement chunking strategy (by file)
  - Generate diff summaries for large changes
  - _Requirements: 5.1, 5.2, 5.3, 5.5_

- [ ]* 4.1 Write property test for diff chunking
  - **Property 14: Diff chunking threshold**
  - **Validates: Requirements 5.2**

- [x] 5. Implement AI provider interface and OpenAI provider





  - Create ai package with Provider interface
  - Define GenerateRequest and GenerateResponse structs
  - Implement OpenAIProvider using go-openai library
  - Implement prompt template system
  - Handle API errors and retries
  - _Requirements: 1.2, 2.1, 2.2, 4.3_

- [x] 5.1 Implement prompt template


  - Create default system prompt for Conventional Commits
  - Create user prompt template with diff/summary placeholders
  - Implement template rendering with diff data
  - Support custom prompts via configuration
  - _Requirements: 4.1, 4.3_

- [ ]* 5.2 Write property test for prompt instruction inclusion
  - **Property 10: Prompt instruction inclusion**
  - **Validates: Requirements 4.3**


- [x] 5.3 Implement response parsing

  - Parse AI response into subject, body, footer
  - Validate Conventional Commits format
  - Extract commit type and scope
  - Handle malformed responses gracefully
  - _Requirements: 4.1, 4.2_

- [ ]* 5.4 Write property test for commit format validation
  - **Property 8: Conventional Commits format validation**
  - **Validates: Requirements 4.1**

- [x] 6. Implement DeepSeek provider





  - Create DeepSeekProvider struct
  - Use OpenAI-compatible client with DeepSeek endpoint
  - Configure default model and endpoint
  - Handle DeepSeek-specific error responses
  - _Requirements: 2.3_

- [x] 7. Implement Ollama provider









  - Create OllamaProvider struct
  - Implement HTTP client for Ollama API
  - Configure local endpoint (default: localhost:11434)
  - Handle Ollama-specific request/response format
  - _Requirements: 2.4_

- [ ]* 7.1 Write property test for provider instantiation
  - **Property 5: Provider instantiation**
  - **Validates: Requirements 2.2, 2.3, 2.4**

- [x] 8. Implement commit message validation





  - Create message package with CommitMessage struct
  - Implement Format() method for multi-line messages
  - Implement Validate() method for format checking
  - Check subject length and warn if > 72 chars
  - Validate commit type against allowed list
  - _Requirements: 4.1, 4.2, 4.4, 4.5_

- [ ]* 8.1 Write property test for multi-line message support
  - **Property 9: Multi-line message support**
  - **Validates: Requirements 4.2**

- [ ]* 8.2 Write property test for subject length warning
  - **Property 11: Subject length warning**
  - **Validates: Requirements 4.4**

- [ ]* 8.3 Write property test for valid commit type recognition
  - **Property 12: Valid commit type recognition**
  - **Validates: Requirements 4.5**

- [x] 9. Implement interactive UI manager





  - Create ui package with Manager interface
  - Implement message display using charmbracelet/huh
  - Implement action prompt (accept/edit/regenerate/cancel)
  - Implement message editor integration
  - Implement spinner for loading states
  - Implement success/error message display
  - _Requirements: 1.3, 6.1, 6.2, 6.3, 6.4, 6.5_

- [x] 9.1 Implement action handling


  - Map user selections to Action enum
  - Handle accept action → return message
  - Handle edit action → open editor → return edited message
  - Handle regenerate action → return regenerate signal
  - Handle cancel action → return cancel signal
  - _Requirements: 6.2, 6.3, 6.4, 6.5_

- [ ]* 9.2 Write property test for user action handling
  - **Property 16: User action handling**
  - **Validates: Requirements 6.2, 6.3, 6.4, 6.5**

- [x] 10. Implement history manager





  - Create history package with Manager interface
  - Define Entry struct with all required fields
  - Implement Save method to append to JSON file
  - Implement List method with limit parameter
  - Implement Clear method
  - Implement automatic rotation at 1000 entries
  - _Requirements: 8.1, 8.2, 8.3, 8.4, 8.5_

- [ ]* 10.1 Write property test for history persistence
  - **Property 20: History persistence**
  - **Validates: Requirements 8.1**

- [ ]* 10.2 Write property test for history rotation
  - **Property 22: History rotation**
  - **Validates: Requirements 8.5**

- [x] 11. Implement core application service





  - Create app package with CommitService struct
  - Inject all dependencies (git, ai, processor, ui, history, config)
  - Implement GenerateAndCommit orchestration method
  - Handle workflow: check staged → get diff → process → generate → display → handle action → commit/save
  - Implement error handling with proper wrapping
  - _Requirements: 1.1, 1.2, 1.3, 1.4, 1.5_

- [x] 11.1 Implement regeneration loop


  - Track previous attempt for context
  - Pass previous message to AI provider
  - Limit regeneration attempts (max 5)
  - _Requirements: 6.3_



- [x] 11.2 Implement chunked diff handling with concurrency





  - Detect when chunking is needed
  - Group chunks for parallel processing (max 3 concurrent)
  - Send chunk groups to AI provider with goroutines
  - Use sync.WaitGroup or errgroup for coordination
  - Aggregate chunk responses into single message
  - Handle partial failures gracefully
  - _Requirements: 5.2, 5.3, 11.1_

- [ ]* 11.3 Write property test for chunk processing
  - **Property 15: Chunk processing**
  - **Validates: Requirements 5.3**

- [x] 12. Implement CLI commands with Cobra





  - Create cmd/gitsage/main.go entry point
  - Set up root command with version info
  - Configure global flags: --verbose, --config, --provider, --model
  - _Requirements: All_

- [x] 12.1 Implement commit command


  - Create `commit` command as default action
  - Wire up CommitService.GenerateAndCommit
  - Handle --dry-run flag
  - Handle --yes flag for non-interactive mode
  - Handle --output flag for file output
  - _Requirements: 1.1, 1.2, 1.3, 1.4, 7.1, 7.2, 7.3, 10.5_

- [ ]* 12.2 Write property test for dry-run no-commit guarantee
  - **Property 17: Dry-run no-commit guarantee**
  - **Validates: Requirements 7.1, 7.5**

- [ ]* 12.3 Write property test for dry-run output
  - **Property 18: Dry-run output**
  - **Validates: Requirements 7.2, 7.3**

- [ ]* 12.4 Write property test for dry-run exit status
  - **Property 19: Dry-run exit status**
  - **Validates: Requirements 7.4**



- [x] 12.5 Implement generate command





  - Create `generate` command as alias for `commit --dry-run`
  - Output message to stdout by default
  - Support --output flag
  - _Requirements: 7.1, 7.2, 7.3_

- [x] 13. Implement history command





  - Create `history` command
  - Display recent entries with formatting
  - Support --limit flag
  - Implement `history clear` subcommand
  - _Requirements: 8.2, 8.3, 8.4_

- [ ]* 13.1 Write property test for history limit parameter
  - **Property 21: History limit parameter**
  - **Validates: Requirements 8.3**

- [x] 14. Implement error handling and logging





  - Create error types with codes and RetryableError interface
  - Implement error wrapping with context using fmt.Errorf
  - Implement verbose logging with --verbose flag
  - Add user-friendly error messages with suggestions
  - Implement retry logic with exponential backoff
  - Implement circuit breaker pattern for API calls
  - Handle rate limit errors with Retry-After header
  - _Requirements: 9.1, 9.2, 9.3, 9.4, 9.5, 11.1_

- [ ]* 14.1 Write property test for verbose logging
  - **Property 23: Verbose logging**
  - **Validates: Requirements 9.2**

- [ ]* 14.2 Write property test for git error handling
  - **Property 24: Git error handling**
  - **Validates: Requirements 9.4**

- [ ]* 14.3 Write property test for error stack traces
  - **Property 25: Error stack traces**
  - **Validates: Requirements 9.5**

- [x] 15. Implement flag override system





  - Implement flag precedence in config loading
  - Support --provider flag override
  - Support --model flag override
  - Support --config flag for custom config path
  - Ensure flags don't persist to config file
  - _Requirements: 10.1, 10.2, 10.3, 10.4_

- [ ]* 15.1 Write property test for command-line flag precedence
  - **Property 26: Command-line flag precedence**
  - **Validates: Requirements 10.1, 10.2, 10.3, 10.4**

- [ ]* 15.2 Write property test for non-interactive mode
  - **Property 27: Non-interactive mode**
  - **Validates: Requirements 10.5**

- [x] 16. Implement security features









  - Set config file permissions to 0600 on creation
  - Mask API keys in logs (show only last 4 chars)
  - Mask API keys in error messages
  - Validate API key format before making requests
  - Add first-use warning about sending diffs to external services
  - Store warning acknowledgment in config to show only once
  - _Requirements: 3.1, 12.1, 12.2, 12.3, 12.4, 12.5_

- [x] 16.1 Implement response caching


  - Create cache package with CacheManager interface
  - Implement in-memory LRU cache with max 100 entries
  - Generate cache keys using SHA256(diff + provider + model + prompt)
  - Set TTL to 1 hour (configurable)
  - Add --no-cache flag to bypass cache
  - Clear cache on relevant config changes
  - _Requirements: 11.1_

- [x] 16.2 Implement performance optimizations


  - Use streaming parser for git diff to avoid loading into memory
  - Implement connection pooling for HTTP clients
  - Add timeout for config loading (100ms)
  - Add timeout for git commands (10s)
  - Profile memory usage and optimize hot paths
  - _Requirements: 11.1, 11.2, 11.3, 11.4, 11.5_

- [x] 17. Add build and release configuration





  - Create .goreleaser.yaml configuration
  - Configure multi-platform builds (Linux, macOS, Windows)
  - Set up version injection via ldflags
  - Configure archive generation
  - Add checksum generation
  - _Requirements: All_

- [x] 18. Create documentation





  - Write README.md with installation and usage instructions
  - Document all commands and flags
  - Add configuration examples
  - Create troubleshooting guide
  - Add contributing guidelines
  - _Requirements: All_

- [x] 19. Checkpoint - Ensure all tests pass





  - Ensure all tests pass, ask the user if questions arise.

- [x] 20. Manual testing and validation





  - Test with real OpenAI API
  - Test with DeepSeek API
  - Test with local Ollama instance
  - Test on Windows, macOS, Linux
  - Test with various repository sizes
  - Test error scenarios
  - Validate user experience
  - _Requirements: All_
