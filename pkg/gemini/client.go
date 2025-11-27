package gemini

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/beekhof/jira-tool/pkg/config"
	"github.com/beekhof/jira-tool/pkg/credentials"
)

// GeminiClient defines the interface for Gemini operations
type GeminiClient interface {
	GenerateQuestion(history []string, context string, issueType string) (string, error)
	GenerateDescription(history []string, context string, issueType string) (string, error)
	EstimateStoryPoints(summary, description string, availablePoints []int) (int, string, error)
}

// geminiClient is the concrete implementation of GeminiClient
type geminiClient struct {
	apiKey                      string
	baseURL                     string
	client                      *http.Client
	questionPromptTemplate      string
	descriptionPromptTemplate   string
	spikeQuestionPromptTemplate string
	spikePromptTemplate         string
}

// ListModels lists available Gemini models
func ListModels(configDir string) ([]ModelInfo, error) {
	// Get API key from credentials
	apiKey, err := credentials.GetSecret(credentials.GeminiServiceKey, "default", configDir)
	if err != nil {
		return nil, fmt.Errorf("failed to get Gemini API key: %w. Please run 'jira init'", err)
	}

	// Try v1 first, then v1beta as fallback
	versions := []string{"v1", "v1beta"}
	var lastErr error

	for _, version := range versions {
		url := fmt.Sprintf("https://generativelanguage.googleapis.com/%s/models?key=%s", version, apiKey)

		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			lastErr = err
			continue
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode == 200 {
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				lastErr = err
				continue
			}

			var listResp ListModelsResponse
			if err := json.Unmarshal(body, &listResp); err != nil {
				lastErr = err
				continue
			}

			return listResp.Models, nil
		}
	}

	return nil, fmt.Errorf("failed to list models: %w", lastErr)
}

// NewClient creates a new Gemini client
// configDir can be empty to use the default ~/.jira-tool
func NewClient(configDir string) (GeminiClient, error) {
	// Get API key from credentials
	// We use a dummy user since we store by service, not user
	apiKey, err := credentials.GetSecret(credentials.GeminiServiceKey, "default", configDir)
	if err != nil {
		return nil, fmt.Errorf("failed to get Gemini API key: %w. Please run 'jira init'", err)
	}

	// Load config to get the model name
	configPath := config.GetConfigPath(configDir)
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		// If config can't be loaded, use default model
		cfg = &config.Config{}
	}

	// Use configured model or default to gemini-2.5-flash
	model := cfg.GeminiModel
	if model == "" {
		model = "gemini-2.5-flash"
	}

	// Strip "models/" prefix if present (ListModels returns names with prefix)
	modelName := model
	if strings.HasPrefix(model, "models/") {
		modelName = strings.TrimPrefix(model, "models/")
	}

	// Get prompt templates or use defaults
	questionTemplate := cfg.QuestionPromptTemplate
	if questionTemplate == "" {
		questionTemplate = getDefaultQuestionPrompt()
	}

	descriptionTemplate := cfg.DescriptionPromptTemplate
	if descriptionTemplate == "" {
		descriptionTemplate = getDefaultDescriptionPrompt()
	}

	spikeQuestionTemplate := cfg.SpikeQuestionPromptTemplate
	if spikeQuestionTemplate == "" {
		spikeQuestionTemplate = getDefaultSpikeQuestionPrompt()
	}

	spikeTemplate := cfg.SpikePromptTemplate
	if spikeTemplate == "" {
		spikeTemplate = getDefaultSpikePrompt()
	}

	return &geminiClient{
		apiKey:                      apiKey,
		baseURL:                     fmt.Sprintf("https://generativelanguage.googleapis.com/v1/models/%s:generateContent", modelName),
		client:                      &http.Client{},
		questionPromptTemplate:      questionTemplate,
		descriptionPromptTemplate:   descriptionTemplate,
		spikeQuestionPromptTemplate: spikeQuestionTemplate,
		spikePromptTemplate:         spikeTemplate,
	}, nil
}

// getDefaultQuestionPrompt returns the default question generation prompt template
func getDefaultQuestionPrompt() string {
	return `You are helping to create a Jira ticket. Based on the following context and conversation history, ask ONE clarifying question to better understand what needs to be done.

Context: {{context}}

{{history}}

Ask only ONE clear, concise question. Do not include any preamble or explanation, just the question.`
}

// getDefaultSpikeQuestionPrompt returns the default question generation prompt template for spikes
func getDefaultSpikeQuestionPrompt() string {
	return `You are helping to create a Jira ticket for research on a specific topic.
Based on the following context and conversation history, ask ONE question to help explain or constrain the task for an engineer to complete.
Important: some research is about finding out what we don't know yet, other research is about finding out if a solution is possible or not.

Areas of interest include: 
- why is this research needed
- what is the problem that needs to be solved, or domain that needs to be understood
- what criteria can we use to determine if the research is complete
- what are the possible outcomes of the research

Context: {{context}}

{{history}}

Ask only ONE clear, concise question that helps define the scope or boundaries of the research. 
Focus on understanding what needs to be investigated, DO NOT try to find or provide a solution.
Do not include any preamble or explanation, just the question.`
}

