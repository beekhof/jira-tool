package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/beekhof/jira-tool/pkg/config"
	"github.com/beekhof/jira-tool/pkg/editor"
	"github.com/beekhof/jira-tool/pkg/gemini"
	"github.com/beekhof/jira-tool/pkg/jira"
	"github.com/beekhof/jira-tool/pkg/parser"

	"github.com/spf13/cobra"
)

var maxPointsFlag int

var decomposeCmd = &cobra.Command{
	Use:   "decompose [TICKET_ID]",
	Short: "Decompose a ticket into smaller child tickets",
	Long: `Decompose an existing ticket into child tickets with story point estimates
no larger than a specified value. Child ticket type depends on the parent ticket type.
Any existing child tickets are considered in the plan, and you can edit the breakdown
before tickets are created in Jira.`,
	Args: cobra.ExactArgs(1),
	RunE: runDecompose,
}

func init() {
	decomposeCmd.Flags().IntVar(&maxPointsFlag, "max-points", 0, "Maximum story points per child ticket")
	rootCmd.AddCommand(decomposeCmd)
}

func runDecompose(_ *cobra.Command, args []string) error {
	configDir := GetConfigDir()
	configPath := config.GetConfigPath(configDir)
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	ticketID := normalizeTicketID(args[0], cfg.DefaultProject)
	client, err := jira.NewClient(configDir, GetNoCache())
	if err != nil {
		return err
	}

	// Validate ticket exists
	parentTicket, err := client.GetIssue(ticketID)
	if err != nil {
		return fmt.Errorf("failed to fetch ticket %s: %w", ticketID, err)
	}

	reader := bufio.NewReader(os.Stdin)

	// Get story point limit
	maxPoints, err := getMaxStoryPoints(maxPointsFlag, cfg, reader)
	if err != nil {
		return err
	}

	// Fetch existing child tickets
	existingChildren, err := jira.GetChildTicketsDetailed(client, ticketID, cfg.EpicLinkFieldID)
	if err != nil {
		fmt.Printf("Warning: Could not fetch existing child tickets: %v\n", err)
		existingChildren = []jira.ChildTicketInfo{}
	}

	// Determine child ticket type
	parentType := parentTicket.Fields.IssueType.Name
	childType, err := jira.GetChildTicketType(parentType, reader, cfg)
	if err != nil {
		return fmt.Errorf("failed to determine child ticket type: %w", err)
	}

	// Get ticket description
	description, err := client.GetTicketDescription(ticketID)
	if err != nil {
		fmt.Printf("Warning: Could not fetch ticket description: %v\n", err)
		description = ""
	}

	// Generate plan with Gemini
	geminiClient, err := gemini.NewClient(configDir)
	if err != nil {
		return fmt.Errorf("failed to create Gemini client: %w", err)
	}

	planText, err := gemini.GenerateDecompositionPlan(
		geminiClient, cfg,
		parentTicket.Fields.Summary,
		description,
		existingChildren,
		childType,
		maxPoints,
	)
	if err != nil {
		return fmt.Errorf("failed to generate decomposition plan: %w", err)
	}

	// Parse plan
	plan, err := parser.ParseDecompositionPlan(planText)
	if err != nil {
		fmt.Printf("Failed to parse plan. Raw output:\n%s\n", planText)
		return fmt.Errorf("failed to parse decomposition plan: %w", err)
	}

	// Detect and filter duplicates
	filteredTickets, warnings := detectAndFilterDuplicates(plan.NewTickets, existingChildren)
	plan.NewTickets = filteredTickets
	for _, warning := range warnings {
		fmt.Printf("Warning: %s\n", warning)
	}

	// Display plan
	if err := displayDecompositionPlan(plan, ticketID, childType); err != nil {
		return err
	}

	// Confirm and handle editing
	plan, err = handleConfirmationAndEditing(reader, plan, ticketID, childType, maxPoints, configDir)
	if err != nil {
		return err
	}
	if plan == nil {
		return nil // User canceled
	}

	// Final confirmation
	if !confirmFinalCreation(reader) {
		return nil
	}

	// Create tickets
	parentIsEpic := jira.IsEpic(parentTicket)
	createdKeys, err := createChildTickets(
		client, cfg, plan, ticketID, parentIsEpic, childType, configDir,
	)
	if err != nil {
		return fmt.Errorf("failed to create tickets: %w", err)
	}

	// Update parent story points
	oldStoryPoints := int(parentTicket.Fields.StoryPoints)
	if err := updateParentStoryPoints(client, cfg, ticketID, plan, existingChildren); err != nil {
		fmt.Printf("Warning: Failed to update parent story points: %v\n", err)
	}

	// Calculate new story points
	newStoryPoints := calculateTotalStoryPoints(plan, existingChildren)

	// Display summary
	if err := displayCreationSummary(createdKeys, plan, ticketID, oldStoryPoints, newStoryPoints); err != nil {
		return err
	}

	return nil
}

