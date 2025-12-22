# Design Document

## Overview

PATH 检测功能为 GitSage 添加首次运行时的环境检查能力。当用户首次执行 GitSage 时，系统会检测可执行文件是否已在系统 PATH 中。如果未找到，系统会询问用户是否自动添加，并根据操作系统和 shell 类型执行相应的配置操作。

该功能集成到现有的配置管理系统中，使用 `~/.gitsage/config.yaml` 存储检测状态，确保检测只在首次运行时执行。

## Architecture

### 集成架构

```
┌─────────────────────────────────────────────────────────────┐
│                         CLI Layer                            │
│                    (Cobra Commands)                          │
│  ┌──────────────────────────────────────────────────────┐   │
│  │                    root.go                            │   │
│  │  • PersistentPreRunE: PATH 检测入口点                 │   │
│  │  • --skip-path-check flag                            │   │
│  └──────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│                    PATH Detection Package                    │
│                  (internal/pkg/pathcheck)                    │
│  ┌────────────────────────────────────────────────────────┐ │
│  │                    Checker                              │ │
│  │  • IsInPath() - 检测是否在 PATH 中                      │ │
│  │  • AddToPath() - 自动添加到 PATH                        │ │
│  │  • GetShellProfile() - 获取 shell 配置文件路径          │ │
│  └────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────┘
                            │
        ┌───────────────────┼───────────────────┐
        ▼                   ▼                   ▼
┌──────────────┐   ┌──────────────┐   ┌──────────────┐
│ Config Mgr   │   │ UI Manager   │   │ OS/Exec      │
│              │   │              │   │              │
│ • 存储检测状态│   │ • 用户确认   │   │ • setx       │
│ • path_check │   │ • 成功/失败  │   │ • shell 操作 │
│   _done      │   │   消息显示   │   │              │
└──────────────┘   └──────────────┘   └──────────────┘
```

### 执行流程

```
┌─────────────┐
│ 用户执行     │
│ gitsage     │
└──────┬──────┘
       │
       ▼
┌─────────────────────┐
│ 检查 --skip-path-   │
│ check flag          │
└──────┬──────────────┘
       │ 未设置
       ▼
┌─────────────────────┐
│ 读取 config:        │
│ path_check_done     │
└──────┬──────────────┘
       │ false 或不存在
       ▼
┌─────────────────────┐
│ 检测 gitsage 是否   │
│ 在 PATH 中          │
└──────┬──────────────┘
       │
   ┌───┴───┐
   │       │
   ▼       ▼
┌─────┐  ┌─────────────────┐
│在PATH│  │不在 PATH        │
└──┬──┘  └───────┬─────────┘
   │             │
   │             ▼
   │     ┌─────────────────┐
   │     │ 询问用户是否    │
   │     │ 自动添加        │
   │     └───────┬─────────┘
   │         ┌───┴───┐
   │         │       │
   │         ▼       ▼
   │     ┌─────┐  ┌─────┐
   │     │ Yes │  │ No  │
   │     └──┬──┘  └──┬──┘
   │        │        │
   │        ▼        │
   │  ┌───────────┐  │
   │  │ 执行添加  │  │
   │  │ 操作      │  │
   │  └─────┬─────┘  │
   │        │        │
   │    ┌───┴───┐    │
   │    │       │    │
   │    ▼       ▼    │
   │ ┌─────┐ ┌─────┐ │
   │ │成功 │ │失败 │ │
   │ └──┬──┘ └──┬──┘ │
   │    │       │    │
   │    │       ▼    │
   │    │  ┌────────┐│
   │    │  │显示手动││
   │    │  │配置说明││
   │    │  └────────┘│
   │    │            │
   └────┴────────────┘
            │
            ▼
   ┌─────────────────┐
   │ 设置 config:    │
   │ path_check_done │
   │ = true          │
   └────────┬────────┘
            │
            ▼
   ┌─────────────────┐
   │ 继续正常执行    │
   └─────────────────┘
```

## Components and Interfaces

