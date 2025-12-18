// Package ai provides AI provider interfaces and implementations for GitSage.
package ai

import (
	"bytes"
	"text/template"

	"github.com/gitsage/gitsage/internal/pkg/git"
)

// DefaultSystemPrompt is the default system prompt for generating commit messages.
const DefaultSystemPrompt = `You are an expert at writing semantic git commit messages.

Format Requirements:
- Use Conventional Commits format: <type>(<scope>): <subject>
- Types: feat, fix, docs, style, refactor, test, chore, perf, ci, build, revert
- Subject: imperative mood, no period, max 72 characters
- Body: optional, explain what and why (not how)
- Footer: optional, reference issues or breaking changes

Rules:
1. Be concise and specific
2. Focus on the "what" and "why", not the "how"
3. Use present tense ("add" not "added")
4. First line should be standalone summary
5. Separate subject from body with blank line

Output only the commit message, no explanations.`

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
