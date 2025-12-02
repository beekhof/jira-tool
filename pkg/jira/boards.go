package jira

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// BoardResponse represents the response from Jira's board API
type BoardResponse struct {
	Values []Board `json:"values"`
}

// GetBoardsForProject retrieves all boards for a project
func (c *jiraClient) GetBoardsForProject(projectKey string) ([]Board, error) {
	endpoint := fmt.Sprintf("%s/rest/agile/1.0/board?projectKeyOrId=%s", c.baseURL, projectKey)

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
		if resp.StatusCode == 404 {
			// No boards found is not an error - return empty list
			return []Board{}, nil
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

	var boardResp BoardResponse
	if err := json.Unmarshal(body, &boardResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return boardResp.Values, nil
}