### 1. Path Checker (`internal/pkg/pathcheck`)

```go
package pathcheck

import (
    "context"
    "runtime"
)

// Checker provides PATH detection and modification functionality.
type Checker interface {
    // IsInPath checks if the executable is accessible via PATH.
    IsInPath(ctx context.Context) (bool, error)
    
    // AddToPath adds the executable directory to the system PATH.
    // Returns the path that was added and any error.
    AddToPath(ctx context.Context) (string, error)
    
    // GetExecutableDir returns the directory containing the executable.
    GetExecutableDir() (string, error)
    
    // GetShellProfile returns the appropriate shell profile path for the current system.
    GetShellProfile() (string, error)
    
    // GetOS returns the current operating system.
    GetOS() string
}

// ShellType represents the type of shell.
type ShellType int

const (
    ShellUnknown ShellType = iota
    ShellBash
    ShellZsh
    ShellFish
    ShellPowerShell
    ShellCmd
)

// PathAddResult contains the result of adding to PATH.
type PathAddResult struct {
    Success     bool
    AddedPath   string
    ProfilePath string
    Message     string
    NeedsReload bool
}
```

### 2. Platform-Specific Implementations

```go
// UnixChecker implements Checker for macOS and Linux.
type UnixChecker struct {
    executablePath string
}

// WindowsChecker implements Checker for Windows.
type WindowsChecker struct {
    executablePath string
}

// NewChecker creates a platform-appropriate Checker.
func NewChecker() (Checker, error) {
    execPath, err := os.Executable()
    if err != nil {
        return nil, fmt.Errorf("failed to get executable path: %w", err)
    }
    
    switch runtime.GOOS {
    case "windows":
        return &WindowsChecker{executablePath: execPath}, nil
    default:
        return &UnixChecker{executablePath: execPath}, nil
    }
}
```

### 3. Configuration Extension

在现有 `Config` 结构中添加 PATH 检测状态：

```go
// SecurityConfig contains security-related settings.
type SecurityConfig struct {
    // WarningAcknowledged indicates if the user has acknowledged the first-use security warning.
    WarningAcknowledged bool `mapstructure:"warning_acknowledged"`
    // PathCheckDone indicates if the PATH check has been performed.
    PathCheckDone bool `mapstructure:"path_check_done"`
}
```

### 4. CLI Integration

在 `root.go` 中添加 PATH 检测逻辑：

```go
// NewRootCmd creates the root command for GitSage CLI.
func NewRootCmd(version, commitHash, date string) *cobra.Command {
    rootCmd := &cobra.Command{
        // ... existing config ...
        PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
            // Skip for config and help commands
            if cmd.Name() == "config" || cmd.Name() == "help" {
                return nil
            }
            
            skipPathCheck, _ := cmd.Flags().GetBool("skip-path-check")
            if skipPathCheck {
                return nil
            }
            
            return runPathCheck(cmd)
        },
    }
    
    // Add skip-path-check flag
    rootCmd.PersistentFlags().Bool("skip-path-check", false, "Skip PATH detection check")
    
    // ... rest of existing code ...
}
```

## Data Models

### Shell Profile Paths

```go
// ShellProfiles maps shell types to their profile file paths.
var ShellProfiles = map[ShellType][]string{
    ShellBash: {
        ".bashrc",
        ".bash_profile",
        ".profile",
    },
    ShellZsh: {
        ".zshrc",
    },
    ShellFish: {
        ".config/fish/config.fish",
    },
}

// WindowsPathMethods defines methods for adding to PATH on Windows.
type WindowsPathMethod int

const (
    WindowsSetx WindowsPathMethod = iota
    WindowsRegistry
)
```

### Export Statement Templates

```go
// UnixExportTemplate is the template for Unix shell export statements.
const UnixExportTemplate = `
# Added by GitSage
export PATH="$PATH:%s"
`

// FishExportTemplate is the template for Fish shell path addition.
const FishExportTemplate = `
# Added by GitSage
set -gx PATH $PATH %s
`
```

## Correctness Properties

