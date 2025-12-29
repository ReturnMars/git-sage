package domain

// CommitType represents the type of a commit (e.g., feat, fix).
type CommitType string

const (
	CommitTypeFeat     CommitType = "feat"
	CommitTypeFix      CommitType = "fix"
	CommitTypeDocs     CommitType = "docs"
	CommitTypeStyle    CommitType = "style"
	CommitTypeRefactor CommitType = "refactor"
	CommitTypePerf     CommitType = "perf"
	CommitTypeTest     CommitType = "test"
	CommitTypeBuild    CommitType = "build"
	CommitTypeCI       CommitType = "ci"
	CommitTypeChore    CommitType = "chore"
	CommitTypeRevert   CommitType = "revert"
)

// CommitMessage represents a structured commit message.
type CommitMessage struct {
	Type    CommitType
	Scope   string
	Subject string
	Body    string
	Footer  string
	Raw     string // The raw message string for fallback/records
}

// ValidationResult represents the result of validating a commit message.
type ValidationResult struct {
	IsValid  bool
	Issues   []string
	Warnings []string
}
