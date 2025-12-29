package prompt

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

	"github.com/gitsage/gitsage/internal/core/domain"
)

// DefaultSystemPrompt ... (Copied from internal/pkg/ai/prompt.go)
const DefaultSystemPrompt = `你是一位拥有上帝视角的 **首席软件架构师 (Principal Software Architect)**。
你的任务是深度解析代码变更 (Diff)，将其转化为一条**逻辑清晰、语义精准**的 Conventional Commits 提交信息。

【核心思维链 (DeepSeek Logic)】
请按照以下步骤思考：
1.  **透视 (Insight)**：透过代码行数的变化，识别背后的**业务目的**。
2.  **抽象 (Abstract)**：将各种语言特有的长路径映射为**功能模块 (Scope)**。
3.  **聚合 (Aggregate)**：将服务于同一目的的多个文件变更合并为一条描述。

【输出规范 (Strict Format)】
必须严格遵守以下格式，不要包含任何 Markdown 代码块标记：

<type>(<scope>): <精炼的中文标题>

- <scope>: <详细描述，侧重于“解决了什么问题”或“提供了什么价值”>
- <scope>: <详细描述>
- chore: <依赖更新> (仅在必要时列出)

【质量标准】
1.  **工程化术语**：描述中使用标准的软件工程术语。
2.  **路径清洗**：正文中**严禁**出现 'src/...', 'internal/...' 或文件扩展名。
3.  **完整性**：如果同时修改了后端(API)、前端(UI) 和 配置(Config)，正文中必须**逐一列出**。

【执行】
请分析下方的 Diff，输出最终的 Commit Message。`

// DefaultUserPromptTemplate ...
const DefaultUserPromptTemplate = `Analyze the code changes below and write the commit message.

[[CODE CHANGES / DIFF]]
{{range .Files}}
--- File: {{.FilePath}} ({{.ChangeType}}) ---
{{.Content}}

{{end}}

[[STATS]]
Files: {{.Stats.FilesChanged}} | +{{.Stats.Insertions}} | -{{.Stats.Deletions}}

[[FINAL INSTRUCTION]]
1. Title: Summarize the main intent in one line (Chinese).
2. Body: List details by module (scope). **Do not use file paths in the body.**
3. Output raw text only.`

type Builder struct {
	systemPrompt string
	userTmpl     *template.Template
	rawUserTmpl  string
	compress     bool
}

// Option defines a functional option for configuring the Builder.
type Option func(*Builder)

// WithSystemPrompt sets a custom system prompt.
func WithSystemPrompt(prompt string) Option {
	return func(b *Builder) {
		if prompt != "" {
			b.systemPrompt = prompt
		}
	}
}

// WithUserTemplate sets a custom user prompt template.
func WithUserTemplate(tmpl string) Option {
	return func(b *Builder) {
		if tmpl != "" {
			b.rawUserTmpl = tmpl
		}
	}
}

// WithCompression enables or disables prompt compression.
func WithCompression(compress bool) Option {
	return func(b *Builder) {
		b.compress = compress
	}
}

// NewBuilder creates a new Builder with optional configurations.
func NewBuilder(opts ...Option) (*Builder, error) {
	b := &Builder{
		systemPrompt: DefaultSystemPrompt,
		rawUserTmpl:  DefaultUserPromptTemplate,
		compress:     false, // Default off
	}

	for _, opt := range opts {
		opt(b)
	}

	// Make sure rawUserTmpl is used to create userTmpl
	tmpl, err := template.New("user").Parse(b.rawUserTmpl)
	if err != nil {
		return nil, fmt.Errorf("failed to parse user template: %w", err)
	}
	b.userTmpl = tmpl

	return b, nil
}

// SetSystemPrompt updates the system prompt dynamically.
func (b *Builder) SetSystemPrompt(prompt string) {
	if prompt != "" {
		b.systemPrompt = prompt
	}
}

func (b *Builder) GetSystemPrompt() string {
	if b.compress {
		return compressString(b.systemPrompt)
	}
	return b.systemPrompt
}

func (b *Builder) BuildUserPrompt(diff *domain.Diff, hint string) (string, error) {
	// We can extend this to include hint if needed
	var buf bytes.Buffer
	if err := b.userTmpl.Execute(&buf, diff); err != nil {
		return "", err
	}

	result := buf.String()
	if hint != "" {
		result = fmt.Sprintf("Hint: %s\n\n%s", hint, result)
	}

	if b.compress {
		result = compressString(result)
	}
	return result, nil
}

// BuildSummaryPrompt is used for the first phase of two-phase generation (for chunks).
func (b *Builder) BuildSummaryPrompt(chunk *domain.Diff) string {
	// Simplified logic for summary
	var sb strings.Builder
	sb.WriteString("Summarize the following code changes concisely:\n")
	for _, f := range chunk.Files {
		sb.WriteString(fmt.Sprintf("\n--- %s ---\n%s\n", f.FilePath, f.Content))
	}

	res := sb.String()
	if b.compress {
		return compressString(res)
	}
	return res
}

// compressString removes consecutive newlines and trims whitespace from lines.
// NOTE: This might affect code formatting in diffs, so use with caution.
// A improved version would strictly only compress template parts, not code content.
// For now, checks for >2 newlines and reduces to 2.
func compressString(s string) string {
	// Replace 3+ newlines with 2
	for strings.Contains(s, "\n\n\n") {
		s = strings.ReplaceAll(s, "\n\n\n", "\n\n")
	}
	return strings.TrimSpace(s)
}
