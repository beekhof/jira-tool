package qa

import (
	"fmt"
	"strings"

	"github.com/beekhof/jira-tool/pkg/gemini"
	"github.com/beekhof/jira-tool/pkg/jira"
)

const (
	// Input method constants
	inputMethodReadline            = "readline"
	inputMethodEditor              = "editor"
	inputMethodReadlineWithPreview = "readline_with_preview"
)

// RunQnAFlow runs the interactive Q&A flow with Gemini
// It asks up to maxQuestions questions and then generates a final description
// If maxQuestions is 0 or negative, defaults to 4
// summaryOrKey is used to detect spikes (tickets with "SPIKE" prefix) and select the appropriate prompt template
// issueTypeName is the Jira issue type name (e.g., "Epic", "Feature", "Task")
// used to select the appropriate prompt template
// existingDescription is included in the context if provided (for improving existing descriptions)
// jiraClient and ticketKey are optional - if provided, child ticket summaries will be included in context
// epicLinkFieldID is optional - required for Epic tickets to fetch epic children
// answerInputMethod controls how answers are input: "readline", "editor", or
// "readline_with_preview" (default: "readline_with_preview")
//
// Users can reject poor questions by entering "reject" or an empty string.
// Rejected questions are skipped, a new question is generated, and the flow continues.
// Rejected questions are added to history as "Q: [question] - REJECTED" for context.
// Users can end the Q&A early by entering "skip" or "done".
// Users can type ":edit" or ":e" during readline input to switch to editor.
func RunQnAFlow(
	client gemini.GeminiClient, initialContext string, maxQuestions int,
	summaryOrKey, issueTypeName, existingDescription string,
	jiraClient jira.JiraClient, ticketKey, epicLinkFieldID, answerInputMethod string,
) (string, error) {
	answerInputMethod = validateInputMethod(answerInputMethod)
	enhancedContext := buildEnhancedContext(initialContext, existingDescription, jiraClient, ticketKey, epicLinkFieldID)
	maxQuestions = normalizeMaxQuestions(maxQuestions)

	history, err := runQuestionLoop(client, enhancedContext, summaryOrKey, issueTypeName, maxQuestions, answerInputMethod)
	if err != nil {
		return "", err
	}

	description, err := client.GenerateDescription(history, enhancedContext, summaryOrKey, issueTypeName)
	if err != nil {
		return "", fmt.Errorf("failed to generate description: %w", err)
	}

	return addDescriptionFooter(description), nil
}

func validateInputMethod(method string) string {
	if method == "" {
		return inputMethodReadlineWithPreview
	}
	if method != inputMethodReadline && method != inputMethodEditor && method != inputMethodReadlineWithPreview {
		return inputMethodReadlineWithPreview
	}
	return method
}

func buildEnhancedContext(
	initialContext, existingDescription string,
	jiraClient jira.JiraClient, ticketKey, epicLinkFieldID string,
) string {
	context := initialContext
	if existingDescription != "" {
		context = fmt.Sprintf(
			"%s\n\nExisting description: %s\n\nImprove or expand this description based on the following questions:",
			initialContext, existingDescription)
	}

	if jiraClient != nil && ticketKey != "" {
		childContext := buildChildTicketContext(jiraClient, ticketKey, epicLinkFieldID)
		if childContext != "" {
			context += childContext
		}
	}

	return context
}

func buildChildTicketContext(jiraClient jira.JiraClient, ticketKey, epicLinkFieldID string) string {
	childSummaries, err := jira.GetChildTickets(jiraClient, ticketKey, epicLinkFieldID)
	if err != nil || len(childSummaries) == 0 {
		return ""
	}

	childContext := "\n\nChild tickets:\n"
	for i, summary := range childSummaries {
		childContext += fmt.Sprintf("- %s\n", summary)
		if i >= 19 {
			remaining := len(childSummaries) - 20
			if remaining > 0 {
				childContext += fmt.Sprintf("... and %d more child tickets\n", remaining)
			}
			break
		}
	}
	return childContext
}

func normalizeMaxQuestions(maxQuestions int) int {
	if maxQuestions <= 0 {
		return 4
	}
	return maxQuestions
}

func runQuestionLoop(
	client gemini.GeminiClient, enhancedContext, summaryOrKey, issueTypeName string,
	maxQuestions int, answerInputMethod string,
) ([]string, error) {
	history := []string{}

	for i := 0; i < maxQuestions; i++ {
		question, err := client.GenerateQuestion(history, enhancedContext, summaryOrKey, issueTypeName)
		if err != nil {
			return nil, fmt.Errorf("failed to generate question: %w", err)
		}

		answer, shouldSkip, shouldDone, err := processQuestionAnswer(client, question, answerInputMethod)
		if err != nil {
			return nil, err
		}

		if shouldSkip {
			if shouldDone {
				break
			}
			history = append(history, fmt.Sprintf("Q: %s - REJECTED", question))
			i--
			continue
		}

		if shouldDone {
			break
		}

		history = append(history, fmt.Sprintf("Q: %s", question), fmt.Sprintf("A: %s", answer))
	}

	return history, nil
}

func processQuestionAnswer(
	_ gemini.GeminiClient, question, answerInputMethod string,
) (answer string, shouldSkip, shouldDone bool, err error) {
	prompt := fmt.Sprintf("Gemini asks: %s? > ", question)
	answer, err = ReadAnswerWithReadline(prompt, answerInputMethod)
	if err != nil {
		return "", false, false, fmt.Errorf("failed to read answer: %w", err)
	}

	answer = trimSpace(answer)

	answer, err = PreviewAndEditLoop(answer, answerInputMethod)
	if err != nil {
		return "", false, false, fmt.Errorf("failed to preview/edit answer: %w", err)
	}

	if answer == "" || strings.EqualFold(answer, "reject") {
		fmt.Println("Question rejected, generating a new one...")
		return "", false, false, nil
	}

	if answer == "skip" || answer == "done" {
		return "", true, true, nil
	}

	return answer, true, false, nil
}

func addDescriptionFooter(description string) string {
	footer := "\n\n---\n\n_This description was generated based on human answers to a " +
		"limited number of robot questions related to the summary._"
	return description + footer
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
