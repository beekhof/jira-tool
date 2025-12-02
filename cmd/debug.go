package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/beekhof/jira-tool/pkg/jira"
	"github.com/spf13/cobra"
)

var debugCmd = &cobra.Command{
	Use:   "debug [TICKET_ID]",
	Short: "Debug: Show raw ticket data",
	Long:  `Debug command to show raw ticket data including assignee field structure.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		configDir := GetConfigDir()
		client, err := jira.NewClient(configDir, GetNoCache())
		if err != nil {
			return err
		}

		// Use reflection to access the private method, or add it to the interface
		// For now, let's fetch via SearchTickets and show what we get
		issues, err := client.SearchTickets(fmt.Sprintf("key = %s", args[0]))
		if err != nil {
			return err
		}

		if len(issues) == 0 {
			return fmt.Errorf("ticket %s not found", args[0])
		}

		// Print the assignee structure
		issue := issues[0]
		fmt.Printf("Ticket %s:\n", args[0])
		jsonData, err := json.MarshalIndent(issue, "", "  ")
		if err != nil {
			return err
		}
		fmt.Printf("Filtered representation:\n%s\n", string(jsonData))

		// Also try to get raw data
		// We need to add GetTicketRaw to the interface, but for now let's use what we have
		rawData, err := client.GetTicketRaw(args[0])
		if err != nil {
			return err
		}
		rawDataJSON, err := json.MarshalIndent(rawData, "", "  ")
		if err != nil {
			return err
		}
		fmt.Printf("\nFull issue JSON:\n%s\n", string(rawDataJSON))

		return nil
	},
}

func init() {
	utilsCmd.AddCommand(debugCmd)
}
