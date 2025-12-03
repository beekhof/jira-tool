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

func runCreate(_ *cobra.Command, args []string) error {
	summary := normalizeSummary(args)

	configDir := GetConfigDir()
	configPath := config.GetConfigPath(configDir)
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	project, taskType, err := determineProjectAndType(cfg)
	if err != nil {
		return err
	}

	client, err := jira.NewClient(configDir, GetNoCache())
	if err != nil {
		return err
	}

	reader := bufio.NewReader(os.Stdin)
	parentKey, isEpic, err := handleParentSelection(client, reader, cfg, project, configPath)
	if err != nil {
		return err
	}

	ticketKey, err := createTicketWithParent(
		client, reader, cfg, project, taskType, summary, parentKey, isEpic, configPath)
	if err != nil {
		return err
	}

	fmt.Printf("Ticket %s created.\n", ticketKey)

	updateRecentParentTickets(configDir, parentKey, ticketKey)

	if err := handleDescriptionGeneration(client, reader, cfg, configDir, summary, taskType, ticketKey); err != nil {
		return err
	}

	return nil
}

func normalizeSummary(args []string) string {
	summary := strings.Join(args, " ")
	if len(args) > 0 && strings.EqualFold(args[0], "spike") {
		if len(args) > 1 {
			summary = "SPIKE: " + strings.Join(args[1:], " ")
		} else {
			summary = "SPIKE"
		}
	}
	return summary
}

func determineProjectAndType(cfg *config.Config) (project, taskType string, err error) {
	project = cfg.DefaultProject
	if projectFlag != "" {
		project = projectFlag
	}
	if project == "" {
		return "", "", fmt.Errorf("project not specified. Use --project flag or set default_project in config")
	}

	taskType = cfg.DefaultTaskType
	if typeFlag != "" {
		taskType = typeFlag
	}
	if taskType == "" {
		return "", "", fmt.Errorf("task type not specified. Use --type flag or set default_task_type in config")
	}

	return project, taskType, nil
}

func handleParentSelection(
	client jira.JiraClient, reader *bufio.Reader, cfg *config.Config,
	project, configPath string,
) (parentKey string, isEpic bool, err error) {
	if parentFlag != "" {
		return handleParentFlag(client, parentFlag)
	}

	return handleInteractiveParentSelection(client, reader, cfg, project, configPath)
}

func handleParentFlag(client jira.JiraClient, parentFlag string) (parentKey string, isEpic bool, err error) {
	parentIssue, err := client.GetIssue(parentFlag)
	if err != nil {
		return "", false, fmt.Errorf("parent ticket %s not found or not accessible: %w", parentFlag, err)
	}
	return parentFlag, jira.IsEpic(parentIssue), nil
}

func handleInteractiveParentSelection(
	client jira.JiraClient, reader *bufio.Reader, cfg *config.Config,
	project, configPath string,
) (parentKey string, isEpic bool, err error) {
	selectedParent, err := selectParentTicket(client, reader, cfg, project, configPath)
	if err != nil {
		fmt.Printf("Skipping parent ticket selection: %v\n", err)
		return "", false, nil
	}

	parentIssue, err := client.GetIssue(selectedParent)
	if err != nil {
		return "", false, fmt.Errorf("failed to fetch parent ticket: %w", err)
	}

	return selectedParent, jira.IsEpic(parentIssue), nil
}

func createTicketWithParent(
	client jira.JiraClient, reader *bufio.Reader, cfg *config.Config,
	project, taskType, summary, parentKey string, isEpic bool, configPath string,
) (string, error) {
	if parentKey == "" {
		return client.CreateTicket(project, taskType, summary)
	}

	if isEpic {
		return createTicketWithEpicLink(client, reader, cfg, project, taskType, summary, parentKey, configPath)
	}

	return client.CreateTicketWithParent(project, taskType, summary, parentKey)
}

func createTicketWithEpicLink(
	client jira.JiraClient, reader *bufio.Reader, cfg *config.Config,
	project, taskType, summary, parentKey, configPath string,
) (string, error) {
	epicLinkFieldID := cfg.EpicLinkFieldID
	if epicLinkFieldID == "" {
		var err error
		epicLinkFieldID, err = detectAndPromptEpicLinkField(client, reader, project, cfg, configPath)
		if err != nil {
			return "", err
		}
	}

	return client.CreateTicketWithEpicLink(project, taskType, summary, parentKey, epicLinkFieldID)
}

