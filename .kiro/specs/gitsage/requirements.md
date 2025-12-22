# Requirements Document

## Introduction

GitSage is an AI-powered command-line tool that automatically generates semantic Git commit messages based on staged changes. The tool analyzes `git diff` output, sends it to configurable AI providers (OpenAI, DeepSeek, Ollama), and presents users with an interactive interface to review, edit, and confirm commit messages before execution. GitSage follows Conventional Commits format and supports multi-line messages (subject + body + footer).

## Glossary

- **GitSage**: The CLI tool being developed
- **AI Provider**: External service or local model that generates commit messages (e.g., OpenAI, DeepSeek, Ollama)
- **Staged Changes**: Files added to Git's staging area via `git add`
- **Conventional Commits**: Industry-standard commit message format: `<type>(<scope>): <subject>`
- **Diff Chunk**: A segment of git diff output, typically representing changes to a single file or logical group
- **Dry-run Mode**: Operation mode where commit messages are generated but not committed
- **Config File**: YAML configuration file stored at `~/.gitsage/config.yaml`
- **Interactive UI**: Terminal-based user interface for reviewing and editing generated messages

## Requirements

### Requirement 1

**User Story:** As a developer, I want to generate commit messages from my staged changes, so that I can quickly create meaningful commits without manually writing messages.

#### Acceptance Criteria

1. WHEN a user executes `gitsage` or `gitsage commit` with staged changes, THEN the System SHALL retrieve the git diff from the staging area
2. WHEN the git diff is retrieved, THEN the System SHALL send the diff to the configured AI Provider
3. WHEN the AI Provider returns a commit message, THEN the System SHALL display the message in an interactive interface
4. WHEN the user confirms the message, THEN the System SHALL execute `git commit` with the generated message
5. IF no staged changes exist, THEN the System SHALL display an error message and exit without calling the AI Provider

### Requirement 2

**User Story:** As a developer, I want to use different AI providers (OpenAI, DeepSeek, Ollama), so that I can choose based on cost, privacy, and availability.

#### Acceptance Criteria

1. WHEN the System initializes an AI Provider, THEN the System SHALL use a common interface that abstracts provider-specific implementations
2. WHEN the configuration specifies `provider: openai`, THEN the System SHALL instantiate the OpenAI client with the configured API key and model
3. WHEN the configuration specifies `provider: deepseek`, THEN the System SHALL instantiate the DeepSeek client with the configured API key and model
4. WHEN the configuration specifies `provider: ollama`, THEN the System SHALL instantiate the Ollama client with the configured local endpoint
5. WHEN adding a new AI Provider, THEN the System SHALL require only implementing the AIProvider interface without modifying core logic

### Requirement 3

**User Story:** As a developer, I want to manage configuration through a config file and CLI commands, so that I can persist my preferences and easily update settings.

#### Acceptance Criteria

1. WHEN a user executes `gitsage config init`, THEN the System SHALL create a configuration file at `~/.gitsage/config.yaml` with default values
2. WHEN a user executes `gitsage config set <key> <value>`, THEN the System SHALL update the specified key in the configuration file
3. WHEN a user executes `gitsage config list`, THEN the System SHALL display all current configuration values
4. WHEN the System loads configuration, THEN the System SHALL prioritize values in this order: command-line flags, environment variables, config file, defaults
5. WHEN the configuration file does not exist and the user runs `gitsage`, THEN the System SHALL prompt the user to run `gitsage config init`

### Requirement 4

**User Story:** As a developer, I want commit messages in Conventional Commits format with multi-line support, so that my commit history follows industry standards.

#### Acceptance Criteria

1. WHEN the AI Provider generates a commit message, THEN the System SHALL ensure the message follows the format `<type>(<scope>): <subject>`
2. WHEN the commit message includes additional details, THEN the System SHALL support multi-line format with subject, body, and footer sections
3. WHEN the System sends a prompt to the AI Provider, THEN the System SHALL include instructions to generate Conventional Commits format
4. WHEN the generated message exceeds 72 characters in the subject line, THEN the System SHALL display a warning to the user
5. THE System SHALL support commit types: feat, fix, docs, style, refactor, test, chore, perf, ci, build, revert

### Requirement 5

**User Story:** As a developer, I want intelligent diff processing that handles large changes, so that I don't exceed AI token limits or waste tokens on irrelevant files.

#### Acceptance Criteria

1. WHEN the git diff includes lock files (package-lock.json, go.sum, yarn.lock, Cargo.lock, pnpm-lock.yaml), THEN the System SHALL exclude these files from the diff sent to the AI Provider
2. WHEN the total diff size exceeds a configurable threshold (default 10KB), THEN the System SHALL split the diff into chunks by file
3. WHEN processing diff chunks, THEN the System SHALL send each chunk to the AI Provider separately
4. WHEN multiple chunks are processed, THEN the System SHALL combine the individual commit suggestions into a single commit message
5. WHEN a single file diff exceeds the maximum chunk size (default 100KB), THEN the System SHALL send only the file path and change statistics instead of the full diff

