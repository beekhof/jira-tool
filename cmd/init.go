package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"syscall"

	"github.com/beekhof/jira-tool/pkg/config"
	"github.com/beekhof/jira-tool/pkg/credentials"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize the jira-tool configuration",
	Long: `Initialize the jira-tool by prompting for Jira URL, API token,
and Gemini API key. Non-sensitive data is saved to config.yaml, while
	API keys are stored in a credentials file.`,
	RunE: runInit,
}

func init() {
	utilsCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	reader := bufio.NewReader(os.Stdin)
	configDir := GetConfigDir()
	configPath := config.GetConfigPath(configDir)

	// Try to load existing config
	var existingCfg *config.Config
	existingCfg, _ = config.LoadConfig(configPath)

	// Prompt for Jira URL
	prompt := "Jira URL (e.g., https://your-company.atlassian.net)"
	if existingCfg != nil && existingCfg.JiraURL != "" {
		prompt = fmt.Sprintf("%s [%s]", prompt, existingCfg.JiraURL)
	}
	fmt.Printf("%s: ", prompt)
	jiraURL, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read Jira URL: %w", err)
	}
	jiraURL = strings.TrimSpace(jiraURL)
	if jiraURL == "" && existingCfg != nil {
		jiraURL = existingCfg.JiraURL
	}

	// Prompt for Jira API Token (password input)
	fmt.Print("Jira API Token (press Enter to keep existing): ")
	jiraTokenBytes, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		return fmt.Errorf("failed to read Jira token: %w", err)
	}
	jiraToken := string(jiraTokenBytes)
	fmt.Println() // New line after password input
	// If empty, try to get existing token
	if jiraToken == "" {
		jiraToken, _ = credentials.GetSecret(credentials.JiraServiceKey, "", configDir)
	}

	// Prompt for Gemini API Key (password input)
	fmt.Print("Gemini API Key (press Enter to keep existing): ")
	geminiKeyBytes, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		return fmt.Errorf("failed to read Gemini key: %w", err)
	}
	geminiKey := string(geminiKeyBytes)
	fmt.Println() // New line after password input
	// If empty, try to get existing key
	if geminiKey == "" {
		geminiKey, _ = credentials.GetSecret(credentials.GeminiServiceKey, "", configDir)
	}

	// Prompt for default project
	prompt = "Default Project Key (e.g., ENG)"
	if existingCfg != nil && existingCfg.DefaultProject != "" {
		prompt = fmt.Sprintf("%s [%s]", prompt, existingCfg.DefaultProject)
	}
	fmt.Printf("%s: ", prompt)
	defaultProject, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read default project: %w", err)
	}
	defaultProject = strings.TrimSpace(defaultProject)
	if defaultProject == "" && existingCfg != nil {
		defaultProject = existingCfg.DefaultProject
	}

	// Prompt for default task type
	prompt = "Default Task Type (e.g., Task)"
	if existingCfg != nil && existingCfg.DefaultTaskType != "" {
		prompt = fmt.Sprintf("%s [%s]", prompt, existingCfg.DefaultTaskType)
	}
	fmt.Printf("%s: ", prompt)
	defaultTaskType, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read default task type: %w", err)
	}
	defaultTaskType = strings.TrimSpace(defaultTaskType)
	if defaultTaskType == "" && existingCfg != nil {
		defaultTaskType = existingCfg.DefaultTaskType
	}

	// Store API keys in credentials file first (needed for field detection)
	// Use empty string for user since we don't need it for Bearer token auth
	if jiraToken != "" {
		if err := credentials.StoreSecret(credentials.JiraServiceKey, "", jiraToken, configDir); err != nil {
			return fmt.Errorf("failed to store Jira token: %w", err)
		}
	}

	if geminiKey != "" {
		if err := credentials.StoreSecret(credentials.GeminiServiceKey, "", geminiKey, configDir); err != nil {
			return fmt.Errorf("failed to store Gemini key: %w", err)
		}
	}

	// Try to detect story points field ID if we have a token and URL
	var storyPointsFieldID string
	if jiraToken != "" && jiraURL != "" {
		fmt.Println("\nDetecting story points field ID...")
		detectedID, err := detectStoryPointsField(jiraURL, jiraToken)
		if err != nil {
			fmt.Printf("Warning: Could not detect story points field ID: %v\n", err)
			if existingCfg != nil && existingCfg.StoryPointsFieldID != "" {
				storyPointsFieldID = existingCfg.StoryPointsFieldID
				fmt.Printf("Keeping existing value: %s\n", storyPointsFieldID)
			} else {
				storyPointsFieldID = "customfield_10016"
				fmt.Println("Using default: customfield_10016")
			}
		} else {
			storyPointsFieldID = detectedID
			fmt.Printf("Detected story points field ID: %s\n", storyPointsFieldID)
		}
	} else {
		// Use existing value or default
		if existingCfg != nil && existingCfg.StoryPointsFieldID != "" {
			storyPointsFieldID = existingCfg.StoryPointsFieldID
		} else {
			storyPointsFieldID = "customfield_10016"
		}
	}

	// Merge with existing config to preserve all settings
	cfg := &config.Config{
		JiraURL:            jiraURL,
		DefaultProject:     defaultProject,
		DefaultTaskType:    defaultTaskType,
		StoryPointsFieldID: storyPointsFieldID,
	}

	// Preserve existing values if they exist
	if existingCfg != nil {
		if cfg.GeminiModel == "" {
			cfg.GeminiModel = existingCfg.GeminiModel
		}
		if cfg.MaxQuestions == 0 {
			cfg.MaxQuestions = existingCfg.MaxQuestions
		}
		if len(cfg.StoryPointOptions) == 0 {
			cfg.StoryPointOptions = existingCfg.StoryPointOptions
		}
		// Recent selections are stored in state.yaml, not config.yaml
		cfg.QuestionPromptTemplate = existingCfg.QuestionPromptTemplate
		cfg.DescriptionPromptTemplate = existingCfg.DescriptionPromptTemplate
		cfg.SpikeQuestionPromptTemplate = existingCfg.SpikeQuestionPromptTemplate
		cfg.SpikePromptTemplate = existingCfg.SpikePromptTemplate
		cfg.ReviewPageSize = existingCfg.ReviewPageSize
	}

	// Set defaults if no existing config
	if cfg.GeminiModel == "" {
		cfg.GeminiModel = "gemini-2.5-flash"
	}
	if cfg.MaxQuestions == 0 {
		cfg.MaxQuestions = 4
	}
	if len(cfg.StoryPointOptions) == 0 {
		cfg.StoryPointOptions = []int{1, 2, 3, 5, 8, 13}
	}

	if err := config.SaveConfig(cfg, configPath); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Println("Configuration saved successfully!")
	return nil
}

