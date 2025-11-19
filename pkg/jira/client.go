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
type JiraClient interface {
	UpdateTicketPoints(ticketID string, points int) error
	UpdateTicketDescription(ticketID, description string) error
	UpdateTicketPriority(ticketID, priorityID string) error
	CreateTicket(project, taskType, summary string) (string, error)
	CreateTicketWithParent(project, taskType, summary, parentKey string) (string, error)
	SearchTickets(jql string) ([]Issue, error)
	SearchUsers(query string) ([]User, error)
	AssignTicket(ticketID, userAccountID string) error
	GetPriorities() ([]Priority, error)
	TransitionTicket(ticketID, transitionID string) error
	GetTicketDescription(ticketID string) (string, error)
	GetTicketAttachments(ticketID string) ([]Attachment, error)
	GetTicketComments(ticketID string) ([]Comment, error)
	GetTransitions(ticketID string) ([]Transition, error)
	AddIssuesToSprint(sprintID int, issueKeys []string) error
	AddIssuesToRelease(releaseID string, issueKeys []string) error
	GetActiveSprints(boardID int) ([]SprintParsed, error)
	GetPlannedSprints(boardID int) ([]SprintParsed, error)
	GetReleases(projectKey string) ([]ReleaseParsed, error)
	GetIssuesForSprint(sprintID int) ([]Issue, error)
	GetIssuesForRelease(releaseID string) ([]Issue, error)
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
		} `json:"assignee"`
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
		body, _ := io.ReadAll(resp.Body)
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
				return fmt.Errorf("Jira API error: %d %s - %s\nNote: The story points field ID (%s) may be incorrect for your Jira instance. You can configure it in your config file with 'story_points_field_id'.", resp.StatusCode, resp.Status, bodyStr, c.storyPointsFieldID)
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
		body, _ := io.ReadAll(resp.Body)
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
		body, _ := io.ReadAll(resp.Body)
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

