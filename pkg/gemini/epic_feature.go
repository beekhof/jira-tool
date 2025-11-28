package gemini

import (
	"strings"
)

// IsEpic checks if a ticket is an Epic based on the issue type name
func IsEpic(issueTypeName string) bool {
	return strings.EqualFold(strings.TrimSpace(issueTypeName), "Epic")
}

// IsFeature checks if a ticket is a Feature based on the issue type name
func IsFeature(issueTypeName string) bool {
	return strings.EqualFold(strings.TrimSpace(issueTypeName), "Feature")
}

