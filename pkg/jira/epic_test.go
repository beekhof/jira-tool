package jira

import (
	"testing"
)

func TestIsEpic(t *testing.T) {
	tests := []struct {
		name     string
		issue    *Issue
		expected bool
	}{
		{
			name: "Epic issue type",
			issue: &Issue{
				Fields: struct {
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
						Key          string `json:"key"`
						Name         string `json:"name"`
						Active       bool   `json:"active"`
					} `json:"assignee"`
					Components []struct {
						ID   string `json:"id"`
						Name string `json:"name"`
					} `json:"components"`
					StoryPoints float64 `json:"customfield_10016"`
				}{
					IssueType: struct {
						Name string `json:"name"`
					}{Name: "Epic"},
				},
			},
			expected: true,
		},
		{
			name: "Story issue type",
			issue: &Issue{
				Fields: struct {
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
						Key          string `json:"key"`
						Name         string `json:"name"`
						Active       bool   `json:"active"`
					} `json:"assignee"`
					Components []struct {
						ID   string `json:"id"`
						Name string `json:"name"`
					} `json:"components"`
					StoryPoints float64 `json:"customfield_10016"`
				}{
					IssueType: struct {
						Name string `json:"name"`
					}{Name: "Story"},
				},
			},
			expected: false,
		},
		{
			name: "Task issue type",
			issue: &Issue{
				Fields: struct {
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
						Key          string `json:"key"`
						Name         string `json:"name"`
						Active       bool   `json:"active"`
					} `json:"assignee"`
					Components []struct {
						ID   string `json:"id"`
						Name string `json:"name"`
					} `json:"components"`
					StoryPoints float64 `json:"customfield_10016"`
				}{
					IssueType: struct {
						Name string `json:"name"`
					}{Name: "Task"},
				},
			},
			expected: false,
		},
		{
			name:     "Nil issue",
			issue:    nil,
			expected: false,
		},
		{
			name: "Case insensitive - epic lowercase",
			issue: &Issue{
				Fields: struct {
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
						Key          string `json:"key"`
						Name         string `json:"name"`
						Active       bool   `json:"active"`
					} `json:"assignee"`
					Components []struct {
						ID   string `json:"id"`
						Name string `json:"name"`
					} `json:"components"`
					StoryPoints float64 `json:"customfield_10016"`
				}{
					IssueType: struct {
						Name string `json:"name"`
					}{Name: "epic"},
				},
			},
			expected: true,
		},
		{
			name: "Case insensitive - EPIC uppercase",
			issue: &Issue{
				Fields: struct {
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
						Key          string `json:"key"`
						Name         string `json:"name"`
						Active       bool   `json:"active"`
					} `json:"assignee"`
					Components []struct {
						ID   string `json:"id"`
						Name string `json:"name"`
					} `json:"components"`
					StoryPoints float64 `json:"customfield_10016"`
				}{
					IssueType: struct {
						Name string `json:"name"`
					}{Name: "EPIC"},
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsEpic(tt.issue)
			if result != tt.expected {
				t.Errorf("IsEpic() = %v, want %v", result, tt.expected)
			}
		})
	}
}
