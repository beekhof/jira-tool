package review

import (
	"bufio"
	"fmt"
	"strconv"
	"strings"

	"github.com/beekhof/jira-tool/pkg/config"
	"github.com/beekhof/jira-tool/pkg/gemini"
	"github.com/beekhof/jira-tool/pkg/jira"
)

// CheckDescriptionQuality checks if a ticket's description meets quality criteria
func CheckDescriptionQuality(
	client jira.JiraClient, ticket *jira.Issue, cfg *config.Config,
) (isValid bool, reason string, err error) {
	// Fetch description
	description, err := client.GetTicketDescription(ticket.Key)
	if err != nil {
		// If we can't get description, assume it's missing
		description = ""
	}

	// Check minimum length
	if cfg.DescriptionMinLength > 0 {
		if len(description) < cfg.DescriptionMinLength {
			return false, fmt.Sprintf("too short (%d chars, need %d)", len(description), cfg.DescriptionMinLength), nil
		}
	}

	// Optional Gemini AI analysis (not implemented yet - would require new method)
	// For now, just check length
	if cfg.DescriptionQualityAI {
		// Placeholder for future AI analysis
		// Would use Gemini to check if description answers "what", "why", "how"
		_ = description // Use description variable
	}

	return true, "", nil
}

// HandleComponentStep checks and assigns component if missing
func HandleComponentStep(
	client jira.JiraClient, reader *bufio.Reader, cfg *config.Config,
	ticket *jira.Issue, configDir string,
) (bool, error) {
	if len(ticket.Fields.Components) > 0 {
		return true, nil
	}

	statePath := config.GetStatePath(configDir)
	state, err := config.LoadState(statePath)
	if err != nil {
		state = &config.State{}
	}

	projectKey := cfg.DefaultProject
	if projectKey == "" {
		return false, fmt.Errorf("default_project not configured")
	}

	components, err := fetchComponentsWithRetry(client, reader, projectKey)
	if err != nil {
		return false, err
	}

	if len(components) == 0 {
		return true, nil
	}

	selectedFromRecent, comp, err := selectFromRecentComponents(reader, components, state.RecentComponents)
	if err != nil {
		return false, err
	}
	if selectedFromRecent {
		return updateComponentAndSave(client, ticket.Key, comp, state, statePath)
	}

	selected, err := selectFromComponentList(reader, components)
	if err != nil {
		return false, err
	}

	if selected == len(components)+2 {
		return false, nil
	}

	if selected == len(components)+1 {
		return handleComponentSearch(client, reader, ticket, projectKey, components, state, statePath)
	}

	if selected < 1 || selected > len(components) {
		return false, fmt.Errorf("invalid selection: %d", selected)
	}

	return updateComponentAndSave(client, ticket.Key, components[selected-1], state, statePath)
}

func fetchComponentsWithRetry(
	client jira.JiraClient, reader *bufio.Reader, projectKey string,
) ([]jira.Component, error) {
	components, err := client.GetComponents(projectKey)
	if err == nil {
		return components, nil
	}

	fmt.Printf("Failed to fetch components: %v\n", err)
	fmt.Print("Retry without cache? [y/N]: ")
	retryInput, err := reader.ReadString('\n')
	if err != nil {
		return nil, err
	}
	retryInput = strings.TrimSpace(strings.ToLower(retryInput))
	if retryInput != "y" && retryInput != "yes" {
		return nil, err
	}

	components, err = client.GetComponents(projectKey)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch components: %w", err)
	}
	return components, nil
}

func selectFromRecentComponents(
	reader *bufio.Reader, components []jira.Component, recent []string,
) (bool, jira.Component, error) {
	if len(recent) == 0 {
		return false, jira.Component{}, nil
	}

	fmt.Println("Recent components:")
	for i, compName := range recent {
		fmt.Printf("[%d] %s\n", i+1, compName)
	}
	fmt.Printf("[%d] Other...\n", len(recent)+1)
	fmt.Print("> ")

	choice, err := reader.ReadString('\n')
	if err != nil {
		return false, jira.Component{}, err
	}
	choice = strings.TrimSpace(choice)
	selected, err := strconv.Atoi(choice)
	if err != nil {
		return false, jira.Component{}, fmt.Errorf("invalid selection: %s", choice)
	}

	if selected >= 1 && selected <= len(recent) {
		compName := recent[selected-1]
		for _, comp := range components {
			if comp.Name == compName {
				return true, comp, nil
			}
		}
	}

	if selected != len(recent)+1 {
		return false, jira.Component{}, fmt.Errorf("invalid selection")
	}

	return false, jira.Component{}, nil
}