func detectAndPromptEpicLinkField(
	client jira.JiraClient, reader *bufio.Reader, project string,
	cfg *config.Config, configPath string,
) (string, error) {
	epicLinkFieldID, err := client.DetectEpicLinkField(project)
	if err != nil {
		return "", fmt.Errorf("failed to detect Epic Link field: %w", err)
	}

	if epicLinkFieldID == "" {
		return promptForEpicLinkFieldID(reader, cfg, configPath)
	}

	cfg.EpicLinkFieldID = epicLinkFieldID
	if err := config.SaveConfig(cfg, configPath); err != nil {
		_ = err // Ignore - config saving is optional
	}
	return epicLinkFieldID, nil
}

func promptForEpicLinkFieldID(reader *bufio.Reader, cfg *config.Config, configPath string) (string, error) {
	fmt.Print("Epic Link field not detected. " +
		"Please enter the custom field ID (e.g., customfield_10011) or press Enter to skip: ")
	fieldIDInput, err := reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("failed to read input: %w", err)
	}
	fieldIDInput = strings.TrimSpace(fieldIDInput)
	if fieldIDInput == "" {
		return "", fmt.Errorf("Epic Link field ID required for Epic parent")
	}
	if !strings.HasPrefix(fieldIDInput, "customfield_") {
		return "", fmt.Errorf("Invalid Epic Link field ID format. Must start with 'customfield_'")
	}

	cfg.EpicLinkFieldID = fieldIDInput
	if err := config.SaveConfig(cfg, configPath); err != nil {
		_ = err // Ignore - config saving is optional
	}
	return fieldIDInput, nil
}

func updateRecentParentTickets(configDir, parentKey, ticketKey string) {
	statePath := config.GetStatePath(configDir)
	state, err := config.LoadState(statePath)
	if err != nil {
		state = &config.State{}
	}

	if parentKey != "" {
		state.AddRecentParentTicket(parentKey)
	}
	state.AddRecentParentTicket(ticketKey)
	if err := config.SaveState(state, statePath); err != nil {
		_ = err // Ignore - state saving is optional
	}
}

func handleDescriptionGeneration(
	client jira.JiraClient, reader *bufio.Reader, cfg *config.Config,
	configDir, summary, taskType, ticketKey string,
) error {
	fmt.Print("Would you like to use Gemini to generate the description? [y/N] ")
	response, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read input: %w", err)
	}
	response = strings.TrimSpace(strings.ToLower(response))

	if response != "y" && response != "yes" {
		return nil
	}

	return generateAndUpdateDescription(client, reader, cfg, configDir, summary, taskType, ticketKey)
}

func generateAndUpdateDescription(
	client jira.JiraClient, reader *bufio.Reader, cfg *config.Config,
	configDir, summary, taskType, ticketKey string,
) error {
	geminiClient, err := gemini.NewClient(configDir)
	if err != nil {
		return err
	}

	answerInputMethod := cfg.AnswerInputMethod
	if answerInputMethod == "" {
		answerInputMethod = defaultInputMethod
	}

	description, err := qa.RunQnAFlow(
		geminiClient, summary, cfg.MaxQuestions, summary, taskType, "",
		client, ticketKey, cfg.EpicLinkFieldID, answerInputMethod)
	if err != nil {
		return err
	}

	fmt.Println("\nGenerated description:")
	fmt.Println("---")
	fmt.Println(description)
	fmt.Println("---")
	fmt.Print("\nUpdate ticket with this description? [Y/n/e(dit)] ")

	confirm, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read input: %w", err)
	}
	confirm = strings.TrimSpace(strings.ToLower(confirm))

	if confirm == "e" || confirm == "edit" {
		editedDescription, err := editor.OpenInEditor(description)
		if err != nil {
			return fmt.Errorf("failed to edit description: %w", err)
		}
		description = editedDescription
	}

	if confirm == "n" || confirm == "no" {
		return nil
	}

	if err := client.UpdateTicketDescription(ticketKey, description); err != nil {
		return err
	}
	fmt.Printf("Updated %s with description.\n", ticketKey)

	return promptForReview(client, reader, cfg, ticketKey)
}

