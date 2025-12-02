package jira

// ApplyTicketFilter applies a JQL filter to an existing JQL query.
// The filter is appended with AND, and the existing query is wrapped in parentheses.
// If filter is empty, returns the original query unchanged.
// If query is empty, returns the filter (no wrapping needed for single filter).
func ApplyTicketFilter(jql, filter string) string {
	// If filter is empty, return original query unchanged
	if filter == "" {
		return jql
	}

	// If query is empty, return filter only (no wrapping needed)
	if jql == "" {
		return filter
	}

	// Wrap existing query in parentheses and append filter with AND
	return "(" + jql + ") AND (" + filter + ")"
}
