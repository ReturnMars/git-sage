// Package pathcheck provides PATH detection and modification functionality for GitSage.
package pathcheck

import (
	"fmt"
	"runtime"
)

// ManualInstructions contains the manual configuration instructions for a platform.
type ManualInstructions struct {
	// Platform is the operating system (windows, darwin, linux).
	Platform string
	// Shell is the shell type (bash, zsh, fish, or empty for Windows).
	Shell string
	// Steps contains the step-by-step instructions.
	Steps []string
	// ExampleCommand contains an example command to add to PATH.
	ExampleCommand string
}

// GetManualInstructions returns manual PATH configuration instructions
// based on the operating system and shell type.
func GetManualInstructions(execDir string, shellType ShellType) *ManualInstructions {
	platform := runtime.GOOS

	switch platform {
	case "windows":
		return getWindowsInstructions(execDir)
	default:
		return getUnixInstructions(execDir, shellType, platform)
	}
}

// getWindowsInstructions returns manual instructions for Windows.
func getWindowsInstructions(execDir string) *ManualInstructions {
	return &ManualInstructions{
		Platform: "windows",
		Shell:    "",
		Steps: []string{
			"1. 打开 系统属性 > 高级 > 环境变量",
			"   (或按 Win+R, 输入 sysdm.cpl, 点击 高级 选项卡)",
			"2. 在 用户变量 中找到 PATH",
			"3. 点击 编辑, 然后点击 新建",
			fmt.Sprintf("4. 添加: %s", execDir),
			"5. 点击 确定 保存更改",
			"6. 重启终端或命令提示符",
		},
		ExampleCommand: fmt.Sprintf("setx PATH \"%%PATH%%;%s\"", execDir),
	}
}

// getUnixInstructions returns manual instructions for Unix-like systems.
func getUnixInstructions(execDir string, shellType ShellType, platform string) *ManualInstructions {
	switch shellType {
	case ShellFish:
		return getFishInstructions(execDir, platform)
	case ShellZsh:
		return getZshInstructions(execDir, platform)
	case ShellBash:
		return getBashInstructions(execDir, platform)
	default:
		return getDefaultUnixInstructions(execDir, platform)
	}
}

// getBashInstructions returns manual instructions for Bash shell.
func getBashInstructions(execDir string, platform string) *ManualInstructions {
	profileFile := ".bashrc"
	if platform == "darwin" {
		profileFile = ".bash_profile"
	}

	return &ManualInstructions{
		Platform: platform,
		Shell:    "bash",
		Steps: []string{
			fmt.Sprintf("1. 编辑 ~/%s", profileFile),
			fmt.Sprintf("2. 添加以下行: export PATH=\"$PATH:%s\"", execDir),
			fmt.Sprintf("3. 保存文件并执行: source ~/%s", profileFile),
			"   或者重启终端",
		},
		ExampleCommand: fmt.Sprintf("echo 'export PATH=\"$PATH:%s\"' >> ~/%s && source ~/%s", execDir, profileFile, profileFile),
	}
}

// getZshInstructions returns manual instructions for Zsh shell.
func getZshInstructions(execDir string, platform string) *ManualInstructions {
	return &ManualInstructions{
		Platform: platform,
		Shell:    "zsh",
		Steps: []string{
			"1. 编辑 ~/.zshrc",
			fmt.Sprintf("2. 添加以下行: export PATH=\"$PATH:%s\"", execDir),
			"3. 保存文件并执行: source ~/.zshrc",
			"   或者重启终端",
		},
		ExampleCommand: fmt.Sprintf("echo 'export PATH=\"$PATH:%s\"' >> ~/.zshrc && source ~/.zshrc", execDir),
	}
}

// getFishInstructions returns manual instructions for Fish shell.
func getFishInstructions(execDir string, platform string) *ManualInstructions {
	return &ManualInstructions{
		Platform: platform,
		Shell:    "fish",
		Steps: []string{
			"1. 编辑 ~/.config/fish/config.fish",
			fmt.Sprintf("2. 添加以下行: set -gx PATH $PATH %s", execDir),
			"3. 保存文件并执行: source ~/.config/fish/config.fish",
			"   或者重启终端",
		},
		ExampleCommand: fmt.Sprintf("echo 'set -gx PATH $PATH %s' >> ~/.config/fish/config.fish && source ~/.config/fish/config.fish", execDir),
	}
}

