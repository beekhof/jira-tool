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

// HandleAssignmentStep handles ticket assignment with auto-actions (transition, sprint, release)
func HandleAssignmentStep(
	client jira.JiraClient, reader *bufio.Reader, cfg *config.Config,
	ticket *jira.Issue, configDir string,
) (bool, error) {
	// Check if ticket is already assigned
	if ticket.Fields.Assignee.DisplayName != "" ||
		ticket.Fields.Assignee.AccountID != "" ||
		ticket.Fields.Assignee.Name != "" {
		return true, nil // Already assigned
	}

	// Prompt for assignment
	fmt.Print("Assign this ticket? [y/N] ")
	response, err := reader.ReadString('\n')
	if err != nil {
		return false, err
	}
	response = strings.TrimSpace(strings.ToLower(response))

	if response != "y" && response != "yes" {
		return true, nil // Step complete, assignment skipped
	}

	// Load state for recent selections
	statePath := config.GetStatePath(configDir)
	state, err := config.LoadState(statePath)
	if err != nil {
		state = &config.State{}
	}

	selectedUser, userIdentifier, err := selectUserForAssignment(client, reader, state.RecentAssignees)
	if err != nil {
		return false, err
	}

	if err := client.AssignTicket(ticket.Key, selectedUser.AccountID, selectedUser.Name); err != nil {
		return false, fmt.Errorf("failed to assign ticket: %w", err)
	}

	if userIdentifier != "" {
		state.AddRecentAssignee(userIdentifier)
	}

	// Auto-actions after assignment
	transitionToInProgress(client, ticket.Key)
	projectKey := cfg.DefaultProject
	if projectKey != "" {
		handleSprintAssignment(client, reader, cfg, projectKey, ticket.Key, state, statePath)
	}

	isSpike := gemini.IsSpike(ticket.Fields.Summary, ticket.Key)
	if !isSpike && projectKey != "" {
		handleReleaseAssignment(client, reader, projectKey, ticket.Key, state, statePath)
	}

	if err := config.SaveState(state, statePath); err != nil {
		_ = err // Log but don't fail
	}

	return true, nil
}

func transitionToInProgress(client jira.JiraClient, ticketKey string) {
	transitions, err := client.GetTransitions(ticketKey)
	if err != nil {
		return
	}

	var inProgressTransitionID string
	for _, t := range transitions {
		if t.To.Name == "In Progress" {
			inProgressTransitionID = t.ID
			break
		}
	}

	if inProgressTransitionID != "" {
		if err := client.TransitionTicket(ticketKey, inProgressTransitionID); err != nil {
			fmt.Printf("Warning: Could not transition to 'In Progress': %v\n", err)
		}
	}
}

func handleSprintAssignment(
	client jira.JiraClient, reader *bufio.Reader, cfg *config.Config,
	projectKey, ticketKey string, state *config.State, statePath string,
) {
	boardID, err := SelectBoard(client, reader, cfg, projectKey)
	if err != nil {
		return
	}

	activeSprints, err := client.GetActiveSprints(boardID)
	if err != nil {
		return
	}
	plannedSprints, err := client.GetPlannedSprints(boardID)
	if err != nil {
		return
	}
	allSprints := make([]jira.SprintParsed, 0, len(activeSprints)+len(plannedSprints))
	allSprints = append(allSprints, activeSprints...)
	allSprints = append(allSprints, plannedSprints...)

	if len(allSprints) == 0 {
		return
	}

	sprintID, sprintName := selectSprintForAssignment(reader, allSprints, state.RecentSprints)
	if sprintID > 0 {
		if err := client.AddIssuesToSprint(sprintID, []string{ticketKey}); err == nil {
			if sprintName != "" {
				state.AddRecentSprint(sprintName)
				if err := config.SaveState(state, statePath); err != nil {
					_ = err // Ignore - state saving is optional
				}
			}
		}
	}
}

func selectSprintForAssignment(
	reader *bufio.Reader, allSprints []jira.SprintParsed, recent []string,
) (sprintID int, sprintName string) {
	if len(recent) > 0 {
		return selectSprintWithRecent(reader, allSprints, recent)
	}
	return selectSprintFromList(reader, allSprints)
}

func selectSprintWithRecent(
	reader *bufio.Reader, allSprints []jira.SprintParsed, recent []string,
) (sprintID int, sprintName string) {
	fmt.Println("Recent sprints:")
	for i, sprintName := range recent {
		fmt.Printf("[%d] %s\n", i+1, sprintName)
	}
	fmt.Printf("[%d] Other...\n", len(recent)+1)
	fmt.Print("> ")

	choice, err := reader.ReadString('\n')
	if err != nil {
		return 0, ""
	}
	choice = strings.TrimSpace(choice)
	selected, err := strconv.Atoi(choice)
	if err != nil {
		return 0, ""
	}

	if selected >= 1 && selected <= len(recent) {
		sprintName := recent[selected-1]
		for _, sprint := range allSprints {
			if sprint.Name == sprintName {
				return sprint.ID, sprintName
			}
		}
		return 0, ""
	}

	if selected == len(recent)+1 {
		return selectSprintFromList(reader, allSprints)
	}

	return 0, ""
}

