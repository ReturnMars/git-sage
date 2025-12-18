你好！作为一个 Go 高级架构师和开发工程师，我非常理解你的需求。通过命令行（CLI）结合 AI 自动生成 Commit Message 是一个非常典型的 "Developer Experience (DevEx)" 提升工具。

我们需要构建一个**轻量、高效、易于维护且符合 Go 社区标准**的 CLI 工具。

以下是我的实施方案，涵盖架构设计、工程结构、核心代码实现以及项目落地建议。

---

### 1. 架构设计 (Architecture Design)

既然要求“只是一个工具”，我们要遵循 **KISS (Keep It Simple, Stupid)** 原则，同时使用 **Clean Architecture** 的思想，确保 AI 供应商（如 OpenAI）与 Git 操作解耦，方便未来更换模型（如切换到 Claude 或本地 Ollama）。

#### 核心流程
1.  **Input**: 用户执行命令 -> 读取本地 `git diff` (暂存区)。
2.  **Process**: 预处理 Diff -> 组装 Prompt -> 调用 AI 接口 -> 获取并清洗结果。
3.  **Output**: 展示生成的 Commit Message -> 用户交互 (确认/编辑/取消) -> 执行 `git commit`。

#### 模块划分
*   **CLI Layer (`cmd`)**: 基于 `Cobra` 框架，处理参数解析、用户交互（TUI）。
*   **Business Layer (`internal/domain`)**: 定义核心逻辑（如 Prompt 生成规则）。
*   **Infrastructure Layer (`internal/infra`)**:
    *   **Git Adapter**: 调用系统 `git` 命令。
    *   **AI Adapter**: 对接 LLM API (OpenAI 兼容接口)。

---

### 2. 项目工程结构 (Project Layout)

遵循 `golang-standards/project-layout`，这是 Go 社区的实际标准。

```text
aicm/
├── cmd/
│   └── aicm/
│       └── main.go           # 程序入口
├── internal/
│   ├── app/                  # 应用编排层 (Service)
│   │   └── generator.go      # 核心流程控制
│   ├── pkg/                  # 内部公共包
│   │   ├── git/              # Git 操作封装
│   │   └── ai/               # AI 接口封装
│   └── config/               # 配置加载 (Viper/Env)
├── go.mod
├── go.sum
└── Makefile                  # 构建脚本
```

---

### 3. 核心代码实现方案

我们需要用到的关键库：
*   **CLI 框架**: `github.com/spf13/cobra`
*   **AI 客户端**: `github.com/sashabaranov/go-openai` (或者直接用 Resty 自己撸，减少依赖，但用库更稳)
*   **交互 UI**: `github.com/charmbracelet/huh` (现代化的终端表单) 或 `github.com/briandowns/spinner` (加载动画)

#### Step 1: Git 操作封装 (`internal/pkg/git/git.go`)

我们直接使用 `os/exec` 调用系统 Git，比引入 `go-git` 库更轻量且符合“工具”的定位（依赖用户环境）。

```go
package git

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

type Client struct{}

func NewClient() *Client {
	return &Client{}
}

// GetStagedDiff 获取暂存区的 Diff
func (c *Client) GetStagedDiff(ctx context.Context) (string, error) {
	// --cached 表示只看 staged 的文件，避免把没 add 的文件也写进 commit
	cmd := exec.CommandContext(ctx, "git", "diff", "--cached")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get git diff: %w", err)
	}
	
	diff := string(out)
	if strings.TrimSpace(diff) == "" {
		return "", fmt.Errorf("no staged changes found. did you run 'git add'?")
	}
	
	// 简单防爆：如果 diff 太大，可能需要截断，避免 token 溢出
	if len(diff) > 10000 {
		diff = diff[:10000] + "\n...[truncated]"
	}
	return diff, nil
}

// Commit 执行提交
func (c *Client) Commit(ctx context.Context, message string) error {
	cmd := exec.CommandContext(ctx, "git", "commit", "-m", message)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git commit failed: %s, %w", string(output), err)
	}
	return nil
}
```

#### Step 2: AI 接口定义与实现 (`internal/pkg/ai/ai.go`)

定义接口是高级开发的素养，方便 Mock 测试和切换供应商。

```go
package ai

import (
	"context"
	"fmt"
	"github.com/sashabaranov/go-openai"
)

type Service interface {
	GenerateCommitMessage(ctx context.Context, diff string) (string, error)
}

type OpenAIClient struct {
	client *openai.Client
	model  string
}

func NewOpenAIClient(apiKey string, model string) *OpenAIClient {
	if model == "" {
		model = openai.GPT4oMini // 推荐用 Mini，速度快且便宜
	}
	return &OpenAIClient{
		client: openai.NewClient(apiKey),
		model:  model,
	}
}

func (c *OpenAIClient) GenerateCommitMessage(ctx context.Context, diff string) (string, error) {
	systemPrompt := `You are a helpful assistant that writes semantic git commit messages. 
	Format: <type>(<scope>): <subject>
	Example: feat(user): add login functionality
	Rules:
	1. Use Conventional Commits format.
	2. Keep it concise (under 72 chars for subject).
	3. Reply ONLY with the commit message, no explanations.`

	req := openai.ChatCompletionRequest{
		Model: c.model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
			{Role: openai.ChatMessageRoleUser, Content: fmt.Sprintf("Generate a commit message for this diff:\n%s", diff)},
		},
		Temperature: 0.2, // 低温，保持准确
	}

	resp, err := c.client.CreateChatCompletion(ctx, req)
	if err != nil {
		return "", err
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no response from AI")
	}

	return resp.Choices[0].Message.Content, nil
}
```

