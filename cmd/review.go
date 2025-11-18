package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"go-jira-helper/pkg/config"
	"go-jira-helper/pkg/editor"
	"go-jira-helper/pkg/gemini"
	"go-jira-helper/pkg/jira"
	"go-jira-helper/pkg/qa"

	"github.com/spf13/cobra"
)

var (
	needsDetailFlag bool
	unassignedFlag  bool
	untriagedFlag   bool
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
	client, err := jira.NewClient(configDir)
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

	reader := bufio.NewReader(os.Stdin)

	// Loop through each ticket
	for _, issue := range issues {
		// Get priority and assignee info
		priority := "None"
		assignee := "None"
		// Note: We'd need to expand the Issue struct to include these fields
		// For now, we'll show a simplified version

		// Print compact summary
		fmt.Printf("\n%s: \"%s\" (Priority: %s, Assignee: %s) -> Action? [a(ssign), t(riage), d(etail), e(stimate), s(kip), q(uit)] ",
			issue.Key, issue.Fields.Summary, priority, assignee)

		// Read user input
		input, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read input: %w", err)
		}
		input = strings.TrimSpace(strings.ToLower(input))

		switch input {
		case "a", "assign":
			if err := handleAssign(client, reader, cfg, issue.Key); err != nil {
				fmt.Printf("Error assigning ticket: %v\n", err)
			}
		case "t", "triage":
			if err := handleTriage(client, reader, issue.Key); err != nil {
				fmt.Printf("Error triaging ticket: %v\n", err)
			}
		case "d", "detail":
			if err := handleDetail(client, reader, issue.Key, issue.Fields.Summary); err != nil {
				fmt.Printf("Error adding detail: %v\n", err)
			}
		case "e", "estimate":
			if err := handleEstimate(client, reader, cfg, issue.Key); err != nil {
				fmt.Printf("Error estimating ticket: %v\n", err)
			}
		case "s", "skip":
			continue
		case "q", "quit":
			return nil
		default:
			fmt.Println("Invalid action. Skipping...")
		}
	}

	return nil
}

func handleAssign(client jira.JiraClient, reader *bufio.Reader, cfg *config.Config, ticketID string) error {
	// Show favorites list
	favorites := cfg.FavoriteAssignees
	if len(favorites) > 0 {
		fmt.Println("Favorite assignees:")
		for i, fav := range favorites {
			fmt.Printf("[%d] %s\n", i+1, fav)
		}
		fmt.Printf("[%d] Other...\n", len(favorites)+1)
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

		if selected >= 1 && selected <= len(favorites) {
			// Parse the favorite (format: "Name (email@example.com)")
			fav := favorites[selected-1]
			// Extract email or name - for now, we'll search
			users, err := client.SearchUsers(fav)
			if err != nil {
				return err
			}
			if len(users) == 0 {
				return fmt.Errorf("user not found: %s", fav)
			}
			return client.AssignTicket(ticketID, users[0].AccountID)
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
		return err
	}

	if len(users) == 0 {
		return fmt.Errorf("no users found matching: %s", query)
	}

	// Show results
	fmt.Println("Found users:")
	for i, user := range users {
		fmt.Printf("[%d] %s (%s)\n", i+1, user.DisplayName, user.EmailAddress)
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

	return client.AssignTicket(ticketID, users[selected-1].AccountID)
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
	geminiClient, err := gemini.NewClient(configDir)
	if err != nil {
		return err
	}

	// Run Q&A flow
	description, err := qa.RunQAFlow(geminiClient, summary)
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

	// Display the Fibonacci prompt
	fmt.Println("Select story points:")
	for i, points := range storyPoints {
		fmt.Printf("[%d] %d\n", i+1, points)
	}
	fmt.Printf("[%d] Other...\n", len(storyPoints)+1)
	fmt.Print("> ")

	input, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read input: %w", err)
	}
	input = strings.TrimSpace(input)

	selected, err := strconv.Atoi(input)
	if err != nil {
		return fmt.Errorf("invalid selection: %s", input)
	}

	var points int
	if selected >= 1 && selected <= len(storyPoints) {
		points = storyPoints[selected-1]
	} else if selected == len(storyPoints)+1 {
		fmt.Print("Enter story points: ")
		customInput, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read input: %w", err)
		}
		customInput = strings.TrimSpace(customInput)
		points, err = strconv.Atoi(customInput)
		if err != nil {
			return fmt.Errorf("invalid number: %s", customInput)
		}
		if points <= 0 {
			return fmt.Errorf("story points must be positive")
		}
	} else {
		return fmt.Errorf("invalid selection: %d", selected)
	}

	return client.UpdateTicketPoints(ticketID, points)
}

func init() {
	reviewCmd.Flags().BoolVar(&needsDetailFlag, "needs-detail", false, "Show only tickets that need detail")
	reviewCmd.Flags().BoolVar(&unassignedFlag, "unassigned", false, "Show only unassigned tickets")
	reviewCmd.Flags().BoolVar(&untriagedFlag, "untriaged", false, "Show only untriaged tickets")
	rootCmd.AddCommand(reviewCmd)
}
