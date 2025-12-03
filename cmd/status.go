package cmd

import (
	"fmt"
	"sort"
	"time"

	"github.com/beekhof/jira-tool/pkg/config"
	"github.com/beekhof/jira-tool/pkg/gemini"
	"github.com/beekhof/jira-tool/pkg/jira"

	"github.com/spf13/cobra"
)

var (
	nextFlag bool
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Display status for sprint or release",
	Long:  `Display progress report for a sprint or release.`,
}

var sprintCmd = &cobra.Command{
	Use:   "sprint",
	Short: "Display sprint status",
	Long:  `Display progress report for the current or next sprint.`,
	RunE:  runSprintStatus,
}

var releaseCmd = &cobra.Command{
	Use:   "release",
	Short: "Display release status",
	Long:  `Display progress report for the current or next release.`,
	RunE:  runReleaseStatus,
}

var spikesCmd = &cobra.Command{
	Use:   "spikes",
	Short: "Display spike tickets status",
	Long:  `Display status report for spike tickets (tickets with "SPIKE" prefix in summary).`,
	RunE:  runSpikesStatus,
}

func runSprintStatus(_ *cobra.Command, _ []string) error {
	configDir := GetConfigDir()
	client, err := jira.NewClient(configDir, GetNoCache())
	if err != nil {
		return err
	}

	selectedSprint, err := selectSprintForStatus(client, 1)
	if err != nil {
		return err
	}

	issues, err := client.GetIssuesForSprint(selectedSprint.ID)
	if err != nil {
		return err
	}

	stats := calculateSprintStats(issues)
	displaySprintStatus(&selectedSprint, stats)
	displaySprintIssues(issues)

	return nil
}

func selectSprintForStatus(client jira.JiraClient, boardID int) (jira.SprintParsed, error) {
	var sprints []jira.SprintParsed
	var err error

	if nextFlag {
		sprints, err = client.GetPlannedSprints(boardID)
	} else {
		sprints, err = client.GetActiveSprints(boardID)
	}
	if err != nil {
		return jira.SprintParsed{}, err
	}

	if len(sprints) == 0 {
		if nextFlag {
			return jira.SprintParsed{}, fmt.Errorf("no planned sprints found")
		}
		return jira.SprintParsed{}, fmt.Errorf("no active sprints found")
	}

	if nextFlag {
		return selectNextSprint(sprints), nil
	}
	return selectActiveSprint(sprints), nil
}

func selectNextSprint(sprints []jira.SprintParsed) jira.SprintParsed {
	sort.Slice(sprints, func(i, j int) bool {
		if sprints[i].StartDate.IsZero() {
			return false
		}
		if sprints[j].StartDate.IsZero() {
			return true
		}
		return sprints[i].StartDate.Before(sprints[j].StartDate)
	})
	return sprints[0]
}

func selectActiveSprint(sprints []jira.SprintParsed) jira.SprintParsed {
	now := time.Now()
	sort.Slice(sprints, func(i, j int) bool {
		if sprints[i].EndDate.IsZero() {
			return false
		}
		if sprints[j].EndDate.IsZero() {
			return true
		}
		diffI := sprints[i].EndDate.Sub(now)
		diffJ := sprints[j].EndDate.Sub(now)
		if diffI < 0 {
			return false
		}
		if diffJ < 0 {
			return true
		}
		return diffI < diffJ
	})
	return sprints[0]
}

type sprintStats struct {
	todoPoints       float64
	inProgressPoints float64
	donePoints       float64
	todoCount        int
	inProgressCount  int
	doneCount        int
	totalPoints      float64
	progressPercent  float64
}

func calculateSprintStats(issues []jira.Issue) sprintStats {
	var stats sprintStats

	for i := range issues {
		issue := &issues[i]
		points := issue.Fields.StoryPoints
		status := issue.Fields.Status.Name

		switch status {
		case "To Do", "Open", "Backlog":
			stats.todoPoints += points
			stats.todoCount++
		case "In Progress", "In Review", "Review":
			stats.inProgressPoints += points
			stats.inProgressCount++
		case "Done", "Closed", "Resolved":
			stats.donePoints += points
			stats.doneCount++
		}
	}

	stats.totalPoints = stats.todoPoints + stats.inProgressPoints + stats.donePoints
	if stats.totalPoints > 0 {
		stats.progressPercent = (stats.donePoints / stats.totalPoints) * 100
	}

	return stats
}

