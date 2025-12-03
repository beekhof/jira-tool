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
	"github.com/beekhof/jira-tool/pkg/parser"
	"github.com/beekhof/jira-tool/pkg/qa"

	"github.com/spf13/cobra"
)

const (
	defaultInputMethod = "readline"
	editCommand        = "edit"
)

var acceptCmd = &cobra.Command{
	Use:   "accept [TICKET_ID]",
	Short: "Convert a research ticket into an Epic and tasks",
	Long: `Accept a completed research ticket and convert it into a new Epic
with decomposed sub-tasks. The ticket will be transitioned to "Done" status.`,
	Args: cobra.ExactArgs(1),
	RunE: runAccept,
}

func runAccept(_ *cobra.Command, args []string) error {
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

	if err := transitionToDone(client, ticketID); err != nil {
		return err
	}

	sources, err := gatherResearchSources(client, ticketID)
	if err != nil {
		return err
	}

	reader := bufio.NewReader(os.Stdin)
	selectedSource, err := selectResearchSource(reader, sources)
	if err != nil {
		return err
	}

	epicSummary, err := promptEpicSummary(reader)
	if err != nil {
		return err
	}

	plan, err := generateEpicPlan(client, cfg, ticketID, epicSummary, selectedSource, configDir)
	if err != nil {
		return err
	}

	plan, err = confirmAndEditPlan(reader, plan)
	if err != nil {
		return err
	}
	if plan == "" {
		return nil // User canceled
	}

	epic, tasks, err := parser.ParseEpicPlan(plan)
	if err != nil {
		return fmt.Errorf("failed to parse epic plan: %w", err)
	}

	issueKeys, err := createEpicAndTasks(client, cfg, epic, tasks)
	if err != nil {
		return err
	}

	if err := promptSprintAssignment(client, reader, issueKeys, configDir); err != nil {
		return err
	}

	if err := promptReleaseAssignment(client, reader, cfg.DefaultProject, issueKeys, configDir); err != nil {
		return err
	}

	return nil
}

func transitionToDone(client jira.JiraClient, ticketID string) error {
	transitions, err := client.GetTransitions(ticketID)
	if err != nil {
		return err
	}

	var doneTransitionID string
	for _, t := range transitions {
		if strings.EqualFold(t.To.Name, "Done") || strings.EqualFold(t.To.Name, "Closed") {
			doneTransitionID = t.ID
			break
		}
	}

	if doneTransitionID == "" {
		return fmt.Errorf("could not find 'Done' transition for ticket %s", ticketID)
	}

	return client.TransitionTicket(ticketID, doneTransitionID)
}

type researchSource struct {
	Type string
	Name string
	Text string
}

func gatherResearchSources(client jira.JiraClient, ticketID string) ([]researchSource, error) {
	sources := []researchSource{}

	description, err := client.GetTicketDescription(ticketID)
	if err == nil && description != "" {
		sources = append(sources, researchSource{"Description", "Ticket Description", description})
	}

	attachments, err := client.GetTicketAttachments(ticketID)
	if err == nil {
		for _, att := range attachments {
			sources = append(sources, researchSource{
				"Attachment",
				att.Filename,
				fmt.Sprintf("Attachment: %s (ID: %s)", att.Filename, att.ID),
			})
		}
	}

	comments, err := client.GetTicketComments(ticketID)
	if err == nil {
		for i, comment := range comments {
			sources = append(sources, researchSource{
				"Comment",
				fmt.Sprintf("Comment #%d (by %s on %s)", i+1, comment.Author.DisplayName, comment.Created),
				comment.Body,
			})
		}
	}

	if len(sources) == 0 {
		return nil, fmt.Errorf("no research sources found in ticket %s", ticketID)
	}

	return sources, nil
}

func selectResearchSource(reader *bufio.Reader, sources []researchSource) (researchSource, error) {
	fmt.Println("Where is the research?")
	for i, source := range sources {
		fmt.Printf("[%d] %s: %s\n", i+1, source.Type, source.Name)
	}
	fmt.Print("> ")

	choice, err := reader.ReadString('\n')
	if err != nil {
		return researchSource{}, err
	}
	choice = strings.TrimSpace(choice)
	selected, err := strconv.Atoi(choice)
	if err != nil {
		return researchSource{}, fmt.Errorf("invalid selection: %s", choice)
	}

	if selected < 1 || selected > len(sources) {
		return researchSource{}, fmt.Errorf("invalid selection: %d", selected)
	}

	return sources[selected-1], nil
}

