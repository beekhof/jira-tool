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
func HandleAssignmentStep(client jira.JiraClient, reader *bufio.Reader, cfg *config.Config, ticket jira.Issue, configDir string) (bool, error) {
	// Check if ticket is already assigned
	if ticket.Fields.Assignee.DisplayName != "" || ticket.Fields.Assignee.AccountID != "" || ticket.Fields.Assignee.Name != "" {
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

	// Reuse assignment logic (similar to handleAssign in cmd/review.go)
	// Show recent assignees
	recent := state.RecentAssignees
	var selectedUser jira.User
	var userIdentifier string

	if len(recent) > 0 {
		fmt.Println("Recent assignees:")
		for i, userID := range recent {
			fmt.Printf("[%d] %s\n", i+1, userID)
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
			userID := recent[selected-1]
			users, err := client.SearchUsers(userID)
			if err != nil {
				return false, err
			}
			if len(users) == 0 {
				return false, fmt.Errorf("user not found: %s", userID)
			}
			selectedUser = users[0]
			userIdentifier = userID
		} else if selected == len(recent)+1 {
			// User selected "Other..." - fall through to search
		} else {
			return false, fmt.Errorf("invalid selection: %d", selected)
		}
	}

	// If no user selected from recent, search
	if selectedUser.AccountID == "" && selectedUser.Name == "" {
		fmt.Print("Search for user: ")
		query, err := reader.ReadString('\n')
		if err != nil {
			return false, err
		}
		query = strings.TrimSpace(query)

		users, err := client.SearchUsers(query)
		if err != nil {
			return false, fmt.Errorf("failed to search for users: %w", err)
		}

		if len(users) == 0 {
			return false, fmt.Errorf("no users found matching: %s", query)
		}

		fmt.Println("Found users:")
		for i, user := range users {
			fmt.Printf("[%d] %s (%s) [AccountID: %s]\n", i+1, user.DisplayName, user.Name, user.AccountID)
		}
		fmt.Print("Select user number: ")

		choice, err := reader.ReadString('\n')
		if err != nil {
			return false, err
		}
		choice = strings.TrimSpace(choice)
		selected, err := strconv.Atoi(choice)
		if err != nil {
			return false, fmt.Errorf("invalid selection: %s", choice)
		}

		if selected < 1 || selected > len(users) {
			return false, fmt.Errorf("invalid selection: %d", selected)
		}

		selectedUser = users[selected-1]
		userIdentifier = selectedUser.Name
		if userIdentifier == "" {
			userIdentifier = selectedUser.AccountID
		}
	}

	// Assign ticket
	if err := client.AssignTicket(ticket.Key, selectedUser.AccountID, selectedUser.Name); err != nil {
		return false, fmt.Errorf("failed to assign ticket: %w", err)
	}

	// Track assignment
	if userIdentifier != "" {
		state.AddRecentAssignee(userIdentifier)
	}

	// Auto-actions after assignment

	// 1. Transition to "In Progress"
	transitions, err := client.GetTransitions(ticket.Key)
	if err == nil {
		var inProgressTransitionID string
		for _, t := range transitions {
			if t.To.Name == "In Progress" {
				inProgressTransitionID = t.ID
				break
			}
		}
		if inProgressTransitionID != "" {
			if err := client.TransitionTicket(ticket.Key, inProgressTransitionID); err != nil {
				// Log but don't fail - transition is optional
				fmt.Printf("Warning: Could not transition to 'In Progress': %v\n", err)
			}
		}
	}

	// 2. Add to sprint
	projectKey := cfg.DefaultProject
	if projectKey != "" {
		boardID, err := SelectBoard(client, reader, cfg, projectKey)
		if err == nil {
			// Get active and planned sprints
			activeSprints, _ := client.GetActiveSprints(boardID)
			plannedSprints, _ := client.GetPlannedSprints(boardID)

			allSprints := append(activeSprints, plannedSprints...)
			if len(allSprints) > 0 {
				// Show recent sprints first
				recent := state.RecentSprints
				if len(recent) > 0 {
					fmt.Println("Recent sprints:")
					for i, sprintName := range recent {
						fmt.Printf("[%d] %s\n", i+1, sprintName)
					}
					fmt.Printf("[%d] Other...\n", len(recent)+1)
					fmt.Print("> ")

					choice, err := reader.ReadString('\n')
					if err == nil {
						choice = strings.TrimSpace(choice)
						selected, err := strconv.Atoi(choice)
						if err == nil && selected >= 1 && selected <= len(recent) {
							// Find sprint by name
							sprintName := recent[selected-1]
							for _, sprint := range allSprints {
								if sprint.Name == sprintName {
									if err := client.AddIssuesToSprint(sprint.ID, []string{ticket.Key}); err == nil {
										state.AddRecentSprint(sprintName)
									}
									break
								}
							}
						} else if selected == len(recent)+1 {
							// Show all sprints
							fmt.Println("Select sprint:")
							for i, sprint := range allSprints {
								fmt.Printf("[%d] %s\n", i+1, sprint.Name)
							}
							fmt.Print("> ")

							choice, err := reader.ReadString('\n')
							if err == nil {
								selected, err := strconv.Atoi(strings.TrimSpace(choice))
								if err == nil && selected >= 1 && selected <= len(allSprints) {
									sprint := allSprints[selected-1]
									if err := client.AddIssuesToSprint(sprint.ID, []string{ticket.Key}); err == nil {
										state.AddRecentSprint(sprint.Name)
									}
								}
							}
						}
					}
				} else {
					// No recent sprints, show all
					fmt.Println("Select sprint:")
					for i, sprint := range allSprints {
						fmt.Printf("[%d] %s\n", i+1, sprint.Name)
					}
					fmt.Print("> ")

					choice, err := reader.ReadString('\n')
					if err == nil {
						selected, err := strconv.Atoi(strings.TrimSpace(choice))
						if err == nil && selected >= 1 && selected <= len(allSprints) {
							sprint := allSprints[selected-1]
							if err := client.AddIssuesToSprint(sprint.ID, []string{ticket.Key}); err == nil {
								state.AddRecentSprint(sprint.Name)
							}
						}
					}
				}
			}
		}
	}

	// 3. Add to release (unless spike)
	isSpike := gemini.IsSpike(ticket.Fields.Summary, ticket.Key)
	if !isSpike && projectKey != "" {
		releases, err := client.GetReleases(projectKey)
		if err == nil {
			// Filter to unreleased versions
			unreleased := []jira.ReleaseParsed{}
			for _, r := range releases {
				if !r.Released {
					unreleased = append(unreleased, r)
				}
			}

			if len(unreleased) > 0 {
				// Show recent releases first
				recent := state.RecentReleases
				if len(recent) > 0 {
					fmt.Println("Recent releases:")
					for i, releaseName := range recent {
						fmt.Printf("[%d] %s\n", i+1, releaseName)
					}
					fmt.Printf("[%d] Other...\n", len(recent)+1)
					fmt.Print("> ")

					choice, err := reader.ReadString('\n')
					if err == nil {
						choice = strings.TrimSpace(choice)
						selected, err := strconv.Atoi(choice)
						if err == nil && selected >= 1 && selected <= len(recent) {
							// Find release by name
							releaseName := recent[selected-1]
							for _, release := range unreleased {
								if release.Name == releaseName {
									if err := client.AddIssuesToRelease(release.ID, []string{ticket.Key}); err == nil {
										state.AddRecentRelease(releaseName)
									}
									break
								}
							}
						} else if selected == len(recent)+1 {
							// Show all releases
							fmt.Println("Select release:")
							for i, release := range unreleased {
								fmt.Printf("[%d] %s\n", i+1, release.Name)
							}
							fmt.Print("> ")

							choice, err := reader.ReadString('\n')
							if err == nil {
								selected, err := strconv.Atoi(strings.TrimSpace(choice))
								if err == nil && selected >= 1 && selected <= len(unreleased) {
									release := unreleased[selected-1]
									if err := client.AddIssuesToRelease(release.ID, []string{ticket.Key}); err == nil {
										state.AddRecentRelease(release.Name)
									}
								}
							}
						}
					}
				} else {
					// No recent releases, show all
					fmt.Println("Select release:")
					for i, release := range unreleased {
						fmt.Printf("[%d] %s\n", i+1, release.Name)
					}
					fmt.Print("> ")

					choice, err := reader.ReadString('\n')
					if err == nil {
						selected, err := strconv.Atoi(strings.TrimSpace(choice))
						if err == nil && selected >= 1 && selected <= len(unreleased) {
							release := unreleased[selected-1]
							if err := client.AddIssuesToRelease(release.ID, []string{ticket.Key}); err == nil {
								state.AddRecentRelease(release.Name)
							}
						}
					}
				}
			}
		}
	}

	// Save state
	if err := config.SaveState(state, statePath); err != nil {
		_ = err // Log but don't fail
	}

	return true, nil
}
