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

func runReview(_ *cobra.Command, args []string) error {
	configDir := GetConfigDir()
	client, err := jira.NewClient(configDir, GetNoCache())
	if err != nil {
		return err
	}

	configPath := config.GetConfigPath(configDir)
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	filter := GetTicketFilter(cfg)
	issues, err := fetchReviewTickets(client, cfg, args, filter)
	if err != nil {
		return err
	}

	if len(issues) == 0 {
		fmt.Println("No tickets found matching the criteria.")
		return nil
	}

	reader := bufio.NewReader(os.Stdin)
	if len(issues) == 1 {
		return handleSingleTicketReview(client, reader, cfg, &issues[0], configDir)
	}

	return handleMultipleTicketsReview(client, reader, cfg, issues, configDir)
}

func fetchReviewTickets(
	client jira.JiraClient, cfg *config.Config, args []string, filter string,
) ([]jira.Issue, error) {
	if len(args) == 1 {
		return fetchSingleTicket(client, cfg, args[0], filter)
	}
	return fetchTicketsByFlags(client, cfg, filter)
}

func fetchSingleTicket(client jira.JiraClient, cfg *config.Config, ticketArg, filter string) ([]jira.Issue, error) {
	ticketID := normalizeTicketID(ticketArg, cfg.DefaultProject)
	jql := fmt.Sprintf("key = %s", ticketID)
	jql = jira.ApplyTicketFilter(jql, filter)
	issues, err := client.SearchTickets(jql)
	if err != nil {
		return nil, err
	}
	if len(issues) == 0 {
		return nil, fmt.Errorf("ticket %s not found", ticketID)
	}
	return issues, nil
}

func fetchTicketsByFlags(client jira.JiraClient, cfg *config.Config, filter string) ([]jira.Issue, error) {
	jql := buildReviewJQL(cfg)
	jql = jira.ApplyTicketFilter(jql, filter)
	return client.SearchTickets(jql)
}

func buildReviewJQL(cfg *config.Config) string {
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

	if !needsDetailFlag && !unassignedFlag && !untriagedFlag {
		jqlParts = append(jqlParts, "(status = \"To Do\" OR assignee is EMPTY OR priority is EMPTY)")
	}

	if len(jqlParts) == 0 {
		return ""
	}

	if len(jqlParts) > 1 && (!needsDetailFlag && !unassignedFlag && !untriagedFlag) {
		projectPart := jqlParts[0]
		conditions := strings.Join(jqlParts[1:], " OR ")
		return fmt.Sprintf("%s AND (%s)", projectPart, conditions)
	}

	return strings.Join(jqlParts, " AND ")
}

func handleSingleTicketReview(
	client jira.JiraClient, reader *bufio.Reader, cfg *config.Config,
	issue *jira.Issue, configDir string,
) error {
	geminiClient, err := gemini.NewClient(configDir)
	if err != nil {
		fmt.Printf("Warning: Could not initialize Gemini client: %v\n", err)
		fmt.Println("Continuing without AI features...")
		geminiClient = nil
	}

	if err := review.ProcessTicketWorkflow(client, geminiClient, reader, cfg, issue, configDir); err != nil {
		return fmt.Errorf("workflow error: %w", err)
	}
	return nil
}

func handleMultipleTicketsReview(
	client jira.JiraClient, reader *bufio.Reader, cfg *config.Config,
	issues []jira.Issue, configDir string,
) error {
	pageSize := calculatePageSize(cfg)
	geminiClient := initializeGeminiClient(configDir)
	selected := make(map[string]bool)
	actedOn := make(map[string]bool)
	currentPage := 0
	totalPages := (len(issues) + pageSize - 1) / pageSize

	for {
		start := currentPage * pageSize
		end := start + pageSize
		if end > len(issues) {
			end = len(issues)
		}
		pageIssues := issues[start:end]

		displayReviewPage(pageIssues, start, currentPage+1, totalPages, len(issues), selected, actedOn)

		input, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read input: %w", err)
		}
		input = strings.TrimSpace(strings.ToLower(input))

		action, newPage, shouldQuit, shouldReview := processReviewInput(
			input, currentPage, totalPages, len(pageIssues), len(issues), selected, pageIssues)
		if shouldQuit {
			return nil
		}
		if shouldReview {
			if geminiClient == nil {
				geminiClient = initializeGeminiClient(configDir)
			}
			return reviewSelectedTickets(client, geminiClient, reader, cfg, issues, selected, actedOn, configDir)
		}
		if action == "toggle" {
			ticketNum, err := strconv.Atoi(input)
			if err == nil {
				toggleTicketSelection(selected, &issues[ticketNum-1])
			}
		}
		currentPage = newPage
	}
}