func selectFromComponentList(reader *bufio.Reader, components []jira.Component) (int, error) {
	fmt.Println("Select component:")
	for i, comp := range components {
		fmt.Printf("[%d] %s\n", i+1, comp.Name)
	}
	fmt.Printf("[%d] Search/Enter component name\n", len(components)+1)
	fmt.Printf("[%d] Skip\n", len(components)+2)
	fmt.Print("> ")

	choice, err := reader.ReadString('\n')
	if err != nil {
		return 0, err
	}
	choice = strings.TrimSpace(choice)
	selected, err := strconv.Atoi(choice)
	if err != nil {
		return 0, fmt.Errorf("invalid selection: %s", choice)
	}
	return selected, nil
}

func handleComponentSearch(
	client jira.JiraClient, reader *bufio.Reader, ticket *jira.Issue,
	projectKey string, components []jira.Component, state *config.State, statePath string,
) (bool, error) {
	fmt.Print("Enter component name to search for (or exact name to create): ")
	searchInput, err := reader.ReadString('\n')
	if err != nil {
		return false, err
	}
	searchInput = strings.TrimSpace(searchInput)
	if searchInput == "" {
		return false, fmt.Errorf("component name cannot be empty")
	}

	matchingComponents := findMatchingComponents(components, searchInput)
	if len(matchingComponents) == 0 {
		return handleComponentNotFound(client, reader, ticket, projectKey, searchInput, state, statePath)
	}

	if len(matchingComponents) == 1 {
		return updateComponentAndSave(client, ticket.Key, matchingComponents[0], state, statePath)
	}

	return selectFromMatchingComponents(client, reader, ticket, matchingComponents, state, statePath)
}

func findMatchingComponents(components []jira.Component, searchInput string) []jira.Component {
	var matches []jira.Component
	searchLower := strings.ToLower(searchInput)
	for _, comp := range components {
		if strings.Contains(strings.ToLower(comp.Name), searchLower) {
			matches = append(matches, comp)
		}
	}
	return matches
}

func handleComponentNotFound(
	client jira.JiraClient, _ *bufio.Reader, ticket *jira.Issue,
	projectKey, searchInput string, state *config.State, statePath string,
) (bool, error) {
	exactMatch := findExactComponentMatch(client, projectKey, searchInput)
	if exactMatch != nil {
		return updateComponentAndSave(client, ticket.Key, *exactMatch, state, statePath)
	}

	fmt.Printf("\nComponent '%s' not found in the component list.\n", searchInput)
	fmt.Println("This might be due to a stale cache. Attempting to refresh...")

	client.ClearComponentCache(projectKey)
	refreshedComponents, err := client.GetComponents(projectKey)
	if err != nil {
		return false, fmt.Errorf("failed to refresh components: %w", err)
	}

	refreshedMatch := findComponentInRefreshedList(refreshedComponents, searchInput)
	if refreshedMatch != nil {
		return updateComponentAndSave(client, ticket.Key, *refreshedMatch, state, statePath)
	}

	fmt.Println("Component still not found after refreshing the list.")
	fmt.Println("Possible reasons:")
	fmt.Println("  - Component might be archived or inactive")
	fmt.Println("  - Component might be in a different project")
	fmt.Println("  - Component name might be different in Jira")
	fmt.Println("\nTip: Try running with --no-cache flag to ensure fresh data:")
	fmt.Printf("  jira review %s --no-cache\n", ticket.Key)
	return false, fmt.Errorf(
		"component '%s' not found in project %s even after refreshing. "+
			"Please verify the component name and ensure it exists in the project",
		searchInput, projectKey)
}

func findExactComponentMatch(client jira.JiraClient, projectKey, searchInput string) *jira.Component {
	components, err := client.GetComponents(projectKey)
	if err != nil {
		return nil
	}
	for i := range components {
		if strings.EqualFold(components[i].Name, searchInput) {
			return &components[i]
		}
	}
	return nil
}

