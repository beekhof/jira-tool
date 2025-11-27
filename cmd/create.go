package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/beekhof/jira-tool/pkg/config"
	"github.com/beekhof/jira-tool/pkg/editor"
	"github.com/beekhof/jira-tool/pkg/gemini"
	"github.com/beekhof/jira-tool/pkg/jira"
	"github.com/beekhof/jira-tool/pkg/qa"
	"strconv"

	"github.com/spf13/cobra"
)

var (
	projectFlag string
	typeFlag    string
	parentFlag  string
)

var createCmd = &cobra.Command{
	Use:   "create [SUMMARY]",
	Short: "Create a new Jira ticket",
	Long: `Create a new Jira ticket with the given summary.
The project and type can be specified via flags, otherwise defaults from config are used.

You can create a spike ticket by using "spike" as the first word:
  jira-tool create spike research authentication options

This is equivalent to:
  jira-tool create "SPIKE: research authentication options"`,
	Args: cobra.MinimumNArgs(1),
	RunE: runCreate,
}

func runCreate(cmd *cobra.Command, args []string) error {
	// Check if first argument is "spike" (case-insensitive)
	// If so, prepend "SPIKE: " to the rest of the summary
	summary := strings.Join(args, " ")
	if len(args) > 0 && strings.ToLower(args[0]) == "spike" {
		// If it's "spike", join the rest and prepend "SPIKE: "
		if len(args) > 1 {
			summary = "SPIKE: " + strings.Join(args[1:], " ")
		} else {
			// Just "spike" with no other text
			summary = "SPIKE"
		}
	}

	// Get config directory
	configDir := GetConfigDir()

	// Load config to get defaults
	configPath := config.GetConfigPath(configDir)
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Determine project and type
	project := cfg.DefaultProject
	if projectFlag != "" {
		project = projectFlag
	}
	if project == "" {
		return fmt.Errorf("project not specified. Use --project flag or set default_project in config")
	}

	taskType := cfg.DefaultTaskType
	if typeFlag != "" {
		taskType = typeFlag
	}
	if taskType == "" {
		return fmt.Errorf("task type not specified. Use --type flag or set default_task_type in config")
	}

	// Create Jira client
	client, err := jira.NewClient(configDir, GetNoCache())
	if err != nil {
		return err
	}

	// Handle parent ticket selection
	var parentKey string
	var isEpic bool
	reader := bufio.NewReader(os.Stdin)

	if parentFlag != "" {
		// Validate parent ticket exists
		parentIssue, err := client.GetIssue(parentFlag)
		if err != nil {
			return fmt.Errorf("parent ticket %s not found or not accessible: %w", parentFlag, err)
		}
		parentKey = parentFlag
		isEpic = jira.IsEpic(parentIssue)
	} else {
		// Interactive selection
		selectedParent, err := selectParentTicket(client, reader, cfg, project, configPath)
		if err != nil {
			// If user cancels or error, proceed without parent
			fmt.Printf("Skipping parent ticket selection: %v\n", err)
		} else {
			parentKey = selectedParent
			// Fetch parent to check if Epic
			parentIssue, err := client.GetIssue(parentKey)
			if err != nil {
				return fmt.Errorf("failed to fetch parent ticket: %w", err)
			}
			isEpic = jira.IsEpic(parentIssue)
		}
	}

	// Create the ticket with or without parent
	var ticketKey string
	if parentKey != "" {
		if isEpic {
			// Use Epic Link field
			epicLinkFieldID := cfg.EpicLinkFieldID
			if epicLinkFieldID == "" {
				// Attempt auto-detection
				epicLinkFieldID, err = client.DetectEpicLinkField(project)
				if err != nil {
					return fmt.Errorf("failed to detect Epic Link field: %w", err)
				}
				if epicLinkFieldID == "" {
					// Prompt user for field ID
					fmt.Print("Epic Link field not detected. Please enter the custom field ID (e.g., customfield_10011) or press Enter to skip: ")
					fieldIDInput, err := reader.ReadString('\n')
					if err != nil {
						return fmt.Errorf("failed to read input: %w", err)
					}
					fieldIDInput = strings.TrimSpace(fieldIDInput)
					if fieldIDInput == "" {
						return fmt.Errorf("Epic Link field ID required for Epic parent")
					}
					if !strings.HasPrefix(fieldIDInput, "customfield_") {
						return fmt.Errorf("Invalid Epic Link field ID format. Must start with 'customfield_'")
					}
					epicLinkFieldID = fieldIDInput
					// Save to config
					cfg.EpicLinkFieldID = epicLinkFieldID
					if err := config.SaveConfig(cfg, configPath); err != nil {
						// Log but don't fail
						fmt.Printf("Warning: Could not save Epic Link field ID to config: %v\n", err)
					}
				} else {
					// Save detected field ID to config
					cfg.EpicLinkFieldID = epicLinkFieldID
					if err := config.SaveConfig(cfg, configPath); err != nil {
						// Log but don't fail
						fmt.Printf("Warning: Could not save Epic Link field ID to config: %v\n", err)
					}
				}
			}
			ticketKey, err = client.CreateTicketWithEpicLink(project, taskType, summary, parentKey, epicLinkFieldID)
		} else {
			// Use Parent Link field
			ticketKey, err = client.CreateTicketWithParent(project, taskType, summary, parentKey)
		}
		if err != nil {
			return err
		}
	} else {
		// No parent - create normally
		ticketKey, err = client.CreateTicket(project, taskType, summary)
		if err != nil {
			return err
		}
	}

	fmt.Printf("Ticket %s created.\n", ticketKey)

	// Update recent parent tickets in state
	if parentKey != "" {
		statePath := config.GetStatePath(configDir)
		state, err := config.LoadState(statePath)
		if err != nil {
			state = &config.State{}
		}
		state.AddRecentParentTicket(parentKey)
		state.AddRecentParentTicket(ticketKey) // Also add newly created ticket
		if err := config.SaveState(state, statePath); err != nil {
			// Log but don't fail - tracking is optional
			fmt.Printf("Warning: Could not save recent parent tickets: %v\n", err)
		}
	} else {
		// Add newly created ticket to recent list even without parent
		statePath := config.GetStatePath(configDir)
		state, err := config.LoadState(statePath)
		if err != nil {
			state = &config.State{}
		}
		state.AddRecentParentTicket(ticketKey)
		if err := config.SaveState(state, statePath); err != nil {
			// Log but don't fail
			_ = err
		}
	}

	// Ask if user wants to use Gemini to generate description
	fmt.Print("Would you like to use Gemini to generate the description? [y/N] ")
	response, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read input: %w", err)
	}
	response = strings.TrimSpace(strings.ToLower(response))

	if response == "y" || response == "yes" {
		// Load config to get max questions
		configPath := config.GetConfigPath(configDir)
		cfg, err := config.LoadConfig(configPath)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		// Initialize Gemini client
		geminiClient, err := gemini.NewClient(configDir)
		if err != nil {
			return err
		}

		// Run Q&A flow (pass summary to detect spike based on SPIKE prefix, no existing description for new tickets)
		description, err := qa.RunQnAFlow(geminiClient, summary, cfg.MaxQuestions, summary, "")
		if err != nil {
			return err
		}

		// Print the generated description
		fmt.Println("\nGenerated description:")
		fmt.Println("---")
		fmt.Println(description)
		fmt.Println("---")

		// Ask for confirmation
		fmt.Print("\nUpdate ticket with this description? [Y/n/e(dit)] ")
		confirm, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read input: %w", err)
		}
		confirm = strings.TrimSpace(strings.ToLower(confirm))

		if confirm == "e" || confirm == "edit" {
			// Open in editor
			editedDescription, err := editor.OpenInEditor(description)
			if err != nil {
				return fmt.Errorf("failed to edit description: %w", err)
			}
			description = editedDescription
		}

		if confirm != "n" && confirm != "no" {
			// Update the ticket
			if err := client.UpdateTicketDescription(ticketKey, description); err != nil {
				return err
			}
			fmt.Printf("Updated %s with description.\n", ticketKey)

			// Prompt to review the ticket
			fmt.Print("\nWould you like to review this ticket? [y/N] ")
			reviewResponse, err := reader.ReadString('\n')
			if err != nil {
				return fmt.Errorf("failed to read input: %w", err)
			}
			reviewResponse = strings.TrimSpace(strings.ToLower(reviewResponse))

			if reviewResponse == "y" || reviewResponse == "yes" {
				// Fetch updated ticket details
				issues, err := client.SearchTickets(fmt.Sprintf("key = %s", ticketKey))
				if err != nil {
					return fmt.Errorf("failed to fetch ticket: %w", err)
				}
				if len(issues) == 0 {
					return fmt.Errorf("ticket %s not found", ticketKey)
				}

				selectedIssue := issues[0]
				if err := reviewTicket(client, reader, cfg, selectedIssue); err != nil {
					return fmt.Errorf("error reviewing ticket: %w", err)
				}
			}
		}
	}

	return nil
}

