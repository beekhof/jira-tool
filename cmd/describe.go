package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/beekhof/jira-tool/pkg/config"
	"github.com/beekhof/jira-tool/pkg/editor"
	"github.com/beekhof/jira-tool/pkg/gemini"
	"github.com/beekhof/jira-tool/pkg/jira"
	"github.com/beekhof/jira-tool/pkg/qa"

	"github.com/spf13/cobra"
)

var describeCmd = &cobra.Command{
	Use:   "describe [TICKET_ID]",
	Short: "Generate or update a ticket description using AI",
	Long: `Generate or update a Jira ticket description using an interactive Q&A flow with Gemini AI.
The ticket ID should be in the format PROJECT-NUMBER (e.g., ENG-123).
If no project prefix is provided, the default project will be used.

This command will:
1. Fetch the ticket details
2. Run an interactive Q&A session to gather information
3. Generate a description based on your answers
4. Ask for confirmation before updating the ticket`,
	Args: cobra.ExactArgs(1),
	RunE: runDescribe,
}

func runDescribe(cmd *cobra.Command, args []string) error {
	configDir := GetConfigDir()

	// Load config
	configPath := config.GetConfigPath(configDir)
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Normalize ticket ID (add default project if needed)
	ticketID := normalizeTicketID(args[0], cfg.DefaultProject)

	// Create Jira client
	client, err := jira.NewClient(configDir, GetNoCache())
	if err != nil {
		return err
	}

	// Get ticket filter
	filter := GetTicketFilter(cfg)

	// Fetch ticket details
	fmt.Printf("Fetching ticket details for %s...\n", ticketID)
	jql := fmt.Sprintf("key = %s", ticketID)
	jql = jira.ApplyTicketFilter(jql, filter)
	issues, err := client.SearchTickets(jql)
	if err != nil {
		return fmt.Errorf("failed to fetch ticket: %w", err)
	}
	if len(issues) == 0 {
		return fmt.Errorf("ticket %s not found", ticketID)
	}

	ticket := issues[0]
	ticketSummary := ticket.Fields.Summary
	issueTypeName := ticket.Fields.IssueType.Name

	// Initialize Gemini client
	geminiClient, err := gemini.NewClient(configDir)
	if err != nil {
		return fmt.Errorf("failed to initialize Gemini client: %w", err)
	}

	// Get existing description if available
	existingDesc, _ := client.GetTicketDescription(ticketID)

	// Run Q&A flow
	answerInputMethod := cfg.AnswerInputMethod
	if answerInputMethod == "" {
		answerInputMethod = "readline"
	}

	fmt.Printf("\nGenerating description for %s: %s\n", ticketID, ticketSummary)
	fmt.Println("Answer the questions below to help generate a comprehensive description.")
	fmt.Println()

	description, err := qa.RunQnAFlow(geminiClient, ticketSummary, cfg.MaxQuestions, ticketSummary, issueTypeName, existingDesc, client, ticketID, cfg.EpicLinkFieldID, answerInputMethod)
	if err != nil {
		return fmt.Errorf("failed to generate description: %w", err)
	}

	// Print and ask for confirmation
	fmt.Println("\nGenerated description:")
	fmt.Println("---")
	fmt.Println(description)
	fmt.Println("---")
	fmt.Print("\nUpdate ticket with this description? [Y/n/e(dit)] ")

	reader := bufio.NewReader(os.Stdin)
	confirm, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read input: %w", err)
	}
	confirm = strings.TrimSpace(strings.ToLower(confirm))

	if confirm == "e" || confirm == "edit" {
		// Open in editor
		editedDescription, err := editor.OpenInEditor(description)
		if err != nil {
			return fmt.Errorf("failed to edit description: %w", err)
		}
		description = editedDescription
	}

	if confirm != "n" && confirm != "no" {
		if err := client.UpdateTicketDescription(ticketID, description); err != nil {
			return fmt.Errorf("failed to update ticket description: %w", err)
		}
		fmt.Printf("\n✓ Description updated for %s\n", ticketID)
		return nil
	}

	fmt.Println("\nDescription not updated.")
	return nil
}

func init() {
	rootCmd.AddCommand(describeCmd)
}
