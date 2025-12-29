package generator

import (
	"regexp"
	"strings"
)

// cleanResponse attempts to extract the actual commit message from AI response.
func cleanResponse(raw string) string {
	raw = strings.TrimSpace(raw)

	// 1. Try to extract from markdown code blocks
	// Matches ``` or ```git or ```text ... ```
	// Regex: ```(?:\w+)?\n([\s\S]*?)\n```
	reCodeBlock := regexp.MustCompile("(?s)```(?:\\w+)?\\s*\\n([\\s\\S]*?)\\n```")
	matches := reCodeBlock.FindStringSubmatch(raw)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}

	// 2. Remove common prefixes if no code block found
	// "Here is the commit message:"
	// "Subject:" (sometimes AI adds explicit headers)
	lower := strings.ToLower(raw)
	if strings.HasPrefix(lower, "here is") {
		// Try to find the start of the message (usually looking for a colon)
		parts := strings.SplitN(raw, ":", 2)
		if len(parts) > 1 {
			return strings.TrimSpace(parts[1])
		}
	}

	return raw
}
