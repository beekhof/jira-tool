package parser

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// DecomposeTicket represents a ticket in a decomposition plan
type DecomposeTicket struct {
	Summary     string
	StoryPoints int
	Type        string
	IsExisting  bool
	Key         string // Only for existing tickets
}

// DecompositionPlan represents a parsed decomposition plan
type DecompositionPlan struct {
	NewTickets      []DecomposeTicket
	ExistingTickets []DecomposeTicket
}

// parseStoryPoints extracts story points from text like "(3 points)" or "(5 point)"
func parseStoryPoints(text string) (int, error) {
	// Match patterns like "(3 points)", "(5 point)", "(3 pts)", etc.
	re := regexp.MustCompile(`\((\d+)\s*(?:point|points|pts?)\)`)
	matches := re.FindStringSubmatch(text)
	if len(matches) < 2 {
		return 0, fmt.Errorf("no story points found in: %s", text)
	}

	points, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0, fmt.Errorf("invalid story points: %s", matches[1])
	}

	return points, nil
}

// isExistingTicket checks if a line indicates an existing ticket
func isExistingTicket(line string) bool {
	return strings.Contains(strings.ToUpper(line), "[EXISTING]") ||
		strings.Contains(line, "[x]") || strings.Contains(line, "[X]")
}

// ParseDecompositionPlan parses a decomposition plan from structured text
// Expected format:
// # DECOMPOSITION PLAN
//
// ## NEW TICKETS
// - [ ] Task summary (3 points)
// - [ ] Another task (5 points)
//
// ## EXISTING TICKETS (for reference)
// - [x] Existing task (5 points) [EXISTING]
func ParseDecompositionPlan(plan string) (*DecompositionPlan, error) {
	result := &DecompositionPlan{
		NewTickets:      []DecomposeTicket{},
		ExistingTickets: []DecomposeTicket{},
	}

	lines := strings.Split(plan, "\n")
	inNewTickets := false
	inExistingTickets := false

	newTicketsRegex := regexp.MustCompile(`^##\s*NEW\s*TICKETS`)
	existingTicketsRegex := regexp.MustCompile(`^##\s*EXISTING\s*TICKETS`)

	// Match task lines: "- [ ] Task summary (3 points)" or "- [x] Task (5 points) [EXISTING]"
	taskRegex := regexp.MustCompile(`^-\s*\[[ xX]\]\s*(.+)$`)

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Check for section headers
		if newTicketsRegex.MatchString(line) {
			inNewTickets = true
			inExistingTickets = false
			continue
		}
		if existingTicketsRegex.MatchString(line) {
			inNewTickets = false
			inExistingTickets = true
			continue
		}

		// Skip empty lines and other headers
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse task lines
		if matches := taskRegex.FindStringSubmatch(line); matches != nil {
			taskText := strings.TrimSpace(matches[1])
			existing := isExistingTicket(line)

			// Extract story points
			points, err := parseStoryPoints(taskText)
			if err != nil {
				// If no points found, default to 0 (will be set later or validated)
				points = 0
			}

			// Remove story points from summary
			summary := taskText
			summary = regexp.MustCompile(`\s*\([^)]*point[^)]*\)\s*`).ReplaceAllString(summary, "")
			summary = strings.TrimSpace(summary)
			summary = strings.TrimSuffix(summary, "[EXISTING]")
			summary = strings.TrimSpace(summary)

			ticket := DecomposeTicket{
				Summary:     summary,
				StoryPoints: points,
				IsExisting:  existing,
			}

			if existing {
				result.ExistingTickets = append(result.ExistingTickets, ticket)
			} else if inNewTickets {
				result.NewTickets = append(result.NewTickets, ticket)
			} else if inExistingTickets {
				result.ExistingTickets = append(result.ExistingTickets, ticket)
			} else {
				// If no section header found yet, assume new tickets
				result.NewTickets = append(result.NewTickets, ticket)
			}
		} else if strings.HasPrefix(line, "-") {
			// Allow tasks without checkbox format
			taskText := strings.TrimPrefix(line, "-")
			taskText = strings.TrimSpace(taskText)
			if taskText != "" {
				existing := isExistingTicket(taskText)

				points, err := parseStoryPoints(taskText)
				if err != nil {
					points = 0
				}

				summary := taskText
				summary = regexp.MustCompile(`\s*\([^)]*point[^)]*\)\s*`).ReplaceAllString(summary, "")
				summary = strings.TrimSpace(summary)
				summary = strings.TrimSuffix(summary, "[EXISTING]")
				summary = strings.TrimSpace(summary)

				ticket := DecomposeTicket{
					Summary:     summary,
					StoryPoints: points,
					IsExisting:  existing,
				}

				if existing {
					result.ExistingTickets = append(result.ExistingTickets, ticket)
				} else {
					result.NewTickets = append(result.NewTickets, ticket)
				}
			}
		}
	}

	return result, nil
}
