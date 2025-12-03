package jira

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/beekhof/jira-tool/pkg/config"
	"github.com/beekhof/jira-tool/pkg/credentials"
)

// JiraClient defines the interface for Jira operations
//
//nolint:revive // Type name is intentional for clarity in public API
type JiraClient interface {
	UpdateTicketPoints(ticketID string, points int) error
	UpdateTicketDescription(ticketID, description string) error
	UpdateTicketPriority(ticketID, priorityID string) error
	CreateTicket(project, taskType, summary string) (string, error)
	CreateTicketWithParent(project, taskType, summary, parentKey string) (string, error)
	CreateTicketWithEpicLink(project, taskType, summary, epicKey, epicLinkFieldID string) (string, error)
	SearchTickets(jql string) ([]Issue, error)
	GetIssue(issueKey string) (*Issue, error)
	SearchUsers(query string) ([]User, error)
	AssignTicket(ticketID, userAccountID, userName string) error
	UnassignTicket(ticketID string) error
	GetPriorities() ([]Priority, error)
	TransitionTicket(ticketID, transitionID string) error
	GetTicketDescription(ticketID string) (string, error)
	GetTicketAttachments(ticketID string) ([]Attachment, error)
	GetTicketComments(ticketID string) ([]Comment, error)
	AddComment(ticketID, comment string) error
	GetTransitions(ticketID string) ([]Transition, error)
	AddIssuesToSprint(sprintID int, issueKeys []string) error
	AddIssuesToRelease(releaseID string, issueKeys []string) error
	GetActiveSprints(boardID int) ([]SprintParsed, error)
	GetPlannedSprints(boardID int) ([]SprintParsed, error)
	GetReleases(projectKey string) ([]ReleaseParsed, error)
	GetIssuesForSprint(sprintID int) ([]Issue, error)
	GetIssuesForRelease(releaseID string) ([]Issue, error)
	GetTicketRaw(ticketID string) (map[string]interface{}, error)
	GetComponents(projectKey string) ([]Component, error)
	UpdateTicketComponents(ticketID string, componentIDs []string) error
	DetectSeverityField(projectKey string) (string, error)
	GetSeverityFieldValues(fieldID string) ([]string, error)
	UpdateTicketSeverity(ticketID, severityFieldID, severityValue string) error
	ClearComponentCache(projectKey string)
	GetBoardsForProject(projectKey string) ([]Board, error)
	DetectEpicLinkField(projectKey string) (string, error)
}

// Attachment represents a Jira attachment
type Attachment struct {
	ID       string `json:"id"`
	Filename string `json:"filename"`
	Content  string `json:"content"` // URL to download
}

// Comment represents a Jira comment
type Comment struct {
	ID     string `json:"id"`
	Body   string `json:"body"`
	Author struct {
		DisplayName string `json:"displayName"`
	} `json:"author"`
	Created string `json:"created"`
}

// Transition represents a Jira status transition
type Transition struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	To   struct {
		Name string `json:"name"`
	} `json:"to"`
}

// User represents a Jira user
type User struct {
	AccountID    string `json:"accountId"`
	DisplayName  string `json:"displayName"`
	EmailAddress string `json:"emailAddress"`
	// Alternative field names that Jira might use
	Key          string `json:"key"`        // Some Jira instances use "key" instead of accountId
	AccountIDAlt string `json:"account_id"` // Snake case variant
	Name         string `json:"name"`       // Username/name field
}

// Priority represents a Jira priority
type Priority struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Component represents a Jira component
type Component struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// Board represents a Jira board
type Board struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

// Sprint represents a Jira sprint
type Sprint struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	State     string `json:"state"`
	StartDate string `json:"startDate"`
	EndDate   string `json:"endDate"`
}

// SprintParsed is Sprint with parsed dates
type SprintParsed struct {
	ID        int
	Name      string
	State     string
	StartDate time.Time
	EndDate   time.Time
}

// Release represents a Jira release/fix version
type Release struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Released    bool   `json:"released"`
	ReleaseDate string `json:"releaseDate"`
}

// ReleaseParsed is Release with parsed date
type ReleaseParsed struct {
	ID          string
	Name        string
	Released    bool
	ReleaseDate time.Time
}

// Issue represents a Jira issue
type Issue struct {
	Key    string `json:"key"`
	Fields struct {
		Summary string `json:"summary"`
		Status  struct {
			Name string `json:"name"`
		} `json:"status"`
		IssueType struct {
			Name string `json:"name"`
		} `json:"issuetype"`
		Priority struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"priority"`
		Assignee struct {
			AccountID    string `json:"accountId"`
			DisplayName  string `json:"displayName"`
			EmailAddress string `json:"emailAddress"`
			Key          string `json:"key"`    // Server/Data Center uses "key"
			Name         string `json:"name"`   // Some instances use "name"
			Active       bool   `json:"active"` // User active status
		} `json:"assignee"`
		Components []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"components"`
		StoryPoints float64 `json:"customfield_10016"`
	} `json:"fields"`
}

// SprintResponse represents the response from Jira's sprint API
type SprintResponse struct {
	Values []Sprint `json:"values"`
}

// ReleaseResponse represents the response from Jira's version API
type ReleaseResponse []Release

// IssueResponse represents the response from Jira's search API
type IssueResponse struct {
	Issues []Issue `json:"issues"`
}

// jiraClient is the concrete implementation of JiraClient
type jiraClient struct {
	baseURL            string
	httpClient         *http.Client
	authToken          string
	cache              *Cache
	storyPointsFieldID string
	noCache            bool
}