func promptForReview(client jira.JiraClient, reader *bufio.Reader, cfg *config.Config, ticketKey string) error {
	fmt.Print("\nWould you like to review this ticket? [y/N] ")
	reviewResponse, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read input: %w", err)
	}
	reviewResponse = strings.TrimSpace(strings.ToLower(reviewResponse))

	if reviewResponse != "y" && reviewResponse != "yes" {
		return nil
	}

	filter := GetTicketFilter(cfg)
	jql := fmt.Sprintf("key = %s", ticketKey)
	jql = jira.ApplyTicketFilter(jql, filter)
	issues, err := client.SearchTickets(jql)
	if err != nil {
		return fmt.Errorf("failed to fetch ticket: %w", err)
	}
	if len(issues) == 0 {
		return fmt.Errorf("ticket %s not found", ticketKey)
	}

	selectedIssue := issues[0]
	if err := reviewTicket(client, reader, cfg, &selectedIssue); err != nil {
		return fmt.Errorf("error reviewing ticket: %w", err)
	}

	return nil
}

// selectParentTicket allows the user to interactively select a parent ticket
func selectParentTicket(
	client jira.JiraClient, reader *bufio.Reader, cfg *config.Config,
	projectKey, _ string,
) (string, error) {
	configDir := GetConfigDir()
	statePath := config.GetStatePath(configDir)
	state, err := config.LoadState(statePath)
	if err != nil {
		state = &config.State{}
	}

	validRecentTickets := getValidRecentTickets(client, state.RecentParentTickets)

	showRecent := len(validRecentTickets) > 0
	if showRecent {
		displayRecentParentTickets(client, validRecentTickets)
	} else {
		if err := displayAllParentTickets(client, cfg, projectKey); err != nil {
			return "", err
		}
	}

	choice, err := readParentTicketChoice(reader)
	if err != nil {
		return "", err
	}

	if ticketKey := validateDirectTicketKey(client, choice); ticketKey != "" {
		return ticketKey, nil
	}

	return processParentTicketSelection(client, reader, cfg, projectKey, choice, validRecentTickets, showRecent)
}

func getValidRecentTickets(client jira.JiraClient, recentTickets []string) []string {
	if len(recentTickets) == 0 {
		return []string{}
	}

	validRecentTickets, err := jira.FilterValidParentTickets(client, recentTickets)
	if err != nil {
		return []string{}
	}
	return validRecentTickets
}

func displayRecentParentTickets(client jira.JiraClient, validRecentTickets []string) {
	fmt.Println("Recent parent tickets:")
	for i, ticketKey := range validRecentTickets {
		issue, err := client.GetIssue(ticketKey)
		if err != nil {
			continue
		}
		issueType := issue.Fields.IssueType.Name
		summary := truncateSummary(issue.Fields.Summary, 50)
		fmt.Printf("[%d] %s [%s]: %s\n", i+1, ticketKey, issueType, summary)
	}
	fmt.Printf("[%d] Other...\n", len(validRecentTickets)+1)
}

func displayAllParentTickets(client jira.JiraClient, cfg *config.Config, projectKey string) error {
	fmt.Println("No recent parent tickets. Searching all tickets in project...")
	issues, err := client.SearchTickets(fmt.Sprintf("project = %s ORDER BY updated DESC", projectKey))
	if err != nil {
		return fmt.Errorf("failed to search tickets: %w", err)
	}
	if len(issues) == 0 {
		return fmt.Errorf("no tickets found in project %s", projectKey)
	}

	validIssues := filterValidParentIssues(client, cfg, issues)
	if len(validIssues) == 0 {
		return fmt.Errorf("no valid parent tickets found in project %s", projectKey)
	}

	displayParentTicketList(validIssues[:minInt(20, len(validIssues))])
	return nil
}

func filterValidParentIssues(client jira.JiraClient, cfg *config.Config, issues []jira.Issue) []jira.Issue {
	var validIssues []jira.Issue
	filter := GetTicketFilter(cfg)

	for i := range issues {
		issue := &issues[i]
		if jira.IsEpic(issue) {
			validIssues = append(validIssues, *issue)
		} else if hasSubtasks(client, issue.Key, filter) {
			validIssues = append(validIssues, *issue)
		}
	}

	return validIssues
}

func hasSubtasks(client jira.JiraClient, issueKey, filter string) bool {
	subtaskJQL := fmt.Sprintf("parent = %s", issueKey)
	subtaskJQL = jira.ApplyTicketFilter(subtaskJQL, filter)
	subtasks, err := client.SearchTickets(subtaskJQL)
	return err == nil && len(subtasks) > 0
}

