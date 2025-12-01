package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"syscall"

	"github.com/beekhof/jira-tool/pkg/config"
	"github.com/beekhof/jira-tool/pkg/credentials"
	"github.com/beekhof/jira-tool/pkg/jira"

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

	// Try to detect Epic Link field ID if we have a token and URL
	var epicLinkFieldID string
	if jiraToken != "" && jiraURL != "" {
		fmt.Println("\nDetecting Epic Link field ID...")
		// Create temporary client for detection
		tempClient, err := jira.NewClient(configDir, true) // noCache for detection
		if err == nil && defaultProject != "" {
			detectedID, err := tempClient.DetectEpicLinkField(defaultProject)
			if err != nil {
				fmt.Printf("Warning: Could not detect Epic Link field ID: %v\n", err)
				if existingCfg != nil && existingCfg.EpicLinkFieldID != "" {
					epicLinkFieldID = existingCfg.EpicLinkFieldID
					fmt.Printf("Keeping existing value: %s\n", epicLinkFieldID)
				}
			} else if detectedID != "" {
				epicLinkFieldID = detectedID
				fmt.Printf("Detected Epic Link field ID: %s\n", epicLinkFieldID)
			} else {
				if existingCfg != nil && existingCfg.EpicLinkFieldID != "" {
					epicLinkFieldID = existingCfg.EpicLinkFieldID
					fmt.Printf("Epic Link field not detected, keeping existing value: %s\n", epicLinkFieldID)
				} else {
					fmt.Println("Epic Link field not detected (optional)")
				}
			}
		} else if existingCfg != nil && existingCfg.EpicLinkFieldID != "" {
			epicLinkFieldID = existingCfg.EpicLinkFieldID
		}
	} else {
		// Use existing value if present
		if existingCfg != nil && existingCfg.EpicLinkFieldID != "" {
			epicLinkFieldID = existingCfg.EpicLinkFieldID
		}
	}

	// Merge with existing config to preserve all settings
	cfg := &config.Config{
		JiraURL:         jiraURL,
		DefaultProject:  defaultProject,
		DefaultTaskType: defaultTaskType,
		StoryPointsFieldID: storyPointsFieldID,
		EpicLinkFieldID: epicLinkFieldID,
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
		cfg.DescriptionMinLength = existingCfg.DescriptionMinLength
		cfg.DescriptionQualityAI = existingCfg.DescriptionQualityAI
		cfg.SeverityFieldID = existingCfg.SeverityFieldID
		cfg.SeverityValues = existingCfg.SeverityValues
		cfg.DefaultBoardID = existingCfg.DefaultBoardID
		cfg.AnswerInputMethod = existingCfg.AnswerInputMethod
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
	if cfg.DescriptionMinLength == 0 {
		cfg.DescriptionMinLength = 128
	}

	// Prompt for description quality settings
	fmt.Print("\nDescription minimum length (characters) [default: 128]: ")
	descLenInput, err := reader.ReadString('\n')
	if err == nil {
		descLenInput = strings.TrimSpace(descLenInput)
		if descLenInput != "" {
			if descLen, err := strconv.Atoi(descLenInput); err == nil && descLen > 0 {
				cfg.DescriptionMinLength = descLen
			} else if existingCfg != nil && existingCfg.DescriptionMinLength > 0 {
				cfg.DescriptionMinLength = existingCfg.DescriptionMinLength
			}
		} else if existingCfg != nil && existingCfg.DescriptionMinLength > 0 {
			cfg.DescriptionMinLength = existingCfg.DescriptionMinLength
		}
	}

	fmt.Print("Enable AI description quality check? [y/N]: ")
	aiCheckInput, err := reader.ReadString('\n')
	if err == nil {
		aiCheckInput = strings.TrimSpace(strings.ToLower(aiCheckInput))
		if aiCheckInput == "y" || aiCheckInput == "yes" {
			cfg.DescriptionQualityAI = true
		} else if existingCfg != nil {
			cfg.DescriptionQualityAI = existingCfg.DescriptionQualityAI
		}
	} else if existingCfg != nil {
		cfg.DescriptionQualityAI = existingCfg.DescriptionQualityAI
	}

	// Prompt for severity field ID
	fmt.Print("\nSeverity field ID [auto-detect/enter manually/skip]: ")
	severityInput, err := reader.ReadString('\n')
	if err == nil {
		severityInput = strings.TrimSpace(severityInput)
		if severityInput == "" || strings.ToLower(severityInput) == "skip" {
			if existingCfg != nil && existingCfg.SeverityFieldID != "" {
				cfg.SeverityFieldID = existingCfg.SeverityFieldID
			}
		} else if strings.ToLower(severityInput) == "auto-detect" || strings.ToLower(severityInput) == "auto" {
			// Try to auto-detect severity field
			fmt.Println("Detecting severity field ID...")
			jiraClient, err := jira.NewClient(configDir, false)
			if err == nil {
				detectedID, err := jiraClient.DetectSeverityField(defaultProject)
				if err == nil && detectedID != "" {
					cfg.SeverityFieldID = detectedID
					fmt.Printf("Detected severity field ID: %s\n", detectedID)
				} else {
					fmt.Printf("Warning: Could not detect severity field ID: %v\n", err)
					if existingCfg != nil && existingCfg.SeverityFieldID != "" {
						cfg.SeverityFieldID = existingCfg.SeverityFieldID
						fmt.Printf("Keeping existing value: %s\n", cfg.SeverityFieldID)
					}
				}
			} else {
				fmt.Printf("Warning: Could not create Jira client for auto-detection: %v\n", err)
				if existingCfg != nil && existingCfg.SeverityFieldID != "" {
					cfg.SeverityFieldID = existingCfg.SeverityFieldID
				}
			}
		} else {
			// Manual entry
			cfg.SeverityFieldID = severityInput
		}
	} else if existingCfg != nil {
		cfg.SeverityFieldID = existingCfg.SeverityFieldID
	}

	// Prompt for severity values if severity field is configured
	if cfg.SeverityFieldID != "" {
		fmt.Print("\nSeverity values (comma-separated, e.g., 'Low,Medium,High,Critical' or 'skip' to use Jira API values only): ")
		severityValuesInput, err := reader.ReadString('\n')
		if err == nil {
			severityValuesInput = strings.TrimSpace(severityValuesInput)
			if severityValuesInput != "" && strings.ToLower(severityValuesInput) != "skip" {
				// Parse comma-separated values
				values := strings.Split(severityValuesInput, ",")
				cfg.SeverityValues = make([]string, 0, len(values))
				for _, v := range values {
					trimmed := strings.TrimSpace(v)
					if trimmed != "" {
						cfg.SeverityValues = append(cfg.SeverityValues, trimmed)
					}
				}
			} else if existingCfg != nil && len(existingCfg.SeverityValues) > 0 {
				cfg.SeverityValues = existingCfg.SeverityValues
			}
		} else if existingCfg != nil && len(existingCfg.SeverityValues) > 0 {
			cfg.SeverityValues = existingCfg.SeverityValues
		}
	} else if existingCfg != nil && len(existingCfg.SeverityValues) > 0 {
		cfg.SeverityValues = existingCfg.SeverityValues
	}

	// Prompt for default board ID
	fmt.Print("\nDefault board ID (optional, press Enter to skip): ")
	boardIDInput, err := reader.ReadString('\n')
	if err == nil {
		boardIDInput = strings.TrimSpace(boardIDInput)
		if boardIDInput != "" {
			if boardID, err := strconv.Atoi(boardIDInput); err == nil && boardID > 0 {
				cfg.DefaultBoardID = boardID
			} else if existingCfg != nil && existingCfg.DefaultBoardID > 0 {
				cfg.DefaultBoardID = existingCfg.DefaultBoardID
			}
		} else if existingCfg != nil && existingCfg.DefaultBoardID > 0 {
			cfg.DefaultBoardID = existingCfg.DefaultBoardID
		}
	} else if existingCfg != nil {
		cfg.DefaultBoardID = existingCfg.DefaultBoardID
	}

	// Prompt for ticket filter
	fmt.Print("\nTicket filter (JQL to append to all ticket queries, optional, press Enter to skip): ")
	filterInput, err := reader.ReadString('\n')
	if err == nil {
		filterInput = strings.TrimSpace(filterInput)
		if filterInput != "" {
			cfg.TicketFilter = filterInput
		} else if existingCfg != nil && existingCfg.TicketFilter != "" {
			cfg.TicketFilter = existingCfg.TicketFilter
		}
	} else if existingCfg != nil {
		cfg.TicketFilter = existingCfg.TicketFilter
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
