// Package pathcheck provides PATH detection and modification functionality for GitSage.
package pathcheck

import "fmt"

// UnixExportTemplateConst is the template for Unix shell export statements (bash/zsh).
const UnixExportTemplateConst = `
# Added by GitSage
export PATH="$PATH:%s"
`

// FishExportTemplateConst is the template for Fish shell path addition.
const FishExportTemplateConst = `
# Added by GitSage
set -gx PATH $PATH %s
`

// GenerateExportStatementForShell generates an export statement for a specific shell type.
// This function is platform-independent and can be used for testing.
func GenerateExportStatementForShell(shellType ShellType, execDir string) string {
	if shellType == ShellFish {
		return fmt.Sprintf(FishExportTemplateConst, execDir)
	}
	return fmt.Sprintf(UnixExportTemplateConst, execDir)
}