func findComponentInRefreshedList(refreshedComponents []jira.Component, searchInput string) *jira.Component {
	searchLower := strings.ToLower(searchInput)
	for i := range refreshedComponents {
		if strings.EqualFold(refreshedComponents[i].Name, searchInput) {
			return &refreshedComponents[i]
		}
		if strings.Contains(strings.ToLower(refreshedComponents[i].Name), searchLower) {
			return &refreshedComponents[i]
		}
	}
	return nil
}

func selectFromMatchingComponents(
	client jira.JiraClient, reader *bufio.Reader, ticket *jira.Issue,
	matchingComponents []jira.Component, state *config.State, statePath string,
) (bool, error) {
	fmt.Println("Found matching components:")
	for i, comp := range matchingComponents {
		fmt.Printf("[%d] %s\n", i+1, comp.Name)
	}
	fmt.Printf("[%d] Cancel\n", len(matchingComponents)+1)
	fmt.Print("> ")

	matchChoice, err := reader.ReadString('\n')
	if err != nil {
		return false, err
	}
	matchChoice = strings.TrimSpace(matchChoice)
	matchSelected, err := strconv.Atoi(matchChoice)
	if err != nil {
		return false, fmt.Errorf("invalid selection: %s", matchChoice)
	}

	if matchSelected == len(matchingComponents)+1 {
		return false, nil
	}

	if matchSelected < 1 || matchSelected > len(matchingComponents) {
		return false, fmt.Errorf("invalid selection: %d", matchSelected)
	}

	return updateComponentAndSave(client, ticket.Key, matchingComponents[matchSelected-1], state, statePath)
}

func updateComponentAndSave(
	client jira.JiraClient, ticketKey string, comp jira.Component,
	state *config.State, statePath string,
) (bool, error) {
	if err := client.UpdateTicketComponents(ticketKey, []string{comp.ID}); err != nil {
		return false, err
	}
	state.AddRecentComponent(comp.Name)
	if err := config.SaveState(state, statePath); err != nil {
		_ = err // Ignore - state saving is optional
	}
	return true, nil
}

// HandlePriorityStep checks and assigns priority if missing
func HandlePriorityStep(client jira.JiraClient, reader *bufio.Reader, ticket *jira.Issue) (bool, error) {
	// Check if priority is set
	if ticket.Fields.Priority.Name != "" {
		return true, nil // Already set
	}

	// Fetch priorities
	priorities, err := client.GetPriorities()
	if err != nil {
		return false, fmt.Errorf("failed to fetch priorities: %w", err)
	}

	fmt.Println("Select priority:")
	for i, p := range priorities {
		fmt.Printf("[%d] %s\n", i+1, p.Name)
	}
	fmt.Printf("[%d] Skip\n", len(priorities)+1)
	fmt.Print("> ")

	choice, err := reader.ReadString('\n')
	if err != nil {
		return false, err
	}
	choice = strings.TrimSpace(choice)
	selected, err := strconv.Atoi(choice)
	if err != nil {
		return false, fmt.Errorf("invalid selection: %s", choice)
	}

	if selected == len(priorities)+1 {
		// User skipped
		return false, nil
	}

	if selected < 1 || selected > len(priorities) {
		return false, fmt.Errorf("invalid selection: %d", selected)
	}

	// Update ticket
	if err := client.UpdateTicketPriority(ticket.Key, priorities[selected-1].ID); err != nil {
		return false, err
	}

	return true, nil
}

// HandleSeverityStep checks and assigns severity if configured and missing
func HandleSeverityStep(
	client jira.JiraClient, reader *bufio.Reader, cfg *config.Config, ticket *jira.Issue,
) (bool, error) {
	if cfg.SeverityFieldID == "" {
		return true, nil
	}

	if isSeverityAlreadySet(client, ticket.Key, cfg.SeverityFieldID) {
		return true, nil
	}

	values, err := getSeverityValues(client, cfg)
	if err != nil {
		return false, err
	}

	if len(values) == 0 {
		return handleSeverityWithoutValues(reader)
	}

	return selectAndSetSeverity(client, reader, ticket.Key, cfg.SeverityFieldID, values)
}

