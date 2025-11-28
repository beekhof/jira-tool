package jira

import "testing"

func TestApplyTicketFilter(t *testing.T) {
	tests := []struct {
		name     string
		jql      string
		filter   string
		expected string
	}{
		{
			name:     "Simple query with filter",
			jql:      "project = PROJ",
			filter:   "assignee = currentUser()",
			expected: "(project = PROJ) AND (assignee = currentUser())",
		},
		{
			name:     "Complex query with AND/OR",
			jql:      "project = PROJ AND (status = \"In Progress\" OR status = \"To Do\")",
			filter:   "assignee = currentUser()",
			expected: "(project = PROJ AND (status = \"In Progress\" OR status = \"To Do\")) AND (assignee = currentUser())",
		},
		{
			name:     "Empty filter",
			jql:      "project = PROJ",
			filter:   "",
			expected: "project = PROJ",
		},
		{
			name:     "Empty query",
			jql:      "",
			filter:   "assignee = currentUser()",
			expected: "assignee = currentUser()",
		},
		{
			name:     "Both empty",
			jql:      "",
			filter:   "",
			expected: "",
		},
		{
			name:     "Query with parentheses already",
			jql:      "(project = PROJ OR project = ENG)",
			filter:   "status != Done",
			expected: "((project = PROJ OR project = ENG)) AND (status != Done)",
		},
		{
			name:     "Filter with complex conditions",
			jql:      "project = PROJ",
			filter:   "assignee = currentUser() AND status != Done",
			expected: "(project = PROJ) AND (assignee = currentUser() AND status != Done)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ApplyTicketFilter(tt.jql, tt.filter)
			if result != tt.expected {
				t.Errorf("ApplyTicketFilter(%q, %q) = %q, want %q", tt.jql, tt.filter, result, tt.expected)
			}
		})
	}
}