func displayParentTicketList(validIssues []jira.Issue) {
	fmt.Println("Select parent ticket:")
	for i := range validIssues {
		issue := &validIssues[i]
		summary := truncateSummary(issue.Fields.Summary, 50)
		fmt.Printf("[%d] %s [%s]: %s\n", i+1, issue.Key, issue.Fields.IssueType.Name, summary)
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func readParentTicketChoice(reader *bufio.Reader) (string, error) {
	fmt.Print("> ")
	choice, err := reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("failed to read input: %w", err)
	}
	return strings.TrimSpace(choice), nil
}

func validateDirectTicketKey(client jira.JiraClient, choice string) string {
	if choice == "" || strings.HasPrefix(choice, "[") {
		return ""
	}
	if !strings.Contains(choice, "-") {
		return ""
	}

	_, err := client.GetIssue(choice)
	if err != nil {
		return ""
	}
	return choice
}

func processParentTicketSelection(
	client jira.JiraClient, reader *bufio.Reader, cfg *config.Config,
	projectKey, choice string, validRecentTickets []string, showRecent bool,
) (string, error) {
	selected, err := strconv.Atoi(choice)
	if err != nil {
		return "", fmt.Errorf("invalid selection: %s", choice)
	}

	if showRecent && selected <= len(validRecentTickets) {
		return validRecentTickets[selected-1], nil
	}

	if showRecent && selected == len(validRecentTickets)+1 {
		return selectFromAllParentTickets(client, reader, cfg, projectKey)
	}

	return "", fmt.Errorf("invalid selection: %d", selected)
}

func selectFromAllParentTickets(
	client jira.JiraClient, reader *bufio.Reader, cfg *config.Config, projectKey string,
) (string, error) {
	filter := GetTicketFilter(cfg)
	jql := fmt.Sprintf("project = %s ORDER BY updated DESC", projectKey)
	jql = jira.ApplyTicketFilter(jql, filter)
	issues, err := client.SearchTickets(jql)
	if err != nil {
		return "", fmt.Errorf("failed to search tickets: %w", err)
	}

	validIssues := filterValidParentIssuesSimple(client, issues)
	if len(validIssues) == 0 {
		return "", fmt.Errorf("no valid parent tickets found")
	}

	displayParentTicketList(validIssues[:minInt(20, len(validIssues))])
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

	return validIssues[selected2-1].Key, nil
}

func filterValidParentIssuesSimple(client jira.JiraClient, issues []jira.Issue) []jira.Issue {
	var validIssues []jira.Issue
	for i := range issues {
		issue := &issues[i]
		if jira.IsEpic(issue) {
			validIssues = append(validIssues, *issue)
		} else {
			subtasks, err := client.SearchTickets(fmt.Sprintf("parent = %s", issue.Key))
			if err == nil && len(subtasks) > 0 {
				validIssues = append(validIssues, *issue)
			}
		}
	}
	return validIssues
}

// reviewTicket handles the review workflow for a single ticket
// This is shared between create and review commands
func reviewTicket(client jira.JiraClient, reader *bufio.Reader, cfg *config.Config, issue *jira.Issue) error {
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
				refreshJQL := fmt.Sprintf("key = %s", issue.Key)
				refreshJQL = jira.ApplyTicketFilter(refreshJQL, GetTicketFilter(cfg))
				updated, err := client.SearchTickets(refreshJQL)
				if err == nil && len(updated) > 0 {
					*issue = updated[0]
				}
			}
		case "t", "triage":
			if err := handleTriage(client, reader, issue.Key); err != nil {
				fmt.Printf("Error triaging ticket: %v\n", err)
			} else {
				fmt.Println("Ticket triaged successfully.")
				// Refresh ticket data
				refreshJQL := fmt.Sprintf("key = %s", issue.Key)
				refreshJQL = jira.ApplyTicketFilter(refreshJQL, GetTicketFilter(cfg))
				updated, err := client.SearchTickets(refreshJQL)
				if err == nil && len(updated) > 0 {
					*issue = updated[0]
				}
			}
		case "e", "estimate":
			if err := handleEstimate(client, reader, cfg, issue.Key); err != nil {
				fmt.Printf("Error estimating ticket: %v\n", err)
			} else {
				fmt.Println("Story points updated successfully.")
				// Refresh ticket data
				refreshJQL := fmt.Sprintf("key = %s", issue.Key)
				refreshJQL = jira.ApplyTicketFilter(refreshJQL, GetTicketFilter(cfg))
				updated, err := client.SearchTickets(refreshJQL)
				if err == nil && len(updated) > 0 {
					*issue = updated[0]
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
