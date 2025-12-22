// Package pathcheck provides PATH detection and modification functionality for GitSage.
package pathcheck

import (
	"fmt"
)

// PathErrorCode represents the category of a PATH-related error.
type PathErrorCode int

const (
	// ErrGetExecutablePath indicates failure to get the executable path.
	ErrGetExecutablePath PathErrorCode = iota
	// ErrReadPATH indicates failure to read the PATH environment variable.
	ErrReadPATH
	// ErrDetectShell indicates failure to detect the shell type.
	ErrDetectShell
	// ErrModifyProfile indicates failure to modify the shell profile file.
	ErrModifyProfile
	// ErrWindowsSetx indicates failure of the Windows setx command.
	ErrWindowsSetx
	// ErrPermissionDenied indicates insufficient permissions for the operation.
	ErrPermissionDenied
	// ErrCreateDirectory indicates failure to create a directory.
	ErrCreateDirectory
	// ErrGetHomeDir indicates failure to get the user's home directory.
	ErrGetHomeDir
	// ErrPathTooLong indicates the PATH value exceeds system limits.
	ErrPathTooLong
)

// String returns a human-readable name for the error code.
func (c PathErrorCode) String() string {
	switch c {
	case ErrGetExecutablePath:
		return "GetExecutablePath"
	case ErrReadPATH:
		return "ReadPATH"
	case ErrDetectShell:
		return "DetectShell"
	case ErrModifyProfile:
		return "ModifyProfile"
	case ErrWindowsSetx:
		return "WindowsSetx"
	case ErrPermissionDenied:
		return "PermissionDenied"
	case ErrCreateDirectory:
		return "CreateDirectory"
	case ErrGetHomeDir:
		return "GetHomeDir"
	case ErrPathTooLong:
		return "PathTooLong"
	default:
		return "Unknown"
	}
}

// PathCheckError represents a PATH check related error with context.
type PathCheckError struct {
	Code    PathErrorCode
	Message string
	Cause   error
}

// Error implements the error interface.
func (e *PathCheckError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

// Unwrap returns the underlying error.
func (e *PathCheckError) Unwrap() error {
	return e.Cause
}

// NewPathCheckError creates a new PathCheckError.
func NewPathCheckError(code PathErrorCode, message string) *PathCheckError {
	return &PathCheckError{
		Code:    code,
		Message: message,
	}
}

// WrapPathCheckError wraps an error with a PathCheckError.
func WrapPathCheckError(err error, code PathErrorCode, message string) *PathCheckError {
	return &PathCheckError{
		Code:    code,
		Message: message,
		Cause:   err,
	}
}

// Common error constructors

// NewGetExecutablePathError creates an error for failing to get executable path.
func NewGetExecutablePathError(err error) *PathCheckError {
	return &PathCheckError{
		Code:    ErrGetExecutablePath,
		Message: "failed to get executable path",
		Cause:   err,
	}
}

// NewReadPATHError creates an error for failing to read PATH.
func NewReadPATHError(err error) *PathCheckError {
	return &PathCheckError{
		Code:    ErrReadPATH,
		Message: "failed to read PATH environment variable",
		Cause:   err,
	}
}

// NewDetectShellError creates an error for failing to detect shell.
func NewDetectShellError() *PathCheckError {
	return &PathCheckError{
		Code:    ErrDetectShell,
		Message: "failed to detect shell type",
	}
}

// NewModifyProfileError creates an error for failing to modify shell profile.
func NewModifyProfileError(profilePath string, err error) *PathCheckError {
	return &PathCheckError{
		Code:    ErrModifyProfile,
		Message: fmt.Sprintf("failed to modify shell profile: %s", profilePath),
		Cause:   err,
	}
}

// NewWindowsSetxError creates an error for Windows setx command failure.
func NewWindowsSetxError(err error, output string) *PathCheckError {
	msg := "setx command failed"
	if output != "" {
		msg = fmt.Sprintf("setx command failed: %s", output)
	}
	return &PathCheckError{
		Code:    ErrWindowsSetx,
		Message: msg,
		Cause:   err,
	}
}

// NewPermissionDeniedError creates an error for permission denied.
func NewPermissionDeniedError(path string, err error) *PathCheckError {
	return &PathCheckError{
		Code:    ErrPermissionDenied,
		Message: fmt.Sprintf("permission denied: %s", path),
		Cause:   err,
	}
}

// NewCreateDirectoryError creates an error for failing to create directory.
func NewCreateDirectoryError(path string, err error) *PathCheckError {
	return &PathCheckError{
		Code:    ErrCreateDirectory,
		Message: fmt.Sprintf("failed to create directory: %s", path),
		Cause:   err,
	}
}

// NewGetHomeDirError creates an error for failing to get home directory.
func NewGetHomeDirError(err error) *PathCheckError {
	return &PathCheckError{
		Code:    ErrGetHomeDir,
		Message: "failed to get home directory",
		Cause:   err,
	}
}

// NewPathTooLongError creates an error for PATH exceeding system limits.
func NewPathTooLongError(limit int) *PathCheckError {
	return &PathCheckError{
		Code:    ErrPathTooLong,
		Message: fmt.Sprintf("PATH value exceeds system limit of %d characters", limit),
	}
}

// IsPathCheckError checks if an error is a PathCheckError.
func IsPathCheckError(err error) bool {
	_, ok := err.(*PathCheckError)
	return ok
}

// GetPathCheckError extracts a PathCheckError from an error.
func GetPathCheckError(err error) *PathCheckError {
	if pce, ok := err.(*PathCheckError); ok {
		return pce
	}
	return nil
}