*A property is a characteristic or behavior that should hold true across all valid executions of a system—essentially, a formal statement about what the system should do. Properties serve as the bridge between human-readable specifications and machine-verifiable correctness guarantees.*



### Core Detection Properties

Property 1: PATH accessibility detection
*For any* system PATH configuration, the IsInPath function should return true if and only if the executable name resolves to the current executable's path when searched in PATH directories
**Validates: Requirements 1.1, 1.3**

Property 2: Config flag persistence round-trip
*For any* PATH check completion (whether user accepts, declines, or is already in PATH), setting path_check_done to true and then reading it should return true, and subsequent runs should skip the PATH check
**Validates: Requirements 1.4, 1.5, 2.6, 3.2**

Property 3: Shell profile selection
*For any* Unix shell type (bash, zsh, fish, unknown), the GetShellProfile function should return the correct profile file path: bash→.bashrc/.bash_profile, zsh→.zshrc, fish→.config/fish/config.fish, unknown→.profile
**Validates: Requirements 4.1, 4.2, 4.3, 4.4, 4.5**

Property 4: Skip flag behavior
*For any* execution with --skip-path-check flag set, the PATH detection logic should not execute regardless of the path_check_done config value
**Validates: Requirements 3.1**

Property 5: Unix export statement format
*For any* valid directory path on Unix systems, the generated export statement should be syntactically correct for the target shell and contain the exact directory path
**Validates: Requirements 2.3**

Property 6: OS detection consistency
*For any* execution, the GetOS function should return a value consistent with runtime.GOOS and the platform-specific implementation should be selected accordingly
**Validates: Requirements 2.1**

## Error Handling

### Error Categories

**Detection Errors**
- Unable to get executable path
- Unable to read PATH environment variable
- Unable to determine shell type

**Modification Errors**
- Permission denied when modifying shell profile
- Unable to create config directory
- Windows setx command failure
- Registry access denied (Windows)

### Error Handling Strategy

```go
type PathCheckError struct {
    Code    PathErrorCode
    Message string
    Cause   error
}

type PathErrorCode int

const (
    ErrGetExecutablePath PathErrorCode = iota
    ErrReadPATH
    ErrDetectShell
    ErrModifyProfile
    ErrWindowsSetx
    ErrPermissionDenied
)
```

**Error Handling Rules:**
1. Detection errors should not block normal execution - log warning and continue
2. Modification errors should display manual instructions as fallback
3. All errors should be wrapped with context for debugging
4. Permission errors should suggest running with appropriate privileges

### Fallback Instructions

当自动添加失败时，显示手动配置说明：

**Windows:**
```
无法自动添加到 PATH。请手动执行以下步骤：
1. 打开"系统属性" > "高级" > "环境变量"
2. 在"用户变量"中找到 PATH
3. 添加: C:\path\to\gitsage
4. 重启终端
```

**Unix (Bash/Zsh):**
```
无法自动添加到 PATH。请手动执行以下步骤：
1. 编辑 ~/.bashrc (或 ~/.zshrc)
2. 添加: export PATH="$PATH:/path/to/gitsage"
3. 执行: source ~/.bashrc
```

**Fish:**
```
无法自动添加到 PATH。请手动执行以下步骤：
1. 编辑 ~/.config/fish/config.fish
2. 添加: set -gx PATH $PATH /path/to/gitsage
3. 重启终端或执行: source ~/.config/fish/config.fish
```

## Testing Strategy

### Unit Testing

**Framework:** Go standard `testing` package with `testify` for assertions

**Coverage Targets:**
- PATH detection logic with mocked environment
- Shell profile path selection
- Export statement generation
- Config flag read/write
- OS detection

**Key Unit Tests:**
- IsInPath with executable in PATH
- IsInPath with executable not in PATH
- GetShellProfile for each shell type
- Export statement format for bash/zsh/fish
- Config flag persistence

### Property-Based Testing

**Framework:** `github.com/leanovate/gopter`

**Configuration:** Each property test should run a minimum of 100 iterations