func calculatePageSize(cfg *config.Config) int {
	pageSize := pageSizeFlag
	if pageSize <= 0 {
		pageSize = cfg.ReviewPageSize
		if pageSize <= 0 {
			pageSize = 10
		}
	}
	if noPagingFlag {
		pageSize = 10000 // Large number to show all
	}
	return pageSize
}

func initializeGeminiClient(configDir string) gemini.GeminiClient {
	geminiClient, err := gemini.NewClient(configDir)
	if err != nil {
		fmt.Printf("Warning: Could not initialize Gemini client: %v\n", err)
		fmt.Println("Continuing without AI features...")
		return nil
	}
	return geminiClient
}

func displayReviewPage(
	pageIssues []jira.Issue, start, currentPage, totalPages, totalIssues int,
	selected, actedOn map[string]bool,
) {
	selectedCount := countSelected(selected)
	fmt.Printf("\n=== Page %d of %d (%d tickets, %d selected) ===\n\n",
		currentPage, totalPages, totalIssues, selectedCount)

	fmt.Printf("%-4s %-12s %-10s %-50s %-12s %-20s %-8s\n",
		"#", "Key", "Type", "Summary", "Priority", "Assignee", "Status")
	fmt.Println(strings.Repeat("-", 120))

	for i := range pageIssues {
		issue := &pageIssues[i]
		idx := start + i + 1
		priority := getPriorityName(issue)
		assignee := getAssigneeName(issue)
		issueType := issue.Fields.IssueType.Name
		summary := truncateSummary(issue.Fields.Summary, 48)
		marker := getTicketMarker(issue.Key, selected, actedOn)

		fmt.Printf("%-4d %-12s %-10s %-50s %-12s %-20s %-8s %s\n",
			idx, issue.Key, issueType, summary, priority, assignee, issue.Fields.Status.Name, marker)
	}

	fmt.Println()
	fmt.Printf("Actions: [1-%d] toggle ticket | [m]ark all | [u]nmark all | "+
		"[r]eview selected | [n]ext | [p]rev | [q]uit\n", len(pageIssues))
	fmt.Print("> ")
}

func countSelected(selected map[string]bool) int {
	count := 0
	for _, v := range selected {
		if v {
			count++
		}
	}
	return count
}

func truncateSummary(summary string, maxLen int) string {
	if len(summary) > maxLen {
		return summary[:maxLen-3] + "..."
	}
	return summary
}

func getTicketMarker(key string, selected, actedOn map[string]bool) string {
	if selected[key] {
		return "✓ "
	}
	if actedOn[key] {
		return "• "
	}
	return ""
}

func processReviewInput(
	input string, currentPage, totalPages, _, totalIssues int,
	selected map[string]bool, pageIssues []jira.Issue,
) (action string, newPage int, shouldQuit, shouldReview bool) {
	switch input {
	case "n", "next":
		if currentPage < totalPages-1 {
			return "", currentPage + 1, false, false
		}
		fmt.Println("Already on last page.")
		return "", currentPage, false, false
	case "p", "prev":
		if currentPage > 0 {
			return "", currentPage - 1, false, false
		}
		fmt.Println("Already on first page.")
		return "", currentPage, false, false
	case "q", "quit":
		return "", currentPage, true, false
	case "m", "mark all":
		for i := range pageIssues {
			selected[pageIssues[i].Key] = true
		}
		fmt.Printf("Marked %d tickets on this page.\n", len(pageIssues))
		return "", currentPage, false, false
	case "u", "unmark all":
		for i := range pageIssues {
			selected[pageIssues[i].Key] = false
		}
		fmt.Printf("Unmarked %d tickets on this page.\n", len(pageIssues))
		return "", currentPage, false, false
	case "r", "review":
		if countSelected(selected) == 0 {
			fmt.Println("No tickets selected. Select tickets first.")
			return "", currentPage, false, false
		}
		return "", currentPage, false, true
	}

	ticketNum, err := strconv.Atoi(input)
	if err != nil {
		fmt.Println("Invalid input. Please enter a ticket number, action, or 'q' to quit.")
		return "", currentPage, false, false
	}

	if ticketNum < 1 || ticketNum > totalIssues {
		fmt.Printf("Invalid ticket number. Please enter a number between 1 and %d.\n", totalIssues)
		return "", currentPage, false, false
	}

	return "toggle", currentPage, false, false
}

