package cmd

import (
	"fmt"
	"sort"
	"time"

	"go-jira-helper/pkg/config"
	"go-jira-helper/pkg/jira"

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

func runSprintStatus(cmd *cobra.Command, args []string) error {
	client, err := jira.NewClient()
	if err != nil {
		return err
	}

	// For now, use board ID 1 as default (this could be configurable)
	boardID := 1

	var sprints []jira.SprintParsed
	if nextFlag {
		sprints, err = client.GetPlannedSprints(boardID)
	} else {
		sprints, err = client.GetActiveSprints(boardID)
	}
	if err != nil {
		return err
	}

	if len(sprints) == 0 {
		if nextFlag {
			return fmt.Errorf("no planned sprints found")
		}
		return fmt.Errorf("no active sprints found")
	}

	// Find the appropriate sprint
	var selectedSprint jira.SprintParsed
	if nextFlag {
		// Find planned sprint with earliest start date
		sort.Slice(sprints, func(i, j int) bool {
			if sprints[i].StartDate.IsZero() {
				return false
			}
			if sprints[j].StartDate.IsZero() {
				return true
			}
			return sprints[i].StartDate.Before(sprints[j].StartDate)
		})
		selectedSprint = sprints[0]
	} else {
		// Find active sprint with nearest end date
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
		selectedSprint = sprints[0]
	}

	// Get issues for the sprint
	issues, err := client.GetIssuesForSprint(selectedSprint.ID)
	if err != nil {
		return err
	}

	// Calculate stats
	var todoPoints, inProgressPoints, donePoints float64
	todoCount := 0
	inProgressCount := 0
	doneCount := 0

	for _, issue := range issues {
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
	progressPercent := 0.0
	if totalPoints > 0 {
		progressPercent = (donePoints / totalPoints) * 100
	}

	// Calculate days remaining
	daysRemaining := 0
	if !selectedSprint.EndDate.IsZero() {
		daysRemaining = int(time.Until(selectedSprint.EndDate).Hours() / 24)
	}

	// Print report
	fmt.Printf("Sprint: %s", selectedSprint.Name)
	if daysRemaining > 0 {
		fmt.Printf(" (ends in %d days)\n", daysRemaining)
	} else if daysRemaining < 0 {
		fmt.Printf(" (ended %d days ago)\n", -daysRemaining)
	} else {
		fmt.Println(" (ends today)")
	}

	// Progress bar (simple text representation)
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

	fmt.Printf("Progress: [%s] %.0f%% (%.0f/%.0f points)\n", bar, progressPercent, donePoints, totalPoints)

	// Determine if on track (simplified - just check if progress > 50% when more than half time has passed)
	onTrack := "Yes"
	if !selectedSprint.StartDate.IsZero() && !selectedSprint.EndDate.IsZero() {
		totalDuration := selectedSprint.EndDate.Sub(selectedSprint.StartDate)
		elapsed := time.Since(selectedSprint.StartDate)
		if totalDuration > 0 {
			timeProgress := float64(elapsed) / float64(totalDuration)
			if timeProgress > 0.5 && progressPercent < 50 {
				onTrack = "No (behind ideal burndown)"
			} else if timeProgress < 0.5 && progressPercent > 50 {
				onTrack = "Yes (ahead of ideal burndown)"
			}
		}
	}
	fmt.Printf("On Track: %s\n", onTrack)
	fmt.Println("---")
	fmt.Printf("To Do:       %.0f points (%d issues)\n", todoPoints, todoCount)
	fmt.Printf("In Progress: %.0f points (%d issues)\n", inProgressPoints, inProgressCount)
	fmt.Printf("Done:        %.0f points (%d issues)\n", donePoints, doneCount)

	return nil
}

func runReleaseStatus(cmd *cobra.Command, args []string) error {
	client, err := jira.NewClient()
	if err != nil {
		return err
	}

	// Load config to get default project
	configPath := config.GetConfigPath()
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	projectKey := cfg.DefaultProject
	if projectKey == "" {
		return fmt.Errorf("default_project not configured. Please run 'jira init'")
	}

	// Get releases
	releases, err := client.GetReleases(projectKey)
	if err != nil {
		return err
	}

	// Filter unreleased releases
	unreleased := []jira.ReleaseParsed{}
	for _, r := range releases {
		if !r.Released {
			unreleased = append(unreleased, r)
		}
	}

	if len(unreleased) == 0 {
		return fmt.Errorf("no unreleased versions found")
	}

	// Find the appropriate release
	var selectedRelease jira.ReleaseParsed
	if nextFlag {
		// Find second-nearest release date
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
			return fmt.Errorf("only one unreleased version found")
		}
		selectedRelease = unreleased[1]
	} else {
		// Find nearest release date
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
		selectedRelease = unreleased[0]
	}

	// Get issues for the release
	issues, err := client.GetIssuesForRelease(selectedRelease.ID)
	if err != nil {
		return err
	}

	// Calculate stats (same as sprint)
	var todoPoints, inProgressPoints, donePoints float64
	todoCount := 0
	inProgressCount := 0
	doneCount := 0

	for _, issue := range issues {
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
	progressPercent := 0.0
	if totalPoints > 0 {
		progressPercent = (donePoints / totalPoints) * 100
	}

	// Calculate days until release
	daysUntilRelease := 0
	if !selectedRelease.ReleaseDate.IsZero() {
		daysUntilRelease = int(time.Until(selectedRelease.ReleaseDate).Hours() / 24)
	}

	// Print report
	fmt.Printf("Release: %s", selectedRelease.Name)
	if daysUntilRelease > 0 {
		fmt.Printf(" (releases in %d days)\n", daysUntilRelease)
	} else if daysUntilRelease < 0 {
		fmt.Printf(" (was scheduled %d days ago)\n", -daysUntilRelease)
	} else {
		fmt.Println(" (releases today)")
	}

	// Progress bar
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

	fmt.Printf("Progress: [%s] %.0f%% (%.0f/%.0f points)\n", bar, progressPercent, donePoints, totalPoints)
	fmt.Println("---")
	fmt.Printf("To Do:       %.0f points (%d issues)\n", todoPoints, todoCount)
	fmt.Printf("In Progress: %.0f points (%d issues)\n", inProgressPoints, inProgressCount)
	fmt.Printf("Done:        %.0f points (%d issues)\n", donePoints, doneCount)

	return nil
}

func init() {
	statusCmd.AddCommand(sprintCmd)
	statusCmd.AddCommand(releaseCmd)
	sprintCmd.Flags().BoolVarP(&nextFlag, "next", "n", false, "Show next sprint/release instead of current")
	releaseCmd.Flags().BoolVarP(&nextFlag, "next", "n", false, "Show next sprint/release instead of current")
	rootCmd.AddCommand(statusCmd)
}
