package jira

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// DetectSeverityField attempts to auto-detect the severity custom field ID
func (c *jiraClient) DetectSeverityField(projectKey string) (string, error) {
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

	// Search for custom fields with "severity" in the name (case-insensitive)
	for _, field := range fields {
		if strings.HasPrefix(field.ID, "customfield_") && strings.Contains(strings.ToLower(field.Name), "severity") {
			return field.ID, nil
		}
	}

	// Not found is not an error - return empty string
	return "", nil
}

// GetSeverityFieldValues retrieves allowed values for a severity field
func (c *jiraClient) GetSeverityFieldValues(fieldID string) ([]string, error) {
	// First, try to get field configuration
	endpoint := fmt.Sprintf("%s/rest/api/2/field/%s", c.baseURL, fieldID)

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
		// If field endpoint doesn't work, return empty list - values may need to be configured manually
		return []string{}, nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Try to parse allowed values from field schema
	var fieldConfig struct {
		AllowedValues []struct {
			Value string `json:"value"`
		} `json:"allowedValues"`
		Schema struct {
			Type string `json:"type"`
		} `json:"schema"`
	}

	if err := json.Unmarshal(body, &fieldConfig); err == nil {
		if len(fieldConfig.AllowedValues) > 0 {
			values := make([]string, len(fieldConfig.AllowedValues))
			for i, av := range fieldConfig.AllowedValues {
				values[i] = av.Value
			}
			return values, nil
		}
	}

	// If we can't get values from field config, return empty - user may need to configure values manually
	return []string{}, nil
}