// selectParentTicket allows the user to interactively select a parent ticket
func selectParentTicket(client jira.JiraClient, reader *bufio.Reader, cfg *config.Config, projectKey string, configPath string) (string, error) {
	configDir := GetConfigDir()
	statePath := config.GetStatePath(configDir)
	state, err := config.LoadState(statePath)
	if err != nil {
		// If state can't be loaded, continue without recent list
		state = &config.State{}
	}

	// Get recent parent tickets and filter to valid ones
	recentTickets := state.RecentParentTickets
	var validRecentTickets []string
	if len(recentTickets) > 0 {
		validRecentTickets, err = jira.FilterValidParentTickets(client, recentTickets)
		if err != nil {
			// Log but continue - filtering is best effort
			validRecentTickets = []string{}
		}
	}

	// Show recent tickets if available
	showRecent := len(validRecentTickets) > 0
	if showRecent {
		fmt.Println("Recent parent tickets:")
		for i, ticketKey := range validRecentTickets {
			// Fetch ticket to show details
			issue, err := client.GetIssue(ticketKey)
			if err != nil {
				// Skip if can't fetch
				continue
			}
			issueType := issue.Fields.IssueType.Name
			summary := issue.Fields.Summary
			if len(summary) > 50 {
				summary = summary[:47] + "..."
			}
			fmt.Printf("[%d] %s [%s]: %s\n", i+1, ticketKey, issueType, summary)
		}
		fmt.Printf("[%d] Other...\n", len(validRecentTickets)+1)
	} else {
		fmt.Println("No recent parent tickets. Searching all tickets in project...")
		// Search for all tickets in project that could be parents
		issues, err := client.SearchTickets(fmt.Sprintf("project = %s ORDER BY updated DESC", projectKey))
		if err != nil {
			return "", fmt.Errorf("failed to search tickets: %w", err)
		}
		if len(issues) == 0 {
			return "", fmt.Errorf("no tickets found in project %s", projectKey)
		}

		// Filter to Epics and parent tickets
		var validIssues []jira.Issue
		for _, issue := range issues {
			if jira.IsEpic(&issue) {
				validIssues = append(validIssues, issue)
			} else {
				// Check if it has subtasks
				subtasks, err := client.SearchTickets(fmt.Sprintf("parent = %s", issue.Key))
				if err == nil && len(subtasks) > 0 {
					validIssues = append(validIssues, issue)
				}
			}
		}

		if len(validIssues) == 0 {
			return "", fmt.Errorf("no valid parent tickets found in project %s", projectKey)
		}

		// Show first 20 valid tickets
		maxShow := 20
		if len(validIssues) > maxShow {
			validIssues = validIssues[:maxShow]
		}

		fmt.Println("Select parent ticket:")
		for i, issue := range validIssues {
			summary := issue.Fields.Summary
			if len(summary) > 50 {
				summary = summary[:47] + "..."
			}
			fmt.Printf("[%d] %s [%s]: %s\n", i+1, issue.Key, issue.Fields.IssueType.Name, summary)
		}
	}

	fmt.Print("> ")
	choice, err := reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("failed to read input: %w", err)
	}
	choice = strings.TrimSpace(choice)

	// Allow user to enter ticket key directly
	if choice != "" && !strings.HasPrefix(choice, "[") {
		// Check if it looks like a ticket key (e.g., PROJ-123)
		if strings.Contains(choice, "-") {
			// Validate it exists
			_, err := client.GetIssue(choice)
			if err != nil {
				return "", fmt.Errorf("ticket %s not found: %w", choice, err)
			}
			return choice, nil
		}
	}

	// Parse as number
	selected, err := strconv.Atoi(choice)
	if err != nil {
		return "", fmt.Errorf("invalid selection: %s", choice)
	}

	var selectedTicketKey string
	if showRecent && selected <= len(validRecentTickets) {
		selectedTicketKey = validRecentTickets[selected-1]
	} else if showRecent && selected == len(validRecentTickets)+1 {
		// "Other..." selected - search all tickets
		issues, err := client.SearchTickets(fmt.Sprintf("project = %s ORDER BY updated DESC", projectKey))
		if err != nil {
			return "", fmt.Errorf("failed to search tickets: %w", err)
		}

		// Filter and show
		var validIssues []jira.Issue
		for _, issue := range issues {
			if jira.IsEpic(&issue) {
				validIssues = append(validIssues, issue)
			} else {
				subtasks, err := client.SearchTickets(fmt.Sprintf("parent = %s", issue.Key))
				if err == nil && len(subtasks) > 0 {
					validIssues = append(validIssues, issue)
				}
			}
		}

		if len(validIssues) == 0 {
			return "", fmt.Errorf("no valid parent tickets found")
		}

		maxShow := 20
		if len(validIssues) > maxShow {
			validIssues = validIssues[:maxShow]
		}

		fmt.Println("Select parent ticket:")
		for i, issue := range validIssues {
			summary := issue.Fields.Summary
			if len(summary) > 50 {
				summary = summary[:47] + "..."
			}
			fmt.Printf("[%d] %s [%s]: %s\n", i+1, issue.Key, issue.Fields.IssueType.Name, summary)
		}
		fmt.Print("> ")

		choice2, err := reader.ReadString('\n')
		if err != nil {
			return "", fmt.Errorf("failed to read input: %w", err)
		}
		choice2 = strings.TrimSpace(choice2)
		selected2, err := strconv.Atoi(choice2)
		if err != nil {
			return "", fmt.Errorf("invalid selection: %s", choice2)
		}
		if selected2 < 1 || selected2 > len(validIssues) {
			return "", fmt.Errorf("invalid selection: %d", selected2)
		}
		selectedTicketKey = validIssues[selected2-1].Key
	} else {
		return "", fmt.Errorf("invalid selection: %d", selected)
	}

	if selectedTicketKey == "" {
		return "", fmt.Errorf("no ticket selected")
	}

	return selectedTicketKey, nil
}