// detectStoryPointsField queries the Jira API to find the story points field ID
func detectStoryPointsField(jiraURL, token string) (string, error) {
	endpoint := fmt.Sprintf("%s/rest/api/2/field", jiraURL)

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("Jira API returned error: %d %s - %s", resp.StatusCode, resp.Status, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	// Parse fields response
	var fields []struct {
		ID     string `json:"id"`
		Name   string `json:"name"`
		Custom bool   `json:"custom"`
	}
	if err := json.Unmarshal(body, &fields); err != nil {
		// Try alternative structure
		var fieldsAlt []map[string]interface{}
		if err2 := json.Unmarshal(body, &fieldsAlt); err2 != nil {
			return "", fmt.Errorf("failed to parse fields response: %w", err)
		}
		// Convert to our structure
		for _, f := range fieldsAlt {
			id, _ := f["id"].(string)
			name, _ := f["name"].(string)
			custom, _ := f["custom"].(bool)
			fields = append(fields, struct {
				ID     string `json:"id"`
				Name   string `json:"name"`
				Custom bool   `json:"custom"`
			}{
				ID:     id,
				Name:   name,
				Custom: custom,
			})
		}
	}

	// Search for story points field
	// Common names: "Story Points", "Story Point Estimate", "Story Points Estimate", etc.
	searchTerms := []string{"story point", "storypoint", "story estimate", "point estimate"}
	for _, field := range fields {
		nameLower := strings.ToLower(field.Name)
		for _, term := range searchTerms {
			if strings.Contains(nameLower, term) {
				return field.ID, nil
			}
		}
	}

	return "", fmt.Errorf("story points field not found in Jira fields")
}