// NewClient creates a new Jira client by loading config and credentials
// configDir can be empty to use the default ~/.jira-tool
// noCache if true, bypasses cache for all operations
func NewClient(configDir string, noCache bool) (JiraClient, error) {
	configPath := config.GetConfigPath(configDir)
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	if cfg.JiraURL == "" {
		return nil, fmt.Errorf("jira_url not configured. Please run 'jira init'")
	}

	token, err := credentials.GetSecret(credentials.JiraServiceKey, "", configDir)
	if err != nil {
		return nil, fmt.Errorf("failed to get Jira token: %w", err)
	}

	// Load cache
	cachePath := GetCachePath(configDir)
	cache := NewCache(cachePath)
	if err := cache.Load(); err != nil {
		// Log error but continue - cache is optional
		_ = err
	}

	// Get story points field ID from config, default to customfield_10016
	storyPointsFieldID := cfg.StoryPointsFieldID
	if storyPointsFieldID == "" {
		storyPointsFieldID = "customfield_10016"
	}

	client := &jiraClient{
		baseURL:            cfg.JiraURL,
		httpClient:         &http.Client{},
		authToken:          token,
		cache:              cache,
		storyPointsFieldID: storyPointsFieldID,
		noCache:            noCache,
	}

	return client, nil
}

// setAuth sets the Bearer token authentication header on the request
func (c *jiraClient) setAuth(req *http.Request) {
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.authToken))
}

// UpdateTicketPoints updates the story points for a ticket
// Uses the configurable story points field ID from config
func (c *jiraClient) UpdateTicketPoints(ticketID string, points int) error {
	// Construct the API endpoint
	endpoint := fmt.Sprintf("%s/rest/api/2/issue/%s", c.baseURL, ticketID)

	// Construct the JSON payload using the configured field ID
	payload := map[string]interface{}{
		"fields": map[string]interface{}{
			c.storyPointsFieldID: points,
		},
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create the PUT request
	req, err := http.NewRequest("PUT", endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	c.setAuth(req)

	// Execute the request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// Read response body for more details
		body, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return fmt.Errorf("Jira API returned error: %d %s (failed to read body: %w)", resp.StatusCode, resp.Status, readErr)
		}
		bodyStr := string(body)

		if resp.StatusCode == 401 || resp.StatusCode == 403 {
			return fmt.Errorf("authentication failed. Your Jira token may be invalid. Please run 'jira init'")
		}
		if resp.StatusCode == 404 {
			return fmt.Errorf("ticket %s not found", ticketID)
		}
		if resp.StatusCode == 400 {
			// Try to parse error message from response
			var apiError struct {
				ErrorMessages []string          `json:"errorMessages"`
				Errors        map[string]string `json:"errors"`
			}
			if err := json.Unmarshal(body, &apiError); err == nil {
				if len(apiError.ErrorMessages) > 0 {
					return fmt.Errorf("Jira API error: %s", strings.Join(apiError.ErrorMessages, "; "))
				}
				if len(apiError.Errors) > 0 {
					var errorMsgs []string
					for k, v := range apiError.Errors {
						errorMsgs = append(errorMsgs, fmt.Sprintf("%s: %s", k, v))
					}
					return fmt.Errorf("Jira API error: %s", strings.Join(errorMsgs, "; "))
				}
			}
			// If parsing failed, check if it's a custom field issue
			if strings.Contains(bodyStr, "customfield") || strings.Contains(bodyStr, "field") {
				return fmt.Errorf(
					"jira API error: %d %s - %s\nnote: the story points field ID (%s) may be incorrect for your Jira instance. "+
						"You can configure it in your config file with 'story_points_field_id'",
					resp.StatusCode, resp.Status, bodyStr, c.storyPointsFieldID)
			}
			return fmt.Errorf("Jira API returned error: %d %s - %s", resp.StatusCode, resp.Status, bodyStr)
		}
		return fmt.Errorf("Jira API returned error: %d %s - %s", resp.StatusCode, resp.Status, bodyStr)
	}

	return nil
}

// UpdateTicketDescription updates the description for a ticket
func (c *jiraClient) UpdateTicketDescription(ticketID, description string) error {
	// Construct the API endpoint
	endpoint := fmt.Sprintf("%s/rest/api/2/issue/%s", c.baseURL, ticketID)

	// Construct the JSON payload
	payload := map[string]interface{}{
		"fields": map[string]interface{}{
			"description": description,
		},
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create the PUT request
	req, err := http.NewRequest("PUT", endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	c.setAuth(req)

	// Execute the request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if resp.StatusCode == 401 || resp.StatusCode == 403 {
			return fmt.Errorf("authentication failed. Your Jira token may be invalid. Please run 'jira init'")
		}
		if resp.StatusCode == 404 {
			return fmt.Errorf("ticket %s not found", ticketID)
		}
		return fmt.Errorf("Jira API returned error: %d %s", resp.StatusCode, resp.Status)
	}

	return nil
}

// CreateTicketResponse represents the response from creating a ticket
type CreateTicketResponse struct {
	ID   string `json:"id"`
	Key  string `json:"key"`
	Self string `json:"self"`
}

// CreateTicket creates a new Jira ticket
func (c *jiraClient) CreateTicket(project, taskType, summary string) (string, error) {
	endpoint := fmt.Sprintf("%s/rest/api/2/issue", c.baseURL)

	// Construct the JSON payload
	payload := map[string]interface{}{
		"fields": map[string]interface{}{
			"project": map[string]interface{}{
				"key": project,
			},
			"summary": summary,
			"issuetype": map[string]interface{}{
				"name": taskType,
			},
		},
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create the POST request
	req, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	c.setAuth(req)

	// Execute the request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if resp.StatusCode == 401 || resp.StatusCode == 403 {
			return "", fmt.Errorf("authentication failed. Your Jira token may be invalid. Please run 'jira init'")
		}
		body, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return "", fmt.Errorf(
				"Jira API returned error: %d %s (failed to read body: %w)",
				resp.StatusCode, resp.Status, readErr)
		}
		return "", fmt.Errorf("Jira API returned error: %d %s - %s", resp.StatusCode, resp.Status, string(body))
	}

	// Parse response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	var createResp CreateTicketResponse
	if err := json.Unmarshal(body, &createResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	return createResp.Key, nil
}

