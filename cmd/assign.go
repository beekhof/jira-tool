package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/beekhof/jira-tool/pkg/config"
	"github.com/beekhof/jira-tool/pkg/jira"

	"github.com/spf13/cobra"
)

var assignCmd = &cobra.Command{
	Use:   "assign [TICKET_ID]",
	Short: "Assign a ticket to a user",
	Long: `Assign a Jira ticket to a user.
The ticket ID should be in the format PROJECT-NUMBER (e.g., ENG-123).

If no ticket ID is provided, shows a paginated list of unassigned tickets
where you can select tickets to assign.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runAssign,
}

func runAssign(cmd *cobra.Command, args []string) error {
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

	// If ticket ID provided, assign that single ticket
	if len(args) == 1 {
		return assignSingleTicket(client, cfg, args[0])
	}

	// Otherwise, show paginated list of unassigned tickets
	return assignMultipleTickets(client, cfg)
}

// assignSingleTicket assigns a single ticket
func assignSingleTicket(client jira.JiraClient, cfg *config.Config, ticketID string) error {
	// Fetch ticket details
	fmt.Printf("Fetching ticket details for %s...\n", ticketID)
	issues, err := client.SearchTickets(fmt.Sprintf("key = %s", ticketID))
	if err != nil {
		return fmt.Errorf("failed to fetch ticket: %w", err)
	}
	if len(issues) == 0 {
		return fmt.Errorf("ticket %s not found", ticketID)
	}

	reader := bufio.NewReader(os.Stdin)
	return handleAssign(client, reader, cfg, ticketID)
}

// assignMultipleTickets shows a paginated list and allows selecting tickets to assign
func assignMultipleTickets(client jira.JiraClient, cfg *config.Config) error {
	// Build JQL to find unassigned tickets
	project := cfg.DefaultProject
	if project == "" {
		return fmt.Errorf("default_project not configured. Please run 'jira utils init'")
	}

	jql := fmt.Sprintf("project = %s AND assignee is EMPTY ORDER BY updated DESC", project)
	allIssues, err := client.SearchTickets(jql)
	if err != nil {
		return fmt.Errorf("failed to search tickets: %w", err)
	}

	// Filter to only unassigned tickets
	issues := []jira.Issue{}
	for _, issue := range allIssues {
		if issue.Fields.Assignee.DisplayName == "" {
			issues = append(issues, issue)
		}
	}

	if len(issues) == 0 {
		fmt.Println("No unassigned tickets found.")
		return nil
	}

	// If only one ticket, automatically select it and proceed
	if len(issues) == 1 {
		return assignSingleTicket(client, cfg, issues[0].Key)
	}

	// Get page size from config (default 10)
	pageSize := cfg.ReviewPageSize
	if pageSize <= 0 {
		pageSize = 10
	}

	reader := bufio.NewReader(os.Stdin)

	// Track selected tickets
	selected := make(map[string]bool)

	// Current page index
	currentPage := 0
	totalPages := (len(issues) + pageSize - 1) / pageSize

	for {
		// Calculate page boundaries
		start := currentPage * pageSize
		end := start + pageSize
		if end > len(issues) {
			end = len(issues)
		}

		pageIssues := issues[start:end]

		// Count selected tickets
		selectedCount := 0
		for _, v := range selected {
			if v {
				selectedCount++
			}
		}

		// Display page header
		fmt.Printf("\n=== Page %d of %d (%d tickets, %d selected) ===\n\n", currentPage+1, totalPages, len(issues), selectedCount)

		// Display tickets in a table format
		fmt.Printf("%-4s %-12s %-50s %-12s %-20s %-8s\n", "#", "Key", "Summary", "Priority", "Assignee", "Status")
		fmt.Println(strings.Repeat("-", 110))

		for i, issue := range pageIssues {
			idx := start + i + 1

			// Get priority and assignee
			priority := getPriorityName(issue)
			assignee := getAssigneeName(issue)

			// Truncate summary if too long
			summary := issue.Fields.Summary
			if len(summary) > 48 {
				summary = summary[:45] + "..."
			}

			// Mark if selected
			marker := ""
			if selected[issue.Key] {
				marker = "✓ "
			}

			fmt.Printf("%-4d %-12s %-50s %-12s %-20s %-8s %s\n",
				idx, issue.Key, summary, priority, assignee, issue.Fields.Status.Name, marker)
		}

		fmt.Println()
		fmt.Printf("Actions: [1-%d] toggle ticket | [m]ark all | [u]nmark all | [a]ssign selected | [n]ext | [p]rev | [q]uit\n", len(pageIssues))
		fmt.Print("> ")

		// Read user input
		input, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read input: %w", err)
		}
		input = strings.TrimSpace(strings.ToLower(input))

		// Handle navigation
		if input == "n" || input == "next" {
			if currentPage < totalPages-1 {
				currentPage++
			} else {
				fmt.Println("Already on last page.")
			}
			continue
		}

		if input == "p" || input == "prev" {
			if currentPage > 0 {
				currentPage--
			} else {
				fmt.Println("Already on first page.")
			}
			continue
		}

		if input == "q" || input == "quit" {
			return nil
		}

		if input == "m" || input == "mark all" {
			// Mark all tickets on current page
			for _, issue := range pageIssues {
				selected[issue.Key] = true
			}
			fmt.Printf("Marked %d tickets on this page.\n", len(pageIssues))
			continue
		}

		if input == "u" || input == "unmark all" {
			// Unmark all tickets on current page
			for _, issue := range pageIssues {
				selected[issue.Key] = false
			}
			fmt.Printf("Unmarked %d tickets on this page.\n", len(pageIssues))
			continue
		}

		if input == "a" || input == "assign" {
			// Count selected tickets
			selectedCount := 0
			for _, v := range selected {
				if v {
					selectedCount++
				}
			}
			if selectedCount == 0 {
				fmt.Println("No tickets selected. Select tickets first.")
				continue
			}
			return assignSelectedTickets(client, cfg, issues, selected)
		}

		// Try to parse as ticket number
		ticketNum, err := strconv.Atoi(input)
		if err != nil {
			fmt.Println("Invalid input. Please enter a ticket number, action, or 'q' to quit.")
			continue
		}

		// Validate ticket number
		if ticketNum < 1 || ticketNum > len(issues) {
			fmt.Printf("Invalid ticket number. Please enter a number between 1 and %d.\n", len(issues))
			continue
		}

		// Toggle selection
		selectedIssue := issues[ticketNum-1]
		selected[selectedIssue.Key] = !selected[selectedIssue.Key]
		if selected[selectedIssue.Key] {
			fmt.Printf("Selected %s\n", selectedIssue.Key)
		} else {
			fmt.Printf("Deselected %s\n", selectedIssue.Key)
		}
	}
}

// assignSelectedTickets assigns each selected ticket one by one
func assignSelectedTickets(client jira.JiraClient, cfg *config.Config, allIssues []jira.Issue, selected map[string]bool) error {
	// Get list of selected tickets
	selectedTickets := []jira.Issue{}
	for _, issue := range allIssues {
		if selected[issue.Key] {
			selectedTickets = append(selectedTickets, issue)
		}
	}

	if len(selectedTickets) == 0 {
		return fmt.Errorf("no tickets selected")
	}

	fmt.Printf("\nAssigning %d ticket(s)...\n\n", len(selectedTickets))

	reader := bufio.NewReader(os.Stdin)

	for i, ticket := range selectedTickets {
		fmt.Printf("=== [%d/%d] %s - %s ===\n", i+1, len(selectedTickets), ticket.Key, ticket.Fields.Summary)

		if err := handleAssign(client, reader, cfg, ticket.Key); err != nil {
			fmt.Printf("Error assigning %s: %v\n", ticket.Key, err)
			fmt.Print("Continue with next ticket? [Y/n] ")
			response, _ := reader.ReadString('\n')
			response = strings.TrimSpace(strings.ToLower(response))
			if response == "n" || response == "no" {
				return fmt.Errorf("assignment cancelled")
			}
			continue
		}

		fmt.Printf("Assigned %s successfully.\n\n", ticket.Key)
	}

	fmt.Println("Assignment complete!")
	return nil
}

func init() {
	rootCmd.AddCommand(assignCmd)
}

