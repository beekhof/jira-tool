package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"go-jira-helper/pkg/config"
	"go-jira-helper/pkg/jira"

	"github.com/spf13/cobra"
)

var estimateCmd = &cobra.Command{
	Use:   "estimate [TICKET_ID]",
	Short: "Estimate story points for a ticket",
	Long: `Estimate story points for a Jira ticket using a Fibonacci sequence prompt.
The ticket ID should be in the format PROJECT-NUMBER (e.g., ENG-123).`,
	Args: cobra.ExactArgs(1),
	RunE: runEstimate,
}

func runEstimate(cmd *cobra.Command, args []string) error {
	ticketID := args[0]

	// Create Jira client
	client, err := jira.NewClient()
	if err != nil {
		return err
	}

	// Load config to get story point options
	configPath := config.GetConfigPath()
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Get story point options (default to Fibonacci if not configured)
	storyPoints := cfg.StoryPointOptions
	if len(storyPoints) == 0 {
		storyPoints = []int{1, 2, 3, 5, 8, 13}
	}

	// Display the Fibonacci prompt
	reader := bufio.NewReader(os.Stdin)
	fmt.Println("Select story points:")
	for i, points := range storyPoints {
		fmt.Printf("[%d] %d\n", i+1, points)
	}
	fmt.Printf("[%d] Other...\n", len(storyPoints)+1)
	fmt.Print("> ")

	input, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read input: %w", err)
	}
	input = strings.TrimSpace(input)

	// Parse selection
	selected, err := strconv.Atoi(input)
	if err != nil {
		return fmt.Errorf("invalid selection: %s", input)
	}

	var points int
	if selected >= 1 && selected <= len(storyPoints) {
		points = storyPoints[selected-1]
	} else if selected == len(storyPoints)+1 {
		// Prompt for custom value
		fmt.Print("Enter story points: ")
		customInput, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read input: %w", err)
		}
		customInput = strings.TrimSpace(customInput)
		points, err = strconv.Atoi(customInput)
		if err != nil {
			return fmt.Errorf("invalid number: %s", customInput)
		}
		if points <= 0 {
			return fmt.Errorf("story points must be positive")
		}
	} else {
		return fmt.Errorf("invalid selection: %d", selected)
	}

	// Update the ticket
	if err := client.UpdateTicketPoints(ticketID, points); err != nil {
		return err
	}

	fmt.Printf("Updated %s with %d story points.\n", ticketID, points)
	return nil
}

func init() {
	rootCmd.AddCommand(estimateCmd)
}
