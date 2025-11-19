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

var (
	projectFlag string
	typeFlag    string
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

func runCreate(cmd *cobra.Command, args []string) error {
	// Check if first argument is "spike" (case-insensitive)
	// If so, prepend "SPIKE: " to the rest of the summary
	summary := strings.Join(args, " ")
	if len(args) > 0 && strings.ToLower(args[0]) == "spike" {
		// If it's "spike", join the rest and prepend "SPIKE: "
		if len(args) > 1 {
			summary = "SPIKE: " + strings.Join(args[1:], " ")
		} else {
			// Just "spike" with no other text
			summary = "SPIKE"
		}
	}

	// Get config directory
	configDir := GetConfigDir()

	// Load config to get defaults
	configPath := config.GetConfigPath(configDir)
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Determine project and type
	project := cfg.DefaultProject
	if projectFlag != "" {
		project = projectFlag
	}
	if project == "" {
		return fmt.Errorf("project not specified. Use --project flag or set default_project in config")
	}

	taskType := cfg.DefaultTaskType
	if typeFlag != "" {
		taskType = typeFlag
	}
	if taskType == "" {
		return fmt.Errorf("task type not specified. Use --type flag or set default_task_type in config")
	}

	// Create Jira client
	client, err := jira.NewClient(configDir)
	if err != nil {
		return err
	}

	// Create the ticket
	ticketKey, err := client.CreateTicket(project, taskType, summary)
	if err != nil {
		return err
	}

	fmt.Printf("Ticket %s created.\n", ticketKey)

	// Ask if user wants to use Gemini to generate description
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Would you like to use Gemini to generate the description? [y/N] ")
	response, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read input: %w", err)
	}
	response = strings.TrimSpace(strings.ToLower(response))

	if response == "y" || response == "yes" {
		// Load config to get max questions
		configPath := config.GetConfigPath(configDir)
		cfg, err := config.LoadConfig(configPath)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		// Initialize Gemini client
		geminiClient, err := gemini.NewClient(configDir)
		if err != nil {
			return err
		}

		// Run Q&A flow (pass summary to detect spike based on SPIKE prefix)
		description, err := qa.RunQnAFlow(geminiClient, summary, cfg.MaxQuestions, summary)
		if err != nil {
			return err
		}

		// Print the generated description
		fmt.Println("\nGenerated description:")
		fmt.Println("---")
		fmt.Println(description)
		fmt.Println("---")

		// Ask for confirmation
		fmt.Print("\nUpdate ticket with this description? [Y/n/e(dit)] ")
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
			// Update the ticket
			if err := client.UpdateTicketDescription(ticketKey, description); err != nil {
				return err
			}
			fmt.Printf("Updated %s with description.\n", ticketKey)
		}
	}

	return nil
}

func init() {
	createCmd.Flags().StringVarP(&projectFlag, "project", "p", "", "Project key (overrides default_project)")
	createCmd.Flags().StringVarP(&typeFlag, "type", "t", "", "Task type (overrides default_task_type)")
	rootCmd.AddCommand(createCmd)
}