// GetTransitions gets available transitions for a ticket
func (c *jiraClient) GetTransitions(ticketID string) ([]Transition, error) {
	endpoint := fmt.Sprintf("%s/rest/api/2/issue/%s/transitions", c.baseURL, ticketID)

	req, err := http.NewRequest("GET", endpoint, nil)
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

// GetTicketDescription gets the description of a ticket
func (c *jiraClient) GetTicketDescription(ticketID string) (string, error) {
	endpoint := fmt.Sprintf("%s/rest/api/2/issue/%s?fields=description", c.baseURL, ticketID)

	req, err := http.NewRequest("GET", endpoint, nil)
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

	req, err := http.NewRequest("GET", endpoint, nil)
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

	req, err := http.NewRequest("GET", endpoint, nil)
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
	req, err := http.NewRequest("GET", endpoint, nil)
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

	req, err := http.NewRequest("GET", endpoint, nil)
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

// searchIssues performs a JQL search
func (c *jiraClient) searchIssues(jql string) ([]Issue, error) {
	endpoint, err := buildURL(c.baseURL, "/rest/api/2/search", map[string]string{
		"jql":        jql,
		"fields":     "summary,status,issuetype,priority,assignee,customfield_10016",
		"maxResults": "1000",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to build URL: %w", err)
	}

	req, err := http.NewRequest("GET", endpoint, nil)
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
	// Check cache first (unless --no-cache is set)
	if !c.noCache {
		c.cache.mu.RLock()
		if users, ok := c.cache.Users[query]; ok && len(users) > 0 {
			userCopy := make([]User, len(users))
			copy(userCopy, users)
			c.cache.mu.RUnlock()
			return userCopy, nil
		}
		c.cache.mu.RUnlock()
	}

	// Helper function to check if response is HTML
	isHTML := func(data []byte) bool {
		dataStr := strings.TrimSpace(string(data))
		return strings.HasPrefix(dataStr, "<!DOCTYPE") || strings.HasPrefix(dataStr, "<html") || strings.HasPrefix(dataStr, "<HTML")
	}

	// Try API v2 first (more widely supported), then v3 as fallback
	var body []byte
	var err error
	var resp *http.Response

	// Try v2 first
	endpoint, err := buildURL(c.baseURL, "/rest/api/2/user/search", map[string]string{
		"username": query,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to build URL: %w", err)
	}

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	c.setAuth(req)

	resp, err = c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err = io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check if v2 returned HTML (endpoint doesn't exist) or error
	if resp.StatusCode < 200 || resp.StatusCode >= 300 || isHTML(body) {
		// Try v3 as fallback
		endpoint, err := buildURL(c.baseURL, "/rest/api/3/user/search", map[string]string{
			"query": query,
		})
		if err == nil {
			req, err := http.NewRequest("GET", endpoint, nil)
			if err == nil {
				req.Header.Set("Accept", "application/json")
				c.setAuth(req)

				resp2, err := c.httpClient.Do(req)
				if err == nil {
					defer resp2.Body.Close()
					body2, err := io.ReadAll(resp2.Body)
					if err == nil && resp2.StatusCode >= 200 && resp2.StatusCode < 300 && !isHTML(body2) {
						// v3 worked, use it
						body = body2
						resp = resp2
					} else {
						// v3 also failed or returned HTML
						if isHTML(body) && isHTML(body2) {
							previewLen := 200
							if len(body) < previewLen {
								previewLen = len(body)
							}
							return nil, fmt.Errorf("both API v2 and v3 returned HTML (endpoints may not exist). v2 response: %s", string(body[:previewLen]))
						}
					}
				}
			}
		}
	}

	// Check if we still have an error or HTML response
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if resp.StatusCode == 401 || resp.StatusCode == 403 {
			return nil, fmt.Errorf("authentication failed. Your Jira token may be invalid. Please run 'jira init'")
		}
		bodyStr := string(body)
		if isHTML(body) {
			previewLen := 500
			if len(bodyStr) < previewLen {
				previewLen = len(bodyStr)
			}
			return nil, fmt.Errorf("Jira API returned HTML instead of JSON (endpoint may not exist). Response preview: %s", bodyStr[:previewLen])
		}
		if bodyStr != "" && len(bodyStr) < 500 {
			return nil, fmt.Errorf("Jira API returned error: %d %s - %s", resp.StatusCode, resp.Status, bodyStr)
		}
		return nil, fmt.Errorf("Jira API returned error: %d %s", resp.StatusCode, resp.Status)
	}

	// Check if response is HTML even with 200 status
	if isHTML(body) {
		previewLen := 500
		if len(body) < previewLen {
			previewLen = len(body)
		}
		return nil, fmt.Errorf("Jira API returned HTML instead of JSON. The user search endpoint may not be available. Response preview: %s", string(body[:previewLen]))
	}
	// Debug: print raw response for first 500 chars if accountId extraction fails
	var users []User
	if err := json.Unmarshal(body, &users); err != nil {
		// If unmarshaling fails, try to see what we got
		bodyStr := string(body)
		if len(bodyStr) > 500 {
			bodyStr = bodyStr[:500] + "..."
		}
		return nil, fmt.Errorf("failed to parse response: %w (response: %s)", err, bodyStr)
	}


	// Normalize AccountID - use alternative fields if accountId is empty
	// Also try to extract from raw JSON if standard fields don't work
	for i := range users {
		if users[i].AccountID == "" {
			if users[i].Key != "" {
				users[i].AccountID = users[i].Key
			} else if users[i].AccountIDAlt != "" {
				users[i].AccountID = users[i].AccountIDAlt
			} else {
				// Try to extract accountId from raw JSON response
				var rawUsers []map[string]interface{}
				if err := json.Unmarshal(body, &rawUsers); err == nil && i < len(rawUsers) {
					// Try accountId first (for Cloud)
					if accountId, ok := rawUsers[i]["accountId"].(string); ok && accountId != "" {
						users[i].AccountID = accountId
					} else if accountId, ok := rawUsers[i]["account_id"].(string); ok && accountId != "" {
						users[i].AccountID = accountId
					} else if key, ok := rawUsers[i]["key"].(string); ok && key != "" {
						// Use key for Server/Data Center instances
						users[i].AccountID = key
					}
				}
			}
		}
		// If AccountID is still empty but we have Key, use Key
		if users[i].AccountID == "" && users[i].Key != "" {
			users[i].AccountID = users[i].Key
		}
	}

	// Save to cache (unless --no-cache is set)
	if !c.noCache {
		c.cache.mu.Lock()
		if c.cache.Users == nil {
			c.cache.Users = make(map[string][]User)
		}
		c.cache.Users[query] = users
		c.cache.mu.Unlock()
		if err := c.cache.Save(); err != nil {
			// Log but don't fail - caching is optional
			_ = err
		}
	}

	return users, nil
}

// AssignTicket assigns a ticket to a user
func (c *jiraClient) AssignTicket(ticketID, userAccountID string) error {
	if userAccountID == "" {
		return fmt.Errorf("user account ID cannot be empty")
	}

	endpoint := fmt.Sprintf("%s/rest/api/2/issue/%s/assignee", c.baseURL, ticketID)

	// Construct the JSON payload
	// Jira Cloud uses "accountId", Server/Data Center uses "key" or "name"
	// If userAccountID looks like a key (starts with "JIRAUSER" or similar), use "key"
	// Otherwise, try "accountId" first, then fall back to "name" or "key"
	payload := make(map[string]interface{})
	if strings.HasPrefix(strings.ToUpper(userAccountID), "JIRAUSER") || strings.HasPrefix(strings.ToUpper(userAccountID), "USER") {
		// Looks like a Server/Data Center key format
		payload["key"] = userAccountID
	} else {
		// Try accountId first (for Cloud)
		payload["accountId"] = userAccountID
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
		// Read response body for more details
		body, _ := io.ReadAll(resp.Body)
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
				// Check if error is about accountId not being recognized (Server/Data Center issue)
				needsKey := false
				if len(apiError.ErrorMessages) > 0 {
					for _, msg := range apiError.ErrorMessages {
						if strings.Contains(msg, "accountId") && strings.Contains(msg, "Unrecognized field") {
							needsKey = true
							break
						}
					}
				}

				// If accountId is not recognized, retry with "key" instead
				if needsKey && payload["accountId"] != nil {
					// Retry with key instead of accountId
					payload = map[string]interface{}{
						"key": userAccountID,
					}
					jsonData, err := json.Marshal(payload)
					if err == nil {
						req, err := http.NewRequest("PUT", endpoint, bytes.NewBuffer(jsonData))
						if err == nil {
							req.Header.Set("Content-Type", "application/json")
							c.setAuth(req)

							resp2, err := c.httpClient.Do(req)
							if err == nil {
								defer resp2.Body.Close()
								if resp2.StatusCode >= 200 && resp2.StatusCode < 300 {
									// Success with key!
									return nil
								}
								// Still failed, read the error
								body, _ = io.ReadAll(resp2.Body)
								bodyStr = string(body)
							}
						}
					}
				}

				if len(apiError.ErrorMessages) > 0 {
					errorMsg := strings.Join(apiError.ErrorMessages, "; ")
					if bodyStr != "" && len(bodyStr) < 500 {
						return fmt.Errorf("Jira API error: %s\nResponse: %s\nAccount ID used: %s", errorMsg, bodyStr, userAccountID)
					}
					return fmt.Errorf("Jira API error: %s\nAccount ID used: %s", errorMsg, userAccountID)
				}
				if len(apiError.Errors) > 0 {
					var errorMsgs []string
					for k, v := range apiError.Errors {
						errorMsgs = append(errorMsgs, fmt.Sprintf("%s: %s", k, v))
					}
					errorMsg := strings.Join(errorMsgs, "; ")
					if bodyStr != "" && len(bodyStr) < 500 {
						return fmt.Errorf("Jira API error: %s\nResponse: %s\nAccount ID used: %s", errorMsg, bodyStr, userAccountID)
					}
					return fmt.Errorf("Jira API error: %s\nAccount ID used: %s", errorMsg, userAccountID)
				}
			}
			// If parsing failed or no structured errors, show the raw response
			if bodyStr != "" {
				// Truncate if too long
				if len(bodyStr) > 500 {
					bodyStr = bodyStr[:500] + "..."
				}
				return fmt.Errorf("Jira API error (400 Bad Request): %s\nAccount ID used: %s", bodyStr, userAccountID)
			}
			return fmt.Errorf("Jira API returned error: %d %s\nAccount ID used: %s", resp.StatusCode, resp.Status, userAccountID)
		}
		return fmt.Errorf("Jira API returned error: %d %s - %s", resp.StatusCode, resp.Status, bodyStr)
	}

	return nil
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

	req, err := http.NewRequest("GET", endpoint, nil)
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