#### Step 3: CLI 主逻辑 (`cmd/aicm/main.go`)

这里我们将所有模块串联起来。

```go
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/briandowns/spinner"
	"github.com/charmbracelet/huh" // 用于漂亮的确认框
	
	"aicm/internal/pkg/ai"
	"aicm/internal/pkg/git"
)

var (
	apiKey string
	model  string
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "aicm",
		Short: "AI powered git commit message generator",
		Run:   run,
	}

	// 实际项目中建议用 Viper 从 Config 文件读取，这里演示用 Flag 或 Env
	rootCmd.PersistentFlags().StringVar(&apiKey, "key", os.Getenv("OPENAI_API_KEY"), "OpenAI API Key")
	rootCmd.PersistentFlags().StringVar(&model, "model", "gpt-4o-mini", "AI Model to use")

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func run(cmd *cobra.Command, args []string) {
	if apiKey == "" {
		fmt.Println("Error: API Key is required. Set OPENAI_API_KEY env or use --key flag.")
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	gitClient := git.NewClient()
	aiClient := ai.NewOpenAIClient(apiKey, model)

	// 1. 获取 Diff
	diff, err := gitClient.GetStagedDiff(ctx)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	// 2. 调用 AI (加个 Loading 动画，提升体验)
	s := spinner.New(spinner.CharSets[11], 100*time.Millisecond)
	s.Suffix = " Generating commit message..."
	s.Start()
	
	msg, err := aiClient.GenerateCommitMessage(ctx, diff)
	s.Stop()
	
	if err != nil {
		fmt.Printf("Error generating message: %v\n", err)
		os.Exit(1)
	}

	// 3. 用户确认 (交互式 UI)
	var confirmed bool
	var finalMsg string = msg

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewText().
				Title("Generated Commit Message").
				Value(&finalMsg), // 允许用户直接在 Text Area 编辑
			huh.NewConfirm().
				Title("Do you want to commit with this message?").
				Value(&confirmed),
		),
	)

	err = form.Run()
	if err != nil {
		fmt.Println("Operation cancelled.")
		return
	}

	if confirmed {
		if err := gitClient.Commit(ctx, finalMsg); err != nil {
			fmt.Printf("Commit failed: %v\n", err)
		} else {
			fmt.Println("✅ Successfully committed!")
		}
	} else {
		fmt.Println("❌ Commit cancelled.")
	}
}
```

---

### 4. 项目实施与发布 (Implementation & Distribution)

作为高级架构师，除了代码，还要考虑交付。

#### 构建 (Makefile)
编写 `Makefile` 来规范常用命令：

```makefile
BINARY_NAME=aicm

build:
	go build -o bin/$(BINARY_NAME) ./cmd/aicm

run:
	go run ./cmd/aicm

install:
	go install ./cmd/aicm

clean:
	rm -rf bin/
```

#### 发布 (GoReleaser)
推荐使用 `GoReleaser` 自动发布到 GitHub Release 并生成 Homebrew Formula。这是 Go 工具发布的标准姿势。

配置 `.goreleaser.yaml`:
```yaml
project_name: aicm
builds:
  - env:
      - CGO_ENABLED=0
    goos:
      - linux
      - windows
      - darwin
```

---

### 5. 高级建议与下一步 (To Senior Level)

如果需要让这个工具更上一层楼，你需要考虑以下几点：

1.  **Token 限制管理**: `git diff` 如果非常大（比如改了 `package-lock.json` 或 `go.sum`），会消耗大量 Token 甚至报错。
    *   *方案*: 解析 diff，过滤掉 `.lock` 文件，或者只取 diff 的前 N 个字符，或者统计文件变更列表作为补充 Context。
2.  **多模型支持**:
    *   *方案*: 在 `internal/pkg/ai` 中实现 `DeepSeek`, `Claude`, `Ollama` (本地模型，保护隐私) 的适配器。
3.  **Prompt 优化**:
    *   *方案*: 允许用户自定义 Prompt 模板（通过配置文件 `~/.aicm.yaml`），适应不同团队的 Commit 规范。
4.  **Git Hook 集成**:
    *   *方案*: 生成 `prepare-commit-msg` 钩子脚本，这样用户直接执行 `git commit` 时也能触发 AI 建议。

### 我需要明确的事项（Feedback Loop）

在开始编写代码之前，我有几个问题需要向你确认，以便更精准地定制：

1.  **AI 供应商**: 你目前首选使用 OpenAI (GPT-4/3.5) 吗？还是有使用本地 LLM (如 Ollama) 的需求？（这对 API 调用的实现有影响）。
2.  **配置方式**: 你希望 API Key 是每次通过环境变量传递，还是工具内部实现一个 `aicm config set-key xxx` 的命令来保存到本地文件？
3.  **复杂性**: 你需要支持 `Conventional Commits` (feat, fix, docs) 这种严格格式，还是随意的自然语言即可？

如果不清楚，建议直接按照上述**OpenAI + 环境变量 + Conventional Commits** 的标准方案先行实施。这是目前最通用的做法。