// reviewTicket handles the review workflow for a single ticket
// This is shared between create and review commands
func reviewTicket(client jira.JiraClient, reader *bufio.Reader, cfg *config.Config, issue jira.Issue) error {
	for {
		// Show ticket details and action menu
		fmt.Printf("\n=== %s - %s ===\n", issue.Key, issue.Fields.Summary)
		fmt.Printf("Priority: %s | Assignee: %s | Status: %s\n",
			getPriorityName(issue), getAssigneeName(issue), issue.Fields.Status.Name)
		fmt.Print("Action? [a(ssign), t(riage), e(stimate), d(one)] > ")

		action, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read input: %w", err)
		}
		action = strings.TrimSpace(strings.ToLower(action))

		switch action {
		case "a", "assign":
			configPath := config.GetConfigPath(configDir)
			if err := handleAssign(client, reader, cfg, issue.Key, configPath); err != nil {
				fmt.Printf("Error assigning ticket: %v\n", err)
			} else {
				fmt.Println("Ticket assigned successfully.")
				// Refresh ticket data
				updated, err := client.SearchTickets(fmt.Sprintf("key = %s", issue.Key))
				if err == nil && len(updated) > 0 {
					issue = updated[0]
				}
			}
		case "t", "triage":
			if err := handleTriage(client, reader, issue.Key); err != nil {
				fmt.Printf("Error triaging ticket: %v\n", err)
			} else {
				fmt.Println("Ticket triaged successfully.")
				// Refresh ticket data
				updated, err := client.SearchTickets(fmt.Sprintf("key = %s", issue.Key))
				if err == nil && len(updated) > 0 {
					issue = updated[0]
				}
			}
		case "e", "estimate":
			if err := handleEstimate(client, reader, cfg, issue.Key); err != nil {
				fmt.Printf("Error estimating ticket: %v\n", err)
			} else {
				fmt.Println("Story points updated successfully.")
				// Refresh ticket data
				updated, err := client.SearchTickets(fmt.Sprintf("key = %s", issue.Key))
				if err == nil && len(updated) > 0 {
					issue = updated[0]
				}
			}
		case "d", "done":
			return nil
		default:
			fmt.Println("Invalid action. Use 'a' for assign, 't' for triage, 'e' for estimate, or 'd' for done.")
		}
	}
}

func init() {
	createCmd.Flags().StringVarP(&projectFlag, "project", "p", "", "Project key (overrides default_project)")
	createCmd.Flags().StringVarP(&typeFlag, "type", "t", "", "Task type (overrides default_task_type)")
	createCmd.Flags().StringVarP(&parentFlag, "parent", "P", "", "Parent ticket key (Epic or parent ticket)")
	rootCmd.AddCommand(createCmd)
}