**Test Tagging:** Each property-based test must include a comment with the format:
```go
// Feature: path-detection, Property X: <property description>
// Validates: Requirements X.Y
```

**Property Test Coverage:**

1. **Shell Profile Selection Property Test**
   - Generate random shell type values
   - Verify correct profile path is returned
   - **Feature: path-detection, Property 3: Shell profile selection**
   - **Validates: Requirements 4.1, 4.2, 4.3, 4.4, 4.5**

2. **Export Statement Format Property Test**
   - Generate random valid directory paths
   - Verify export statement is syntactically correct
   - **Feature: path-detection, Property 5: Unix export statement format**
   - **Validates: Requirements 2.3**

3. **Config Flag Round-Trip Property Test**
   - Set path_check_done to various values
   - Verify read returns the same value
   - **Feature: path-detection, Property 2: Config flag persistence round-trip**
   - **Validates: Requirements 1.4, 1.5, 2.6, 3.2**

### Integration Testing

**Scope:** Test interactions between components

**Key Integration Tests:**
- Full PATH check flow with mocked UI
- Config persistence across multiple runs
- Shell profile modification (in temp directory)

**Test Environment:**
- Use temporary directories for config and profile files
- Mock UI interactions
- Use environment variable manipulation for PATH testing

## Security Considerations

### File Permissions

- Shell profile modifications should preserve existing file permissions
- Config file should maintain 0600 permissions
- Avoid writing to system-wide PATH (user PATH only)

### Input Validation

- Validate executable path before adding to PATH
- Sanitize paths to prevent injection attacks
- Verify shell profile exists before modification

## Implementation Notes

### Platform Detection

```go
func getPlatform() string {
    return runtime.GOOS // "windows", "darwin", "linux"
}
```

### Shell Detection (Unix)

```go
func detectShell() ShellType {
    shell := os.Getenv("SHELL")
    switch {
    case strings.Contains(shell, "bash"):
        return ShellBash
    case strings.Contains(shell, "zsh"):
        return ShellZsh
    case strings.Contains(shell, "fish"):
        return ShellFish
    default:
        return ShellUnknown
    }
}
```

### PATH Check Logic

```go
func (c *UnixChecker) IsInPath(ctx context.Context) (bool, error) {
    execDir, err := c.GetExecutableDir()
    if err != nil {
        return false, err
    }
    
    pathEnv := os.Getenv("PATH")
    paths := filepath.SplitList(pathEnv)
    
    for _, p := range paths {
        if p == execDir {
            return true, nil
        }
    }
    return false, nil
}
```

### Windows PATH Addition

```go
func (c *WindowsChecker) AddToPath(ctx context.Context) (string, error) {
    execDir, err := c.GetExecutableDir()
    if err != nil {
        return "", err
    }
    
    // Use setx to add to user PATH
    cmd := exec.CommandContext(ctx, "setx", "PATH", "%PATH%;"+execDir)
    if err := cmd.Run(); err != nil {
        return "", fmt.Errorf("setx failed: %w", err)
    }
    
    return execDir, nil
}
```

### Unix PATH Addition

```go
func (c *UnixChecker) AddToPath(ctx context.Context) (string, error) {
    execDir, err := c.GetExecutableDir()
    if err != nil {
        return "", err
    }
    
    profilePath, err := c.GetShellProfile()
    if err != nil {
        return "", err
    }
    
    // Generate export statement
    var exportStmt string
    if c.detectShell() == ShellFish {
        exportStmt = fmt.Sprintf(FishExportTemplate, execDir)
    } else {
        exportStmt = fmt.Sprintf(UnixExportTemplate, execDir)
    }
    
    // Append to profile
    f, err := os.OpenFile(profilePath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
    if err != nil {
        return "", fmt.Errorf("failed to open profile: %w", err)
    }
    defer f.Close()
    
    if _, err := f.WriteString(exportStmt); err != nil {
        return "", fmt.Errorf("failed to write to profile: %w", err)
    }
    
    return execDir, nil
}
```