// CreateTicketWithParent creates a new Jira ticket with a parent (for subtasks)
func (c *jiraClient) CreateTicketWithParent(project, taskType, summary, parentKey string) (string, error) {
	endpoint := fmt.Sprintf("%s/rest/api/2/issue", c.baseURL)

	// Construct the JSON payload
	payload := map[string]interface{}{
		"fields": map[string]interface{}{
			"project": map[string]interface{}{
				"key": project,
			},
			"summary": summary,
			"issuetype": map[string]interface{}{
				"name": taskType,
			},
			"parent": map[string]interface{}{
				"key": parentKey,
			},
		},
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	c.setAuth(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if resp.StatusCode == 401 || resp.StatusCode == 403 {
			return "", fmt.Errorf("authentication failed. Your Jira token may be invalid. Please run 'jira init'")
		}
		body, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return "", fmt.Errorf(
				"Jira API returned error: %d %s (failed to read body: %w)",
				resp.StatusCode, resp.Status, readErr)
		}
		return "", fmt.Errorf("Jira API returned error: %d %s - %s", resp.StatusCode, resp.Status, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	var createResp CreateTicketResponse
	if err := json.Unmarshal(body, &createResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	return createResp.Key, nil
}

// CreateTicketWithEpicLink creates a new Jira ticket with Epic Link field
func (c *jiraClient) CreateTicketWithEpicLink(
	project, taskType, summary, epicKey, epicLinkFieldID string) (string, error) {
	endpoint := fmt.Sprintf("%s/rest/api/2/issue", c.baseURL)

	// Construct the JSON payload
	payload := map[string]interface{}{
		"fields": map[string]interface{}{
			"project": map[string]interface{}{
				"key": project,
			},
			"summary": summary,
			"issuetype": map[string]interface{}{
				"name": taskType,
			},
		},
	}

	// Add Epic Link field dynamically
	fields, ok := payload["fields"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("invalid payload structure: fields is not a map")
	}
	fields[epicLinkFieldID] = epicKey

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create the POST request
	req, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	c.setAuth(req)

	// Execute the request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if resp.StatusCode == 401 || resp.StatusCode == 403 {
			return "", fmt.Errorf("authentication failed. Your Jira token may be invalid. Please run 'jira init'")
		}
		body, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return "", fmt.Errorf(
				"Jira API returned error: %d %s (failed to read body: %w)",
				resp.StatusCode, resp.Status, readErr)
		}
		return "", fmt.Errorf("Jira API returned error: %d %s - %s", resp.StatusCode, resp.Status, string(body))
	}

	// Parse response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	var createResp CreateTicketResponse
	if err := json.Unmarshal(body, &createResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	return createResp.Key, nil
}

// GetTransitions gets available transitions for a ticket
func (c *jiraClient) GetTransitions(ticketID string) ([]Transition, error) {
	endpoint := fmt.Sprintf("%s/rest/api/2/issue/%s/transitions", c.baseURL, ticketID)

	req, err := http.NewRequest("GET", endpoint, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	c.setAuth(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if resp.StatusCode == 401 || resp.StatusCode == 403 {
			return nil, fmt.Errorf("authentication failed. Your Jira token may be invalid. Please run 'jira init'")
		}
		return nil, fmt.Errorf("Jira API returned error: %d %s", resp.StatusCode, resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var transitionResp struct {
		Transitions []Transition `json:"transitions"`
	}
	if err := json.Unmarshal(body, &transitionResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return transitionResp.Transitions, nil
}

// TransitionTicket transitions a ticket to a new status
func (c *jiraClient) TransitionTicket(ticketID, transitionID string) error {
	endpoint := fmt.Sprintf("%s/rest/api/2/issue/%s/transitions", c.baseURL, ticketID)

	payload := map[string]interface{}{
		"transition": map[string]interface{}{
			"id": transitionID,
		},
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	c.setAuth(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if resp.StatusCode == 401 || resp.StatusCode == 403 {
			return fmt.Errorf("authentication failed. Your Jira token may be invalid. Please run 'jira init'")
		}
		return fmt.Errorf("Jira API returned error: %d %s", resp.StatusCode, resp.Status)
	}

	return nil
}

// GetTicketRaw fetches a ticket with all fields for debugging
func (c *jiraClient) GetTicketRaw(ticketID string) (map[string]interface{}, error) {
	endpoint := fmt.Sprintf("%s/rest/api/2/issue/%s", c.baseURL, ticketID)

	req, err := http.NewRequest("GET", endpoint, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	c.setAuth(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if resp.StatusCode == 404 {
			return nil, fmt.Errorf("ticket %s not found", ticketID)
		}
		return nil, fmt.Errorf("Jira API returned error: %d %s", resp.StatusCode, resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var issueData map[string]interface{}
	if err := json.Unmarshal(body, &issueData); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return issueData, nil
}

// GetTicketDescription gets the description of a ticket
func (c *jiraClient) GetTicketDescription(ticketID string) (string, error) {
	endpoint := fmt.Sprintf("%s/rest/api/2/issue/%s?fields=description", c.baseURL, ticketID)

	req, err := http.NewRequest("GET", endpoint, http.NoBody)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	c.setAuth(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if resp.StatusCode == 404 {
			return "", fmt.Errorf("ticket %s not found", ticketID)
		}
		return "", fmt.Errorf("Jira API returned error: %d %s", resp.StatusCode, resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	var issueResp struct {
		Fields struct {
			Description string `json:"description"`
		} `json:"fields"`
	}
	if err := json.Unmarshal(body, &issueResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	return issueResp.Fields.Description, nil
}

// GetTicketAttachments gets attachments for a ticket
func (c *jiraClient) GetTicketAttachments(ticketID string) ([]Attachment, error) {
	endpoint := fmt.Sprintf("%s/rest/api/2/issue/%s?fields=attachment", c.baseURL, ticketID)

	req, err := http.NewRequest("GET", endpoint, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	c.setAuth(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if resp.StatusCode == 404 {
			return nil, fmt.Errorf("ticket %s not found", ticketID)
		}
		return nil, fmt.Errorf("Jira API returned error: %d %s", resp.StatusCode, resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var issueResp struct {
		Fields struct {
			Attachment []Attachment `json:"attachment"`
		} `json:"fields"`
	}
	if err := json.Unmarshal(body, &issueResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return issueResp.Fields.Attachment, nil
}

// GetTicketComments gets comments for a ticket
func (c *jiraClient) GetTicketComments(ticketID string) ([]Comment, error) {
	endpoint := fmt.Sprintf("%s/rest/api/2/issue/%s/comment", c.baseURL, ticketID)

	req, err := http.NewRequest("GET", endpoint, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	c.setAuth(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if resp.StatusCode == 404 {
			return nil, fmt.Errorf("ticket %s not found", ticketID)
		}
		return nil, fmt.Errorf("Jira API returned error: %d %s", resp.StatusCode, resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var commentResp struct {
		Comments []Comment `json:"comments"`
	}
	if err := json.Unmarshal(body, &commentResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return commentResp.Comments, nil
}

// AddComment adds a comment to a ticket
func (c *jiraClient) AddComment(ticketID, comment string) error {
	endpoint := fmt.Sprintf("%s/rest/api/2/issue/%s/comment", c.baseURL, ticketID)

	// Construct the JSON payload
	payload := map[string]interface{}{
		"body": comment,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	c.setAuth(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if resp.StatusCode == 401 || resp.StatusCode == 403 {
			return fmt.Errorf("authentication failed. Your Jira token may be invalid. Please run 'jira init'")
		}
		if resp.StatusCode == 404 {
			return fmt.Errorf("ticket %s not found", ticketID)
		}
		body, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return fmt.Errorf("Jira API returned error: %d %s (failed to read body: %w)", resp.StatusCode, resp.Status, readErr)
		}
		return fmt.Errorf("Jira API returned error: %d %s - %s", resp.StatusCode, resp.Status, string(body))
	}

	return nil
}

// AddIssuesToSprint adds issues to a sprint
func (c *jiraClient) AddIssuesToSprint(sprintID int, issueKeys []string) error {
	endpoint := fmt.Sprintf("%s/rest/agile/1.0/sprint/%d/issue", c.baseURL, sprintID)

	payload := map[string]interface{}{
		"issues": issueKeys,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	c.setAuth(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Jira API returned error: %d %s", resp.StatusCode, resp.Status)
	}

	return nil
}

// AddIssuesToRelease adds issues to a release/fix version
func (c *jiraClient) AddIssuesToRelease(releaseID string, issueKeys []string) error {
	// For each issue, update its fixVersion field
	for _, key := range issueKeys {
		endpoint := fmt.Sprintf("%s/rest/api/2/issue/%s", c.baseURL, key)

		payload := map[string]interface{}{
			"fields": map[string]interface{}{
				"fixVersions": []map[string]interface{}{
					{"id": releaseID},
				},
			},
		}

		jsonData, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("failed to marshal request: %w", err)
		}

		req, err := http.NewRequest("PUT", endpoint, bytes.NewBuffer(jsonData))
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Content-Type", "application/json")
		c.setAuth(req)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("failed to execute request: %w", err)
		}
		resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return fmt.Errorf("Jira API returned error for %s: %d %s", key, resp.StatusCode, resp.Status)
		}
	}

	return nil
}

// GetActiveSprints retrieves active sprints for a board
func (c *jiraClient) GetActiveSprints(boardID int) ([]SprintParsed, error) {
	endpoint := fmt.Sprintf("%s/rest/agile/1.0/board/%d/sprint?state=active", c.baseURL, boardID)
	return c.getSprints(endpoint)
}

// GetPlannedSprints retrieves planned sprints for a board
func (c *jiraClient) GetPlannedSprints(boardID int) ([]SprintParsed, error) {
	endpoint := fmt.Sprintf("%s/rest/agile/1.0/board/%d/sprint?state=future", c.baseURL, boardID)
	return c.getSprints(endpoint)
}

// getSprints is a helper to fetch sprints from an endpoint
func (c *jiraClient) getSprints(endpoint string) ([]SprintParsed, error) {
	req, err := http.NewRequest("GET", endpoint, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	c.setAuth(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if resp.StatusCode == 401 || resp.StatusCode == 403 {
			return nil, fmt.Errorf("authentication failed. Your Jira token may be invalid. Please run 'jira init'")
		}
		return nil, fmt.Errorf("Jira API returned error: %d %s", resp.StatusCode, resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var sprintResp SprintResponse
	if err := json.Unmarshal(body, &sprintResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Parse dates and convert to SprintParsed
	result := make([]SprintParsed, len(sprintResp.Values))
	for i, s := range sprintResp.Values {
		result[i] = SprintParsed{
			ID:        s.ID,
			Name:      s.Name,
			State:     s.State,
			StartDate: parseDateString(s.StartDate),
			EndDate:   parseDateString(s.EndDate),
		}
	}

	return result, nil
}

// GetReleases retrieves releases for a project
func (c *jiraClient) GetReleases(projectKey string) ([]ReleaseParsed, error) {
	endpoint := fmt.Sprintf("%s/rest/api/2/project/%s/versions", c.baseURL, projectKey)

	req, err := http.NewRequest("GET", endpoint, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	c.setAuth(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if resp.StatusCode == 401 || resp.StatusCode == 403 {
			return nil, fmt.Errorf("authentication failed. Your Jira token may be invalid. Please run 'jira init'")
		}
		return nil, fmt.Errorf("Jira API returned error: %d %s", resp.StatusCode, resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var releases []Release
	if err := json.Unmarshal(body, &releases); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Parse release dates and convert to ReleaseParsed
	result := make([]ReleaseParsed, len(releases))
	for i, r := range releases {
		result[i] = ReleaseParsed{
			ID:          r.ID,
			Name:        r.Name,
			Released:    r.Released,
			ReleaseDate: parseDateString(r.ReleaseDate),
		}
	}

	return result, nil
}

// GetIssuesForSprint retrieves issues for a sprint
func (c *jiraClient) GetIssuesForSprint(sprintID int) ([]Issue, error) {
	jql := fmt.Sprintf("sprint=%d", sprintID)
	return c.searchIssues(jql)
}

// GetIssuesForRelease retrieves issues for a release
func (c *jiraClient) GetIssuesForRelease(releaseID string) ([]Issue, error) {
	jql := fmt.Sprintf("fixVersion=%s", releaseID)
	return c.searchIssues(jql)
}

// SearchTickets performs a JQL search and returns issues
func (c *jiraClient) SearchTickets(jql string) ([]Issue, error) {
	return c.searchIssues(jql)
}

// GetIssue fetches a single ticket by key
func (c *jiraClient) GetIssue(issueKey string) (*Issue, error) {
	issues, err := c.SearchTickets(fmt.Sprintf("key = %s", issueKey))
	if err != nil {
		return nil, fmt.Errorf("failed to fetch issue %s: %w", issueKey, err)
	}
	if len(issues) == 0 {
		return nil, fmt.Errorf("issue %s not found", issueKey)
	}
	return &issues[0], nil
}

// searchIssues performs a JQL search
func (c *jiraClient) searchIssues(jql string) ([]Issue, error) {
	// Use configured story points field ID, default to customfield_10016
	storyPointsField := c.storyPointsFieldID
	if storyPointsField == "" {
		storyPointsField = "customfield_10016"
	}

	endpoint, err := buildURL(c.baseURL, "/rest/api/2/search", map[string]string{
		"jql":        jql,
		"fields":     fmt.Sprintf("summary,status,issuetype,priority,assignee,%s,components", storyPointsField),
		"maxResults": "1000",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to build URL: %w", err)
	}

	req, err := http.NewRequest("GET", endpoint, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	c.setAuth(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if resp.StatusCode == 401 || resp.StatusCode == 403 {
			return nil, fmt.Errorf("authentication failed. Your Jira token may be invalid. Please run 'jira init'")
		}
		return nil, fmt.Errorf("Jira API returned error: %d %s", resp.StatusCode, resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var issueResp IssueResponse
	if err := json.Unmarshal(body, &issueResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Post-process to extract story points from dynamic field ID if different from default
	if storyPointsField != "customfield_10016" {
		var rawResp struct {
			Issues []struct {
				Key    string          `json:"key"`
				Fields json.RawMessage `json:"fields"`
			} `json:"issues"`
		}
		if err := json.Unmarshal(body, &rawResp); err == nil {
			for i := range issueResp.Issues {
				if i < len(rawResp.Issues) {
					var fieldsMap map[string]interface{}
					if err := json.Unmarshal(rawResp.Issues[i].Fields, &fieldsMap); err == nil {
						if spValue, ok := fieldsMap[storyPointsField]; ok {
							if spFloat, ok := spValue.(float64); ok {
								issueResp.Issues[i].Fields.StoryPoints = spFloat
							}
						}
					}
				}
			}
		}
	}

	return issueResp.Issues, nil
}

// Helper function to build URL with query parameters
func buildURL(baseURL, path string, params map[string]string) (string, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	u.Path = path
	q := u.Query()
	for k, v := range params {
		q.Set(k, v)
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// parseDateString parses a date string from Jira API
func parseDateString(dateStr string) time.Time {
	if dateStr == "" {
		return time.Time{}
	}
	// Try common Jira date formats
	formats := []string{
		"2006-01-02T15:04:05.000Z",
		"2006-01-02T15:04:05Z",
		"2006-01-02",
	}
	for _, format := range formats {
		if parsed, err := time.Parse(format, dateStr); err == nil {
			return parsed
		}
	}
	return time.Time{}
}

// SearchUsers searches for users in Jira
func (c *jiraClient) SearchUsers(query string) ([]User, error) {
	if users := c.getCachedUsers(query); users != nil {
		return users, nil
	}

	body, resp, err := c.searchUsersWithFallback(query)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if err := c.validateUserSearchResponse(resp, body); err != nil {
		return nil, err
	}

	users, err := c.parseUserSearchResponse(body)
	if err != nil {
		return nil, err
	}

	c.saveUsersToCache(query, users)
	return users, nil
}

func (c *jiraClient) getCachedUsers(query string) []User {
	if c.noCache {
		return nil
	}

	c.cache.mu.RLock()
	defer c.cache.mu.RUnlock()

	if users, ok := c.cache.Users[query]; ok && len(users) > 0 {
		userCopy := make([]User, len(users))
		copy(userCopy, users)
		return userCopy
	}
	return nil
}

func (c *jiraClient) searchUsersWithFallback(query string) ([]byte, *http.Response, error) {
	body, resp, err := c.trySearchUsersV2(query)
	if err == nil && resp.StatusCode >= 200 && resp.StatusCode < 300 && !isHTMLResponse(body) {
		return body, resp, nil
	}

	return c.trySearchUsersV3(query, body)
}

func (c *jiraClient) trySearchUsersV2(query string) ([]byte, *http.Response, error) {
	endpoint, err := buildURL(c.baseURL, "/rest/api/2/user/search", map[string]string{
		"username": query,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to build URL: %w", err)
	}

	req, err := http.NewRequest("GET", endpoint, http.NoBody)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	c.setAuth(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to execute request: %w", err)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		resp.Body.Close()
		return nil, nil, fmt.Errorf("failed to read response: %w", err)
	}

	return body, resp, nil
}

func (c *jiraClient) trySearchUsersV3(query string, v2Body []byte) ([]byte, *http.Response, error) {
	endpoint, err := buildURL(c.baseURL, "/rest/api/3/user/search", map[string]string{
		"query": query,
	})
	if err != nil {
		return handleUserSearchError(v2Body, nil)
	}

	req, err := http.NewRequest("GET", endpoint, http.NoBody)
	if err != nil {
		return handleUserSearchError(v2Body, nil)
	}

	req.Header.Set("Accept", "application/json")
	c.setAuth(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return handleUserSearchError(v2Body, nil)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		resp.Body.Close()
		return handleUserSearchError(v2Body, nil)
	}

	if resp.StatusCode >= 200 && resp.StatusCode < 300 && !isHTMLResponse(body) {
		return body, resp, nil
	}

	if isHTMLResponse(v2Body) && isHTMLResponse(body) {
		return nil, nil, fmt.Errorf(
			"both API v2 and v3 returned HTML (endpoints may not exist). v2 response: %s",
			previewResponse(v2Body, 200))
	}

	return handleUserSearchError(body, resp)
}

func isHTMLResponse(data []byte) bool {
	dataStr := strings.TrimSpace(string(data))
	return strings.HasPrefix(dataStr, "<!DOCTYPE") ||
		strings.HasPrefix(dataStr, "<html") ||
		strings.HasPrefix(dataStr, "<HTML")
}

func previewResponse(body []byte, maxLen int) string {
	previewLen := maxLen
	if len(body) < previewLen {
		previewLen = len(body)
	}
	return string(body[:previewLen])
}

func handleUserSearchError(body []byte, resp *http.Response) ([]byte, *http.Response, error) {
	if resp != nil {
		if resp.StatusCode == 401 || resp.StatusCode == 403 {
			return nil, nil, fmt.Errorf("authentication failed. Your Jira token may be invalid. Please run 'jira init'")
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			bodyStr := string(body)
			if isHTMLResponse(body) {
				return nil, nil, fmt.Errorf(
					"Jira API returned HTML instead of JSON (endpoint may not exist). Response preview: %s",
					previewResponse(body, 500))
			}
			if bodyStr != "" && len(bodyStr) < 500 {
				return nil, nil, fmt.Errorf("Jira API returned error: %d %s - %s", resp.StatusCode, resp.Status, bodyStr)
			}
			return nil, nil, fmt.Errorf("Jira API returned error: %d %s", resp.StatusCode, resp.Status)
		}
	}

	if isHTMLResponse(body) {
		return nil, nil, fmt.Errorf(
			"Jira API returned HTML instead of JSON. The user search endpoint may not be available. Response preview: %s",
			previewResponse(body, 500))
	}

	return nil, nil, fmt.Errorf("failed to search users")
}

func (c *jiraClient) validateUserSearchResponse(resp *http.Response, body []byte) error {
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if resp.StatusCode == 401 || resp.StatusCode == 403 {
			return fmt.Errorf("authentication failed. Your Jira token may be invalid. Please run 'jira init'")
		}
		bodyStr := string(body)
		if isHTMLResponse(body) {
			return fmt.Errorf(
				"Jira API returned HTML instead of JSON (endpoint may not exist). Response preview: %s",
				previewResponse(body, 500))
		}
		if bodyStr != "" && len(bodyStr) < 500 {
			return fmt.Errorf("Jira API returned error: %d %s - %s", resp.StatusCode, resp.Status, bodyStr)
		}
		return fmt.Errorf("Jira API returned error: %d %s", resp.StatusCode, resp.Status)
	}

	if isHTMLResponse(body) {
		return fmt.Errorf(
			"Jira API returned HTML instead of JSON. The user search endpoint may not be available. Response preview: %s",
			previewResponse(body, 500))
	}

	return nil
}

func (c *jiraClient) parseUserSearchResponse(body []byte) ([]User, error) {
	var users []User
	if err := json.Unmarshal(body, &users); err != nil {
		bodyStr := string(body)
		if len(bodyStr) > 500 {
			bodyStr = bodyStr[:500] + "..."
		}
		return nil, fmt.Errorf("failed to parse response: %w (response: %s)", err, bodyStr)
	}
	return users, nil
}

func (c *jiraClient) saveUsersToCache(query string, users []User) {
	if c.noCache {
		return
	}

	c.cache.mu.Lock()
	defer c.cache.mu.Unlock()

	if c.cache.Users == nil {
		c.cache.Users = make(map[string][]User)
	}
	c.cache.Users[query] = users
	if err := c.cache.Save(); err != nil {
		_ = err // Ignore - cache saving is optional
	}
}

// AssignTicket assigns a ticket to a user
// userAccountID can be an accountId, key, or name (email). If empty, userName will be used as the name field.
func (c *jiraClient) AssignTicket(ticketID, userAccountID, userName string) error {
	endpoint := fmt.Sprintf("%s/rest/api/2/issue/%s/assignee", c.baseURL, ticketID)

	payload, err := buildAssignmentPayload(userAccountID, userName)
	if err != nil {
		return err
	}

	resp, bodyStr, err := c.executeAssignmentRequest(endpoint, payload)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return c.handleAssignmentError(resp, bodyStr, endpoint, userAccountID, payload)
	}

	return c.checkAssignmentResponseBody(resp, bodyStr, userAccountID)
}

func buildAssignmentPayload(userAccountID, userName string) (map[string]interface{}, error) {
	payload := make(map[string]interface{})
	if userAccountID == "" {
		if userName == "" {
			return nil, fmt.Errorf("user account ID and user name cannot both be empty")
		}
		payload["name"] = userName
	} else {
		payload["accountId"] = userAccountID
	}
	return payload, nil
}

func (c *jiraClient) executeAssignmentRequest(
	endpoint string, payload map[string]interface{},
) (*http.Response, string, error) {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("PUT", endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	c.setAuth(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("failed to execute request: %w", err)
	}

	body, readErr := io.ReadAll(resp.Body)
	bodyStr := ""
	if readErr == nil {
		bodyStr = string(body)
	}

	return resp, bodyStr, nil
}

func (c *jiraClient) handleAssignmentError(
	resp *http.Response, bodyStr, endpoint, userAccountID string,
	originalPayload map[string]interface{},
) error {
	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return fmt.Errorf("authentication failed. Your Jira token may be invalid. Please run 'jira init'")
	}
	if resp.StatusCode == 404 {
		return fmt.Errorf("ticket not found")
	}
	if resp.StatusCode == 400 {
		return c.handle400AssignmentError(resp, bodyStr, endpoint, userAccountID, originalPayload)
	}
	return fmt.Errorf("Jira API returned error: %d %s - %s", resp.StatusCode, resp.Status, bodyStr)
}

func (c *jiraClient) handle400AssignmentError(
	_ *http.Response, bodyStr, endpoint, userAccountID string,
	originalPayload map[string]interface{},
) error {
	var apiError struct {
		ErrorMessages []string          `json:"errorMessages"`
		Errors        map[string]string `json:"errors"`
	}
	if err := json.Unmarshal([]byte(bodyStr), &apiError); err != nil {
		return formatRaw400Error(bodyStr, userAccountID)
	}

	if needsKeyRetry(apiError.ErrorMessages, originalPayload) {
		if retryResp, retryBodyStr, err := c.retryAssignmentWithKey(endpoint, userAccountID); err == nil {
			if retryResp.StatusCode >= 200 && retryResp.StatusCode < 300 {
				return c.checkAssignmentResponseBody(retryResp, retryBodyStr, userAccountID)
			}
			return formatAPIError(apiError.ErrorMessages, apiError.Errors, retryBodyStr, userAccountID)
		}
	}

	return formatAPIError(apiError.ErrorMessages, apiError.Errors, bodyStr, userAccountID)
}

func needsKeyRetry(errorMessages []string, payload map[string]interface{}) bool {
	if payload["accountId"] == nil {
		return false
	}
	for _, msg := range errorMessages {
		if strings.Contains(msg, "accountId") && strings.Contains(msg, "Unrecognized field") {
			return true
		}
	}
	return false
}

func (c *jiraClient) retryAssignmentWithKey(endpoint, userAccountID string) (*http.Response, string, error) {
	payload := map[string]interface{}{"key": userAccountID}
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, "", err
	}

	req, err := http.NewRequest("PUT", endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, "", err
	}

	req.Header.Set("Content-Type", "application/json")
	c.setAuth(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, "", err
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		resp.Body.Close()
		return nil, "", fmt.Errorf("failed to read response: %w", err)
	}
	return resp, string(body), nil
}

func formatRaw400Error(bodyStr, userAccountID string) error {
	if bodyStr != "" {
		if len(bodyStr) > 500 {
			bodyStr = bodyStr[:500] + "..."
		}
		return fmt.Errorf("Jira API error (400 Bad Request): %s\nAccount ID used: %s", bodyStr, userAccountID)
	}
	return fmt.Errorf("Jira API returned error: 400 Bad Request\nAccount ID used: %s", userAccountID)
}

func formatAPIError(errorMessages []string, errors map[string]string, bodyStr, userAccountID string) error {
	var errorMsgs []string
	errorMsgs = append(errorMsgs, errorMessages...)
	for k, v := range errors {
		errorMsgs = append(errorMsgs, fmt.Sprintf("%s: %s", k, v))
	}
	errorMsg := strings.Join(errorMsgs, "; ")
	if bodyStr != "" && len(bodyStr) < 500 {
		return fmt.Errorf("Jira API error: %s\nResponse: %s\nAccount ID used: %s", errorMsg, bodyStr, userAccountID)
	}
	return fmt.Errorf("Jira API error: %s\nAccount ID used: %s", errorMsg, userAccountID)
}

func (c *jiraClient) checkAssignmentResponseBody(resp *http.Response, bodyStr, userAccountID string) error {
	if bodyStr != "" && (strings.Contains(bodyStr, "\"errorMessages\"") || strings.Contains(bodyStr, "\"errors\"")) {
		var apiError struct {
			ErrorMessages []string          `json:"errorMessages"`
			Errors        map[string]string `json:"errors"`
		}
		if err := json.Unmarshal([]byte(bodyStr), &apiError); err == nil {
			if len(apiError.ErrorMessages) > 0 || len(apiError.Errors) > 0 {
				return formatAPIError(apiError.ErrorMessages, apiError.Errors, bodyStr, userAccountID)
			}
		}
	}

	if resp.StatusCode == 204 || resp.StatusCode == 200 {
		return nil
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	return fmt.Errorf("unexpected status code: %d %s", resp.StatusCode, resp.Status)
}

// UnassignTicket unassigns a ticket (removes the current assignee)
func (c *jiraClient) UnassignTicket(ticketID string) error {
	endpoint := fmt.Sprintf("%s/rest/api/2/issue/%s/assignee", c.baseURL, ticketID)

	payload := map[string]interface{}{"accountId": nil}
	resp, bodyStr, err := c.executeUnassignRequest(endpoint, payload)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return c.handleUnassignError(
			resp, bodyStr, endpoint, ticketID)
	}

	return nil
}

func (c *jiraClient) executeUnassignRequest(
	endpoint string, payload map[string]interface{},
) (*http.Response, string, error) {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("PUT", endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	c.setAuth(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("failed to execute request: %w", err)
	}

	body, readErr := io.ReadAll(resp.Body)
	bodyStr := ""
	if readErr == nil {
		bodyStr = string(body)
	}

	return resp, bodyStr, nil
}

func (c *jiraClient) handleUnassignError(resp *http.Response, bodyStr, endpoint, ticketID string) error {
	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return fmt.Errorf("authentication failed. Your Jira token may be invalid. Please run 'jira utils init'")
	}
	if resp.StatusCode == 404 {
		return fmt.Errorf("ticket %s not found", ticketID)
	}
	if resp.StatusCode == 400 {
		return c.handle400UnassignError(resp, bodyStr, endpoint)
	}
	return fmt.Errorf("Jira API returned error: %d %s - %s", resp.StatusCode, resp.Status, bodyStr)
}

func (c *jiraClient) handle400UnassignError(_ *http.Response, bodyStr, endpoint string) error {
	var apiError struct {
		ErrorMessages []string          `json:"errorMessages"`
		Errors        map[string]string `json:"errors"`
	}
	if err := json.Unmarshal([]byte(bodyStr), &apiError); err != nil {
		return formatRaw400Error(bodyStr, "")
	}

	if needsKeyRetry(apiError.ErrorMessages, map[string]interface{}{"accountId": nil}) {
		if err := c.retryUnassignWithKey(endpoint); err == nil {
			return nil
		}
	}

	return formatAPIError(apiError.ErrorMessages, apiError.Errors, bodyStr, "")
}

func (c *jiraClient) retryUnassignWithKey(endpoint string) error {
	payload := map[string]interface{}{"key": nil}
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("PUT", endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	c.setAuth(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	return fmt.Errorf("retry with key failed: %d", resp.StatusCode)
}

// GetPriorities retrieves all available priorities
func (c *jiraClient) GetPriorities() ([]Priority, error) {
	// Check cache first (unless --no-cache is set)
	if !c.noCache {
		c.cache.mu.RLock()
		if len(c.cache.Priorities) > 0 {
			priorities := make([]Priority, len(c.cache.Priorities))
			copy(priorities, c.cache.Priorities)
			c.cache.mu.RUnlock()
			return priorities, nil
		}
		c.cache.mu.RUnlock()
	}

	endpoint := fmt.Sprintf("%s/rest/api/2/priority", c.baseURL)

	req, err := http.NewRequest("GET", endpoint, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	c.setAuth(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if resp.StatusCode == 401 || resp.StatusCode == 403 {
			return nil, fmt.Errorf("authentication failed. Your Jira token may be invalid. Please run 'jira init'")
		}
		return nil, fmt.Errorf("Jira API returned error: %d %s", resp.StatusCode, resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var priorities []Priority
	if err := json.Unmarshal(body, &priorities); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Save to cache (unless --no-cache is set)
	if !c.noCache {
		c.cache.mu.Lock()
		c.cache.Priorities = priorities
		c.cache.mu.Unlock()
		if err := c.cache.Save(); err != nil {
			// Log but don't fail - caching is optional
			_ = err
		}
	}

	return priorities, nil
}

// GetComponents retrieves all components for a project
func (c *jiraClient) GetComponents(projectKey string) ([]Component, error) {
	// Check cache first (unless --no-cache is set)
	if !c.noCache {
		c.cache.mu.RLock()
		if components, ok := c.cache.Components[projectKey]; ok && len(components) > 0 {
			result := make([]Component, len(components))
			copy(result, components)
			c.cache.mu.RUnlock()
			return result, nil
		}
		c.cache.mu.RUnlock()
	}

	endpoint := fmt.Sprintf("%s/rest/api/2/project/%s/components", c.baseURL, projectKey)

	req, err := http.NewRequest("GET", endpoint, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	c.setAuth(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if resp.StatusCode == 401 || resp.StatusCode == 403 {
			return nil, fmt.Errorf("authentication failed. Your Jira token may be invalid. Please run 'jira init'")
		}
		if resp.StatusCode == 404 {
			return nil, fmt.Errorf("project %s not found", projectKey)
		}
		body, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return nil, fmt.Errorf(
				"Jira API returned error: %d %s (failed to read body: %w)",
				resp.StatusCode, resp.Status, readErr)
		}
		return nil, fmt.Errorf("Jira API returned error: %d %s - %s", resp.StatusCode, resp.Status, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var components []Component
	if err := json.Unmarshal(body, &components); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Save to cache (unless --no-cache is set)
	if !c.noCache {
		c.cache.mu.Lock()
		c.cache.Components[projectKey] = components
		c.cache.mu.Unlock()
		if err := c.cache.Save(); err != nil {
			// Log but don't fail - caching is optional
			_ = err
		}
	}

	return components, nil
}

// ClearComponentCache clears the cached components for a project
func (c *jiraClient) ClearComponentCache(projectKey string) {
	if c.cache != nil {
		c.cache.ClearComponentsForProject(projectKey)
	}
}

// UpdateTicketComponents updates the components for a ticket
func (c *jiraClient) UpdateTicketComponents(ticketID string, componentIDs []string) error {
	endpoint := fmt.Sprintf("%s/rest/api/2/issue/%s", c.baseURL, ticketID)

	// Construct component objects
	components := make([]map[string]interface{}, len(componentIDs))
	for i, id := range componentIDs {
		components[i] = map[string]interface{}{
			"id": id,
		}
	}

	// Construct the JSON payload
	payload := map[string]interface{}{
		"fields": map[string]interface{}{
			"components": components,
		},
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("PUT", endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	c.setAuth(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if resp.StatusCode == 401 || resp.StatusCode == 403 {
			return fmt.Errorf("authentication failed. Your Jira token may be invalid. Please run 'jira init'")
		}
		if resp.StatusCode == 404 {
			return fmt.Errorf("ticket %s not found", ticketID)
		}
		body, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return fmt.Errorf("Jira API returned error: %d %s (failed to read body: %w)", resp.StatusCode, resp.Status, readErr)
		}
		bodyStr := string(body)
		if resp.StatusCode == 400 {
			// Try to parse error message from response
			var apiError struct {
				ErrorMessages []string          `json:"errorMessages"`
				Errors        map[string]string `json:"errors"`
			}
			if err := json.Unmarshal(body, &apiError); err == nil {
				if len(apiError.ErrorMessages) > 0 {
					return fmt.Errorf("Jira API error: %s", strings.Join(apiError.ErrorMessages, "; "))
				}
				if len(apiError.Errors) > 0 {
					var errorMsgs []string
					for k, v := range apiError.Errors {
						errorMsgs = append(errorMsgs, fmt.Sprintf("%s: %s", k, v))
					}
					return fmt.Errorf("Jira API error: %s", strings.Join(errorMsgs, "; "))
				}
			}
		}
		return fmt.Errorf("Jira API returned error: %d %s - %s", resp.StatusCode, resp.Status, bodyStr)
	}

	return nil
}

// UpdateTicketPriority updates the priority of a ticket
func (c *jiraClient) UpdateTicketPriority(ticketID, priorityID string) error {
	endpoint := fmt.Sprintf("%s/rest/api/2/issue/%s", c.baseURL, ticketID)

	// Construct the JSON payload
	payload := map[string]interface{}{
		"fields": map[string]interface{}{
			"priority": map[string]interface{}{
				"id": priorityID,
			},
		},
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("PUT", endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	c.setAuth(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if resp.StatusCode == 401 || resp.StatusCode == 403 {
			return fmt.Errorf("authentication failed. Your Jira token may be invalid. Please run 'jira init'")
		}
		if resp.StatusCode == 404 {
			return fmt.Errorf("ticket %s not found", ticketID)
		}
		return fmt.Errorf("Jira API returned error: %d %s", resp.StatusCode, resp.Status)
	}

	return nil
}