func getMaxStoryPoints(flagValue int, cfg *config.Config, reader *bufio.Reader) (int, error) {
	if flagValue > 0 {
		return flagValue, nil
	}

	if cfg.DefaultMaxDecomposePoints > 0 {
		return cfg.DefaultMaxDecomposePoints, nil
	}

	fmt.Print("Maximum story points per child ticket (default: 5): ")
	input, err := reader.ReadString('\n')
	if err != nil {
		return 0, err
	}
	input = strings.TrimSpace(input)

	if input == "" {
		return 5, nil
	}

	value, err := strconv.Atoi(input)
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("invalid story point limit: must be a positive integer")
	}

	return value, nil
}

func handleConfirmationAndEditing(
	reader *bufio.Reader, plan *parser.DecompositionPlan,
	ticketID, childType string, maxPoints int, configDir string,
) (*parser.DecompositionPlan, error) {
	confirmed, shouldEdit, err := confirmDecompositionPlan(reader, plan)
	if err != nil {
		return nil, err
	}

	if !confirmed {
		if err := saveRejectedPlan(plan, ticketID, configDir); err != nil {
			fmt.Printf("Warning: Failed to save rejected plan: %v\n", err)
		}
		fmt.Println("Decomposition canceled.")
		return nil, nil
	}

	if shouldEdit {
		editedPlan, err := editDecompositionPlan(reader, plan, maxPoints)
		if err != nil {
			return nil, fmt.Errorf("failed to edit plan: %w", err)
		}
		plan = editedPlan

		if err := displayDecompositionPlan(plan, ticketID, childType); err != nil {
			return nil, err
		}

		confirmed, _, err = confirmDecompositionPlan(reader, plan)
		if err != nil {
			return nil, err
		}
		if !confirmed {
			if err := saveRejectedPlan(plan, ticketID, configDir); err != nil {
				fmt.Printf("Warning: Failed to save rejected plan: %v\n", err)
			}
			fmt.Println("Decomposition canceled.")
			return nil, nil
		}
	}

	return plan, nil
}

func confirmFinalCreation(reader *bufio.Reader) bool {
	fmt.Print("\nCreate these tickets? [Y/n] ")
	finalConfirm, err := reader.ReadString('\n')
	if err != nil {
		return false
	}
	finalConfirm = strings.TrimSpace(strings.ToLower(finalConfirm))
	if finalConfirm == "n" || finalConfirm == "no" {
		fmt.Println("Canceled.")
		return false
	}
	return true
}

func detectAndFilterDuplicates(
	newTickets []parser.DecomposeTicket,
	existingChildren []jira.ChildTicketInfo,
) (filtered []parser.DecomposeTicket, warnings []string) {
	existingSummaries := make(map[string]string) // summary -> key

	for _, child := range existingChildren {
		existingSummaries[strings.ToLower(child.Summary)] = child.Key
	}

	for _, ticket := range newTickets {
		summaryLower := strings.ToLower(ticket.Summary)
		if key, exists := existingSummaries[summaryLower]; exists {
			warnings = append(warnings, fmt.Sprintf("Skipping %q - already exists as %s", ticket.Summary, key))
			continue
		}

		// Check for fuzzy matches (substring)
		isDuplicate := false
		for existingSummary, key := range existingSummaries {
			if strings.Contains(summaryLower, existingSummary) || strings.Contains(existingSummary, summaryLower) {
				warnings = append(warnings, fmt.Sprintf("Skipping %q - similar to existing %s", ticket.Summary, key))
				isDuplicate = true
				break
			}
		}

		if !isDuplicate {
			filtered = append(filtered, ticket)
		}
	}

	return filtered, warnings
}

