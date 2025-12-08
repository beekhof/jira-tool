package gemini

import (
	"fmt"
	"strings"

	"github.com/beekhof/jira-tool/pkg/config"
	"github.com/beekhof/jira-tool/pkg/jira"
)

const defaultDecomposePromptTemplate = `You are helping to decompose a Jira ticket into smaller child tickets.

Parent Ticket: {{parent_summary}}
Description: {{parent_description}}

Existing Child Tickets:
{{existing_children}}

Requirements:
- Create child tickets with type: {{child_type}}
- Each child ticket must have story points ≤ {{max_points}}
- Avoid duplicating existing child tickets
- Do not consider the parent's current story points
- Break down the work into logical, independent pieces

Output a decomposition plan in the following format:

# DECOMPOSITION PLAN

## NEW TICKETS
- [ ] Ticket summary (story points)
- [ ] Another ticket summary (story points)

## EXISTING TICKETS (for reference)
- [x] Existing ticket (points) [EXISTING]

Each new ticket should:
- Have a clear, concise summary
- Have story points that are ≤ {{max_points}}
- Be independent and completable on its own
- Not duplicate any existing child tickets`

// formatExistingChildren formats existing children as a readable list
func formatExistingChildren(children []jira.ChildTicketInfo) string {
	if len(children) == 0 {
		return "None"
	}

	var lines []string
	for _, child := range children {
		lines = append(lines, fmt.Sprintf("- %s (%d points) - %s [EXISTING]", child.Summary, child.StoryPoints, child.Type))
	}
	return strings.Join(lines, "\n")
}

// buildDecomposeContext builds the context string for decomposition
func buildDecomposeContext(
	parentSummary, parentDescription, existingChildrenText, childType string,
	maxPoints int,
) string {
	context := fmt.Sprintf("Parent Ticket: %s\n\n", parentSummary)
	if parentDescription != "" {
		context += fmt.Sprintf("Description:\n%s\n\n", parentDescription)
	}
	context += fmt.Sprintf("Existing Child Tickets:\n%s\n\n", existingChildrenText)
	context += "Requirements:\n"
	context += fmt.Sprintf("- Child ticket type: %s\n", childType)
	context += fmt.Sprintf("- Maximum story points per ticket: %d\n", maxPoints)
	context += "- Do not duplicate existing child tickets\n"
	context += "- Do not consider parent's current story points\n"
	return context
}

// GenerateDecompositionPlan generates a decomposition plan using Gemini AI
func GenerateDecompositionPlan(
	client GeminiClient,
	cfg *config.Config,
	parentSummary, parentDescription string,
	existingChildren []jira.ChildTicketInfo,
	childType string,
	maxPoints int,
) (string, error) {
	// Format existing children
	existingChildrenText := formatExistingChildren(existingChildren)

	// Get prompt template
	promptTemplate := cfg.DecomposePromptTemplate
	if promptTemplate == "" {
		promptTemplate = defaultDecomposePromptTemplate
	}

	// Replace placeholders
	prompt := promptTemplate
	prompt = strings.ReplaceAll(prompt, "{{parent_summary}}", parentSummary)
	prompt = strings.ReplaceAll(prompt, "{{parent_description}}", parentDescription)
	prompt = strings.ReplaceAll(prompt, "{{existing_children}}", existingChildrenText)
	prompt = strings.ReplaceAll(prompt, "{{child_type}}", childType)
	prompt = strings.ReplaceAll(prompt, "{{max_points}}", fmt.Sprintf("%d", maxPoints))

	// Build context with our prompt embedded
	context := buildDecomposeContext(parentSummary, parentDescription, existingChildrenText, childType, maxPoints)

	// Embed the prompt instructions in the context
	// This way the AI gets both the instructions and the data
	fullContext := prompt + "\n\n" + context

	// Use GenerateDescription with our enhanced context
	// The default template will add its own wrapper, but our context contains all instructions
	result, err := client.GenerateDescription([]string{}, fullContext, parentSummary, childType)
	if err != nil {
		return "", fmt.Errorf("failed to generate decomposition plan: %w", err)
	}

	return result, nil
}