func displaySprintStatus(sprint *jira.SprintParsed, stats sprintStats) {
	daysRemaining := calculateDaysRemaining(sprint.EndDate)

	fmt.Printf("Sprint: %s", sprint.Name)
	if daysRemaining > 0 {
		fmt.Printf(" (ends in %d days)\n", daysRemaining)
	} else if daysRemaining < 0 {
		fmt.Printf(" (ended %d days ago)\n", -daysRemaining)
	} else {
		fmt.Println(" (ends today)")
	}

	bar := buildProgressBar(stats.progressPercent)
	fmt.Printf("Progress: [%s] %.0f%% (%.0f/%.0f points)\n",
		bar, stats.progressPercent, stats.donePoints, stats.totalPoints)

	onTrack := calculateOnTrackStatus(sprint, stats.progressPercent)
	fmt.Printf("On Track: %s\n", onTrack)
	fmt.Println("---")
	fmt.Printf("To Do:       %.0f points (%d issues)\n", stats.todoPoints, stats.todoCount)
	fmt.Printf("In Progress: %.0f points (%d issues)\n", stats.inProgressPoints, stats.inProgressCount)
	fmt.Printf("Done:        %.0f points (%d issues)\n", stats.donePoints, stats.doneCount)
}

func calculateDaysRemaining(endDate time.Time) int {
	if endDate.IsZero() {
		return 0
	}
	return int(time.Until(endDate).Hours() / 24)
}

func buildProgressBar(progressPercent float64) string {
	barLength := 20
	filled := int(progressPercent / 100 * float64(barLength))
	bar := ""
	for i := 0; i < barLength; i++ {
		if i < filled {
			bar += "#"
		} else {
			bar += "-"
		}
	}
	return bar
}

func calculateOnTrackStatus(sprint *jira.SprintParsed, progressPercent float64) string {
	if sprint.StartDate.IsZero() || sprint.EndDate.IsZero() {
		return "Yes"
	}

	totalDuration := sprint.EndDate.Sub(sprint.StartDate)
	elapsed := time.Since(sprint.StartDate)
	if totalDuration <= 0 {
		return "Yes"
	}

	timeProgress := float64(elapsed) / float64(totalDuration)
	if timeProgress > 0.5 && progressPercent < 50 {
		return "No (behind ideal burndown)"
	}
	if timeProgress < 0.5 && progressPercent > 50 {
		return "Yes (ahead of ideal burndown)"
	}
	return "Yes"
}

func displaySprintIssues(issues []jira.Issue) {
	statusGroups := groupIssuesByStatus(issues)

	fmt.Println("\n---")
	fmt.Println("Tickets:")
	fmt.Println()

	statusOrder := []string{"To Do", "Open", "Backlog", "In Progress", "In Review", "Review", "Done", "Closed", "Resolved"}
	for _, statusName := range statusOrder {
		if groupIssues, ok := statusGroups[statusName]; ok {
			fmt.Printf("[%s]\n", statusName)
			for i := range groupIssues {
				issue := &groupIssues[i]
				points := issue.Fields.StoryPoints
				if points > 0 {
					fmt.Printf("  %s: %s (%.0f points)\n", issue.Key, issue.Fields.Summary, points)
				} else {
					fmt.Printf("  %s: %s\n", issue.Key, issue.Fields.Summary)
				}
			}
			fmt.Println()
		}
	}
}

func groupIssuesByStatus(issues []jira.Issue) map[string][]jira.Issue {
	statusGroups := make(map[string][]jira.Issue)
	for i := range issues {
		issue := &issues[i]
		status := issue.Fields.Status.Name
		statusGroups[status] = append(statusGroups[status], *issue)
	}
	return statusGroups
}

