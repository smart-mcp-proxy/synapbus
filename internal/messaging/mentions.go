package messaging

import (
	"regexp"
	"strings"
)

// mentionPattern matches @agentname patterns where agent names contain
// alphanumeric characters, hyphens, and underscores (max 64 chars).
// It uses a negative lookbehind-style approach: we exclude matches that
// look like email addresses (preceded by alphanumeric chars).
var mentionPattern = regexp.MustCompile(`(?:^|[^a-zA-Z0-9.@])@([a-zA-Z0-9][a-zA-Z0-9_-]{0,63})`)

// ParseMentions extracts unique @agentname mentions from message body text.
// It returns a deduplicated list of agent names (without the @ prefix).
//
// Rules:
//   - Agent names start with an alphanumeric character
//   - Agent names can contain: alphanumeric, hyphens, underscores (max 64 chars)
//   - Email addresses (e.g., user@example.com) are NOT matched
//   - Duplicate mentions are deduplicated
//   - @ at end of string or @@ are ignored
func ParseMentions(body string) []string {
	matches := mentionPattern.FindAllStringSubmatch(body, -1)
	if len(matches) == 0 {
		return nil
	}

	seen := make(map[string]bool, len(matches))
	var result []string
	for _, m := range matches {
		name := strings.ToLower(m[1])
		if !seen[name] {
			seen[name] = true
			result = append(result, name)
		}
	}
	return result
}