// getDefaultDescriptionPrompt returns the default description generation prompt template
func getDefaultDescriptionPrompt() string {
	return `You are helping to create a Jira ticket description for a research on a specific topic. 
Based on the following context and conversation history, write a clear, and concise Jira ticket description that includes:
- Clear explanation of what needs to be done
- Any relevant context or background
- Expected outcomes or acceptance criteria
Format it as plain text suitable for a Jira description field.

Context: {{context}}

{{history}}
`
}

// getDefaultSpikePrompt returns the default spike/research prompt template
func getDefaultSpikePrompt() string {
	return `You are helping to create a research spike description for a Jira ticket. Based on the following context and conversation history, write a clear, comprehensive research plan.

Context: {{context}}

{{history}}

Write a concise and professional description that includes:
- Core question to be answered
- Clear explanation of what needs to be researched
- Any relevant context or background
- Expected outcomes or acceptance criteria

Format it as plain text suitable for a Jira description field.`
}

// GetDefaultTemplates returns all default prompt templates in a map
// This is useful for displaying defaults or initializing config
func GetDefaultTemplates() map[string]string {
	return map[string]string{
		"question_prompt_template":       getDefaultQuestionPrompt(),
		"description_prompt_template":    getDefaultDescriptionPrompt(),
		"spike_question_prompt_template": getDefaultSpikeQuestionPrompt(),
		"spike_prompt_template":          getDefaultSpikePrompt(),
	}
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

// ListModelsResponse represents the response from ListModels API
type ListModelsResponse struct {
	Models []ModelInfo `json:"models"`
}

// ModelInfo represents information about a model
type ModelInfo struct {
	Name             string   `json:"name"`
	DisplayName      string   `json:"displayName"`
	SupportedMethods []string `json:"supportedGenerationMethods"`
}

// GenerateQuestion generates a clarifying question based on history and context
func (c *geminiClient) GenerateQuestion(history []string, context string, issueType string) (string, error) {
	prompt := c.buildQuestionPrompt(history, context, issueType)
	return c.generateContent(prompt)
}

// GenerateDescription generates a description based on history and context
// Uses spike prompt template if context indicates a spike (SPIKE prefix), otherwise uses normal template
func (c *geminiClient) GenerateDescription(history []string, context string, issueType string) (string, error) {
	prompt := c.buildDescriptionPrompt(history, context, issueType)
	return c.generateContent(prompt)
}

// EstimateStoryPoints estimates story points for a ticket based on summary and description
// Returns the estimated points, reasoning text, and any error
func (c *geminiClient) EstimateStoryPoints(summary, description string, availablePoints []int) (int, string, error) {
	// Build the prompt
	var pointsList strings.Builder
	for i, points := range availablePoints {
		if i > 0 {
			pointsList.WriteString(", ")
		}
		pointsList.WriteString(fmt.Sprintf("%d", points))
	}
	if len(availablePoints) > 0 {
		pointsList.WriteString(" (or any other positive integer)")
	}

	prompt := fmt.Sprintf(`You are an expert at estimating story points for software development tasks using Agile/Scrum methodology.

Ticket Summary: %s

Ticket Description:
%s

Available story point options: %s

Please provide a story point estimate for this ticket. Consider:
- Complexity and technical difficulty
- Amount of work required
- Risk and uncertainty
- Dependencies and integration effort

Respond with ONLY a single number (the story point estimate), followed by a brief one-sentence explanation of your reasoning.

Example format:
5
This task involves moderate complexity with clear requirements and minimal risk.`, summary, description, pointsList.String())

	response, err := c.generateContent(prompt)
	if err != nil {
		return 0, "", err
	}

	// Parse the response to extract the number
	// Look for the first number in the response
	lines := strings.Split(strings.TrimSpace(response), "\n")
	if len(lines) == 0 {
		return 0, response, fmt.Errorf("could not parse story point estimate from response")
	}

	// Try to extract number from first line
	firstLine := strings.TrimSpace(lines[0])
	var estimate int
	_, err = fmt.Sscanf(firstLine, "%d", &estimate)
	if err != nil {
		// Try to find any number in the response
		var found bool
		for _, line := range lines {
			line = strings.TrimSpace(line)
			_, err := fmt.Sscanf(line, "%d", &estimate)
			if err == nil && estimate > 0 {
				found = true
				break
			}
		}
		if !found {
			return 0, response, fmt.Errorf("could not find a valid story point estimate in response")
		}
	}

	// Build reasoning from remaining lines
	reasoning := strings.TrimSpace(strings.Join(lines[1:], " "))
	if reasoning == "" {
		reasoning = response
	}

	return estimate, reasoning, nil
}

// buildQuestionPrompt constructs the prompt for generating a question
// Uses spike question template if the context indicates a spike (SPIKE prefix in summary/key)
func (c *geminiClient) buildQuestionPrompt(history []string, context string, issueType string) string {
	// Build history section
	historySection := ""
	if len(history) > 0 {
		var sb strings.Builder
		sb.WriteString("Conversation history:\n")
		for i, entry := range history {
			sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, entry))
		}
		historySection = sb.String()
	}

	// Check if this is a spike based on context (summary) or issueType (which may be summary/key)
	// The issueType parameter now contains the summary or key for spike detection
	isSpike := IsSpike(context, "")
	if !isSpike && issueType != "" {
		// Check if the summary/key contains SPIKE
		isSpike = IsSpike(issueType, "")
	}

	// Use spike question template for spikes, normal template for others
	template := c.questionPromptTemplate
	if isSpike {
		template = c.spikeQuestionPromptTemplate
	}

	// Replace template placeholders
	prompt := strings.ReplaceAll(template, "{{context}}", context)
	prompt = strings.ReplaceAll(prompt, "{{history}}", historySection)

	return prompt
}

