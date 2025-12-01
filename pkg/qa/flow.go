package qa

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/beekhof/jira-tool/pkg/gemini"
	"github.com/beekhof/jira-tool/pkg/jira"
)

// RunQnAFlow runs the interactive Q&A flow with Gemini
// It asks up to maxQuestions questions and then generates a final description
// If maxQuestions is 0 or negative, defaults to 4
// summaryOrKey is used to detect spikes (tickets with "SPIKE" prefix) and select the appropriate prompt template
// issueTypeName is the Jira issue type name (e.g., "Epic", "Feature", "Task") used to select the appropriate prompt template
// existingDescription is included in the context if provided (for improving existing descriptions)
// jiraClient and ticketKey are optional - if provided, child ticket summaries will be included in context
// epicLinkFieldID is optional - required for Epic tickets to fetch epic children
//
// Users can reject poor questions by entering "reject" or an empty string.
// Rejected questions are skipped, a new question is generated, and the flow continues.
// Rejected questions are added to history as "Q: [question] - REJECTED" for context.
// Users can end the Q&A early by entering "skip" or "done".
func RunQnAFlow(client gemini.GeminiClient, initialContext string, maxQuestions int, summaryOrKey string, issueTypeName string, existingDescription string, jiraClient jira.JiraClient, ticketKey string, epicLinkFieldID string) (string, error) {
	history := []string{}
	reader := bufio.NewReader(os.Stdin)

	// Include existing description in context if provided
	enhancedContext := initialContext
	if existingDescription != "" {
		enhancedContext = fmt.Sprintf("%s\n\nExisting description: %s\n\nImprove or expand this description based on the following questions:", initialContext, existingDescription)
	}

	// Include child ticket summaries in context if available
	if jiraClient != nil && ticketKey != "" {
		childSummaries, err := jira.GetChildTickets(jiraClient, ticketKey, epicLinkFieldID)
		if err == nil && len(childSummaries) > 0 {
			childContext := "\n\nChild tickets:\n"
			for i, summary := range childSummaries {
				childContext += fmt.Sprintf("- %s\n", summary)
				// Limit to first 20 child tickets to avoid overwhelming context
				if i >= 19 {
					remaining := len(childSummaries) - 20
					if remaining > 0 {
						childContext += fmt.Sprintf("... and %d more child tickets\n", remaining)
					}
					break
				}
			}
			enhancedContext += childContext
		}
	}

	// Default to 4 if not specified
	if maxQuestions <= 0 {
		maxQuestions = 4
	}

	// Loop up to maxQuestions times
	for i := 0; i < maxQuestions; i++ {
		// Generate a question
		question, err := client.GenerateQuestion(history, enhancedContext, summaryOrKey, issueTypeName)
		if err != nil {
			return "", fmt.Errorf("failed to generate question: %w", err)
		}

		// Print the question and prompt for answer
		fmt.Printf("Gemini asks: %s? > ", question)

		// Read the user's answer
		answer, err := reader.ReadString('\n')
		if err != nil {
			return "", fmt.Errorf("failed to read answer: %w", err)
		}

		// Trim whitespace
		answer = trimSpace(answer)
		question = trimSpace(question)

		// Handle rejection (empty string or "reject")
		if answer == "" || strings.EqualFold(answer, "reject") {
			fmt.Println("Question rejected, generating a new one...")
			history = append(history, fmt.Sprintf("Q: %s - REJECTED", question))
			i-- // Decrement to retry without counting toward maxQuestions
			continue
		}

		// Handle skip/done (end Q&A loop)
		if answer == "skip" || answer == "done" {
			break
		}

		// Normal answer - add to history
		history = append(history, fmt.Sprintf("Q: %s", question))
		history = append(history, fmt.Sprintf("A: %s", answer))
	}

	// Generate the final description
	description, err := client.GenerateDescription(history, enhancedContext, summaryOrKey, issueTypeName)
	if err != nil {
		return "", fmt.Errorf("failed to generate description: %w", err)
	}

	// Add footer to the description
	footer := "\n\n---\n\n_This description was generated based on human answers to a limited number of robot questions related to the summary._"
	description = description + footer

	return description, nil
}

// trimSpace removes leading and trailing whitespace
func trimSpace(s string) string {
	// Remove leading and trailing whitespace, including newlines
	start := 0
	end := len(s)

	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}

	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}

	return s[start:end]
}