func selectSprintFromList(reader *bufio.Reader, allSprints []jira.SprintParsed) (sprintID int, sprintName string) {
	fmt.Println("Select sprint:")
	for i, sprint := range allSprints {
		fmt.Printf("[%d] %s\n", i+1, sprint.Name)
	}
	fmt.Print("> ")

	choice, err := reader.ReadString('\n')
	if err != nil {
		return 0, ""
	}
	selected, err := strconv.Atoi(strings.TrimSpace(choice))
	if err != nil || selected < 1 || selected > len(allSprints) {
		return 0, ""
	}

	sprint := allSprints[selected-1]
	return sprint.ID, sprint.Name
}

func handleReleaseAssignment(
	client jira.JiraClient, reader *bufio.Reader, projectKey, ticketKey string,
	state *config.State, statePath string,
) {
	releases, err := client.GetReleases(projectKey)
	if err != nil {
		return
	}

	unreleased := []jira.ReleaseParsed{}
	for _, r := range releases {
		if !r.Released {
			unreleased = append(unreleased, r)
		}
	}

	if len(unreleased) == 0 {
		return
	}

	releaseID, releaseName := selectReleaseForAssignment(reader, unreleased, state.RecentReleases)
	if releaseID != "" {
		if err := client.AddIssuesToRelease(releaseID, []string{ticketKey}); err == nil {
			if releaseName != "" {
				state.AddRecentRelease(releaseName)
				if err := config.SaveState(state, statePath); err != nil {
					_ = err // Ignore - state saving is optional
				}
			}
		}
	}
}

func selectReleaseForAssignment(
	reader *bufio.Reader, unreleased []jira.ReleaseParsed, recent []string,
) (releaseID, releaseName string) {
	if len(recent) > 0 {
		return selectReleaseWithRecent(reader, unreleased, recent)
	}
	return selectReleaseFromList(reader, unreleased)
}

func selectReleaseWithRecent(
	reader *bufio.Reader, unreleased []jira.ReleaseParsed, recent []string,
) (releaseID, releaseName string) {
	fmt.Println("Recent releases:")
	for i, releaseName := range recent {
		fmt.Printf("[%d] %s\n", i+1, releaseName)
	}
	fmt.Printf("[%d] Other...\n", len(recent)+1)
	fmt.Print("> ")

	choice, err := reader.ReadString('\n')
	if err != nil {
		return "", ""
	}
	choice = strings.TrimSpace(choice)
	selected, err := strconv.Atoi(choice)
	if err != nil {
		return "", ""
	}

	if selected >= 1 && selected <= len(recent) {
		releaseName := recent[selected-1]
		for _, release := range unreleased {
			if release.Name == releaseName {
				return release.ID, releaseName
			}
		}
		return "", ""
	}

	if selected == len(recent)+1 {
		return selectReleaseFromList(reader, unreleased)
	}

	return "", ""
}

func selectReleaseFromList(reader *bufio.Reader, unreleased []jira.ReleaseParsed) (releaseID, releaseName string) {
	fmt.Println("Select release:")
	for i, release := range unreleased {
		fmt.Printf("[%d] %s\n", i+1, release.Name)
	}
	fmt.Print("> ")

	choice, err := reader.ReadString('\n')
	if err != nil {
		return "", ""
	}
	selected, err := strconv.Atoi(strings.TrimSpace(choice))
	if err != nil || selected < 1 || selected > len(unreleased) {
		return "", ""
	}

	release := unreleased[selected-1]
	return release.ID, release.Name
}

func selectUserForAssignment(
	client jira.JiraClient, reader *bufio.Reader, recent []string,
) (user jira.User, userIdentifier string, err error) {
	if len(recent) > 0 {
		selectedFromRecent, user, identifier, err := selectUserFromRecent(client, reader, recent)
		if err != nil {
			return jira.User{}, "", err
		}
		if selectedFromRecent {
			return user, identifier, nil
		}
	}

	return selectUserFromSearch(client, reader)
}

func selectUserFromRecent(
	client jira.JiraClient, reader *bufio.Reader, recent []string,
) (found bool, user jira.User, userIdentifier string, err error) {
	fmt.Println("Recent assignees:")
	for i, userID := range recent {
		fmt.Printf("[%d] %s\n", i+1, userID)
	}
	fmt.Printf("[%d] Other...\n", len(recent)+1)
	fmt.Print("> ")

	choice, err := reader.ReadString('\n')
	if err != nil {
		return false, jira.User{}, "", err
	}
	choice = strings.TrimSpace(choice)
	selected, err := strconv.Atoi(choice)
	if err != nil {
		return false, jira.User{}, "", fmt.Errorf("invalid selection: %s", choice)
	}

	if selected >= 1 && selected <= len(recent) {
		userID := recent[selected-1]
		users, err := client.SearchUsers(userID)
		if err != nil {
			return false, jira.User{}, "", err
		}
		if len(users) == 0 {
			return false, jira.User{}, "", fmt.Errorf("user not found: %s", userID)
		}
		return true, users[0], userID, nil
	}

	if selected != len(recent)+1 {
		return false, jira.User{}, "", fmt.Errorf("invalid selection: %d", selected)
	}

	return false, jira.User{}, "", nil
}

func selectUserFromSearch(
	client jira.JiraClient, reader *bufio.Reader,
) (user jira.User, userIdentifier string, err error) {
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
	for i, u := range users {
		fmt.Printf("[%d] %s (%s) [AccountID: %s]\n", i+1, u.DisplayName, u.Name, u.AccountID)
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

	user = users[selected-1]
	userIdentifier = user.Name
	if userIdentifier == "" {
		userIdentifier = user.AccountID
	}

	return user, userIdentifier, nil
}
