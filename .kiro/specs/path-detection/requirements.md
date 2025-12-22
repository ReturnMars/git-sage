# Requirements Document

## Introduction

GitSage 需要在首次执行时检测自身是否已注册到系统的全局 PATH 环境变量中。如果未注册，系统应询问用户是否自动添加到 PATH，用户确认后自动完成配置。此功能提升用户体验，确保工具可以在全局范围内便捷使用。

## Glossary

- **GitSage**: 正在开发的 AI 驱动的 Git 提交消息生成 CLI 工具
- **PATH**: 操作系统环境变量，包含可执行文件的搜索路径列表
- **First_Run_Detection**: 检测工具是否为首次运行的机制
- **Path_Registration**: 将可执行文件路径添加到系统 PATH 环境变量的过程
- **Config_File**: 存储在 `~/.gitsage/config.yaml` 的配置文件
- **Shell_Profile**: Unix 系统的 shell 配置文件（如 `.bashrc`, `.zshrc`, `.profile`）

## Requirements

### Requirement 1

**User Story:** As a developer, I want GitSage to detect if it's registered in the global PATH on first run, so that I can be guided to set it up correctly for global access.

#### Acceptance Criteria

1. WHEN a user executes GitSage for the first time, THEN the System SHALL check if the executable is accessible via the global PATH
2. WHEN the executable is not found in PATH, THEN the System SHALL prompt the user asking if they want to automatically add it to PATH
3. WHEN the executable is found in PATH, THEN the System SHALL proceed normally without displaying any PATH-related messages
4. WHEN the PATH check has been performed once, THEN the System SHALL store a flag in the config file to skip future checks
5. IF the config file indicates PATH check was already performed, THEN the System SHALL skip the PATH detection logic

### Requirement 2

**User Story:** As a developer, I want GitSage to automatically add itself to my PATH when I confirm, so that I don't need to manually configure environment variables.

#### Acceptance Criteria

1. WHEN the user confirms to add GitSage to PATH, THEN the System SHALL detect the user's operating system (Windows, macOS, Linux)
2. WHEN the operating system is Windows, THEN the System SHALL add the executable directory to the user's PATH environment variable via registry or setx command
3. WHEN the operating system is macOS or Linux, THEN the System SHALL append an export statement to the appropriate shell profile file (.bashrc, .zshrc, or .profile)
4. WHEN the PATH modification is successful, THEN the System SHALL display a success message and instruct the user to restart their terminal or source the profile
5. IF the PATH modification fails, THEN the System SHALL display an error message with manual configuration instructions
6. WHEN the user declines to add to PATH, THEN the System SHALL proceed with normal execution and mark the check as done

### Requirement 3

**User Story:** As a developer, I want to be able to skip or reset the PATH check, so that I have control over when this check occurs.

#### Acceptance Criteria

1. WHEN a user executes GitSage with `--skip-path-check` flag, THEN the System SHALL skip the PATH detection for that execution
2. WHEN a user executes `gitsage config set path_check_done false`, THEN the System SHALL reset the PATH check flag to trigger detection on next run
3. WHEN a user executes `gitsage config list`, THEN the System SHALL display the current value of the `path_check_done` setting

### Requirement 4

**User Story:** As a developer, I want the PATH detection to work correctly on different shells, so that my preferred shell environment is properly configured.

#### Acceptance Criteria

1. WHEN detecting the shell on Unix systems, THEN the System SHALL check the SHELL environment variable to determine the active shell
2. WHEN the shell is bash, THEN the System SHALL modify `.bashrc` or `.bash_profile`
3. WHEN the shell is zsh, THEN the System SHALL modify `.zshrc`
4. WHEN the shell is fish, THEN the System SHALL modify `~/.config/fish/config.fish` with fish-specific syntax
5. WHEN the shell cannot be determined, THEN the System SHALL default to modifying `.profile` and inform the user