func promptEpicSummary(reader *bufio.Reader) (string, error) {
	fmt.Print("New Epic Summary: ")
	epicSummary, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(epicSummary), nil
}

func generateEpicPlan(
	client jira.JiraClient, cfg *config.Config, ticketID, epicSummary string,
	selectedSource researchSource, configDir string,
) (string, error) {
	context := fmt.Sprintf("Epic Summary: %s\n\nResearch Text:\n%s", epicSummary, selectedSource.Text)

	geminiClient, err := gemini.NewClient(configDir)
	if err != nil {
		return "", err
	}

	issues, err := client.SearchTickets(fmt.Sprintf("key = %s", ticketID))
	var ticketSummary string
	if err == nil && len(issues) > 0 {
		ticketSummary = issues[0].Fields.Summary
	}

	spikeIdentifier := epicSummary
	if ticketSummary != "" {
		spikeIdentifier = ticketSummary
	}

	answerInputMethod := cfg.AnswerInputMethod
	if answerInputMethod == "" {
		answerInputMethod = defaultInputMethod
	}

	return qa.RunQnAFlow(
		geminiClient, context, cfg.MaxQuestions, spikeIdentifier, "Epic", "",
		nil, "", "", answerInputMethod)
}

func confirmAndEditPlan(reader *bufio.Reader, plan string) (string, error) {
	fmt.Println("\nGenerated Epic Plan:")
	fmt.Println("---")
	fmt.Println(plan)
	fmt.Println("---")
	fmt.Print("\nCreate this Epic and all sub-tasks? [Y/n/e(dit)] ")

	confirm, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	confirm = strings.TrimSpace(strings.ToLower(confirm))

	if confirm == "e" || confirm == editCommand {
		editedPlan, err := editor.OpenInEditor(plan)
		if err != nil {
			return "", fmt.Errorf("failed to edit plan: %w", err)
		}
		plan = editedPlan
	}

	if confirm == "n" || confirm == "no" {
		fmt.Println("Canceled.")
		return "", nil
	}

	return plan, nil
}

func createEpicAndTasks(
	client jira.JiraClient, cfg *config.Config,
	epic parser.Epic, tasks []parser.Task,
) ([]string, error) {
	project := cfg.DefaultProject
	if project == "" {
		return nil, fmt.Errorf("default_project not configured. Please run 'jira init'")
	}

	epicKey, err := client.CreateTicket(project, "Epic", epic.Title)
	if err != nil {
		return nil, fmt.Errorf("failed to create epic: %w", err)
	}

	if epic.Description != "" {
		if err := client.UpdateTicketDescription(epicKey, epic.Description); err != nil {
			return nil, fmt.Errorf("failed to update epic description: %w", err)
		}
	}

	fmt.Printf("Created Epic: %s\n", epicKey)

	issueKeys := []string{epicKey}
	for _, task := range tasks {
		taskKey, err := client.CreateTicketWithParent(project, "Task", task.Summary, epicKey)
		if err != nil {
			return nil, fmt.Errorf("failed to create task: %w", err)
		}
		issueKeys = append(issueKeys, taskKey)
		fmt.Printf("Created Task: %s\n", taskKey)
	}

	return issueKeys, nil
}

func promptSprintAssignment(client jira.JiraClient, reader *bufio.Reader, issueKeys []string, configDir string) error {
	fmt.Print("\nAdd this Epic and its tasks to an active Sprint? [y/N] ")
	choice, err := reader.ReadString('\n')
	if err != nil {
		return err
	}
	choice = strings.TrimSpace(strings.ToLower(choice))
	if choice != "y" && choice != "yes" {
		return nil
	}

	boardID := 1
	sprints, err := client.GetActiveSprints(boardID)
	if err != nil || len(sprints) == 0 {
		return err
	}

	statePath := config.GetStatePath(configDir)
	state, err := config.LoadState(statePath)
	if err != nil {
		state = &config.State{}
	}

	sprintID, sprintName, err := selectSprint(reader, sprints, state.RecentSprints)
	if err != nil || sprintID == 0 {
		return err
	}

	if err := client.AddIssuesToSprint(sprintID, issueKeys); err != nil {
		return fmt.Errorf("failed to add issues to sprint: %w", err)
	}

	if sprintName != "" {
		state.AddRecentSprint(sprintName)
		if err := config.SaveState(state, statePath); err != nil {
			_ = err // Ignore - state saving is optional
		}
	}
	fmt.Printf("Added issues to sprint.\n")
	return nil
}

