package qa

import (
	"bufio"
	"fmt"
	"os"

	"go-jira-helper/pkg/gemini"
)

// RunQnAFlow runs the interactive Q&A flow with Gemini
// It asks up to maxQuestions questions and then generates a final description
// If maxQuestions is 0 or negative, defaults to 4
// summaryOrKey is used to detect spikes (tickets with "SPIKE" prefix) and select the appropriate prompt template
func RunQnAFlow(client gemini.GeminiClient, initialContext string, maxQuestions int, summaryOrKey string) (string, error) {
	history := []string{}
	reader := bufio.NewReader(os.Stdin)

	// Default to 4 if not specified
	if maxQuestions <= 0 {
		maxQuestions = 4
	}

	// Loop up to maxQuestions times
	for i := 0; i < maxQuestions; i++ {
		// Generate a question
		question, err := client.GenerateQuestion(history, initialContext, summaryOrKey)
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

		// Add to history
		history = append(history, fmt.Sprintf("Q: %s", question))
		history = append(history, fmt.Sprintf("A: %s", answer))

		// If the answer is empty or user wants to skip, break early
		if answer == "" || answer == "skip" || answer == "done" {
			break
		}
	}

	// Generate the final description
	description, err := client.GenerateDescription(history, initialContext, summaryOrKey)
	if err != nil {
		return "", fmt.Errorf("failed to generate description: %w", err)
	}

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
