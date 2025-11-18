package gemini

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"go-jira-helper/pkg/config"
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
// configDir can be empty to use the default ~/.jira-helper
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

	return &geminiClient{
		apiKey:  apiKey,
		baseURL: fmt.Sprintf("https://generativelanguage.googleapis.com/v1/models/%s:generateContent", modelName),
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

// ListModelsResponse represents the response from ListModels API
type ListModelsResponse struct {
	Models []ModelInfo `json:"models"`
}

// ModelInfo represents information about a model
type ModelInfo struct {
	Name         string   `json:"name"`
	DisplayName  string   `json:"displayName"`
	SupportedMethods []string `json:"supportedGenerationMethods"`
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

// generateContent makes the actual API call to Gemini with automatic retry for transient errors
func (c *geminiClient) generateContent(prompt string) (string, error) {
	const maxRetries = 3
	const initialBackoff = 5 * time.Second
	
	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff: 1s, 2s, 4s
			backoff := initialBackoff * time.Duration(1<<uint(attempt-1))
			time.Sleep(backoff)
		}
		
		result, err := c.generateContentOnce(prompt)
		if err == nil {
			return result, nil
		}
		
		lastErr = err
		errStr := err.Error()
		
		// Only retry on transient errors (503, 500, 502, 504, 429)
		if !strings.Contains(errStr, "503") &&
			!strings.Contains(errStr, "500") &&
			!strings.Contains(errStr, "502") &&
			!strings.Contains(errStr, "504") &&
			!strings.Contains(errStr, "429") {
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
