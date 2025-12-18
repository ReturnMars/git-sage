// Package ai provides AI provider interfaces and implementations for GitSage.
package ai

import (
	"bytes"
	"text/template"

	"github.com/gitsage/gitsage/internal/pkg/git"
)

// DefaultSystemPrompt is the default system prompt for generating commit messages.
const DefaultSystemPrompt = `你是 Git commit message 专家。用中文生成 Conventional Commits 格式的提交信息。

格式: <type>(<scope>): <中文描述>

type 必须是: feat|fix|docs|style|refactor|test|chore|perf|ci|build|revert
scope 可选，用英文小写表示模块名（如 api, config, ui）
描述用中文，简洁准确，不超过50字，不加句号

示例:
feat(auth): 添加用户登录功能
fix(api): 修复空指针异常
refactor(config): 重构配置加载逻辑
docs: 更新 README 文档

如果改动较大，可加 body 说明:
feat(core): 实现消息队列

支持异步消息处理，提升系统吞吐量

只输出 commit message，不要解释。`

// DefaultUserPromptTemplate is the default user prompt template.
const DefaultUserPromptTemplate = `Generate a commit message for these changes:

{{if .DiffStats}}
Files changed: {{.DiffStats.TotalFiles}}
Additions: {{.DiffStats.TotalAdditions}}
Deletions: {{.DiffStats.TotalDeletions}}
{{end}}

{{if .RequiresChunking}}
Summary of changes:
{{range .Chunks}}
- {{.FilePath}}: {{.ChangeType}} (+{{.Additions}} -{{.Deletions}})
{{end}}
{{else}}
Diff:
{{range .Chunks}}
{{.Content}}
{{end}}
{{end}}

{{if .PreviousAttempt}}
Previous attempt (user requested regeneration):
{{.PreviousAttempt}}
{{end}}`

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