func isSeverityAlreadySet(client jira.JiraClient, ticketKey, severityFieldID string) bool {
	rawTicket, err := client.GetTicketRaw(ticketKey)
	if err != nil {
		return false
	}

	fields, ok := rawTicket["fields"].(map[string]interface{})
	if !ok {
		return false
	}

	severityValue, ok := fields[severityFieldID]
	if !ok || severityValue == nil {
		return false
	}

	currentValue := extractSeverityValue(severityValue)
	return currentValue != ""
}

func extractSeverityValue(severityValue interface{}) string {
	switch v := severityValue.(type) {
	case map[string]interface{}:
		if val, ok := v["value"].(string); ok {
			return val
		}
		if val, ok := v["name"].(string); ok {
			return val
		}
	case string:
		return v
	}
	return ""
}

func getSeverityValues(client jira.JiraClient, cfg *config.Config) ([]string, error) {
	values, err := client.GetSeverityFieldValues(cfg.SeverityFieldID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch severity values: %w", err)
	}

	if len(values) == 0 && len(cfg.SeverityValues) > 0 {
		return cfg.SeverityValues, nil
	}

	return values, nil
}

func handleSeverityWithoutValues(reader *bufio.Reader) (bool, error) {
	fmt.Println("Severity field is configured but has no predefined values.")
	fmt.Print("Set severity? [y/N] ")
	response, err := reader.ReadString('\n')
	if err != nil {
		return false, err
	}
	response = strings.TrimSpace(strings.ToLower(response))
	if response != "y" && response != "yes" {
		return false, nil
	}

	fmt.Println("Note: Setting custom severity values is not yet implemented.")
	fmt.Println("You may need to set the severity manually in Jira.")
	return false, nil
}

func selectAndSetSeverity(
	client jira.JiraClient, reader *bufio.Reader,
	ticketKey, severityFieldID string, values []string,
) (bool, error) {
	fmt.Println("Select severity:")
	for i, v := range values {
		fmt.Printf("[%d] %s\n", i+1, v)
	}
	fmt.Printf("[%d] Skip\n", len(values)+1)
	fmt.Print("> ")

	choice, err := reader.ReadString('\n')
	if err != nil {
		return false, err
	}
	choice = strings.TrimSpace(choice)
	selected, err := strconv.Atoi(choice)
	if err != nil {
		return false, fmt.Errorf("invalid selection: %s", choice)
	}

	if selected == len(values)+1 {
		return false, nil
	}

	if selected < 1 || selected > len(values) {
		return false, fmt.Errorf("invalid selection: %d", selected)
	}

	selectedValue := values[selected-1]
	if err := client.UpdateTicketSeverity(ticketKey, severityFieldID, selectedValue); err != nil {
		return false, fmt.Errorf("failed to update severity: %w", err)
	}

	fmt.Printf("Severity set to: %s\n", selectedValue)
	return true, nil
}