func selectSprint(
	reader *bufio.Reader, sprints []jira.SprintParsed, recent []string,
) (sprintID int, sprintName string, err error) {
	showRecent := len(recent) > 0
	if showRecent {
		fmt.Println("Select sprint:")
		for i, sprintName := range recent {
			fmt.Printf("[%d] %s\n", i+1, sprintName)
		}
		fmt.Printf("[%d] Other...\n", len(recent)+1)
	} else {
		fmt.Println("Select sprint:")
		for i, sprint := range sprints {
			fmt.Printf("[%d] %s\n", i+1, sprint.Name)
		}
	}
	fmt.Print("> ")

	choice, err := reader.ReadString('\n')
	if err != nil {
		return 0, "", err
	}
	choice = strings.TrimSpace(choice)
	selected, err := strconv.Atoi(choice)
	if err != nil {
		return 0, "", fmt.Errorf("invalid selection: %s", choice)
	}

	if showRecent && selected <= len(recent) {
		sprintName := recent[selected-1]
		for _, sprint := range sprints {
			if sprint.Name == sprintName {
				return sprint.ID, sprintName, nil
			}
		}
		return 0, "", fmt.Errorf("sprint not found: %s", sprintName)
	}

	idx := selected - 1
	if showRecent {
		idx = selected - len(recent) - 1
	}
	if idx >= 0 && idx < len(sprints) {
		return sprints[idx].ID, sprints[idx].Name, nil
	}

	return 0, "", fmt.Errorf("invalid selection: %d", selected)
}

func promptReleaseAssignment(
	client jira.JiraClient, reader *bufio.Reader, project string,
	issueKeys []string, configDir string,
) error {
	fmt.Print("\nAdd this Epic and its tasks to a Release/Fix Version? [y/N] ")
	choice, err := reader.ReadString('\n')
	if err != nil {
		return err
	}
	choice = strings.TrimSpace(strings.ToLower(choice))
	if choice != "y" && choice != "yes" {
		return nil
	}

	releases, err := client.GetReleases(project)
	if err != nil {
		return err
	}

	unreleased := []jira.ReleaseParsed{}
	for _, r := range releases {
		if !r.Released {
			unreleased = append(unreleased, r)
		}
	}

	if len(unreleased) == 0 {
		return nil
	}

	statePath := config.GetStatePath(configDir)
	state, err := config.LoadState(statePath)
	if err != nil {
		state = &config.State{}
	}

	releaseID, releaseName, err := selectRelease(reader, unreleased, state.RecentReleases)
	if err != nil || releaseID == "" {
		return err
	}

	if err := client.AddIssuesToRelease(releaseID, issueKeys); err != nil {
		return fmt.Errorf("failed to add issues to release: %w", err)
	}

	if releaseName != "" {
		state.AddRecentRelease(releaseName)
		if err := config.SaveState(state, statePath); err != nil {
			_ = err // Ignore - state saving is optional
		}
	}
	fmt.Printf("Added issues to release.\n")
	return nil
}

func selectRelease(
	reader *bufio.Reader, unreleased []jira.ReleaseParsed, recent []string,
) (releaseID, releaseName string, err error) {
	showRecent := len(recent) > 0
	if showRecent {
		fmt.Println("Select release:")
		for i, releaseName := range recent {
			fmt.Printf("[%d] %s\n", i+1, releaseName)
		}
		fmt.Printf("[%d] Other...\n", len(recent)+1)
	} else {
		fmt.Println("Select release:")
		for i, release := range unreleased {
			fmt.Printf("[%d] %s\n", i+1, release.Name)
		}
	}
	fmt.Print("> ")

	choice, err := reader.ReadString('\n')
	if err != nil {
		return "", "", err
	}
	choice = strings.TrimSpace(choice)
	selected, err := strconv.Atoi(choice)
	if err != nil {
		return "", "", fmt.Errorf("invalid selection: %s", choice)
	}

	if showRecent && selected <= len(recent) {
		releaseName := recent[selected-1]
		for _, r := range unreleased {
			if r.Name == releaseName {
				return r.ID, releaseName, nil
			}
		}
		return "", "", fmt.Errorf("release not found: %s", releaseName)
	}

	idx := selected - 1
	if showRecent {
		idx = selected - len(recent) - 1
	}
	if idx >= 0 && idx < len(unreleased) {
		return unreleased[idx].ID, unreleased[idx].Name, nil
	}

	return "", "", fmt.Errorf("invalid selection: %d", selected)
}

func init() {
	rootCmd.AddCommand(acceptCmd)
}