func runReleaseStatus(_ *cobra.Command, _ []string) error {
	configDir := GetConfigDir()
	client, err := jira.NewClient(configDir, GetNoCache())
	if err != nil {
		return err
	}

	configPath := config.GetConfigPath(configDir)
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	projectKey := cfg.DefaultProject
	if projectKey == "" {
		return fmt.Errorf("default_project not configured. Please run 'jira init'")
	}

	selectedRelease, err := selectReleaseForStatus(client, projectKey)
	if err != nil {
		return err
	}

	issues, err := client.GetIssuesForRelease(selectedRelease.ID)
	if err != nil {
		return err
	}

	stats := calculateSprintStats(issues)
	displayReleaseStatus(&selectedRelease, stats)
	displaySprintIssues(issues)

	return nil
}

func selectReleaseForStatus(client jira.JiraClient, projectKey string) (jira.ReleaseParsed, error) {
	releases, err := client.GetReleases(projectKey)
	if err != nil {
		return jira.ReleaseParsed{}, err
	}

	unreleased := filterUnreleasedReleases(releases)
	if len(unreleased) == 0 {
		return jira.ReleaseParsed{}, fmt.Errorf("no unreleased versions found")
	}

	if nextFlag {
		return selectNextRelease(unreleased)
	}
	return selectNearestRelease(unreleased), nil
}

func filterUnreleasedReleases(releases []jira.ReleaseParsed) []jira.ReleaseParsed {
	unreleased := []jira.ReleaseParsed{}
	for _, r := range releases {
		if !r.Released {
			unreleased = append(unreleased, r)
		}
	}
	return unreleased
}

func selectNextRelease(unreleased []jira.ReleaseParsed) (jira.ReleaseParsed, error) {
	sort.Slice(unreleased, func(i, j int) bool {
		if unreleased[i].ReleaseDate.IsZero() {
			return false
		}
		if unreleased[j].ReleaseDate.IsZero() {
			return true
		}
		return unreleased[i].ReleaseDate.Before(unreleased[j].ReleaseDate)
	})
	if len(unreleased) < 2 {
		return jira.ReleaseParsed{}, fmt.Errorf("only one unreleased version found")
	}
	return unreleased[1], nil
}

func selectNearestRelease(unreleased []jira.ReleaseParsed) jira.ReleaseParsed {
	now := time.Now()
	sort.Slice(unreleased, func(i, j int) bool {
		if unreleased[i].ReleaseDate.IsZero() {
			return false
		}
		if unreleased[j].ReleaseDate.IsZero() {
			return true
		}
		diffI := unreleased[i].ReleaseDate.Sub(now)
		diffJ := unreleased[j].ReleaseDate.Sub(now)
		if diffI < 0 {
			return false
		}
		if diffJ < 0 {
			return true
		}
		return diffI < diffJ
	})
	return unreleased[0]
}

func displayReleaseStatus(release *jira.ReleaseParsed, stats sprintStats) {
	daysUntilRelease := calculateDaysRemaining(release.ReleaseDate)

	fmt.Printf("Release: %s", release.Name)
	if daysUntilRelease > 0 {
		fmt.Printf(" (releases in %d days)\n", daysUntilRelease)
	} else if daysUntilRelease < 0 {
		fmt.Printf(" (was scheduled %d days ago)\n", -daysUntilRelease)
	} else {
		fmt.Println(" (releases today)")
	}

	bar := buildProgressBar(stats.progressPercent)
	fmt.Printf("Progress: [%s] %.0f%% (%.0f/%.0f points)\n",
		bar, stats.progressPercent, stats.donePoints, stats.totalPoints)
	fmt.Println("---")
	fmt.Printf("To Do:       %.0f points (%d issues)\n", stats.todoPoints, stats.todoCount)
	fmt.Printf("In Progress: %.0f points (%d issues)\n", stats.inProgressPoints, stats.inProgressCount)
	fmt.Printf("Done:        %.0f points (%d issues)\n", stats.donePoints, stats.doneCount)
}

