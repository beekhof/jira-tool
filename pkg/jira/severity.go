package jira

import (
	"bytes"
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

// UpdateTicketSeverity updates the severity field for a ticket
func (c *jiraClient) UpdateTicketSeverity(ticketID, severityFieldID, severityValue string) error {
	endpoint := fmt.Sprintf("%s/rest/api/2/issue/%s", c.baseURL, ticketID)

	// Severity fields typically require a value object, but some may accept a string directly
	// Try value object format first (most common for select list fields)
	payload := map[string]interface{}{
		"fields": map[string]interface{}{
			severityFieldID: map[string]interface{}{
				"value": severityValue,
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

	// Check response status
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		bodyStr := string(body)

		if resp.StatusCode == 401 || resp.StatusCode == 403 {
			return fmt.Errorf("authentication failed. Your Jira token may be invalid. Please run 'jira init'")
		}
		if resp.StatusCode == 404 {
			return fmt.Errorf("ticket %s not found", ticketID)
		}
		if resp.StatusCode == 400 {
			// Try value object format failed, try direct string format
			payload2 := map[string]interface{}{
				"fields": map[string]interface{}{
					severityFieldID: severityValue,
				},
			}

			jsonData2, err := json.Marshal(payload2)
			if err == nil {
				req2, err := http.NewRequest("PUT", endpoint, bytes.NewBuffer(jsonData2))
				if err == nil {
					req2.Header.Set("Content-Type", "application/json")
					c.setAuth(req2)

					resp2, err := c.httpClient.Do(req2)
					if err == nil {
						defer resp2.Body.Close()
						if resp2.StatusCode >= 200 && resp2.StatusCode < 300 {
							return nil // Success with direct string format
						}
					}
				}
			}

			// Both formats failed, parse error message
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
			// Check if it's an invalid value error
			if strings.Contains(bodyStr, "value") || strings.Contains(bodyStr, "invalid") || strings.Contains(bodyStr, "not allowed") {
				return fmt.Errorf("invalid severity value '%s'. Please check that the value matches one of the allowed values for field %s", severityValue, severityFieldID)
			}
			if strings.Contains(bodyStr, "customfield") || strings.Contains(bodyStr, "field") {
				return fmt.Errorf("Jira API error: %d %s - %s\nNote: The severity field ID (%s) may be incorrect for your Jira instance. You can configure it in your config file with 'severity_field_id'.", resp.StatusCode, resp.Status, bodyStr, severityFieldID)
			}
			return fmt.Errorf("Jira API returned error: %d %s - %s", resp.StatusCode, resp.Status, bodyStr)
		}
		return fmt.Errorf("Jira API returned error: %d %s - %s", resp.StatusCode, resp.Status, bodyStr)
	}

	return nil
}

