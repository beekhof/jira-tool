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
	"go-jira-helper/pkg/parser"
	"go-jira-helper/pkg/qa"

	"github.com/spf13/cobra"
)

var acceptCmd = &cobra.Command{
	Use:   "accept [TICKET_ID]",
	Short: "Convert a research ticket into an Epic and tasks",
	Long: `Accept a completed research ticket and convert it into a new Epic
with decomposed sub-tasks. The ticket will be transitioned to "Done" status.`,
	Args: cobra.ExactArgs(1),
	RunE: runAccept,
}

func runAccept(cmd *cobra.Command, args []string) error {
	ticketID := args[0]

	client, err := jira.NewClient()
	if err != nil {
		return err
	}

	// Load config
	configPath := config.GetConfigPath()
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Get transitions and find "Done" transition
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

	// Transition ticket to Done
	if err := client.TransitionTicket(ticketID, doneTransitionID); err != nil {
		return fmt.Errorf("failed to transition ticket: %w", err)
	}

	// Gather research sources
	sources := []struct {
		Type string
		Name string
		Text string
	}{}

	// Get description
	description, err := client.GetTicketDescription(ticketID)
	if err == nil && description != "" {
		sources = append(sources, struct {
			Type string
			Name string
			Text string
		}{"Description", "Ticket Description", description})
	}

	// Get attachments
	attachments, err := client.GetTicketAttachments(ticketID)
	if err == nil {
		for _, att := range attachments {
			// Note: We'd need to download the attachment content
			// For now, we'll just list them
			sources = append(sources, struct {
				Type string
				Name string
				Text string
			}{"Attachment", att.Filename, fmt.Sprintf("Attachment: %s (ID: %s)", att.Filename, att.ID)})
		}
	}

	// Get comments
	comments, err := client.GetTicketComments(ticketID)
	if err == nil {
		for i, comment := range comments {
			sources = append(sources, struct {
				Type string
				Name string
				Text string
			}{"Comment", fmt.Sprintf("Comment #%d (by %s on %s)", i+1, comment.Author.DisplayName, comment.Created), comment.Body})
		}
	}

	if len(sources) == 0 {
		return fmt.Errorf("no research sources found in ticket %s", ticketID)
	}

	// Present source selection
	reader := bufio.NewReader(os.Stdin)
	fmt.Println("Where is the research?")
	for i, source := range sources {
		fmt.Printf("[%d] %s: %s\n", i+1, source.Type, source.Name)
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

	if selected < 1 || selected > len(sources) {
		return fmt.Errorf("invalid selection: %d", selected)
	}

	selectedSource := sources[selected-1]

	// Prompt for epic summary
	fmt.Print("New Epic Summary: ")
	epicSummary, err := reader.ReadString('\n')
	if err != nil {
		return err
	}
	epicSummary = strings.TrimSpace(epicSummary)

	// Build context for Gemini
	context := fmt.Sprintf("Epic Summary: %s\n\nResearch Text:\n%s", epicSummary, selectedSource.Text)

	// Run Q&A flow
	geminiClient, err := gemini.NewClient()
	if err != nil {
		return err
	}

	plan, err := qa.RunQAFlow(geminiClient, context)
	if err != nil {
		return err
	}

	// Print plan and ask for confirmation
	fmt.Println("\nGenerated Epic Plan:")
	fmt.Println("---")
	fmt.Println(plan)
	fmt.Println("---")
	fmt.Print("\nCreate this Epic and all sub-tasks? [Y/n/e(dit)] ")

	confirm, err := reader.ReadString('\n')
	if err != nil {
		return err
	}
	confirm = strings.TrimSpace(strings.ToLower(confirm))

	if confirm == "e" || confirm == "edit" {
		editedPlan, err := editor.OpenInEditor(plan)
		if err != nil {
			return fmt.Errorf("failed to edit plan: %w", err)
		}
		plan = editedPlan
	}

	if confirm == "n" || confirm == "no" {
		fmt.Println("Cancelled.")
		return nil
	}

	// Parse the plan
	epic, tasks, err := parser.ParseEpicPlan(plan)
	if err != nil {
		return fmt.Errorf("failed to parse epic plan: %w", err)
	}

	// Create the Epic
	project := cfg.DefaultProject
	if project == "" {
		return fmt.Errorf("default_project not configured. Please run 'jira init'")
	}

	epicKey, err := client.CreateTicket(project, "Epic", epic.Title)
	if err != nil {
		return fmt.Errorf("failed to create epic: %w", err)
	}

	// Update epic description
	if epic.Description != "" {
		if err := client.UpdateTicketDescription(epicKey, epic.Description); err != nil {
			return fmt.Errorf("failed to update epic description: %w", err)
		}
	}

	fmt.Printf("Created Epic: %s\n", epicKey)

	// Create tasks
	issueKeys := []string{epicKey}
	for _, task := range tasks {
		taskKey, err := client.CreateTicketWithParent(project, "Task", task.Summary, epicKey)
		if err != nil {
			return fmt.Errorf("failed to create task: %w", err)
		}
		issueKeys = append(issueKeys, taskKey)
		fmt.Printf("Created Task: %s\n", taskKey)
	}

	// Ask about sprint assignment
	fmt.Print("\nAdd this Epic and its tasks to an active Sprint? [y/N] ")
	sprintChoice, err := reader.ReadString('\n')
	if err != nil {
		return err
	}
	sprintChoice = strings.TrimSpace(strings.ToLower(sprintChoice))

	if sprintChoice == "y" || sprintChoice == "yes" {
		boardID := 1 // Default board ID
		sprints, err := client.GetActiveSprints(boardID)
		if err != nil {
			return err
		}

		if len(sprints) > 0 {
			fmt.Println("Select sprint:")
			favorites := cfg.FavoriteSprints
			showFavorites := len(favorites) > 0

			if showFavorites {
				for i, fav := range favorites {
					fmt.Printf("[%d] %s\n", i+1, fav)
				}
				fmt.Printf("[%d] Other...\n", len(favorites)+1)
			} else {
				for i, sprint := range sprints {
					fmt.Printf("[%d] %s\n", i+1, sprint.Name)
				}
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

			var sprintID int
			if showFavorites && selected <= len(favorites) {
				// Map favorite to actual sprint (simplified - would need better mapping)
				if selected <= len(sprints) {
					sprintID = sprints[selected-1].ID
				}
			} else {
				idx := selected - 1
				if showFavorites {
					idx = selected - len(favorites) - 1
				}
				if idx >= 0 && idx < len(sprints) {
					sprintID = sprints[idx].ID
				}
			}

			if sprintID > 0 {
				if err := client.AddIssuesToSprint(sprintID, issueKeys); err != nil {
					return fmt.Errorf("failed to add issues to sprint: %w", err)
				}
				fmt.Printf("Added issues to sprint.\n")
			}
		}
	}

	// Ask about release assignment
	fmt.Print("\nAdd this Epic and its tasks to a Release/Fix Version? [y/N] ")
	releaseChoice, err := reader.ReadString('\n')
	if err != nil {
		return err
	}
	releaseChoice = strings.TrimSpace(strings.ToLower(releaseChoice))

	if releaseChoice == "y" || releaseChoice == "yes" {
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

		if len(unreleased) > 0 {
			fmt.Println("Select release:")
			favorites := cfg.FavoriteReleases
			showFavorites := len(favorites) > 0

			if showFavorites {
				for i, fav := range favorites {
					fmt.Printf("[%d] %s\n", i+1, fav)
				}
				fmt.Printf("[%d] Other...\n", len(favorites)+1)
			} else {
				for i, release := range unreleased {
					fmt.Printf("[%d] %s\n", i+1, release.Name)
				}
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

			var releaseID string
			if showFavorites && selected <= len(favorites) {
				// Find release by name
				for _, r := range unreleased {
					if r.Name == favorites[selected-1] {
						releaseID = r.ID
						break
					}
				}
			} else {
				idx := selected - 1
				if showFavorites {
					idx = selected - len(favorites) - 1
				}
				if idx >= 0 && idx < len(unreleased) {
					releaseID = unreleased[idx].ID
				}
			}

			if releaseID != "" {
				if err := client.AddIssuesToRelease(releaseID, issueKeys); err != nil {
					return fmt.Errorf("failed to add issues to release: %w", err)
				}
				fmt.Printf("Added issues to release.\n")
			}
		}
	}

	return nil
}

func init() {
	rootCmd.AddCommand(acceptCmd)
}
