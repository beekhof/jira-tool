package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/beekhof/jira-tool/pkg/config"
	"github.com/beekhof/jira-tool/pkg/gemini"
	"github.com/beekhof/jira-tool/pkg/jira"
	"github.com/beekhof/jira-tool/pkg/review"

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

	// Get ticket filter
	filter := GetTicketFilter(cfg)

	// If a specific ticket ID is provided, fetch just that one
	if len(args) == 1 {
		ticketID := normalizeTicketID(args[0], cfg.DefaultProject)
		jql := fmt.Sprintf("key = %s", ticketID)
		jql = jira.ApplyTicketFilter(jql, filter)
		issues, err = client.SearchTickets(jql)
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

		// Apply filter
		jql = jira.ApplyTicketFilter(jql, filter)
		issues, err = client.SearchTickets(jql)
		if err != nil {
			return err
		}
	}

	if len(issues) == 0 {
		fmt.Println("No tickets found matching the criteria.")
		return nil
	}

	reader := bufio.NewReader(os.Stdin)
	configDir = GetConfigDir()

	// If only one ticket, automatically run guided workflow
	if len(issues) == 1 {
		selectedIssue := issues[0]

		// Initialize Gemini client
		var geminiClient gemini.GeminiClient
		geminiClient, err = gemini.NewClient(configDir)
		if err != nil {
			fmt.Printf("Warning: Could not initialize Gemini client: %v\n", err)
			fmt.Println("Continuing without AI features...")
			geminiClient = nil
		}

		if err := review.ProcessTicketWorkflow(client, geminiClient, reader, cfg, &selectedIssue, configDir); err != nil {
			return fmt.Errorf("workflow error: %w", err)
		}
		return nil
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

	// Initialize Gemini client
	var geminiClient gemini.GeminiClient
	geminiClient, err = gemini.NewClient(configDir)
	if err != nil {
		fmt.Printf("Warning: Could not initialize Gemini client: %v\n", err)
		fmt.Println("Continuing without AI features...")
		geminiClient = nil
	}

	// Track selected tickets
	selected := make(map[string]bool)
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

		// Count selected tickets
		selectedCount := 0
		for _, v := range selected {
			if v {
				selectedCount++
			}
		}

		// Display page header
		fmt.Printf("\n=== Page %d of %d (%d tickets, %d selected) ===\n\n",
			currentPage+1, totalPages, len(issues), selectedCount)

		// Display tickets in a table format
		fmt.Printf("%-4s %-12s %-10s %-50s %-12s %-20s %-8s\n",
			"#", "Key", "Type", "Summary", "Priority", "Assignee", "Status")
		fmt.Println(strings.Repeat("-", 120))

		for i := range pageIssues {
			issue := &pageIssues[i]
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

			// Get issue type
			issueType := issue.Fields.IssueType.Name

			// Truncate summary if too long
			summary := issue.Fields.Summary
			if len(summary) > 48 {
				summary = summary[:45] + "..."
			}

			// Mark if selected or acted on
			marker := ""
			if selected[issue.Key] {
				marker = "✓ "
			} else if actedOn[issue.Key] {
				marker = "• "
			}

			fmt.Printf("%-4d %-12s %-10s %-50s %-12s %-20s %-8s %s\n",
				idx, issue.Key, issueType, summary, priority, assignee, issue.Fields.Status.Name, marker)
		}

		fmt.Println()
		fmt.Printf("Actions: [1-%d] toggle ticket | [m]ark all | [u]nmark all | "+
			"[r]eview selected | [n]ext | [p]rev | [q]uit\n", len(pageIssues))
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
			for i := range pageIssues {
				selected[pageIssues[i].Key] = true
			}
			fmt.Printf("Marked %d tickets on this page.\n", len(pageIssues))
			continue
		}

		if input == "u" || input == "unmark all" {
			// Unmark all tickets on current page
			for i := range pageIssues {
				selected[pageIssues[i].Key] = false
			}
			fmt.Printf("Unmarked %d tickets on this page.\n", len(pageIssues))
			continue
		}

		if input == "r" || input == "review" {
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
			// Initialize Gemini client if not already done
			if geminiClient == nil {
				geminiClient, err = gemini.NewClient(configDir)
				if err != nil {
					fmt.Printf("Warning: Could not initialize Gemini client: %v\n", err)
					fmt.Println("Continuing without AI features...")
					geminiClient = nil
				}
			}
			return reviewSelectedTickets(client, geminiClient, reader, cfg, issues, selected, actedOn, configDir)
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

// reviewSelectedTickets processes each selected ticket through the guided workflow
func reviewSelectedTickets(
	client jira.JiraClient,
	geminiClient gemini.GeminiClient,
	reader *bufio.Reader,
	cfg *config.Config,
	allIssues []jira.Issue,
	selected, actedOn map[string]bool,
	configDir string,
) error {
	// Get list of selected tickets
	selectedTickets := []jira.Issue{}
	for i := range allIssues {
		issue := &allIssues[i]
		if selected[issue.Key] {
			selectedTickets = append(selectedTickets, *issue)
		}
	}

	if len(selectedTickets) == 0 {
		return fmt.Errorf("no tickets selected")
	}

	fmt.Printf("\nReviewing %d ticket(s)...\n\n", len(selectedTickets))

	for i := range selectedTickets {
		ticket := &selectedTickets[i]
		fmt.Printf("=== [%d/%d] %s - %s ===\n", i+1, len(selectedTickets), ticket.Key, ticket.Fields.Summary)

		if err := review.ProcessTicketWorkflow(client, geminiClient, reader, cfg, ticket, configDir); err != nil {
			fmt.Printf("Error in workflow for %s: %v\n", ticket.Key, err)
			fmt.Print("Continue with next ticket? [Y/n] ")
			response, readErr := reader.ReadString('\n')
			if readErr != nil {
				return fmt.Errorf("failed to read response: %w", readErr)
			}
			response = strings.TrimSpace(strings.ToLower(response))
			if response == "n" || response == "no" {
				return fmt.Errorf("review canceled")
			}
			continue
		}

		// Mark as acted on and clear selection
		actedOn[ticket.Key] = true
		selected[ticket.Key] = false
		fmt.Printf("✓ Completed review for %s\n\n", ticket.Key)
	}

	return nil
}

// Helper functions to safely get priority and assignee names
func getPriorityName(issue *jira.Issue) string {
	if issue.Fields.Priority.Name != "" {
		return issue.Fields.Priority.Name
	}
	return "None"
}

func getAssigneeName(issue *jira.Issue) string {
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

func handleEstimate(client jira.JiraClient, reader *bufio.Reader, cfg *config.Config, ticketID string) error {
	// Get story point options
	storyPoints := cfg.StoryPointOptions
	if len(storyPoints) == 0 {
		storyPoints = []int{1, 2, 3, 5, 8, 13}
	}

	// Get ticket filter
	filter := GetTicketFilter(cfg)

	// Fetch ticket details for Gemini estimation
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
