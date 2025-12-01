package jira

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// DetectEpicLinkField attempts to auto-detect the Epic Link custom field ID
func (c *jiraClient) DetectEpicLinkField(projectKey string) (string, error) {
	endpoint := fmt.Sprintf("%s/rest/api/2/field", c.baseURL)

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
		if resp.StatusCode == 401 || resp.StatusCode == 403 {
			return "", fmt.Errorf("authentication failed. Your Jira token may be invalid. Please run 'jira init'")
		}
		return "", fmt.Errorf("Jira API returned error: %d %s", resp.StatusCode, resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	var fields []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		Type string `json:"type"`
	}

	if err := json.Unmarshal(body, &fields); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	// Search for custom fields with "Epic Link" in the name (case-insensitive)
	// Also check for field type containing "epic" (case-insensitive)
	for _, field := range fields {
		fieldNameLower := strings.ToLower(field.Name)
		fieldTypeLower := strings.ToLower(field.Type)
		
		if strings.HasPrefix(field.ID, "customfield_") {
			// Check for "Epic Link" in name
			if strings.Contains(fieldNameLower, "epic link") {
				return field.ID, nil
			}
			// Check for field type containing "epic"
			if strings.Contains(fieldTypeLower, "epic") {
				return field.ID, nil
			}
		}
	}

	// Not found is not an error - return empty string
	return "", nil
}

// IsEpic checks if a ticket is an Epic by examining its issue type
func IsEpic(issue *Issue) bool {
	if issue == nil {
		return false
	}
	return strings.EqualFold(issue.Fields.IssueType.Name, "Epic")
}

// FilterValidParentTickets filters a list of ticket keys to only include Epics and tickets with subtasks
func FilterValidParentTickets(client JiraClient, ticketKeys []string) ([]string, error) {
	var validTickets []string

	for _, ticketKey := range ticketKeys {
		// Fetch ticket to check if it's an Epic
		issue, err := client.GetIssue(ticketKey)
		if err != nil {
			// Skip tickets that can't be fetched (log but don't fail)
			continue
		}

		// Check if it's an Epic
		if IsEpic(issue) {
			validTickets = append(validTickets, ticketKey)
			continue
		}

		// Check if it has subtasks by querying for tickets with this as parent
		subtasks, err := client.SearchTickets(fmt.Sprintf("parent = %s", ticketKey))
		if err != nil {
			// Skip if search fails
			continue
		}

		// If it has subtasks, it's a valid parent
		if len(subtasks) > 0 {
			validTickets = append(validTickets, ticketKey)
		}
	}

	return validTickets, nil
}

// GetChildTickets retrieves all child tickets (subtasks and epic children) for a given ticket
// Returns a list of ticket summaries
func GetChildTickets(client JiraClient, ticketKey string, epicLinkFieldID string) ([]string, error) {
	var childSummaries []string

	// Get the ticket to check if it's an Epic
	issue, err := client.GetIssue(ticketKey)
	if err != nil {
		// If we can't get the ticket, return empty list (not an error)
		return childSummaries, nil
	}

	// Get subtasks (for any ticket type)
	subtasks, err := client.SearchTickets(fmt.Sprintf("parent = %s", ticketKey))
	if err == nil {
		for _, subtask := range subtasks {
			childSummaries = append(childSummaries, subtask.Fields.Summary)
		}
	}

	// If it's an Epic, also get tickets linked via Epic Link
	if IsEpic(issue) && epicLinkFieldID != "" {
		// Search for tickets with this Epic Link
		epicChildren, err := client.SearchTickets(fmt.Sprintf("%s = %s", epicLinkFieldID, ticketKey))
		if err == nil {
			for _, child := range epicChildren {
				// Avoid duplicates (in case a ticket is both a subtask and epic child)
				summary := child.Fields.Summary
				isDuplicate := false
				for _, existing := range childSummaries {
					if existing == summary {
						isDuplicate = true
						break
					}
				}
				if !isDuplicate {
					childSummaries = append(childSummaries, summary)
				}
			}
		}
	}

	return childSummaries, nil
}

