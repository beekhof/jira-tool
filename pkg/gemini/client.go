package gemini

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"go-jira-helper/pkg/credentials"
)

// GeminiClient defines the interface for Gemini operations
type GeminiClient interface {
	GenerateQuestion(history []string, context string) (string, error)
	GenerateDescription(history []string, context string) (string, error)
}

// geminiClient is the concrete implementation of GeminiClient
type geminiClient struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

// NewClient creates a new Gemini client
func NewClient() (GeminiClient, error) {
	// Get API key from credentials
	// We use a dummy user since we store by service, not user
	apiKey, err := credentials.GetSecret(credentials.GeminiServiceKey, "default")
	if err != nil {
		return nil, fmt.Errorf("failed to get Gemini API key: %w. Please run 'jira init'", err)
	}

	return &geminiClient{
		apiKey:  apiKey,
		baseURL: "https://generativelanguage.googleapis.com/v1beta/models/gemini-pro:generateContent",
		client:  &http.Client{},
	}, nil
}

// GeminiRequest represents the request payload
type GeminiRequest struct {
	Contents []Content `json:"contents"`
}

// Content represents a content item in the request
type Content struct {
	Parts []Part `json:"parts"`
}

// Part represents a part of content
type Part struct {
	Text string `json:"text"`
}

// GeminiResponse represents the response from Gemini API
type GeminiResponse struct {
	Candidates []Candidate `json:"candidates"`
}

// Candidate represents a candidate response
type Candidate struct {
	Content Content `json:"content"`
}

// GenerateQuestion generates a clarifying question based on history and context
func (c *geminiClient) GenerateQuestion(history []string, context string) (string, error) {
	prompt := c.buildQuestionPrompt(history, context)
	return c.generateContent(prompt)
}

// GenerateDescription generates a description based on history and context
func (c *geminiClient) GenerateDescription(history []string, context string) (string, error) {
	prompt := c.buildDescriptionPrompt(history, context)
	return c.generateContent(prompt)
}

// buildQuestionPrompt constructs the prompt for generating a question
func (c *geminiClient) buildQuestionPrompt(history []string, context string) string {
	var sb strings.Builder

	sb.WriteString("You are helping to create a Jira ticket. ")
	sb.WriteString("Based on the following context and conversation history, ask ONE clarifying question to better understand what needs to be done.\n\n")

	sb.WriteString("Context: ")
	sb.WriteString(context)
	sb.WriteString("\n\n")

	if len(history) > 0 {
		sb.WriteString("Conversation history:\n")
		for i, entry := range history {
			sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, entry))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("Ask only ONE clear, concise question. Do not include any preamble or explanation, just the question.")

	return sb.String()
}

// buildDescriptionPrompt constructs the prompt for generating a description
func (c *geminiClient) buildDescriptionPrompt(history []string, context string) string {
	var sb strings.Builder

	sb.WriteString("You are helping to create a Jira ticket description. ")
	sb.WriteString("Based on the following context and conversation history, write a clear, comprehensive Jira ticket description.\n\n")

	sb.WriteString("Context: ")
	sb.WriteString(context)
	sb.WriteString("\n\n")

	if len(history) > 0 {
		sb.WriteString("Conversation history:\n")
		for i, entry := range history {
			sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, entry))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("Write a professional Jira ticket description that includes:\n")
	sb.WriteString("- Clear explanation of what needs to be done\n")
	sb.WriteString("- Any relevant context or background\n")
	sb.WriteString("- Expected outcomes or acceptance criteria if applicable\n")
	sb.WriteString("Format it as plain text suitable for a Jira description field.")

	return sb.String()
}

// generateContent makes the actual API call to Gemini
func (c *geminiClient) generateContent(prompt string) (string, error) {
	// Build the request payload
	reqPayload := GeminiRequest{
		Contents: []Content{
			{
				Parts: []Part{
					{Text: prompt},
				},
			},
		},
	}

	jsonData, err := json.Marshal(reqPayload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	// Build the URL with API key
	url := fmt.Sprintf("%s?key=%s", c.baseURL, c.apiKey)

	// Create the POST request
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// Execute the request
	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode == 401 || resp.StatusCode == 403 {
			return "", fmt.Errorf("authentication failed. Your Gemini API key may be invalid. Please run 'jira init'")
		}
		if resp.StatusCode == 429 {
			return "", fmt.Errorf("Gemini API rate limit exceeded. Please wait and try again")
		}
		return "", fmt.Errorf("Gemini API returned error: %d %s - %s", resp.StatusCode, resp.Status, string(body))
	}

	// Parse response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	var geminiResp GeminiResponse
	if err := json.Unmarshal(body, &geminiResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if len(geminiResp.Candidates) == 0 || len(geminiResp.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("no response from Gemini API")
	}

	return geminiResp.Candidates[0].Content.Parts[0].Text, nil
}
