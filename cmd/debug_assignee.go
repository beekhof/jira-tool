package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/beekhof/jira-tool/pkg/jira"
)

// Debug function to inspect assignee field structure
func debugAssignee(ticketID string) error {
	configDir := GetConfigDir()
	client, err := jira.NewClient(configDir, GetNoCache())
	if err != nil {
		return err
	}

	// Fetch the ticket
	issues, err := client.SearchTickets(fmt.Sprintf("key = %s", ticketID))
	if err != nil {
		return err
	}

	if len(issues) == 0 {
		return fmt.Errorf("ticket %s not found", ticketID)
	}

	// Print the raw JSON structure
	issue := issues[0]
	jsonData, err := json.MarshalIndent(issue, "", "  ")
	if err != nil {
		return err
	}

	fmt.Printf("Ticket %s assignee structure:\n", ticketID)
	fmt.Println(string(jsonData))

	return nil
}

func init() {
	// This is a temporary debug function - we'll call it manually
	_ = debugAssignee
}
