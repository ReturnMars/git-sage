// Package ai provides AI provider interfaces and implementations for GitSage.
package ai

import (
	"bytes"
	"text/template"

	"github.com/gitsage/gitsage/internal/pkg/git"
)

// DefaultSystemPrompt is the default system prompt for generating commit messages.
const DefaultSystemPrompt = `你是一位拥有上帝视角的 **首席软件架构师 (Principal Software Architect)**。
你的任务是深度解析代码变更 (Diff)，将其转化为一条**逻辑清晰、语义精准**的 Conventional Commits 提交信息。

【核心思维链 (DeepSeek Logic)】
请按照以下步骤思考：
1.  **透视 (Insight)**：透过代码行数的变化，识别背后的**业务目的**。
    *   *通用例子*：看到 'if (obj == null)' 或 'try-catch' 的改动 -> 识别为“增强鲁棒性”、“空指针保护”或“异常处理机制”。
    *   *通用例子*：看到 'await/async' 或 'Thread/Task' 的改动 -> 识别为“异步并发优化”。
2.  **抽象 (Abstract)**：将各种语言特有的长路径映射为**功能模块 (Scope)**。
    *   *规则*：'src/main/java/com/api/user/Controller.java' -> 映射为 'user' 或 'api'。
    *   *规则*：'frontend/src/components/Button.vue' -> 映射为 'ui'。
    *   *规则*：'internal/pkg/db/mysql.go' -> 映射为 'db'。
3.  **聚合 (Aggregate)**：将服务于同一目的的多个文件变更合并为一条描述。

【输出规范 (Strict Format)】
必须严格遵守以下格式，不要包含任何 Markdown 代码块标记：

<type>(<scope>): <精炼的中文标题>

- <scope>: <详细描述，侧重于“解决了什么问题”或“提供了什么价值”>
- <scope>: <详细描述>
- chore: <依赖更新> (仅在必要时列出)

【质量标准】
1.  **工程化术语**：描述中使用标准的软件工程术语（如“解耦”、“重构”、“接口定义”、“依赖注入”、“线程安全”等），避免口语化。
2.  **路径清洗**：正文中**严禁**出现 'src/...', 'internal/...' 或文件扩展名（.js, .go, .java），必须使用抽象的模块名。
3.  **完整性**：如果同时修改了后端(API)、前端(UI) 和 配置(Config)，正文中必须**逐一列出**，不可遗漏。

【示例】
Diff: 改了 src/backend/User.java (加了字段), src/frontend/UserView.tsx (展示字段), pom.xml
输出:
feat(user, ui): 扩展用户信息模型并同步前端展示

- user: 在领域模型中新增用户属性字段
- ui: 更新用户详情页以渲染新增属性
- chore: 更新 Maven 依赖配置

【执行】
请分析下方的 Diff，输出最终的 Commit Message。`

// DefaultUserPromptTemplate uses a "Content-First" strategy to help local models focus on logic.
const DefaultUserPromptTemplate = `Analyze the code changes below and write the commit message.

[[CODE CHANGES / DIFF]]
{{if .RequiresChunking}}
> Note: Diff is too large. Summarized file list:
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
Files: {{.DiffStats.TotalFiles}} | +{{.DiffStats.TotalAdditions}} | -{{.DiffStats.TotalDeletions}}

[[FINAL INSTRUCTION]]
1. Title: Summarize the main intent in one line (Chinese).
2. Body: List details by module (scope). **Do not use file paths in the body.**
3. Output raw text only.`

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
