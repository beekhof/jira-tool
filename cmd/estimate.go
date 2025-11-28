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

	"github.com/spf13/cobra"
)

var estimateCmd = &cobra.Command{
	Use:   "estimate [TICKET_ID]",
	Short: "Estimate story points for a ticket",
	Long: `Estimate story points for a Jira ticket using a Fibonacci sequence prompt.
The ticket ID should be in the format PROJECT-NUMBER (e.g., ENG-123).

If no ticket ID is provided, shows a paginated list of tickets without story points
where you can select multiple tickets to estimate.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runEstimate,
}

func runEstimate(cmd *cobra.Command, args []string) error {
	// Get config directory
	configDir := GetConfigDir()

	// Create Jira client
	client, err := jira.NewClient(configDir, GetNoCache())
	if err != nil {
		return err
	}

	// Load config to get story point options
	configPath := config.GetConfigPath(configDir)
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Get story point options (default to Fibonacci if not configured)
	storyPoints := cfg.StoryPointOptions
	if len(storyPoints) == 0 {
		storyPoints = []int{1, 2, 3, 5, 8, 13}
	}

	// If ticket ID provided, estimate that single ticket
	if len(args) == 1 {
		return estimateSingleTicket(client, cfg, args[0], storyPoints, configDir)
	}

	// Otherwise, show paginated list of tickets without story points
	return estimateMultipleTickets(client, cfg, storyPoints, configDir)
}

// estimateSingleTicket estimates a single ticket
func estimateSingleTicket(client jira.JiraClient, cfg *config.Config, ticketID string, storyPoints []int, configDir string) error {
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
	reader := bufio.NewReader(os.Stdin)
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

	// Update the ticket
	if err := client.UpdateTicketPoints(ticketID, points); err != nil {
		return err
	}

	fmt.Printf("Updated %s with %d story points.\n", ticketID, points)
	return nil
}

// estimateMultipleTickets shows a paginated list and allows selecting multiple tickets
func estimateMultipleTickets(client jira.JiraClient, cfg *config.Config, storyPoints []int, configDir string) error {
	// Get story points field ID from config
	storyPointsFieldID := cfg.StoryPointsFieldID
	if storyPointsFieldID == "" {
		storyPointsFieldID = "customfield_10016"
	}

	// Build JQL to find tickets without story points
	project := cfg.DefaultProject
	if project == "" {
		return fmt.Errorf("default_project not configured. Please run 'jira init'")
	}

	jql := fmt.Sprintf("project = %s AND %s is EMPTY ORDER BY updated DESC", project, storyPointsFieldID)
	// Apply ticket filter
	filter := GetTicketFilter(cfg)
	jql = jira.ApplyTicketFilter(jql, filter)
	allIssues, err := client.SearchTickets(jql)
	if err != nil {
		return fmt.Errorf("failed to search tickets: %w", err)
	}

	// Filter to only tickets without story points (in case the field ID doesn't match)
	issues := []jira.Issue{}
	for _, issue := range allIssues {
		if issue.Fields.StoryPoints == 0 {
			issues = append(issues, issue)
		}
	}

	if len(issues) == 0 {
		fmt.Println("No tickets found without story points.")
		return nil
	}

	// If only one ticket, automatically select it and proceed
	if len(issues) == 1 {
		selected := make(map[string]bool)
		selected[issues[0].Key] = true
		return estimateSelectedTickets(client, cfg, issues, selected, storyPoints, configDir)
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
		fmt.Printf("Actions: [1-%d] toggle ticket | [m]ark all | [u]nmark all | [e]stimate selected | [n]ext | [p]rev | [q]uit\n", len(pageIssues))
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

		if input == "e" || input == "estimate" {
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
			return estimateSelectedTickets(client, cfg, issues, selected, storyPoints, configDir)
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

// estimateSelectedTickets estimates each selected ticket one by one
func estimateSelectedTickets(client jira.JiraClient, cfg *config.Config, allIssues []jira.Issue, selected map[string]bool, storyPoints []int, configDir string) error {
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

	fmt.Printf("\nEstimating %d ticket(s)...\n\n", len(selectedTickets))

	reader := bufio.NewReader(os.Stdin)
	geminiClient, err := gemini.NewClient(configDir)
	if err != nil {
		fmt.Printf("Warning: Could not initialize Gemini client: %v\n", err)
		fmt.Println("Continuing without AI estimates...")
		geminiClient = nil
	}

	for i, ticket := range selectedTickets {
		fmt.Printf("=== [%d/%d] %s - %s ===\n", i+1, len(selectedTickets), ticket.Key, ticket.Fields.Summary)

		summary := ticket.Fields.Summary
		description, err := client.GetTicketDescription(ticket.Key)
		if err != nil {
			description = ""
		}

		// Get Gemini estimate if available
		if geminiClient != nil {
			fmt.Println("Getting AI story point estimate...")
			estimate, reasoning, err := geminiClient.EstimateStoryPoints(summary, description, storyPoints)
			if err != nil {
				fmt.Printf("Warning: Could not get AI estimate: %v\n", err)
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
		for j, points := range storyPoints {
			letter := string(rune('a' + j))
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
				fmt.Printf("Invalid: story points must be positive. Skipping %s.\n", ticket.Key)
				continue
			}
			points = num
		} else if len(input) == 1 {
			// Try to parse as letter
			letter := input[0]
			index := int(letter - 'a')
			if index >= 0 && index < len(storyPoints) {
				points = storyPoints[index]
			} else {
				fmt.Printf("Invalid selection: %s. Skipping %s.\n", input, ticket.Key)
				continue
			}
		} else {
			fmt.Printf("Invalid input: %s. Skipping %s.\n", input, ticket.Key)
			continue
		}

		// Update the ticket
		if err := client.UpdateTicketPoints(ticket.Key, points); err != nil {
			fmt.Printf("Error updating %s: %v\n", ticket.Key, err)
			continue
		}

		fmt.Printf("Updated %s with %d story points.\n\n", ticket.Key, points)
	}

	fmt.Println("Estimation complete!")
	return nil
}

func init() {
	rootCmd.AddCommand(estimateCmd)
}
