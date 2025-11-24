package review

import (
	"bufio"
	"fmt"
	"strconv"
	"strings"

	"github.com/beekhof/jira-tool/pkg/config"
	"github.com/beekhof/jira-tool/pkg/gemini"
	"github.com/beekhof/jira-tool/pkg/jira"
	"github.com/beekhof/jira-tool/pkg/qa"
)

// CheckDescriptionQuality checks if a ticket's description meets quality criteria
func CheckDescriptionQuality(ticket jira.Issue, cfg *config.Config, geminiClient gemini.GeminiClient) (bool, string, error) {
	description := ""
	// Try to get description from ticket fields (may need to fetch full ticket)
	// For now, we'll check length from what we have
	// In practice, we may need to call GetTicketDescription for full text

	// Check minimum length
	if cfg.DescriptionMinLength > 0 {
		if len(description) < cfg.DescriptionMinLength {
			return false, fmt.Sprintf("too short (%d chars, need %d)", len(description), cfg.DescriptionMinLength), nil
		}
	}

	// Optional Gemini AI analysis
	if cfg.DescriptionQualityAI {
		// Fetch full description
		fullDesc, err := geminiClient.(interface {
			GetTicketDescription(string) (string, error)
		}).GetTicketDescription(ticket.Key)
		if err == nil && fullDesc != "" {
			description = fullDesc
		}

		// Use Gemini to analyze if description answers "what", "why", "how"
		prompt := fmt.Sprintf("Does this Jira ticket description answer what needs to be done, why it's needed, and how it will be accomplished? Description: %s", description)
		// For now, skip actual Gemini call - would need to add analysis method
		// This is a placeholder for the AI analysis
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

	// Check if severity is already set (would need to check custom field)
	// For now, assume we need to set it

	// Fetch severity values
	values, err := client.GetSeverityFieldValues(cfg.SeverityFieldID)
	if err != nil {
		return false, fmt.Errorf("failed to fetch severity values: %w", err)
	}

	if len(values) == 0 {
		// No predefined values, skip
		return true, nil
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

	// Update ticket severity (would need UpdateTicketSeverity method)
	// For now, placeholder
	_ = values[selected-1]

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