### Requirement 6

**User Story:** As a developer, I want an interactive interface to review, edit, and regenerate commit messages, so that I have full control over the final message.

#### Acceptance Criteria

1. WHEN the System displays a generated commit message, THEN the System SHALL provide options to: accept, edit, regenerate, or cancel
2. WHEN the user selects "edit", THEN the System SHALL open a text editor allowing modification of the message
3. WHEN the user selects "regenerate", THEN the System SHALL call the AI Provider again with the same diff
4. WHEN the user selects "cancel", THEN the System SHALL exit without committing
5. WHEN the user selects "accept", THEN the System SHALL proceed with the git commit operation

### Requirement 7

**User Story:** As a developer, I want a dry-run mode that generates messages without committing, so that I can preview AI-generated messages or use them elsewhere.

#### Acceptance Criteria

1. WHEN a user executes `gitsage generate` or `gitsage --dry-run`, THEN the System SHALL generate a commit message without executing git commit
2. WHEN dry-run mode is active, THEN the System SHALL display the generated message to stdout
3. WHEN a user specifies `--output <file>` in dry-run mode, THEN the System SHALL write the generated message to the specified file
4. WHEN dry-run mode completes, THEN the System SHALL exit with status code 0
5. WHEN dry-run mode is active, THEN the System SHALL not modify the git repository state

### Requirement 8

**User Story:** As a developer, I want to save a history of generated commit messages, so that I can reference previous AI suggestions.

#### Acceptance Criteria

1. WHEN the System generates a commit message, THEN the System SHALL append the message, timestamp, and diff summary to a history file at `~/.gitsage/history.json`
2. WHEN a user executes `gitsage history`, THEN the System SHALL display the most recent 20 entries from the history file
3. WHEN a user executes `gitsage history --limit <n>`, THEN the System SHALL display the most recent n entries
4. WHEN a user executes `gitsage history clear`, THEN the System SHALL delete all entries from the history file
5. WHEN the history file exceeds 1000 entries, THEN the System SHALL automatically remove the oldest entries to maintain the limit

### Requirement 9

**User Story:** As a developer, I want clear error messages and logging, so that I can troubleshoot issues quickly.

#### Acceptance Criteria

1. WHEN an error occurs, THEN the System SHALL display a user-friendly error message describing the issue and suggested resolution
2. WHEN a user specifies `--verbose` flag, THEN the System SHALL output detailed logs including API requests, responses, and internal operations
3. WHEN the AI Provider API call fails, THEN the System SHALL display the error message and suggest checking API key and network connectivity
4. WHEN git commands fail, THEN the System SHALL display the git error output and exit with a non-zero status code
5. WHEN the System encounters an unexpected error, THEN the System SHALL log the full stack trace in verbose mode

### Requirement 10

**User Story:** As a developer, I want to temporarily override configuration via command-line flags, so that I can experiment with different settings without modifying my config file.

#### Acceptance Criteria

1. WHEN a user specifies `--provider <name>`, THEN the System SHALL use the specified provider for the current execution only
2. WHEN a user specifies `--model <name>`, THEN the System SHALL use the specified model for the current execution only
3. WHEN a user specifies `--config <path>`, THEN the System SHALL load configuration from the specified file instead of the default location
4. WHEN command-line flags conflict with config file values, THEN the System SHALL prioritize command-line flags
5. WHEN a user specifies `--yes` flag, THEN the System SHALL skip interactive confirmation and commit immediately with the generated message


### Requirement 11

**User Story:** As a developer, I want the tool to perform efficiently, so that it doesn't slow down my development workflow.

#### Acceptance Criteria

1. WHEN the System processes a diff under 10KB, THEN the System SHALL complete the entire workflow within 5 seconds (excluding AI API latency)
2. WHEN the System makes an AI Provider API call, THEN the System SHALL timeout after 30 seconds and display an error
3. WHEN the System loads configuration, THEN the System SHALL complete loading within 100 milliseconds
4. WHEN the System parses git diff output, THEN the System SHALL use streaming to avoid loading the entire diff into memory
5. THE System SHALL limit maximum diff size to 1MB to prevent excessive memory usage

### Requirement 12

**User Story:** As a developer, I want my API keys and code diffs to be handled securely, so that sensitive information is not exposed.

#### Acceptance Criteria

1. WHEN the System creates a configuration file, THEN the System SHALL set file permissions to 0600 (user read/write only)
2. WHEN the System logs information, THEN the System SHALL never log API keys in full (mask all but last 4 characters)
3. WHEN the System displays error messages, THEN the System SHALL mask API keys showing only the last 4 characters
4. WHEN the System sends diffs to external AI providers, THEN the System SHALL display a warning on first use
5. WHEN the System validates API keys, THEN the System SHALL check format before making API calls to fail fast
