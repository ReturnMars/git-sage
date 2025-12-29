package domain

// ChangeType represents the type of change in a file.
type ChangeType string

const (
	ChangeTypeModified ChangeType = "MODIFIED"
	ChangeTypeAdded    ChangeType = "ADDED"
	ChangeTypeDeleted  ChangeType = "DELETED"
	ChangeTypeRenamed  ChangeType = "RENAMED"
	ChangeTypeCopied   ChangeType = "COPIED"
	ChangeTypeUnknown  ChangeType = "UNKNOWN"
)

// DiffFile represents the diff of a single file.
type DiffFile struct {
	FilePath    string
	OldFilePath string // For renames/copies
	ChangeType  ChangeType
	Content     string // The actual diff content
	Additions   int
	Deletions   int
}

// DiffStats holds global statistics about the diff.
type DiffStats struct {
	FilesChanged int
	Insertions   int
	Deletions    int
}

// Diff represents a collection of file changes.
type Diff struct {
	Files []DiffFile
	Stats DiffStats
}