// getDefaultUnixInstructions returns manual instructions for unknown Unix shells.
func getDefaultUnixInstructions(execDir string, platform string) *ManualInstructions {
	return &ManualInstructions{
		Platform: platform,
		Shell:    "unknown",
		Steps: []string{
			"1. 编辑 ~/.profile",
			fmt.Sprintf("2. 添加以下行: export PATH=\"$PATH:%s\"", execDir),
			"3. 保存文件并执行: source ~/.profile",
			"   或者重启终端",
			"",
			"注意: 如果您使用的是其他 shell, 请参考该 shell 的文档",
			"      来了解如何修改 PATH 环境变量。",
		},
		ExampleCommand: fmt.Sprintf("echo 'export PATH=\"$PATH:%s\"' >> ~/.profile && source ~/.profile", execDir),
	}
}

// FormatInstructions formats the manual instructions as a human-readable string.
func FormatInstructions(instructions *ManualInstructions) string {
	if instructions == nil {
		return ""
	}

	result := "无法自动添加到 PATH。请手动执行以下步骤:\n\n"

	for _, step := range instructions.Steps {
		result += step + "\n"
	}

	if instructions.ExampleCommand != "" {
		result += "\n或者直接执行以下命令:\n"
		result += "  " + instructions.ExampleCommand + "\n"
	}

	return result
}

// FormatInstructionsEnglish formats the manual instructions in English.
func FormatInstructionsEnglish(instructions *ManualInstructions) string {
	if instructions == nil {
		return ""
	}

	result := "Unable to automatically add to PATH. Please follow these steps manually:\n\n"

	// Convert Chinese instructions to English
	switch instructions.Platform {
	case "windows":
		result += "1. Open System Properties > Advanced > Environment Variables\n"
		result += "   (Or press Win+R, type sysdm.cpl, click Advanced tab)\n"
		result += "2. Find PATH in User Variables\n"
		result += "3. Click Edit, then click New\n"
		result += fmt.Sprintf("4. Add: %s\n", getExecDirFromSteps(instructions.Steps))
		result += "5. Click OK to save changes\n"
		result += "6. Restart your terminal or command prompt\n"
	default:
		result += formatUnixInstructionsEnglish(instructions)
	}

	if instructions.ExampleCommand != "" {
		result += "\nOr run this command directly:\n"
		result += "  " + instructions.ExampleCommand + "\n"
	}

	return result
}

// formatUnixInstructionsEnglish formats Unix instructions in English.
func formatUnixInstructionsEnglish(instructions *ManualInstructions) string {
	var result string
	var profileFile string
	var exportCmd string

	switch instructions.Shell {
	case "fish":
		profileFile = "~/.config/fish/config.fish"
		exportCmd = "set -gx PATH $PATH <path>"
	case "zsh":
		profileFile = "~/.zshrc"
		exportCmd = "export PATH=\"$PATH:<path>\""
	case "bash":
		if instructions.Platform == "darwin" {
			profileFile = "~/.bash_profile"
		} else {
			profileFile = "~/.bashrc"
		}
		exportCmd = "export PATH=\"$PATH:<path>\""
	default:
		profileFile = "~/.profile"
		exportCmd = "export PATH=\"$PATH:<path>\""
	}

	result += fmt.Sprintf("1. Edit %s\n", profileFile)
	result += fmt.Sprintf("2. Add this line: %s\n", exportCmd)
	result += fmt.Sprintf("3. Save the file and run: source %s\n", profileFile)
	result += "   Or restart your terminal\n"

	if instructions.Shell == "unknown" {
		result += "\nNote: If you're using a different shell, please refer to its documentation\n"
		result += "      for instructions on modifying the PATH environment variable.\n"
	}

	return result
}

// getExecDirFromSteps extracts the executable directory from instruction steps.
func getExecDirFromSteps(steps []string) string {
	for _, step := range steps {
		// Look for the step that contains the path (step 4)
		if len(step) > 10 {
			// Check for Chinese format "4. 添加: "
			prefix := "4. 添加: "
			if len(step) >= len(prefix) && step[:len(prefix)] == prefix {
				return step[len(prefix):]
			}
		}
	}
	return "<path>"
}