// buildDescriptionPrompt constructs the prompt for generating a description
// Uses spike prompt template if the context indicates a spike (SPIKE prefix in summary/key)
func (c *geminiClient) buildDescriptionPrompt(history []string, context string, issueType string) string {
	// Build history section
	historySection := ""
	if len(history) > 0 {
		var sb strings.Builder
		sb.WriteString("Conversation history:\n")
		for i, entry := range history {
			sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, entry))
		}
		historySection = sb.String()
	}

	// Check if this is a spike based on context (summary) or issueType (which may be summary/key)
	// The issueType parameter now contains the summary or key for spike detection
	isSpike := IsSpike(context, "")
	if !isSpike && issueType != "" {
		// Check if the summary/key contains SPIKE
		isSpike = IsSpike(issueType, "")
	}

	// Use spike template for spikes, normal template for others
	template := c.descriptionPromptTemplate
	if isSpike {
		template = c.spikePromptTemplate
	}

	// Replace template placeholders
	prompt := strings.ReplaceAll(template, "{{context}}", context)
	prompt = strings.ReplaceAll(prompt, "{{history}}", historySection)

	return prompt
}

// generateContent makes the actual API call to Gemini with automatic retry for transient errors
func (c *geminiClient) generateContent(prompt string) (string, error) {
	const maxRetries = 3
	const initialBackoff = 5 * time.Second

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff: 5s, 10s, 20s
			backoff := initialBackoff * time.Duration(1<<uint(attempt-1))
			fmt.Fprintf(os.Stderr, "Gemini API error (attempt %d/%d). Retrying in %v...\n", attempt, maxRetries+1, backoff)
			time.Sleep(backoff)
		}

		result, err := c.generateContentOnce(prompt)
		if err == nil {
			if attempt > 0 {
				fmt.Fprintf(os.Stderr, "Request succeeded after %d retry(ies).\n", attempt)
			}
			return result, nil
		}

		lastErr = err
		errStr := err.Error()

		// Only retry on transient errors (503, 500, 502, 504, 429)
		// Check for both status codes and error messages
		isRetryable := strings.Contains(errStr, "503") ||
			strings.Contains(errStr, "500") ||
			strings.Contains(errStr, "502") ||
			strings.Contains(errStr, "504") ||
			strings.Contains(errStr, "429") ||
			strings.Contains(errStr, "temporarily unavailable") ||
			strings.Contains(errStr, "server error") ||
			strings.Contains(errStr, "rate limit")

		if !isRetryable {
			return "", err
		}

		// On last attempt, return the error
		if attempt == maxRetries {
			return "", fmt.Errorf("%w (after %d retries)", err, maxRetries)
		}
	}

	return "", lastErr
}

// generateContentOnce makes a single API call to Gemini
func (c *geminiClient) generateContentOnce(prompt string) (string, error) {
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

		// Parse error response for better error messages
		var apiError struct {
			Error struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
				Status  string `json:"status"`
			} `json:"error"`
		}
		json.Unmarshal(body, &apiError)

		// Provide user-friendly error messages
		switch resp.StatusCode {
		case 401, 403:
			return "", fmt.Errorf("authentication failed. Your Gemini API key may be invalid. Please run 'jira init'")
		case 429:
			return "", fmt.Errorf("Gemini API rate limit exceeded. Please wait a moment and try again")
		case 503:
			errorMsg := "Gemini API is temporarily unavailable (service overloaded)"
			if apiError.Error.Message != "" {
				errorMsg = fmt.Sprintf("%s: %s", errorMsg, apiError.Error.Message)
			}
			return "", fmt.Errorf("%s. Please try again in a few moments", errorMsg)
		case 500, 502, 504:
			return "", fmt.Errorf("Gemini API server error. Please try again in a few moments")
		default:
			// For other errors, include the API's error message if available
			if apiError.Error.Message != "" {
				return "", fmt.Errorf("Gemini API error: %s", apiError.Error.Message)
			}
			return "", fmt.Errorf("Gemini API returned error: %d %s", resp.StatusCode, resp.Status)
		}
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
