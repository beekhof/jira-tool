package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"syscall"

	"go-jira-helper/pkg/config"
	"go-jira-helper/pkg/credentials"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize the jira-helper configuration",
	Long: `Initialize the jira-helper by prompting for Jira URL, API token,
	and Gemini API key. Non-sensitive data is saved to config.yaml, while
	API keys are stored in a credentials file.`,
	RunE: runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	reader := bufio.NewReader(os.Stdin)

	// Prompt for Jira URL
	fmt.Print("Jira URL (e.g., https://your-company.atlassian.net): ")
	jiraURL, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read Jira URL: %w", err)
	}
	jiraURL = strings.TrimSpace(jiraURL)

	// Prompt for Jira API Token (password input)
	fmt.Print("Jira API Token: ")
	jiraTokenBytes, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		return fmt.Errorf("failed to read Jira token: %w", err)
	}
	jiraToken := string(jiraTokenBytes)
	fmt.Println() // New line after password input

	// Prompt for Gemini API Key (password input)
	fmt.Print("Gemini API Key: ")
	geminiKeyBytes, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		return fmt.Errorf("failed to read Gemini key: %w", err)
	}
	geminiKey := string(geminiKeyBytes)
	fmt.Println() // New line after password input

	// Prompt for default project
	fmt.Print("Default Project Key (e.g., ENG): ")
	defaultProject, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read default project: %w", err)
	}
	defaultProject = strings.TrimSpace(defaultProject)

	// Prompt for default task type
	fmt.Print("Default Task Type (e.g., Task): ")
	defaultTaskType, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read default task type: %w", err)
	}
	defaultTaskType = strings.TrimSpace(defaultTaskType)

	// Save non-sensitive config
	cfg := &config.Config{
		JiraURL:           jiraURL,
		DefaultProject:    defaultProject,
		DefaultTaskType:   defaultTaskType,
		GeminiModel:       "gemini-2.5-flash", // Default to latest flash model
		MaxQuestions:      4,                  // Default to 4 questions
		StoryPointOptions: []int{1, 2, 3, 5, 8, 13}, // Default Fibonacci sequence
	}

	configDir := GetConfigDir()
	configPath := config.GetConfigPath(configDir)
	if err := config.SaveConfig(cfg, configPath); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	// Store API keys in credentials file
	// Use empty string for user since we don't need it for Bearer token auth
	if err := credentials.StoreSecret(credentials.JiraServiceKey, "", jiraToken, configDir); err != nil {
		return fmt.Errorf("failed to store Jira token: %w", err)
	}

	if err := credentials.StoreSecret(credentials.GeminiServiceKey, "", geminiKey, configDir); err != nil {
		return fmt.Errorf("failed to store Gemini key: %w", err)
	}

	fmt.Println("Configuration saved successfully!")
	return nil
}