func runSpikesStatus(_ *cobra.Command, _ []string) error {
	configDir := GetConfigDir()
	client, err := jira.NewClient(configDir, GetNoCache())
	if err != nil {
		return err
	}

	// Load config to get default project
	configPath := config.GetConfigPath(configDir)
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	projectKey := cfg.DefaultProject
	if projectKey == "" {
		return fmt.Errorf("default_project not configured. Please run 'jira init'")
	}

	// Get ticket filter
	filter := GetTicketFilter(cfg)

	// Search for spike tickets - JQL ~ operator is case-insensitive
	// Search for tickets with "SPIKE" anywhere in summary, then filter to those starting with SPIKE
	// We search broadly first to catch all variations, then filter in code
	jql := fmt.Sprintf("project = %s AND summary ~ \"spike\" ORDER BY status, updated DESC", projectKey)
	jql = jira.ApplyTicketFilter(jql, filter)
	allIssues, err := client.SearchTickets(jql)
	if err != nil {
		return fmt.Errorf("failed to search for spike tickets: %w", err)
	}

	// Filter to only tickets that start with "SPIKE" prefix (case-insensitive) or have SPIKE in key
	// This ensures we only show actual spike tickets, not tickets that just mention "spike" in the description
	issues := []jira.Issue{}
	for i := range allIssues {
		issue := &allIssues[i]
		summary := issue.Fields.Summary
		key := issue.Key
		// Use IsSpike function to check if this is a spike ticket
		// This checks if summary starts with "SPIKE" (case-insensitive) or key contains "SPIKE"
		if gemini.IsSpike(summary, key) {
			issues = append(issues, *issue)
		}
	}

	if len(issues) == 0 {
		fmt.Println("No spike tickets found.")
		return nil
	}

	// Group by status
	statusGroups := make(map[string][]jira.Issue)
	for i := range issues {
		issue := &issues[i]
		status := issue.Fields.Status.Name
		statusGroups[status] = append(statusGroups[status], *issue)
	}

	// Calculate stats
	var todoPoints, inProgressPoints, donePoints float64
	todoCount := 0
	inProgressCount := 0
	doneCount := 0

	for i := range issues {
		issue := &issues[i]
		points := issue.Fields.StoryPoints
		status := issue.Fields.Status.Name

		switch status {
		case "To Do", "Open", "Backlog":
			todoPoints += points
			todoCount++
		case "In Progress", "In Review", "Review":
			inProgressPoints += points
			inProgressCount++
		case "Done", "Closed", "Resolved":
			donePoints += points
			doneCount++
		}
	}

	totalPoints := todoPoints + inProgressPoints + donePoints
	totalCount := len(issues)

	// Print summary
	fmt.Printf("Spike Tickets Summary\n")
	fmt.Printf("Total: %d tickets (%.0f points)\n", totalCount, totalPoints)
	fmt.Println("---")

	// Print by status
	statusOrder := []string{
		"New", "To Do", "Open", "Backlog", "In Progress", "In Review",
		"Review", "Done", "Closed", "Resolved"}
	for _, statusName := range statusOrder {
		if issues, ok := statusGroups[statusName]; ok {
			var points float64
			for i := range issues {
				points += issues[i].Fields.StoryPoints
			}
			fmt.Printf("%s: %d tickets (%.0f points)\n", statusName, len(issues), points)
		}
	}

	// Print detailed list
	fmt.Println("\n---")
	fmt.Println("Spike Tickets:")
	fmt.Println()

	// Sort status groups by the order above
	for _, statusName := range statusOrder {
		if issues, ok := statusGroups[statusName]; ok {
			fmt.Printf("[%s]\n", statusName)
			for i := range issues {
				issue := &issues[i]
				points := issue.Fields.StoryPoints
				if points > 0 {
					fmt.Printf("  %s: %s (%.0f points)\n", issue.Key, issue.Fields.Summary, points)
				} else {
					fmt.Printf("  %s: %s\n", issue.Key, issue.Fields.Summary)
				}
			}
			fmt.Println()
		}
	}

	return nil
}

func init() {
	statusCmd.AddCommand(sprintCmd)
	statusCmd.AddCommand(releaseCmd)
	statusCmd.AddCommand(spikesCmd)
	sprintCmd.Flags().BoolVarP(&nextFlag, "next", "n", false, "Show next sprint/release instead of current")
	releaseCmd.Flags().BoolVarP(&nextFlag, "next", "n", false, "Show next sprint/release instead of current")
	rootCmd.AddCommand(statusCmd)
}
