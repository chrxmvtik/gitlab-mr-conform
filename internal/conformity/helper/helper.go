package helper

import (
	"regexp"
	"strings"
)

// HeaderRegex is the regular expression used for Conventional Commits 1.0.0.
var (
	HeaderRegex = regexp.MustCompile(`^(\w*)(\(([^)]+)\))?(!)?:\s{1}(.*)($|\n{2})`)
	JiraRegex   = regexp.MustCompile(`.*\s\[?([A-Z]+)-[1-9]\d*\]?.*`)
)

func ParseHeader(msg string) []string {
	header := strings.Split(strings.TrimPrefix(msg, "\n"), "\n")[0]
	return HeaderRegex.FindStringSubmatch(header)
}

func Contains(slice []string, value string) bool {
	for _, elem := range slice {
		if elem == value {
			return true
		}
	}
	return false
}

// Helper function to truncate commit messages for display
func TruncateCommitMessage(msg string, maxLen int) string {
	msg = strings.TrimSpace(msg)
	if len(msg) <= maxLen {
		return msg
	}
	return msg[:maxLen] + "..."
}