func toggleTicketSelection(selected map[string]bool, issue *jira.Issue) {
	selected[issue.Key] = !selected[issue.Key]
	if selected[issue.Key] {
		fmt.Printf("Selected %s\n", issue.Key)
	} else {
		fmt.Printf("Deselected %s\n", issue.Key)
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

func handleAssign(client jira.JiraClient, reader *bufio.Reader, _ *config.Config, ticketID, _ string) error {
	configDir := GetConfigDir()
	statePath := config.GetStatePath(configDir)
	state, err := config.LoadState(statePath)
	if err != nil {
		state = &config.State{}
	}

	selectedUser, userIdentifier, err := selectUserForAssignmentInReview(client, reader, state.RecentAssignees)
	if err != nil {
		return err
	}

	if userIdentifier != "" {
		state.AddRecentAssignee(userIdentifier)
		if err := config.SaveState(state, statePath); err != nil {
			_ = err // Ignore - state saving is optional
		}
	}

	return client.AssignTicket(ticketID, selectedUser.AccountID, selectedUser.Name)
}

func selectUserForAssignmentInReview(
	client jira.JiraClient, reader *bufio.Reader, recent []string,
) (jira.User, string, error) {
	if len(recent) > 0 {
		user, identifier, err := selectUserFromRecentInReview(client, reader, recent)
		if err == nil && user.AccountID != "" {
			return user, identifier, nil
		}
	}

	return selectUserFromSearchInReview(client, reader)
}

func selectUserFromRecentInReview(
	client jira.JiraClient, reader *bufio.Reader, recent []string,
) (jira.User, string, error) {
	fmt.Println("Recent assignees:")
	for i, userID := range recent {
		fmt.Printf("[%d] %s\n", i+1, userID)
	}
	fmt.Printf("[%d] Other...\n", len(recent)+1)
	fmt.Print("> ")

	choice, err := reader.ReadString('\n')
	if err != nil {
		return jira.User{}, "", err
	}
	choice = strings.TrimSpace(choice)
	selected, err := strconv.Atoi(choice)
	if err != nil {
		return jira.User{}, "", fmt.Errorf("invalid selection: %s", choice)
	}

	if selected >= 1 && selected <= len(recent) {
		userID := recent[selected-1]
		users, err := client.SearchUsers(userID)
		if err != nil {
			return jira.User{}, "", err
		}
		if len(users) == 0 {
			return jira.User{}, "", fmt.Errorf("user not found: %s", userID)
		}
		return users[0], userID, nil
	}

	return jira.User{}, "", nil
}

func selectUserFromSearchInReview(client jira.JiraClient, reader *bufio.Reader) (jira.User, string, error) {
	fmt.Print("Search for user: ")
	query, err := reader.ReadString('\n')
	if err != nil {
		return jira.User{}, "", err
	}
	query = strings.TrimSpace(query)

	users, err := client.SearchUsers(query)
	if err != nil {
		return jira.User{}, "", fmt.Errorf("failed to search for users: %w", err)
	}

	if len(users) == 0 {
		return jira.User{}, "", fmt.Errorf("no users found matching: %s", query)
	}

	fmt.Println("Found users:")
	for i, user := range users {
		fmt.Printf("[%d] %s (%s) [AccountID: %s]\n", i+1, user.DisplayName, user.Name, user.AccountID)
	}
	fmt.Print("Select user number: ")

	choice, err := reader.ReadString('\n')
	if err != nil {
		return jira.User{}, "", err
	}
	choice = strings.TrimSpace(choice)
	selected, err := strconv.Atoi(choice)
	if err != nil {
		return jira.User{}, "", fmt.Errorf("invalid selection: %s", choice)
	}

	if selected < 1 || selected > len(users) {
		return jira.User{}, "", fmt.Errorf("invalid selection: %d", selected)
	}

	selectedUser := users[selected-1]
	userIdentifier := selectedUser.Name
	if userIdentifier == "" {
		userIdentifier = selectedUser.AccountID
	}

	return selectedUser, userIdentifier, nil
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