func displayDecompositionPlan(plan *parser.DecompositionPlan, parentKey, childType string) error {
	fmt.Printf("\nDecomposition Plan for %s:\n\n", parentKey)

	if len(plan.NewTickets) > 0 {
		fmt.Println("NEW TICKETS:")
		for i, ticket := range plan.NewTickets {
			fmt.Printf("[%d] %s (%d points) - %s\n", i+1, ticket.Summary, ticket.StoryPoints, childType)
		}
		fmt.Println()
	}

	if len(plan.ExistingTickets) > 0 {
		fmt.Println("EXISTING TICKETS:")
		for _, ticket := range plan.ExistingTickets {
			fmt.Printf("[x] %s (%d points) - %s [EXISTING]\n", ticket.Summary, ticket.StoryPoints, ticket.Type)
		}
		fmt.Println()
	}

	// Calculate summary
	newCount, newPoints, existingCount, existingPoints := calculatePlanSummary(plan)
	totalCount := newCount + existingCount
	totalPoints := newPoints + existingPoints

	fmt.Println("Summary:")
	fmt.Printf("- New tickets: %d (%d total story points)\n", newCount, newPoints)
	if existingCount > 0 {
		fmt.Printf("- Existing tickets: %d (%d total story points)\n", existingCount, existingPoints)
	}
	fmt.Printf("- Total: %d tickets (%d total story points)\n\n", totalCount, totalPoints)

	return nil
}

func calculatePlanSummary(plan *parser.DecompositionPlan) (newCount, newPoints, existingCount, existingPoints int) {
	for _, ticket := range plan.NewTickets {
		newCount++
		newPoints += ticket.StoryPoints
	}
	for _, ticket := range plan.ExistingTickets {
		existingCount++
		existingPoints += ticket.StoryPoints
	}
	return newCount, newPoints, existingCount, existingPoints
}

func confirmDecompositionPlan(
	reader *bufio.Reader, plan *parser.DecompositionPlan,
) (confirmed, shouldEdit bool, err error) {
	newCount, _, _, _ := calculatePlanSummary(plan)
	fmt.Printf("Create these %d tickets? [Y/n/e(dit)/s(how)] ", newCount)

	choice, err := reader.ReadString('\n')
	if err != nil {
		return false, false, err
	}
	choice = strings.TrimSpace(strings.ToLower(choice))

	switch choice {
	case "y", "yes", "":
		return true, false, nil
	case "n", "no":
		return false, false, nil
	case "e", "edit":
		return false, true, nil
	case "s", "show":
		return false, false, nil // Will re-display
	default:
		return false, false, nil
	}
}

func saveRejectedPlan(plan *parser.DecompositionPlan, parentKey, configDir string) error {
	rejectionsDir := fmt.Sprintf("%s/decompose-rejections", configDir)
	if err := os.MkdirAll(rejectionsDir, 0755); err != nil {
		return fmt.Errorf("failed to create rejections directory: %w", err)
	}

	timestamp := time.Now().Format("20060102-150405")
	filename := fmt.Sprintf("%s/%s-%s.md", rejectionsDir, parentKey, timestamp)

	content := "# Rejected Decomposition Plan\n\n"
	content += fmt.Sprintf("Parent Ticket: %s\n", parentKey)
	content += fmt.Sprintf("Rejected: %s\n\n", time.Now().Format("2006-01-02 15:04:05"))
	content += formatPlanForEditing(plan)

	return os.WriteFile(filename, []byte(content), 0600)
}

func formatPlanForEditing(plan *parser.DecompositionPlan) string {
	var content strings.Builder
	content.WriteString("# DECOMPOSITION PLAN\n\n")

	content.WriteString("## NEW TICKETS\n")
	for _, ticket := range plan.NewTickets {
		content.WriteString(fmt.Sprintf("- [ ] %s (%d points)\n", ticket.Summary, ticket.StoryPoints))
	}

	content.WriteString("\n## EXISTING TICKETS (read-only)\n")
	for _, ticket := range plan.ExistingTickets {
		content.WriteString(fmt.Sprintf("- [x] %s (%d points) [EXISTING]\n", ticket.Summary, ticket.StoryPoints))
	}

	return content.String()
}

