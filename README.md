# GitSage

AI-powered Git commit message generator that analyzes your staged changes and creates semantic commit messages following the Conventional Commits format.

English | [简体中文](README_CN.md)

## Features

- **AI-Powered Messages**: Generates meaningful commit messages based on your actual code changes
- **Multiple AI Providers**: Supports OpenAI, DeepSeek, and local Ollama models
- **Conventional Commits**: Follows the industry-standard commit message format
- **Interactive Review**: Review, edit, or regenerate messages before committing
- **Smart Diff Processing**: Handles large diffs by chunking and excludes lock files
- **History Tracking**: Keeps a history of generated messages for reference
- **Dry-Run Mode**: Preview messages without committing
- **Response Caching**: Caches AI responses to avoid redundant API calls
- **Automatic PATH Setup**: First-run detection and automatic PATH configuration for global access

## Installation

### Using Go Install

```bash
go install github.com/gitsage/gitsage@latest
```

### From Source

```bash
git clone https://github.com/gitsage/gitsage.git
cd gitsage
make build
```

The binary will be available at `bin/gitsage`.

### Pre-built Binaries

Download pre-built binaries from the [Releases](https://github.com/gitsage/gitsage/releases) page.

Available platforms:
- Linux (amd64, arm64)
- macOS (amd64, arm64)
- Windows (amd64)

## Quick Start

1. **Initialize configuration**:
   ```bash
   gitsage config init
   ```

2. **Set your API key**:
   ```bash
   gitsage config set provider.api_key sk-your-openai-key
   ```

3. **Stage your changes and generate a commit**:
   ```bash
   git add .
   gitsage
   ```

### First Run PATH Detection

On first run, GitSage will check if it's accessible from your system PATH. If not found, it will offer to automatically add itself:

- **Windows**: Adds to user PATH via `setx` command
- **macOS/Linux**: Appends export statement to your shell profile (`.bashrc`, `.zshrc`, `.config/fish/config.fish`)

You can skip this check with the `--skip-path-check` flag or reset it later with:
```bash
gitsage config set security.path_check_done false
```

## Usage

### Basic Commands

```bash
# Generate and commit (default action)
gitsage

# Same as above, explicit command
gitsage commit

# Generate without committing (dry-run)
gitsage generate

# Auto-accept generated message
gitsage --yes

# Save message to file
gitsage generate -o commit-msg.txt
```

### Configuration Commands

```bash
# Initialize config file
gitsage config init

# Set a configuration value
gitsage config set <key> <value>

# List all configuration values
gitsage config list
```

### History Commands

```bash
# View recent history (default: 20 entries)
gitsage history

# View specific number of entries
gitsage history --limit 5

# Clear all history
gitsage history clear
```

## Commands Reference

### `gitsage` / `gitsage commit`

Generate a commit message and optionally commit.

| Flag | Short | Description |
|------|-------|-------------|
| `--dry-run` | | Generate message without committing |
| `--yes` | `-y` | Skip interactive confirmation |
| `--output` | `-o` | Write message to file (implies --dry-run) |
| `--no-cache` | | Bypass response cache |

### `gitsage generate`

Generate a commit message without committing (alias for `commit --dry-run`).

| Flag | Short | Description |
|------|-------|-------------|
| `--yes` | `-y` | Skip interactive confirmation |
| `--output` | `-o` | Write message to file |

### `gitsage config`

Manage configuration settings.

#### `gitsage config init`

Create a new configuration file at `~/.gitsage/config.yaml` with default values.

#### `gitsage config set <key> <value>`

Set a configuration value. Supports nested keys with dot notation.

Examples:
```bash
gitsage config set provider.name openai
gitsage config set provider.api_key sk-xxx
gitsage config set provider.model gpt-4o-mini
gitsage config set git.diff_size_threshold 20480
```

#### `gitsage config list`

Display all current configuration values (API keys are masked).

### `gitsage history`

View commit message history.

| Flag | Short | Description |
|------|-------|-------------|
| `--limit` | `-l` | Number of entries to display (default: 20) |

#### `gitsage history clear`

Delete all history entries.

### Global Flags

These flags work with all commands:

| Flag | Short | Description |
|------|-------|-------------|
| `--verbose` | `-v` | Enable verbose logging |
| `--config` | | Custom config file path |
| `--provider` | | Override AI provider for this execution |
| `--model` | | Override AI model for this execution |
| `--skip-path-check` | | Skip PATH detection check |
| `--version` | | Show version information |
| `--help` | `-h` | Show help |

## Configuration

Configuration is stored in `~/.gitsage/config.yaml`. Create it with `gitsage config init`.

### Configuration File Structure

```yaml
provider:
  name: openai          # AI provider: openai, deepseek, ollama
  api_key: ""           # API key (not needed for ollama)
  model: gpt-4o-mini    # Model to use
  endpoint: ""          # Custom endpoint (optional)
  temperature: 0.2      # Response creativity (0.0-1.0)
  max_tokens: 500       # Maximum response tokens

git:
  diff_size_threshold: 10240  # Chunk diffs larger than this (bytes)
  exclude_patterns:           # Files to exclude from diff
    - "*.lock"
    - "go.sum"
    - "package-lock.json"
    - "yarn.lock"
    - "pnpm-lock.yaml"
    - "Cargo.lock"

ui:
  editor: ""            # Editor for message editing (uses $EDITOR)
  color_enabled: true   # Enable colored output
  spinner_style: dots   # Loading spinner style

history:
  enabled: true         # Enable history tracking
  max_entries: 1000     # Maximum history entries
  file_path: ~/.gitsage/history.json

cache:
  enabled: true         # Enable response caching
  max_entries: 100      # Maximum cache entries
  ttl_minutes: 60       # Cache TTL in minutes

security:
  warning_acknowledged: false  # First-use security warning flag
  path_check_done: false       # PATH detection completion flag
```

### Configuration Priority

Values are loaded in this order (highest priority first):

1. Command-line flags (`--provider`, `--model`)
2. Environment variables (`GITSAGE_API_KEY`, etc.)
3. Configuration file (`~/.gitsage/config.yaml`)
4. Default values

### Environment Variables

| Variable | Description |
|----------|-------------|
| `GITSAGE_API_KEY` | API key for the AI provider |
| `GITSAGE_PROVIDER` | AI provider name |
| `GITSAGE_MODEL` | AI model name |
| `GITSAGE_SECURITY_PATH_CHECK_DONE` | Skip PATH detection if set to `true` |

## AI Providers

### OpenAI (Default)

```bash
gitsage config set provider.name openai
gitsage config set provider.api_key sk-your-key
gitsage config set provider.model gpt-4o-mini
```

Supported models: `gpt-4o`, `gpt-4o-mini`, `gpt-4-turbo`, `gpt-3.5-turbo`

### DeepSeek

```bash
gitsage config set provider.name deepseek
gitsage config set provider.api_key your-deepseek-key
gitsage config set provider.model deepseek-chat
```

### Ollama (Local)

```bash
gitsage config set provider.name ollama
gitsage config set provider.model codellama
gitsage config set provider.endpoint http://localhost:11434
```

No API key required. Make sure Ollama is running locally.

## Troubleshooting

### "No staged changes found"

Make sure you've staged your changes with `git add`:
```bash
git add .
# or
git add <specific-files>
```

### "Configuration not initialized"

Run the init command first:
```bash
gitsage config init
```

### "API key validation failed"

Check that your API key is set correctly:
```bash
gitsage config set provider.api_key your-key
gitsage config list  # Verify (key will be masked)
```

### "AI provider API call failed"

1. Check your internet connection
2. Verify your API key is valid
3. Check if the provider service is available
4. Try with `--verbose` flag for more details:
   ```bash
   gitsage --verbose
   ```

### "Request timeout"

The default timeout is 30 seconds. For large diffs or slow connections:
- Try with a smaller diff
- Check your network connection
- Consider using a local Ollama instance

### Large Diffs

GitSage automatically handles large diffs by:
1. Excluding lock files (package-lock.json, go.sum, etc.)
2. Chunking diffs larger than 10KB
3. Summarizing very large files (>100KB)

You can adjust the threshold:
```bash
gitsage config set git.diff_size_threshold 20480  # 20KB
```

### Cache Issues

If you're getting stale responses, bypass the cache:
```bash
gitsage --no-cache
```

Or disable caching entirely:
```bash
gitsage config set cache.enabled false
```

### PATH Not Found After Installation

If GitSage is not found in your PATH after installation:

1. **Check if PATH detection ran**: On first run, GitSage should automatically detect and offer to add itself to PATH
2. **Manually trigger PATH check**: Reset the flag and run again:
   ```bash
   gitsage config set security.path_check_done false
   gitsage
   ```
3. **Skip automatic detection**: Use the `--skip-path-check` flag if you prefer manual setup
4. **Manual setup instructions**:
   
   **Windows**:
   - Open System Properties → Advanced → Environment Variables
   - Find PATH in User Variables
   - Add the directory containing `gitsage.exe`
   - Restart your terminal
   
   **macOS/Linux (Bash/Zsh)**:
   ```bash
   echo 'export PATH="$PATH:/path/to/gitsage"' >> ~/.bashrc  # or ~/.zshrc
   source ~/.bashrc
   ```
   
   **Fish**:
   ```bash
   echo 'set -gx PATH $PATH /path/to/gitsage' >> ~/.config/fish/config.fish
   source ~/.config/fish/config.fish
   ```

## Contributing

Contributions are welcome! Please follow these guidelines:

### Development Setup

1. Clone the repository:
   ```bash
   git clone https://github.com/gitsage/gitsage.git
   cd gitsage
   ```

2. Install dependencies:
   ```bash
   make deps
   ```

3. Build:
   ```bash
   make build
   ```

4. Run tests:
   ```bash
   make test
   ```

### Code Style

- Follow standard Go conventions
- Run `make fmt` before committing
- Run `make lint` to check for issues
- Run `make vet` for static analysis

### Testing

- Write tests for new functionality
- Ensure all tests pass: `make test`
- Check coverage: `make test-coverage`

### Pull Requests

1. Fork the repository
2. Create a feature branch: `git checkout -b feature/my-feature`
3. Make your changes
4. Run tests and linting
5. Commit with a meaningful message (use GitSage!)
6. Push and create a Pull Request

### Commit Message Format

We follow Conventional Commits:

```
<type>(<scope>): <subject>

[optional body]

[optional footer]
```

Types: `feat`, `fix`, `docs`, `style`, `refactor`, `test`, `chore`, `perf`, `ci`, `build`, `revert`

## License

MIT License - see [LICENSE](LICENSE) for details.

## Acknowledgments

- [Cobra](https://github.com/spf13/cobra) - CLI framework
- [Viper](https://github.com/spf13/viper) - Configuration management
- [go-openai](https://github.com/sashabaranov/go-openai) - OpenAI client
- [Charm](https://charm.sh/) - Terminal UI libraries
