package cmd

import (
	"bufio"
	"fmt"
	"os"

	"github.com/beekhof/jira-tool/pkg/config"
	"github.com/beekhof/jira-tool/pkg/jira"

	"github.com/spf13/cobra"
)

// getAssigneeName is defined in review.go, but we need it here too
// We'll import it from review.go by keeping it in the same package

var (
	unassignFlag bool
)

var assignCmd = &cobra.Command{
	Use:   "assign TICKET_ID",
	Short: "Assign or unassign a ticket",
	Long: `Assign or unassign a Jira ticket.
The ticket ID should be in the format PROJECT-NUMBER (e.g., ENG-123).
If no project prefix is provided, the default project will be used.

Use --unassign flag to unassign the ticket instead of assigning it.`,
	Args: cobra.ExactArgs(1),
	RunE: runAssign,
}

func runAssign(_ *cobra.Command, args []string) error {
	// Get config directory
	configDir := GetConfigDir()

	// Create Jira client
	client, err := jira.NewClient(configDir, GetNoCache())
	if err != nil {
		return err
	}

	// Load config
	configPath := config.GetConfigPath(configDir)
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Normalize ticket ID (add default project if needed)
	ticketID := normalizeTicketID(args[0], cfg.DefaultProject)

	// Assign or unassign the ticket
	if unassignFlag {
		return unassignSingleTicket(client, ticketID)
	}
	return assignSingleTicket(client, cfg, ticketID)
}

// assignSingleTicket assigns a single ticket
func assignSingleTicket(client jira.JiraClient, cfg *config.Config, ticketID string) error {
	configDir := GetConfigDir()
	configPath := config.GetConfigPath(configDir)

	// Get ticket filter
	filter := GetTicketFilter(cfg)

	// Fetch ticket details
	fmt.Printf("Fetching ticket details for %s...\n", ticketID)
	jql := fmt.Sprintf("key = %s", ticketID)
	jql = jira.ApplyTicketFilter(jql, filter)
	issues, err := client.SearchTickets(jql)
	if err != nil {
		return fmt.Errorf("failed to fetch ticket: %w", err)
	}
	if len(issues) == 0 {
		return fmt.Errorf("ticket %s not found", ticketID)
	}

	reader := bufio.NewReader(os.Stdin)
	if err := handleAssign(client, reader, cfg, ticketID, configPath); err != nil {
		return err
	}

	// Verify assignment
	refreshJQL := fmt.Sprintf("key = %s", ticketID)
	refreshJQL = jira.ApplyTicketFilter(refreshJQL, filter)
	updated, err := client.SearchTickets(refreshJQL)
	if err == nil && len(updated) > 0 {
		assignee := getAssigneeName(&updated[0])
		if assignee != "Unassigned" {
			fmt.Printf("Assigned %s successfully to %s.\n", ticketID, assignee)
		} else {
			return fmt.Errorf("assignment reported success but ticket %s is still unassigned", ticketID)
		}
	}

	return nil
}

// unassignSingleTicket unassigns a single ticket
func unassignSingleTicket(client jira.JiraClient, ticketID string) error {
	fmt.Printf("Unassigning ticket %s...\n", ticketID)
	if err := client.UnassignTicket(ticketID); err != nil {
		return err
	}
	fmt.Printf("Unassigned %s successfully.\n", ticketID)
	return nil
}

func init() {
	rootCmd.AddCommand(assignCmd)
	assignCmd.Flags().BoolVar(&unassignFlag, "unassign", false, "Unassign the ticket instead of assigning it")
}