// HandleStoryPointsStep checks and estimates story points if missing
func HandleStoryPointsStep(
	client jira.JiraClient,
	geminiClient gemini.GeminiClient,
	reader *bufio.Reader,
	_ *config.Config,
	ticket *jira.Issue,
) (bool, error) {
	// Check if story points are set
	if ticket.Fields.StoryPoints > 0 {
		return true, nil // Already set
	}

	// Get description for AI estimate
	description, err := client.GetTicketDescription(ticket.Key)
	if err != nil {
		description = "" // Continue with empty description if unavailable
	}

	// Get AI suggestion
	options := []int{1, 2, 3, 5, 8, 13}
	var aiReasoning string
	estimate, reasoning, err := geminiClient.EstimateStoryPoints(ticket.Fields.Summary, description, options)
	if err != nil {
		// If AI fails, continue with manual selection
		fmt.Println("Could not get AI estimate, proceeding with manual selection")
	} else {
		fmt.Printf("🤖 AI Estimate: %d story points\n", estimate)
		fmt.Printf("   Reasoning: %s\n", reasoning)
		aiReasoning = reasoning // Store for later use
	}

	fmt.Println("\nSelect story points:")
	letters := []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j"}
	for i, opt := range options {
		if i < len(letters) {
			fmt.Printf("[%s] %d  ", letters[i], opt)
		}
	}
	fmt.Println()
	fmt.Print("Enter letter, number, or 'skip': > ")

	input, err := reader.ReadString('\n')
	if err != nil {
		return false, err
	}
	input = strings.TrimSpace(strings.ToLower(input))

	if input == "skip" {
		return false, nil
	}

	// Try to parse as letter
	if len(input) == 1 {
		for i, letter := range letters {
			if input == letter && i < len(options) {
				points := options[i]
				if err := client.UpdateTicketPoints(ticket.Key, points); err != nil {
					return false, err
				}
				// Add AI reasoning as comment if available
				if aiReasoning != "" {
					comment := fmt.Sprintf("🤖 *AI Story Point Estimate: %d points*\n\n%s", points, aiReasoning)
					if err := client.AddComment(ticket.Key, comment); err != nil {
						// Log but don't fail - comment is optional
						fmt.Printf("Warning: Could not add reasoning comment: %v\n", err)
					}
				}
				return true, nil
			}
		}
	}

	// Try to parse as number
	points, err := strconv.Atoi(input)
	if err == nil {
		if err := client.UpdateTicketPoints(ticket.Key, points); err != nil {
			return false, err
		}
		// Add AI reasoning as comment if available
		if aiReasoning != "" {
			comment := fmt.Sprintf("🤖 *AI Story Point Estimate: %d points*\n\n%s", points, aiReasoning)
			if err := client.AddComment(ticket.Key, comment); err != nil {
				// Log but don't fail - comment is optional
				fmt.Printf("Warning: Could not add reasoning comment: %v\n", err)
			}
		}
		return true, nil
	}

	return false, fmt.Errorf("invalid input: %s", input)
}

// HandleBacklogTransitionStep transitions ticket to Backlog if in "New" state
func HandleBacklogTransitionStep(client jira.JiraClient, ticket *jira.Issue) (bool, error) {
	// Check if ticket is in "New" state
	if ticket.Fields.Status.Name != "New" {
		return true, nil // Not in New state, step complete
	}

	// Get available transitions
	transitions, err := client.GetTransitions(ticket.Key)
	if err != nil {
		return false, fmt.Errorf("failed to get transitions: %w", err)
	}

	// Find "Backlog" transition
	var backlogTransitionID string
	for _, t := range transitions {
		if t.To.Name == "Backlog" {
			backlogTransitionID = t.ID
			break
		}
	}

	if backlogTransitionID == "" {
		// Transition not available, skip
		return true, nil
	}

	// Execute transition
	if err := client.TransitionTicket(ticket.Key, backlogTransitionID); err != nil {
		return false, fmt.Errorf("failed to transition to Backlog: %w", err)
	}

	return true, nil
}

// SelectBoard selects a board for a project - auto-selects if one board, prompts if multiple
func SelectBoard(client jira.JiraClient, reader *bufio.Reader, cfg *config.Config, projectKey string) (int, error) {
	boards, err := client.GetBoardsForProject(projectKey)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch boards: %w", err)
	}

	if len(boards) == 0 {
		// No boards found - use default if configured
		if cfg.DefaultBoardID > 0 {
			return cfg.DefaultBoardID, nil
		}
		return 0, fmt.Errorf("no boards found for project %s. Please configure default_board_id in config", projectKey)
	}

	if len(boards) == 1 {
		// Auto-select if only one board
		return boards[0].ID, nil
	}

	// Multiple boards - prompt user
	fmt.Println("Select board:")
	for i, board := range boards {
		fmt.Printf("[%d] %s (%s)\n", i+1, board.Name, board.Type)
	}
	fmt.Print("> ")

	choice, err := reader.ReadString('\n')
	if err != nil {
		return 0, err
	}
	choice = strings.TrimSpace(choice)
	selected, err := strconv.Atoi(choice)
	if err != nil {
		return 0, fmt.Errorf("invalid selection: %s", choice)
	}

	if selected < 1 || selected > len(boards) {
		return 0, fmt.Errorf("invalid selection: %d", selected)
	}

	return boards[selected-1].ID, nil
}
