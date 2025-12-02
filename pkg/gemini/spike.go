package gemini

import (
	"strings"
)

// IsSpike checks if a ticket is a spike based on the summary or ticket key
// Spikes are identified by a "SPIKE" prefix in the summary or key (case-insensitive)
func IsSpike(summary, ticketKey string) bool {
	// Check if summary starts with "SPIKE" (case-insensitive)
	if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(summary)), "SPIKE") {
		return true
	}

	// Check if ticket key contains "SPIKE" (case-insensitive)
	// This handles cases like "ENG-SPIKE-123" or "SPIKE-456"
	if strings.Contains(strings.ToUpper(ticketKey), "SPIKE") {
		return true
	}

	return false
}
