package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/beekhof/jira-tool/pkg/config"
	"github.com/beekhof/jira-tool/pkg/gemini"
	"github.com/beekhof/jira-tool/pkg/jira"

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

	// Get config directory
	configDir := GetConfigDir()

	// Create Jira client
	client, err := jira.NewClient(configDir)
	if err != nil {
		return err
	}

	// Load config to get story point options
	configPath := config.GetConfigPath(configDir)
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Get story point options (default to Fibonacci if not configured)
	storyPoints := cfg.StoryPointOptions
	if len(storyPoints) == 0 {
		storyPoints = []int{1, 2, 3, 5, 8, 13}
	}

	// Fetch ticket details for Gemini estimation
	fmt.Printf("Fetching ticket details for %s...\n", ticketID)
	issues, err := client.SearchTickets(fmt.Sprintf("key = %s", ticketID))
	if err != nil {
		return fmt.Errorf("failed to fetch ticket: %w", err)
	}
	if len(issues) == 0 {
		return fmt.Errorf("ticket %s not found", ticketID)
	}

	ticket := issues[0]
	summary := ticket.Fields.Summary
	description, err := client.GetTicketDescription(ticketID)
	if err != nil {
		// Description might be empty, that's okay
		description = ""
	}

	// Get Gemini estimate
	fmt.Println("Getting AI story point estimate...")
	geminiClient, err := gemini.NewClient(configDir)
	if err != nil {
		// If Gemini fails, continue with manual selection
		fmt.Printf("Warning: Could not initialize Gemini client: %v\n", err)
		fmt.Println("Continuing with manual selection...")
	} else {
		estimate, reasoning, err := geminiClient.EstimateStoryPoints(summary, description, storyPoints)
		if err != nil {
			fmt.Printf("Warning: Could not get AI estimate: %v\n", err)
			fmt.Println("Continuing with manual selection...")
		} else {
			fmt.Printf("\n🤖 AI Estimate: %d story points\n", estimate)
			if reasoning != "" {
				fmt.Printf("   Reasoning: %s\n", reasoning)
			}
			fmt.Println()
		}
	}

	// Display the Fibonacci prompt with letters
	reader := bufio.NewReader(os.Stdin)
	fmt.Println("Select story points:")
	for i, points := range storyPoints {
		letter := string(rune('a' + i))
		fmt.Printf("[%s] %d\n", letter, points)
	}
	fmt.Println("Or enter a number directly")
	fmt.Print("> ")

	input, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read input: %w", err)
	}
	input = strings.TrimSpace(strings.ToLower(input))

	var points int
	// Try to parse as number first
	if num, err := strconv.Atoi(input); err == nil {
		// Direct number entry
		if num <= 0 {
			return fmt.Errorf("story points must be positive")
		}
		points = num
	} else if len(input) == 1 {
		// Try to parse as letter
		letter := input[0]
		index := int(letter - 'a')
		if index >= 0 && index < len(storyPoints) {
			points = storyPoints[index]
		} else {
			return fmt.Errorf("invalid selection: %s", input)
		}
	} else {
		return fmt.Errorf("invalid input: %s (use a letter or number)", input)
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
