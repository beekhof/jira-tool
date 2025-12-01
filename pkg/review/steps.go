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
func CheckDescriptionQuality(client jira.JiraClient, ticket jira.Issue, cfg *config.Config) (bool, string, error) {
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
func HandleComponentStep(client jira.JiraClient, reader *bufio.Reader, cfg *config.Config, ticket jira.Issue, configDir string) (bool, error) {
	// Check if ticket has components
	if len(ticket.Fields.Components) > 0 {
		return true, nil // Already has components
	}

	// Load state for recent components
	statePath := config.GetStatePath(configDir)
	state, err := config.LoadState(statePath)
	if err != nil {
		state = &config.State{}
	}

	// Fetch components for project
	projectKey := cfg.DefaultProject
	if projectKey == "" {
		return false, fmt.Errorf("default_project not configured")
	}

	components, err := client.GetComponents(projectKey)
	if err != nil {
		return false, fmt.Errorf("failed to fetch components: %w", err)
	}

	if len(components) == 0 {
		// No components available, skip this step
		return true, nil
	}

	// Show recent components first
	recent := state.RecentComponents
	if len(recent) > 0 {
		fmt.Println("Recent components:")
		for i, compName := range recent {
			fmt.Printf("[%d] %s\n", i+1, compName)
		}
		fmt.Printf("[%d] Other...\n", len(recent)+1)
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

		if selected >= 1 && selected <= len(recent) {
			// Find component by name from recent list
			compName := recent[selected-1]
			for _, comp := range components {
				if comp.Name == compName {
					// Update ticket
					if err := client.UpdateTicketComponents(ticket.Key, []string{comp.ID}); err != nil {
						return false, err
					}
					// Track selection
					state.AddRecentComponent(compName)
					if err := config.SaveState(state, statePath); err != nil {
						_ = err // Log but don't fail
					}
					return true, nil
				}
			}
		}
		// User selected "Other..."
		if selected == len(recent)+1 {
			// Fall through to show all components
		} else {
			return false, fmt.Errorf("invalid selection")
		}
	}

	// Show all components
	fmt.Println("Select component:")
	for i, comp := range components {
		fmt.Printf("[%d] %s\n", i+1, comp.Name)
	}
	fmt.Printf("[%d] Skip\n", len(components)+1)
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

	if selected == len(components)+1 {
		// User skipped - return false to skip remaining steps
		return false, nil
	}

	if selected < 1 || selected > len(components) {
		return false, fmt.Errorf("invalid selection: %d", selected)
	}

	selectedComp := components[selected-1]

	// Update ticket
	if err := client.UpdateTicketComponents(ticket.Key, []string{selectedComp.ID}); err != nil {
		return false, err
	}

	// Track selection
	state.AddRecentComponent(selectedComp.Name)
	if err := config.SaveState(state, statePath); err != nil {
		_ = err // Log but don't fail
	}

	return true, nil
}

// HandlePriorityStep checks and assigns priority if missing
func HandlePriorityStep(client jira.JiraClient, reader *bufio.Reader, ticket jira.Issue) (bool, error) {
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
func HandleSeverityStep(client jira.JiraClient, reader *bufio.Reader, cfg *config.Config, ticket jira.Issue) (bool, error) {
	// Check if severity field is configured
	if cfg.SeverityFieldID == "" {
		return true, nil // Not configured, skip step
	}

	// Check if severity is already set by fetching raw ticket data
	rawTicket, err := client.GetTicketRaw(ticket.Key)
	if err == nil {
		if fields, ok := rawTicket["fields"].(map[string]interface{}); ok {
			if severityValue, ok := fields[cfg.SeverityFieldID]; ok && severityValue != nil {
				// Check if it's a value object (map) or direct string
				var currentValue string
				if severityMap, ok := severityValue.(map[string]interface{}); ok {
					if val, ok := severityMap["value"].(string); ok {
						currentValue = val
					} else if val, ok := severityMap["name"].(string); ok {
						currentValue = val
					}
				} else if val, ok := severityValue.(string); ok {
					currentValue = val
				}
				if currentValue != "" {
					// Severity is already set, skip step
					return true, nil
				}
			}
		}
	}

	// Fetch severity values from Jira API
	values, err := client.GetSeverityFieldValues(cfg.SeverityFieldID)
	if err != nil {
		return false, fmt.Errorf("failed to fetch severity values: %w", err)
	}

	// If API doesn't return values, use configured values from config.yaml
	if len(values) == 0 && len(cfg.SeverityValues) > 0 {
		values = cfg.SeverityValues
	}

	if len(values) == 0 {
		// No predefined values available - severity field may not have a fixed set of values
		// Still show the step but inform user that severity field is configured but has no predefined values
		fmt.Println("Severity field is configured but has no predefined values.")
		fmt.Print("Set severity? [y/N] ")
		response, err := reader.ReadString('\n')
		if err != nil {
			return false, err
		}
		response = strings.TrimSpace(strings.ToLower(response))
		if response != "y" && response != "yes" {
			return false, nil // User skipped
		}
		// TODO: Implement UpdateTicketSeverity to set custom severity value
		// For now, inform user that this feature is not yet implemented
		fmt.Println("Note: Setting custom severity values is not yet implemented.")
		fmt.Println("You may need to set the severity manually in Jira.")
		return false, nil // Mark as incomplete since we can't actually set it
	}

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
		return false, nil // Skip
	}

	if selected < 1 || selected > len(values) {
		return false, fmt.Errorf("invalid selection: %d", selected)
	}

	// Update ticket severity
	selectedValue := values[selected-1]
	if err := client.UpdateTicketSeverity(ticket.Key, cfg.SeverityFieldID, selectedValue); err != nil {
		return false, fmt.Errorf("failed to update severity: %w", err)
	}

	fmt.Printf("Severity set to: %s\n", selectedValue)
	return true, nil
}

// HandleStoryPointsStep checks and estimates story points if missing
func HandleStoryPointsStep(client jira.JiraClient, geminiClient gemini.GeminiClient, reader *bufio.Reader, cfg *config.Config, ticket jira.Issue) (bool, error) {
	// Check if story points are set
	if ticket.Fields.StoryPoints > 0 {
		return true, nil // Already set
	}

	// Get description for AI estimate
	description, _ := client.GetTicketDescription(ticket.Key)

	// Get AI suggestion
	options := []int{1, 2, 3, 5, 8, 13}
	estimate, reasoning, err := geminiClient.EstimateStoryPoints(ticket.Fields.Summary, description, options)
	if err != nil {
		// If AI fails, continue with manual selection
		fmt.Println("Could not get AI estimate, proceeding with manual selection")
	} else {
		fmt.Printf("🤖 AI Estimate: %d story points\n", estimate)
		fmt.Printf("   Reasoning: %s\n", reasoning)
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
				if err := client.UpdateTicketPoints(ticket.Key, options[i]); err != nil {
					return false, err
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
		return true, nil
	}

	return false, fmt.Errorf("invalid input: %s", input)
}

// HandleBacklogTransitionStep transitions ticket to Backlog if in "New" state
func HandleBacklogTransitionStep(client jira.JiraClient, ticket jira.Issue) (bool, error) {
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

