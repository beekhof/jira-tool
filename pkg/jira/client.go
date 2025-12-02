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
		return strings.HasPrefix(dataStr, "<!DOCTYPE") ||
			strings.HasPrefix(dataStr, "<html") ||
			strings.HasPrefix(dataStr, "<HTML")
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

	req, err := http.NewRequest("GET", endpoint, http.NoBody)
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
			req, err := http.NewRequest("GET", endpoint, http.NoBody)
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
					} else if isHTML(body) && isHTML(body2) {
						// v3 also failed or returned HTML
						previewLen := 200
						if len(body) < previewLen {
							previewLen = len(body)
						}
						return nil, fmt.Errorf(
							"both API v2 and v3 returned HTML (endpoints may not exist). v2 response: %s",
							string(body[:previewLen]))
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
			return nil, fmt.Errorf(
				"Jira API returned HTML instead of JSON (endpoint may not exist). Response preview: %s",
				bodyStr[:previewLen])
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
		return nil, fmt.Errorf(
			"Jira API returned HTML instead of JSON. The user search endpoint may not be available. Response preview: %s",
			string(body[:previewLen]))
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
// userAccountID can be an accountId, key, or name (email). If empty, userName will be used as the name field.
func (c *jiraClient) AssignTicket(ticketID, userAccountID, userName string) error {
	endpoint := fmt.Sprintf("%s/rest/api/2/issue/%s/assignee", c.baseURL, ticketID)

	// Construct the JSON payload
	// Jira Cloud uses "accountId", Server/Data Center uses "key" or "name"
	payload := make(map[string]interface{})
	if userAccountID == "" {
		if userName == "" {
			return fmt.Errorf("user account ID and user name cannot both be empty")
		}
		// Use name field when userAccountID is empty
		payload["name"] = userName
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
								var bodyStr2 string
								body2, readErr2 := io.ReadAll(resp2.Body)
								if readErr2 != nil {
									bodyStr2 = ""
								} else {
									bodyStr2 = string(body2)
								}

								if resp2.StatusCode >= 200 && resp2.StatusCode < 300 {
									// Check if body contains error messages even with success status
									if bodyStr2 != "" &&
										(strings.Contains(bodyStr2, "\"errorMessages\"") ||
											strings.Contains(bodyStr2, "\"errors\"")) {
										// Try to parse as error
										var apiError2 struct {
											ErrorMessages []string          `json:"errorMessages"`
											Errors        map[string]string `json:"errors"`
										}
										if err := json.Unmarshal(body2, &apiError2); err == nil {
											if len(apiError2.ErrorMessages) > 0 || len(apiError2.Errors) > 0 {
												var errorMsgs []string
												errorMsgs = append(errorMsgs, apiError2.ErrorMessages...)
												for k, v := range apiError2.Errors {
													errorMsgs = append(errorMsgs, fmt.Sprintf("%s: %s", k, v))
												}
												return fmt.Errorf(
													"Jira API returned error in response body: %s\nAccount ID used: %s",
													strings.Join(errorMsgs, "; "), userAccountID)
											}
										}
									}
									// Success with key!
									return nil
								}
								// Still failed, use the error from resp2
								bodyStr = bodyStr2
								resp = resp2
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

	// Read response body even on success to check for any error messages
	body, err := io.ReadAll(resp.Body)
	if err == nil && len(body) > 0 {
		bodyStr := string(body)
		// Check if body contains error messages even with success status
		if strings.Contains(bodyStr, "\"errorMessages\"") || strings.Contains(bodyStr, "\"errors\"") {
			// Try to parse as error
			var apiError struct {
				ErrorMessages []string          `json:"errorMessages"`
				Errors        map[string]string `json:"errors"`
			}
			if err := json.Unmarshal(body, &apiError); err == nil {
				if len(apiError.ErrorMessages) > 0 || len(apiError.Errors) > 0 {
					var errorMsgs []string
					errorMsgs = append(errorMsgs, apiError.ErrorMessages...)
					for k, v := range apiError.Errors {
						errorMsgs = append(errorMsgs, fmt.Sprintf("%s: %s", k, v))
					}
					return fmt.Errorf(
						"Jira API returned error in response body: %s\nAccount ID used: %s",
						strings.Join(errorMsgs, "; "), userAccountID)
				}
			}
		}
	}

	// Jira API typically returns 204 No Content for successful assignment
	// But 200 OK is also acceptable
	if resp.StatusCode == 204 || resp.StatusCode == 200 {
		return nil
	}

	// If we get here with a 2xx status but not 200/204, log it but consider it success
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	return fmt.Errorf("unexpected status code: %d %s", resp.StatusCode, resp.Status)
}

// UnassignTicket unassigns a ticket (removes the current assignee)
func (c *jiraClient) UnassignTicket(ticketID string) error {
	endpoint := fmt.Sprintf("%s/rest/api/2/issue/%s/assignee", c.baseURL, ticketID)

	// Construct the JSON payload - set assignee to null
	// Try accountId: null first (for Cloud), fallback to key: null for Server/Data Center
	payload := map[string]interface{}{
		"accountId": nil,
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
		body, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return fmt.Errorf("Jira API returned error: %d %s (failed to read body: %w)", resp.StatusCode, resp.Status, readErr)
		}
		bodyStr := string(body)

		if resp.StatusCode == 401 || resp.StatusCode == 403 {
			return fmt.Errorf("authentication failed. Your Jira token may be invalid. Please run 'jira utils init'")
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

				// If accountId is not recognized, retry with "key": null for Server/Data Center
				if needsKey {
					payload = map[string]interface{}{
						"key": nil,
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
								body, readErr2 := io.ReadAll(resp2.Body)
								if readErr2 != nil {
									bodyStr = ""
								} else {
									bodyStr = string(body)
								}
							}
						}
					}
				}

				if len(apiError.ErrorMessages) > 0 {
					errorMsg := strings.Join(apiError.ErrorMessages, "; ")
					if bodyStr != "" && len(bodyStr) < 500 {
						return fmt.Errorf("Jira API error: %s\nResponse: %s", errorMsg, bodyStr)
					}
					return fmt.Errorf("Jira API error: %s", errorMsg)
				}
				if len(apiError.Errors) > 0 {
					var errorMsgs []string
					for k, v := range apiError.Errors {
						errorMsgs = append(errorMsgs, fmt.Sprintf("%s: %s", k, v))
					}
					errorMsg := strings.Join(errorMsgs, "; ")
					if bodyStr != "" && len(bodyStr) < 500 {
						return fmt.Errorf("Jira API error: %s\nResponse: %s", errorMsg, bodyStr)
					}
					return fmt.Errorf("Jira API error: %s", errorMsg)
				}
			}
			// If parsing failed or no structured errors, show the raw response
			if bodyStr != "" {
				// Truncate if too long
				if len(bodyStr) > 500 {
					bodyStr = bodyStr[:500] + "..."
				}
				return fmt.Errorf("Jira API error (400 Bad Request): %s", bodyStr)
			}
			return fmt.Errorf("Jira API returned error: %d %s", resp.StatusCode, resp.Status)
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
