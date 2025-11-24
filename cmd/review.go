package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/beekhof/jira-tool/pkg/config"
	"github.com/beekhof/jira-tool/pkg/editor"
	"github.com/beekhof/jira-tool/pkg/gemini"
	"github.com/beekhof/jira-tool/pkg/jira"
	"github.com/beekhof/jira-tool/pkg/qa"

	"github.com/spf13/cobra"
)

var (
	needsDetailFlag bool
	unassignedFlag  bool
	untriagedFlag   bool
	pageSizeFlag    int
	noPagingFlag    bool
)

var reviewCmd = &cobra.Command{
	Use:   "review [TICKET_ID]",
	Short: "Review and triage tickets",
	Long: `Review tickets interactively. You can review a specific ticket by ID,
or review a queue of tickets based on filters.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runReview,
}

func runReview(cmd *cobra.Command, args []string) error {
	configDir := GetConfigDir()
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

	var issues []jira.Issue

	// If a specific ticket ID is provided, fetch just that one
	if len(args) == 1 {
		ticketID := args[0]
		issues, err = client.SearchTickets(fmt.Sprintf("key = %s", ticketID))
		if err != nil {
			return err
		}
		if len(issues) == 0 {
			return fmt.Errorf("ticket %s not found", ticketID)
		}
	} else {
		// Build JQL query based on flags
		var jqlParts []string
		project := cfg.DefaultProject
		if project != "" {
			jqlParts = append(jqlParts, fmt.Sprintf("project = %s", project))
		}

		if needsDetailFlag {
			jqlParts = append(jqlParts, "status = \"To Do\"")
		}
		if unassignedFlag {
			jqlParts = append(jqlParts, "assignee is EMPTY")
		}
		if untriagedFlag {
			jqlParts = append(jqlParts, "priority is EMPTY")
		}

		// If no flags, combine all conditions with OR
		if !needsDetailFlag && !unassignedFlag && !untriagedFlag {
			jqlParts = append(jqlParts, "(status = \"To Do\" OR assignee is EMPTY OR priority is EMPTY)")
		}

		jql := strings.Join(jqlParts, " AND ")
		if len(jqlParts) > 1 && (!needsDetailFlag && !unassignedFlag && !untriagedFlag) {
			// For default case, use OR for the conditions
			projectPart := jqlParts[0]
			conditions := strings.Join(jqlParts[1:], " OR ")
			jql = fmt.Sprintf("%s AND (%s)", projectPart, conditions)
		}

		issues, err = client.SearchTickets(jql)
		if err != nil {
			return err
		}
	}

	if len(issues) == 0 {
		fmt.Println("No tickets found matching the criteria.")
		return nil
	}

	// If only one ticket, automatically show action menu in a loop
	if len(issues) == 1 {
		selectedIssue := issues[0]
		reader := bufio.NewReader(os.Stdin)
		for {
			shouldContinue, _ := handleReviewAction(client, reader, cfg, selectedIssue, issues, 0)
			if !shouldContinue {
				return nil
			}
			// Refresh ticket data
			updated, err := client.SearchTickets(fmt.Sprintf("key = %s", selectedIssue.Key))
			if err == nil && len(updated) > 0 {
				selectedIssue = updated[0]
				issues[0] = updated[0]
			}
		}
	}

	// Determine page size: command flag > config > default
	pageSize := pageSizeFlag
	if pageSize <= 0 {
		pageSize = cfg.ReviewPageSize
		if pageSize <= 0 {
			pageSize = 10
		}
	}

	// If no-paging flag is set, set page size to total number of issues
	if noPagingFlag {
		pageSize = len(issues)
	}

	reader := bufio.NewReader(os.Stdin)

	// Track acted-on tickets
	actedOn := make(map[string]bool)

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

		// Display page header
		fmt.Printf("\n=== Page %d of %d (%d tickets) ===\n\n", currentPage+1, totalPages, len(issues))

		// Display tickets in a table format
		fmt.Printf("%-4s %-12s %-50s %-12s %-20s %-8s\n", "#", "Key", "Summary", "Priority", "Assignee", "Status")
		fmt.Println(strings.Repeat("-", 110))

		for i, issue := range pageIssues {
			idx := start + i + 1

			// Get priority and assignee
			priority := "None"
			if issue.Fields.Priority.Name != "" {
				priority = issue.Fields.Priority.Name
			}

			assignee := "Unassigned"
			if issue.Fields.Assignee.DisplayName != "" {
				assignee = issue.Fields.Assignee.DisplayName
			}

			// Truncate summary if too long
			summary := issue.Fields.Summary
			if len(summary) > 48 {
				summary = summary[:45] + "..."
			}

			// Mark if acted on
			marker := ""
			if actedOn[issue.Key] {
				marker = "✓ "
			}

			fmt.Printf("%-4d %-12s %-50s %-12s %-20s %-8s %s\n",
				idx, issue.Key, summary, priority, assignee, issue.Fields.Status.Name, marker)
		}

		fmt.Println()
		fmt.Printf("Actions: [1-%d] select ticket | [n]ext | [p]rev | [q]uit\n", len(pageIssues))
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

		// Try to parse as ticket number
		ticketNum, err := strconv.Atoi(input)
		if err != nil {
			fmt.Println("Invalid input. Please enter a ticket number, 'n' for next, 'p' for prev, or 'q' to quit.")
			continue
		}

		// Validate ticket number
		if ticketNum < 1 || ticketNum > len(issues) {
			fmt.Printf("Invalid ticket number. Please enter a number between 1 and %d.\n", len(issues))
			continue
		}

		// Get the selected ticket
		selectedIssue := issues[ticketNum-1]

		_, success := handleReviewAction(client, reader, cfg, selectedIssue, issues, ticketNum-1)
		// For multiple tickets, we always go back to the list (outer loop continues)
		// shouldContinue is only meaningful for single ticket case

		// Mark as acted on if successful
		if success {
			actedOn[selectedIssue.Key] = true
		}
		// Continue outer loop to show list again
	}
}

// handleReviewAction shows the action menu for a ticket and handles the selected action
// Returns (shouldContinue, success) - shouldContinue indicates if we should go back to the list
func handleReviewAction(client jira.JiraClient, reader *bufio.Reader, cfg *config.Config, selectedIssue jira.Issue, issues []jira.Issue, issueIndex int) (bool, bool) {
	// Get config path for saving recent selections
	configDir := GetConfigDir()
	configPath := config.GetConfigPath(configDir)

	// Show ticket details and action menu
	fmt.Printf("\nSelected: %s - %s\n", selectedIssue.Key, selectedIssue.Fields.Summary)
	fmt.Printf("Priority: %s | Assignee: %s | Status: %s\n",
		getPriorityName(selectedIssue), getAssigneeName(selectedIssue), selectedIssue.Fields.Status.Name)

	// For single ticket, don't show "back" option
	if len(issues) == 1 {
		fmt.Print("Action? [a(ssign), t(riage), d(etail), e(stimate), q(uit)] > ")
	} else {
		fmt.Print("Action? [a(ssign), t(riage), d(etail), e(stimate), b(ack)] > ")
	}

	action, err := reader.ReadString('\n')
	if err != nil {
		fmt.Printf("Error reading input: %v\n", err)
		return true, false
	}
	action = strings.TrimSpace(strings.ToLower(action))

	success := false
	switch action {
	case "a", "assign":
		if err := handleAssign(client, reader, cfg, selectedIssue.Key, configPath); err != nil {
			fmt.Printf("Error assigning ticket: %v\n", err)
		} else {
			success = true
			fmt.Println("Ticket assigned successfully.")
		}
	case "t", "triage":
		if err := handleTriage(client, reader, selectedIssue.Key); err != nil {
			fmt.Printf("Error triaging ticket: %v\n", err)
		} else {
			success = true
			fmt.Println("Ticket triaged successfully.")
		}
	case "d", "detail":
		if err := handleDetail(client, reader, selectedIssue.Key, selectedIssue.Fields.Summary); err != nil {
			fmt.Printf("Error adding detail: %v\n", err)
		} else {
			success = true
			fmt.Println("Description updated successfully.")
		}
	case "e", "estimate":
		if err := handleEstimate(client, reader, cfg, selectedIssue.Key); err != nil {
			fmt.Printf("Error estimating ticket: %v\n", err)
		} else {
			success = true
			fmt.Println("Story points updated successfully.")
		}
	case "b", "back":
		// Just go back to the list
		return true, false
	case "q", "quit":
		// Quit (only shown for single ticket)
		return false, false
	default:
		fmt.Println("Invalid action.")
		// For single ticket, continue loop; for multiple, go back to list
		return len(issues) == 1, false
	}

	// Refresh the ticket data to show updated info
	if success && issueIndex >= 0 && issueIndex < len(issues) {
		updated, err := client.SearchTickets(fmt.Sprintf("key = %s", selectedIssue.Key))
		if err == nil && len(updated) > 0 {
			issues[issueIndex] = updated[0]
		}
	}

	// For single ticket, continue loop to allow multiple actions (return true)
	// For multiple tickets, go back to list (return false, but outer loop continues anyway)
	// shouldContinue=false means quit (only for single ticket)
	return len(issues) == 1, success
}

// Helper functions to safely get priority and assignee names
func getPriorityName(issue jira.Issue) string {
	if issue.Fields.Priority.Name != "" {
		return issue.Fields.Priority.Name
	}
	return "None"
}

func getAssigneeName(issue jira.Issue) string {
	if issue.Fields.Assignee.DisplayName != "" {
		return issue.Fields.Assignee.DisplayName
	}
	return "Unassigned"
}

func handleAssign(client jira.JiraClient, reader *bufio.Reader, cfg *config.Config, ticketID string, configPath string) error {
	// Load state for recent selections
	configDir := GetConfigDir()
	statePath := config.GetStatePath(configDir)
	state, err := config.LoadState(statePath)
	if err != nil {
		// If state can't be loaded, continue without recent list
		state = &config.State{}
	}

	// Show recent assignees list
	recent := state.RecentAssignees
	if len(recent) > 0 {
		fmt.Println("Recent assignees:")
		for i, userID := range recent {
			fmt.Printf("[%d] %s\n", i+1, userID)
		}
		fmt.Printf("[%d] Other...\n", len(recent)+1)
		fmt.Print("> ")

		choice, err := reader.ReadString('\n')
		if err != nil {
			return err
		}
		choice = strings.TrimSpace(choice)
		selected, err := strconv.Atoi(choice)
		if err != nil {
			return fmt.Errorf("invalid selection: %s", choice)
		}

		if selected >= 1 && selected <= len(recent) {
			// Use the recent user identifier
			userID := recent[selected-1]
			// Search for the user
			users, err := client.SearchUsers(userID)
			if err != nil {
				return err
			}
			if len(users) == 0 {
				return fmt.Errorf("user not found: %s", userID)
			}
			// Track this selection (move to end of recent list)
			state.AddRecentAssignee(userID)
			if err := config.SaveState(state, statePath); err != nil {
				// Log but don't fail - tracking is optional
				_ = err
			}
			return client.AssignTicket(ticketID, users[0].AccountID, users[0].Name)
		}
	}

	// Search for user
	fmt.Print("Search for user: ")
	query, err := reader.ReadString('\n')
	if err != nil {
		return err
	}
	query = strings.TrimSpace(query)

	users, err := client.SearchUsers(query)
	if err != nil {
		return fmt.Errorf("failed to search for users: %w", err)
	}

	if len(users) == 0 {
		return fmt.Errorf("no users found matching: %s", query)
	}

	// Show results
	fmt.Println("Found users:")
	for i, user := range users {
		fmt.Printf("[%d] %s (%s) [AccountID: %s]\n", i+1, user.DisplayName, user.Name, user.AccountID)
	}
	fmt.Print("Select user number: ")

	choice, err := reader.ReadString('\n')
	if err != nil {
		return err
	}
	choice = strings.TrimSpace(choice)
	selected, err := strconv.Atoi(choice)
	if err != nil {
		return fmt.Errorf("invalid selection: %s", choice)
	}

	if selected < 1 || selected > len(users) {
		return fmt.Errorf("invalid selection: %d", selected)
	}

	selectedUser := users[selected-1]

	// Track this selection - use Name if available, otherwise AccountID
	userIdentifier := selectedUser.Name
	if userIdentifier == "" {
		userIdentifier = selectedUser.AccountID
	}
	if userIdentifier != "" {
		state.AddRecentAssignee(userIdentifier)
		if err := config.SaveState(state, statePath); err != nil {
			// Log but don't fail - tracking is optional
			_ = err
		}
	}

	return client.AssignTicket(ticketID, selectedUser.AccountID, selectedUser.Name)
}

func handleTriage(client jira.JiraClient, reader *bufio.Reader, ticketID string) error {
	priorities, err := client.GetPriorities()
	if err != nil {
		return err
	}

	fmt.Println("Select priority:")
	for i, p := range priorities {
		fmt.Printf("[%d] %s\n", i+1, p.Name)
	}
	fmt.Print("> ")

	choice, err := reader.ReadString('\n')
	if err != nil {
		return err
	}
	choice = strings.TrimSpace(choice)
	selected, err := strconv.Atoi(choice)
	if err != nil {
		return fmt.Errorf("invalid selection: %s", choice)
	}

	if selected < 1 || selected > len(priorities) {
		return fmt.Errorf("invalid selection: %d", selected)
	}

	return client.UpdateTicketPriority(ticketID, priorities[selected-1].ID)
}

func handleDetail(client jira.JiraClient, reader *bufio.Reader, ticketID, summary string) error {
	configDir := GetConfigDir()

	// Load config to get max questions
	configPath := config.GetConfigPath(configDir)
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Get ticket details to check for spike (need summary and key)
	issues, err := client.SearchTickets(fmt.Sprintf("key = %s", ticketID))
	if err != nil {
		return fmt.Errorf("failed to get ticket details: %w", err)
	}
	if len(issues) == 0 {
		return fmt.Errorf("ticket %s not found", ticketID)
	}
	ticketSummary := issues[0].Fields.Summary

	geminiClient, err := gemini.NewClient(configDir)
	if err != nil {
		return err
	}

	// Run Q&A flow (pass summary to detect spike based on SPIKE prefix)
		// Get existing description if available
		existingDesc, _ := client.GetTicketDescription(ticketKey)
		description, err := qa.RunQnAFlow(geminiClient, summary, cfg.MaxQuestions, ticketSummary, existingDesc)
	if err != nil {
		return err
	}

	// Print and ask for confirmation
	fmt.Println("\nGenerated description:")
	fmt.Println("---")
	fmt.Println(description)
	fmt.Println("---")
	fmt.Print("\nUpdate ticket with this description? [Y/n/e(dit)] ")

	confirm, err := reader.ReadString('\n')
	if err != nil {
		return err
	}
	confirm = strings.TrimSpace(strings.ToLower(confirm))

	if confirm == "e" || confirm == "edit" {
		editedDescription, err := editor.OpenInEditor(description)
		if err != nil {
			return fmt.Errorf("failed to edit description: %w", err)
		}
		description = editedDescription
	}

	if confirm != "n" && confirm != "no" {
		return client.UpdateTicketDescription(ticketID, description)
	}

	return nil
}

func handleEstimate(client jira.JiraClient, reader *bufio.Reader, cfg *config.Config, ticketID string) error {
	// Get story point options
	storyPoints := cfg.StoryPointOptions
	if len(storyPoints) == 0 {
		storyPoints = []int{1, 2, 3, 5, 8, 13}
	}

	// Fetch ticket details for Gemini estimation
	fmt.Printf("Fetching ticket details for %s...\n", ticketID)
	issues, err := client.SearchTickets(fmt.Sprintf("key = %s", ticketID))
	if err != nil {
		return fmt.Errorf("failed to fetch ticket: %w", err)
	}
	if len(issues) == 0 {
		return fmt.Errorf("ticket %s not found", ticketID)
	}

	ticket := issues[0]
	summary := ticket.Fields.Summary
	description, err := client.GetTicketDescription(ticketID)
	if err != nil {
		// Description might be empty, that's okay
		description = ""
	}

	// Get Gemini estimate
	fmt.Println("Getting AI story point estimate...")
	configDir := GetConfigDir()
	geminiClient, err := gemini.NewClient(configDir)
	if err != nil {
		// If Gemini fails, continue with manual selection
		fmt.Printf("Warning: Could not initialize Gemini client: %v\n", err)
		fmt.Println("Continuing with manual selection...")
	} else {
		estimate, reasoning, err := geminiClient.EstimateStoryPoints(summary, description, storyPoints)
		if err != nil {
			fmt.Printf("Warning: Could not get AI estimate: %v\n", err)
			fmt.Println("Continuing with manual selection...")
		} else {
			fmt.Printf("\n🤖 AI Estimate: %d story points\n", estimate)
			if reasoning != "" {
				fmt.Printf("   Reasoning: %s\n", reasoning)
			}
			fmt.Println()
		}
	}

	// Display the Fibonacci prompt with letters
	fmt.Println("Select story points:")
	for i, points := range storyPoints {
		letter := string(rune('a' + i))
		fmt.Printf("[%s] %d\n", letter, points)
	}
	fmt.Println("Or enter a number directly")
	fmt.Print("> ")

	input, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read input: %w", err)
	}
	input = strings.TrimSpace(strings.ToLower(input))

	var points int
	// Try to parse as number first
	if num, err := strconv.Atoi(input); err == nil {
		// Direct number entry
		if num <= 0 {
			return fmt.Errorf("story points must be positive")
		}
		points = num
	} else if len(input) == 1 {
		// Try to parse as letter
		letter := input[0]
		index := int(letter - 'a')
		if index >= 0 && index < len(storyPoints) {
			points = storyPoints[index]
		} else {
			return fmt.Errorf("invalid selection: %s", input)
		}
	} else {
		return fmt.Errorf("invalid input: %s (use a letter or number)", input)
	}

	return client.UpdateTicketPoints(ticketID, points)
}

func init() {
	reviewCmd.Flags().BoolVar(&needsDetailFlag, "needs-detail", false, "Show only tickets that need detail")
	reviewCmd.Flags().BoolVar(&unassignedFlag, "unassigned", false, "Show only unassigned tickets")
	reviewCmd.Flags().BoolVar(&untriagedFlag, "untriaged", false, "Show only untriaged tickets")
	reviewCmd.Flags().IntVar(&pageSizeFlag, "page-size", 0, "Number of tickets per page (0 = use config default)")
	reviewCmd.Flags().BoolVar(&noPagingFlag, "no-paging", false, "Disable paging and show all tickets at once")
	rootCmd.AddCommand(reviewCmd)
}