func editDecompositionPlan(
	_ *bufio.Reader, plan *parser.DecompositionPlan, maxPoints int,
) (*parser.DecompositionPlan, error) {
	formatted := formatPlanForEditing(plan)

	edited, err := editor.OpenInEditor(formatted)
	if err != nil {
		return nil, fmt.Errorf("failed to edit plan: %w", err)
	}

	editedPlan, err := parser.ParseDecompositionPlan(edited)
	if err != nil {
		return nil, fmt.Errorf("failed to parse edited plan: %w", err)
	}

	if err := validateEditedPlan(editedPlan, maxPoints); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	return editedPlan, nil
}

func validateEditedPlan(plan *parser.DecompositionPlan, maxPoints int) error {
	for i, ticket := range plan.NewTickets {
		if ticket.Summary == "" {
			return fmt.Errorf("ticket %d has empty summary", i+1)
		}
		if ticket.StoryPoints <= 0 {
			return fmt.Errorf("ticket \"%s\" has invalid story points: %d (must be > 0)", ticket.Summary, ticket.StoryPoints)
		}
		if ticket.StoryPoints > maxPoints {
			return fmt.Errorf(
				"ticket \"%s\" has story points %d exceeding limit %d",
				ticket.Summary, ticket.StoryPoints, maxPoints)
		}
	}
	return nil
}

func createChildTickets(
	client jira.JiraClient, cfg *config.Config, plan *parser.DecompositionPlan,
	parentKey string, parentIsEpic bool, childType string, _ string,
) ([]string, error) {
	var createdKeys []string
	project := cfg.DefaultProject

	for i, ticket := range plan.NewTickets {
		fmt.Printf("Creating ticket %d of %d...\n", i+1, len(plan.NewTickets))

		var ticketKey string
		var err error

		if parentIsEpic {
			epicLinkFieldID := cfg.EpicLinkFieldID
			if epicLinkFieldID == "" {
				// Try to detect
				epicLinkFieldID, err = client.DetectEpicLinkField(project)
				if err != nil || epicLinkFieldID == "" {
					return createdKeys, fmt.Errorf("Epic Link field not configured and could not be detected")
				}
			}
			ticketKey, err = client.CreateTicketWithEpicLink(project, childType, ticket.Summary, parentKey, epicLinkFieldID)
		} else {
			ticketKey, err = client.CreateTicketWithParent(project, childType, ticket.Summary, parentKey)
		}

		if err != nil {
			fmt.Printf("Warning: Failed to create ticket \"%s\": %v\n", ticket.Summary, err)
			continue
		}

		// Set story points
		if ticket.StoryPoints > 0 {
			if err := client.UpdateTicketPoints(ticketKey, ticket.StoryPoints); err != nil {
				fmt.Printf("Warning: Failed to set story points for %s: %v\n", ticketKey, err)
			}
		}

		createdKeys = append(createdKeys, ticketKey)
	}

	return createdKeys, nil
}

func calculateTotalStoryPoints(
	plan *parser.DecompositionPlan, existingChildren []jira.ChildTicketInfo,
) int {
	total := 0

	for _, ticket := range plan.NewTickets {
		total += ticket.StoryPoints
	}

	for _, child := range existingChildren {
		total += child.StoryPoints
	}

	return total
}

func updateParentStoryPoints(
	client jira.JiraClient, cfg *config.Config, parentKey string,
	plan *parser.DecompositionPlan, existingChildren []jira.ChildTicketInfo,
) error {
	if cfg.StoryPointsFieldID == "" {
		return fmt.Errorf("story points field not configured")
	}

	totalPoints := calculateTotalStoryPoints(plan, existingChildren)
	return client.UpdateTicketPoints(parentKey, totalPoints)
}

func displayCreationSummary(
	createdKeys []string, plan *parser.DecompositionPlan,
	parentKey string, oldStoryPoints, newStoryPoints int,
) error {
	fmt.Println("\nCreated tickets:")
	for i, key := range createdKeys {
		if i < len(plan.NewTickets) {
			ticket := plan.NewTickets[i]
			fmt.Printf("- %s: %s (%d points)\n", key, ticket.Summary, ticket.StoryPoints)
		}
	}

	if oldStoryPoints != newStoryPoints {
		fmt.Printf("\nUpdated parent %s story points to %d (was %d)\n", parentKey, newStoryPoints, oldStoryPoints)
	}

	return nil
}
