package jira

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
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
			AccountID   string `json:"accountId"`
			DisplayName string `json:"displayName"`
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
	baseURL    string
	httpClient *http.Client
	authToken  string
	cache      *Cache
}

// NewClient creates a new Jira client by loading config and credentials
// configDir can be empty to use the default ~/.jira-tool
func NewClient(configDir string) (JiraClient, error) {
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

	client := &jiraClient{
		baseURL:    cfg.JiraURL,
		httpClient: &http.Client{},
		authToken:  token,
		cache:      cache,
	}

	return client, nil
}

// setAuth sets the Bearer token authentication header on the request
func (c *jiraClient) setAuth(req *http.Request) {
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.authToken))
}

// UpdateTicketPoints updates the story points for a ticket
// Note: The story points field ID (customfield_10016) may need to be configured per Jira instance
func (c *jiraClient) UpdateTicketPoints(ticketID string, points int) error {
	// Construct the API endpoint
	endpoint := fmt.Sprintf("%s/rest/api/2/issue/%s", c.baseURL, ticketID)

	// Construct the JSON payload
	// Note: The field ID for story points varies by Jira instance
	// This uses a common default, but may need to be configurable
	payload := map[string]interface{}{
		"fields": map[string]interface{}{
			"customfield_10016": points,
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
	// Check cache first
	c.cache.mu.RLock()
	if users, ok := c.cache.Users[query]; ok && len(users) > 0 {
		userCopy := make([]User, len(users))
		copy(userCopy, users)
		c.cache.mu.RUnlock()
		return userCopy, nil
	}
	c.cache.mu.RUnlock()

	endpoint, err := buildURL(c.baseURL, "/rest/api/2/user/search", map[string]string{
		"query": query,
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

	var users []User
	if err := json.Unmarshal(body, &users); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Save to cache
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

	return users, nil
}

// AssignTicket assigns a ticket to a user
func (c *jiraClient) AssignTicket(ticketID, userAccountID string) error {
	endpoint := fmt.Sprintf("%s/rest/api/2/issue/%s/assignee", c.baseURL, ticketID)

	// Construct the JSON payload
	payload := map[string]interface{}{
		"accountId": userAccountID,
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

// GetPriorities retrieves all available priorities
func (c *jiraClient) GetPriorities() ([]Priority, error) {
	// Check cache first
	c.cache.mu.RLock()
	if len(c.cache.Priorities) > 0 {
		priorities := make([]Priority, len(c.cache.Priorities))
		copy(priorities, c.cache.Priorities)
		c.cache.mu.RUnlock()
		return priorities, nil
	}
	c.cache.mu.RUnlock()

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

	// Save to cache
	c.cache.mu.Lock()
	c.cache.Priorities = priorities
	c.cache.mu.Unlock()
	if err := c.cache.Save(); err != nil {
		// Log but don't fail - caching is optional
		_ = err
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
