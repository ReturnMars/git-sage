// Package ai provides AI provider interfaces and implementations for GitSage.
package ai

import (
	"bytes"
	"text/template"

	"github.com/gitsage/gitsage/internal/pkg/git"
)

// DefaultSystemPrompt is the default system prompt for generating commit messages.
const DefaultSystemPrompt = `你是一位拥有极致洞察力的资深代码审查专家 (Senior Code Reviewer)。
你的任务是阅读代码变更 (Diff)，生成符合 Conventional Commits 规范的 **中文** 提交信息。

【核心目标】
将分散的代码变更，重组为一条**逻辑清晰、覆盖全面、不带机器味**的 Commit Message。

【三大铁律 (Critical Rules)】
1.  **完整性 (Completeness)**：
    *   必须覆盖所有 **独立的业务功能变更**。
    *   如果 Diff 中同时包含了 AI 逻辑优化、UI 进度条新增、App 并发控制，**必须在正文列表中逐一列出**，严禁因为“偷懒”而把它们合并或丢弃。
2.  **抽象化 (Abstraction)**：
    *   **严禁**在输出中包含具体的文件路径（如 internal/pkg/ui/manager.go）。
    *   必须将路径映射为简短的 **Scope（模块名）**（如 ui, app, git, ai）。
3.  **单一性 (Single Subject)**：
    *   整个输出只能有一个标题行。

【输出格式规范】
<type>(<scope>): <高度概括的中文标题>

- <scope>: <详细描述1>
- <scope>: <详细描述2>
- <scope>: <详细描述3>
- chore: <依赖更新或琐事 (如有)>

【思维链 (Step-by-Step Guide)】
1.  **扫描**: 快速浏览所有文件名，识别出涉及的模块（例如：改了 ui/manager.go -> 涉及 UI；改了 deepseek.go -> 涉及 AI）。
2.  **清洗**: 脑内过滤掉 internal/pkg/... 等冗余路径。
3.  **撰写**:
    - 标题：用一句话概括最重要的改动。
    - 正文：按模块分组，列出所有实质性改动。**确保 UI、App、Git 等不同模块的改动都有独立的条目。**

【示例对比】
❌ 错误 (漏细节或带路径):
feat(ai): 更新提示词
- internal/pkg/ui/manager.go: 添加进度条  <-- 错误：带路径
(这里漏掉了 App 模块的改动)             <-- 错误：漏项

✅ 正确 (完美归纳):
feat(ai, ui): 优化提示词生成并新增进度条交互

- ai: 支持自定义提示词并允许空 DiffChunks
- ui: 新增进度条显示与操作确认功能
- app: 优化并发控制逻辑
- chore: 更新 go.mod 依赖

【执行指令】
只输出最终的 Commit Message，不包含任何解释或 Markdown 标记。`

// DefaultUserPromptTemplate uses a "Content-First" strategy to help local models focus on logic.
const DefaultUserPromptTemplate = `Analyze the following code changes (Diff) and write the commit message.

[[CHANGES]]
{{if .RequiresChunking}}
> Note: The diff is too large to show fully. Below is a summary of changed files.
> Instruction: Infer the intent based on file names and change types. Group them logically.

{{range .Chunks}}
- {{.FilePath}} ({{.ChangeType}})
{{end}}
{{else}}
{{range .Chunks}}
--- File: {{.FilePath}} ---
{{.Content}}

{{end}}
{{end}}

[[STATS]]
Files: {{.DiffStats.TotalFiles}} | Insertions: {{.DiffStats.TotalAdditions}} | Deletions: {{.DiffStats.TotalDeletions}}

{{if .PreviousAttempt}}
[[REVISION REQUEST]]
The previous output was not satisfactory. Please improve based on this feedback:
{{.PreviousAttempt}}
{{end}}

[[REMINDER]]
- Focus on the *business logic* (Why was this changed?).
- Merge related changes into one scope.
- Ignore dependency config files if code logic is modified.`

// PromptTemplate handles prompt generation for AI providers.
type PromptTemplate struct {
	SystemPrompt string
	UserPrompt   string
	tmpl         *template.Template
}

// PromptData contains the data used to render the user prompt template.
type PromptData struct {
	DiffStats        *git.DiffStats
	Chunks           []git.DiffChunk
	RequiresChunking bool
	PreviousAttempt  string
	CustomPrompt     string
}

// NewPromptTemplate creates a new PromptTemplate with default prompts.
func NewPromptTemplate() *PromptTemplate {
	return &PromptTemplate{
		SystemPrompt: DefaultSystemPrompt,
		UserPrompt:   DefaultUserPromptTemplate,
	}
}

// NewPromptTemplateWithCustom creates a new PromptTemplate with custom prompts.
// If systemPrompt or userPrompt is empty, the default is used.
func NewPromptTemplateWithCustom(systemPrompt, userPrompt string) *PromptTemplate {
	pt := &PromptTemplate{
		SystemPrompt: DefaultSystemPrompt,
		UserPrompt:   DefaultUserPromptTemplate,
	}

	if systemPrompt != "" {
		pt.SystemPrompt = systemPrompt
	}
	if userPrompt != "" {
		pt.UserPrompt = userPrompt
	}

	return pt
}

// RenderUserPrompt renders the user prompt template with the given data.
func (pt *PromptTemplate) RenderUserPrompt(data *PromptData) (string, error) {
	// If custom prompt is provided, use it directly
	if data.CustomPrompt != "" {
		return data.CustomPrompt, nil
	}

	// Parse the template if not already parsed
	if pt.tmpl == nil {
		tmpl, err := template.New("userPrompt").Parse(pt.UserPrompt)
		if err != nil {
			return "", err
		}
		pt.tmpl = tmpl
	}

	var buf bytes.Buffer
	if err := pt.tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// GetSystemPrompt returns the system prompt.
func (pt *PromptTemplate) GetSystemPrompt() string {
	return pt.SystemPrompt
}

// BuildPromptData creates PromptData from a GenerateRequest.
func BuildPromptData(req *GenerateRequest, requiresChunking bool) *PromptData {
	return &PromptData{
		DiffStats:        req.DiffStats,
		Chunks:           req.DiffChunks,
		RequiresChunking: requiresChunking,
		PreviousAttempt:  req.PreviousAttempt,
		CustomPrompt:     req.CustomPrompt,
	}
}
