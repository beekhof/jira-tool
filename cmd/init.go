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

func runInit(_ *cobra.Command, _ []string) error {
	reader := bufio.NewReader(os.Stdin)
	configDir := GetConfigDir()
	configPath := config.GetConfigPath(configDir)

	existingCfg, err := config.LoadConfig(configPath)
	if err != nil {
		existingCfg = nil
	}

	jiraURL, jiraToken, geminiKey, err := promptBasicConfig(reader, existingCfg, configDir)
	if err != nil {
		return err
	}

	defaultProject, defaultTaskType, err := promptProjectConfig(reader, existingCfg)
	if err != nil {
		return err
	}

	if err := storeCredentials(jiraToken, geminiKey, configDir); err != nil {
		return err
	}

	storyPointsFieldID := detectStoryPointsFieldID(jiraURL, jiraToken, existingCfg)

	epicLinkFieldID := detectEpicLinkFieldID(jiraURL, jiraToken, defaultProject, existingCfg, configDir)

	cfg := &config.Config{
		JiraURL:            jiraURL,
		DefaultProject:     defaultProject,
		DefaultTaskType:    defaultTaskType,
		StoryPointsFieldID: storyPointsFieldID,
		EpicLinkFieldID:    epicLinkFieldID,
	}

	mergeExistingConfig(cfg, existingCfg)
	setDefaultValues(cfg)

	if err := promptAdvancedSettings(reader, cfg, existingCfg, defaultProject, configDir); err != nil {
		return err
	}

	if err := config.SaveConfig(cfg, configPath); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Println("Configuration saved successfully!")
	return nil
}

func promptBasicConfig(
	reader *bufio.Reader, existingCfg *config.Config, configDir string,
) (jiraURL, jiraToken, geminiKey string, err error) {
	jiraURL, err = promptWithDefault(
		reader, "Jira URL (e.g., https://your-company.atlassian.net)", existingCfg,
		func(c *config.Config) string { return c.JiraURL })
	if err != nil {
		return "", "", "", fmt.Errorf("failed to read Jira URL: %w", err)
	}

	jiraToken, err = promptPassword(
		"Jira API Token (press Enter to keep existing)", credentials.JiraServiceKey, configDir)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to read Jira token: %w", err)
	}

	geminiKey, err = promptPassword(
		"Gemini API Key (press Enter to keep existing)", credentials.GeminiServiceKey, configDir)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to read Gemini key: %w", err)
	}

	return jiraURL, jiraToken, geminiKey, nil
}

func promptWithDefault(
	reader *bufio.Reader, promptText string, existingCfg *config.Config,
	getValue func(*config.Config) string,
) (string, error) {
	prompt := promptText
	if existingCfg != nil {
		if value := getValue(existingCfg); value != "" {
			prompt = fmt.Sprintf("%s [%s]", prompt, value)
		}
	}
	fmt.Printf("%s: ", prompt)
	input, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	input = strings.TrimSpace(input)
	if input == "" && existingCfg != nil {
		return getValue(existingCfg), nil
	}
	return input, nil
}

func promptPassword(promptText, serviceKey, configDir string) (string, error) {
	fmt.Print(promptText + ": ")
	tokenBytes, err := term.ReadPassword(syscall.Stdin)
	if err != nil {
		return "", err
	}
	token := string(tokenBytes)
	fmt.Println()
	if token == "" {
		token, err = credentials.GetSecret(serviceKey, "", configDir)
		if err != nil {
			token = ""
		}
	}
	return token, nil
}

func promptProjectConfig(
	reader *bufio.Reader, existingCfg *config.Config,
) (defaultProject, defaultTaskType string, err error) {
	defaultProject, err = promptWithDefault(
		reader, "Default Project Key (e.g., ENG)", existingCfg,
		func(c *config.Config) string { return c.DefaultProject })
	if err != nil {
		return "", "", fmt.Errorf("failed to read default project: %w", err)
	}

	defaultTaskType, err = promptWithDefault(
		reader, "Default Task Type (e.g., Task)", existingCfg,
		func(c *config.Config) string { return c.DefaultTaskType })
	if err != nil {
		return "", "", fmt.Errorf("failed to read default task type: %w", err)
	}

	return defaultProject, defaultTaskType, nil
}

func storeCredentials(jiraToken, geminiKey, configDir string) error {
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
	return nil
}

func detectStoryPointsFieldID(jiraURL, jiraToken string, existingCfg *config.Config) string {
	if jiraToken == "" || jiraURL == "" {
		if existingCfg != nil && existingCfg.StoryPointsFieldID != "" {
			return existingCfg.StoryPointsFieldID
		}
		return "customfield_10016"
	}

	fmt.Println("\nDetecting story points field ID...")
	detectedID, err := detectStoryPointsField(jiraURL, jiraToken)
	if err != nil {
		fmt.Printf("Warning: Could not detect story points field ID: %v\n", err)
		if existingCfg != nil && existingCfg.StoryPointsFieldID != "" {
			fmt.Printf("Keeping existing value: %s\n", existingCfg.StoryPointsFieldID)
			return existingCfg.StoryPointsFieldID
		}
		fmt.Println("Using default: customfield_10016")
		return "customfield_10016"
	}

	fmt.Printf("Detected story points field ID: %s\n", detectedID)
	return detectedID
}

func detectEpicLinkFieldID(
	jiraURL, jiraToken, defaultProject string,
	existingCfg *config.Config, configDir string,
) string {
	if jiraToken == "" || jiraURL == "" {
		if existingCfg != nil && existingCfg.EpicLinkFieldID != "" {
			return existingCfg.EpicLinkFieldID
		}
		return ""
	}

	fmt.Println("\nDetecting Epic Link field ID...")
	tempClient, err := jira.NewClient(configDir, true)
	if err != nil || defaultProject == "" {
		if existingCfg != nil && existingCfg.EpicLinkFieldID != "" {
			return existingCfg.EpicLinkFieldID
		}
		return ""
	}

	detectedID, err := tempClient.DetectEpicLinkField(defaultProject)
	if err != nil {
		fmt.Printf("Warning: Could not detect Epic Link field ID: %v\n", err)
		if existingCfg != nil && existingCfg.EpicLinkFieldID != "" {
			fmt.Printf("Keeping existing value: %s\n", existingCfg.EpicLinkFieldID)
			return existingCfg.EpicLinkFieldID
		}
		return ""
	}

	if detectedID != "" {
		fmt.Printf("Detected Epic Link field ID: %s\n", detectedID)
		return detectedID
	}

	if existingCfg != nil && existingCfg.EpicLinkFieldID != "" {
		fmt.Printf("Epic Link field not detected, keeping existing value: %s\n", existingCfg.EpicLinkFieldID)
		return existingCfg.EpicLinkFieldID
	}

	fmt.Println("Epic Link field not detected (optional)")
	return ""
}

func mergeExistingConfig(cfg, existingCfg *config.Config) {
	if existingCfg == nil {
		return
	}

	if cfg.GeminiModel == "" {
		cfg.GeminiModel = existingCfg.GeminiModel
	}
	if cfg.MaxQuestions == 0 {
		cfg.MaxQuestions = existingCfg.MaxQuestions
	}
	if len(cfg.StoryPointOptions) == 0 {
		cfg.StoryPointOptions = existingCfg.StoryPointOptions
	}
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
	cfg.TicketFilter = existingCfg.TicketFilter
}

func setDefaultValues(cfg *config.Config) {
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
}

func promptAdvancedSettings(
	reader *bufio.Reader, cfg, existingCfg *config.Config,
	defaultProject, configDir string,
) error {
	if err := promptDescriptionQuality(reader, cfg, existingCfg); err != nil {
		return err
	}

	if err := promptSeveritySettings(reader, cfg, existingCfg, defaultProject, configDir); err != nil {
		return err
	}

	if err := promptBoardID(reader, cfg, existingCfg); err != nil {
		return err
	}

	if err := promptAnswerInputMethod(reader, cfg, existingCfg); err != nil {
		return err
	}

	if err := promptTicketFilter(reader, cfg, existingCfg); err != nil {
		return err
	}

	return nil
}

func promptDescriptionQuality(reader *bufio.Reader, cfg, existingCfg *config.Config) error {
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

	return nil
}

func promptSeveritySettings(
	reader *bufio.Reader, cfg, existingCfg *config.Config,
	defaultProject, configDir string,
) error {
	fmt.Print("\nSeverity field ID [auto-detect/enter manually/skip]: ")
	severityInput, err := reader.ReadString('\n')
	if err == nil {
		severityInput = strings.TrimSpace(severityInput)
		if severityInput == "" || strings.EqualFold(severityInput, "skip") {
			if existingCfg != nil && existingCfg.SeverityFieldID != "" {
				cfg.SeverityFieldID = existingCfg.SeverityFieldID
			}
		} else if strings.EqualFold(severityInput, "auto-detect") || strings.EqualFold(severityInput, "auto") {
			if err := detectSeverityField(cfg, existingCfg, defaultProject, configDir); err != nil {
				return err
			}
		} else {
			cfg.SeverityFieldID = severityInput
		}
	} else if existingCfg != nil {
		cfg.SeverityFieldID = existingCfg.SeverityFieldID
	}

	if cfg.SeverityFieldID != "" {
		if err := promptSeverityValues(reader, cfg, existingCfg); err != nil {
			return err
		}
	} else if existingCfg != nil && len(existingCfg.SeverityValues) > 0 {
		cfg.SeverityValues = existingCfg.SeverityValues
	}

	return nil
}

func detectSeverityField(cfg, existingCfg *config.Config, defaultProject, configDir string) error {
	fmt.Println("Detecting severity field ID...")
	jiraClient, err := jira.NewClient(configDir, false)
	if err != nil {
		fmt.Printf("Warning: Could not create Jira client for auto-detection: %v\n", err)
		if existingCfg != nil && existingCfg.SeverityFieldID != "" {
			cfg.SeverityFieldID = existingCfg.SeverityFieldID
		}
		return nil
	}

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
	return nil
}

func promptSeverityValues(reader *bufio.Reader, cfg, existingCfg *config.Config) error {
	fmt.Print("\nSeverity values (comma-separated, e.g., 'Low,Medium,High,Critical' " +
		"or 'skip' to use Jira API values only): ")
	severityValuesInput, err := reader.ReadString('\n')
	if err == nil {
		severityValuesInput = strings.TrimSpace(severityValuesInput)
		if severityValuesInput != "" && !strings.EqualFold(severityValuesInput, "skip") {
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
	return nil
}

func promptBoardID(reader *bufio.Reader, cfg, existingCfg *config.Config) error {
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
	return nil
}

func promptAnswerInputMethod(reader *bufio.Reader, cfg, existingCfg *config.Config) error {
	prompt := "Answer input method [readline/editor/readline_with_preview]"
	if existingCfg != nil && existingCfg.AnswerInputMethod != "" {
		prompt = fmt.Sprintf("%s [%s]", prompt, existingCfg.AnswerInputMethod)
	}
	fmt.Printf("\n%s: ", prompt)
	answerInputMethodInput, err := reader.ReadString('\n')
	if err == nil {
		answerInputMethodInput = strings.TrimSpace(answerInputMethodInput)
		if answerInputMethodInput != "" {
			validMethods := map[string]bool{
				"readline":              true,
				"editor":                true,
				"readline_with_preview": true,
			}
			if validMethods[strings.ToLower(answerInputMethodInput)] {
				cfg.AnswerInputMethod = strings.ToLower(answerInputMethodInput)
			} else if existingCfg != nil && existingCfg.AnswerInputMethod != "" {
				cfg.AnswerInputMethod = existingCfg.AnswerInputMethod
			} else {
				cfg.AnswerInputMethod = defaultInputMethod
			}
		} else if existingCfg != nil && existingCfg.AnswerInputMethod != "" {
			cfg.AnswerInputMethod = existingCfg.AnswerInputMethod
		} else {
			cfg.AnswerInputMethod = "readline"
		}
	} else if existingCfg != nil && existingCfg.AnswerInputMethod != "" {
		cfg.AnswerInputMethod = existingCfg.AnswerInputMethod
	} else {
		cfg.AnswerInputMethod = "readline"
	}
	return nil
}

func promptTicketFilter(reader *bufio.Reader, cfg, existingCfg *config.Config) error {
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
	return nil
}

// detectStoryPointsField queries the Jira API to find the story points field ID
func detectStoryPointsField(jiraURL, token string) (string, error) {
	endpoint := fmt.Sprintf("%s/rest/api/2/field", jiraURL)

	req, err := http.NewRequest("GET", endpoint, http.NoBody)
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
		body, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return "", fmt.Errorf(
				"Jira API returned error: %d %s (failed to read body: %w)",
				resp.StatusCode, resp.Status, readErr)
		}
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
			id, ok := f["id"].(string)
			if !ok {
				id = ""
			}
			name, ok := f["name"].(string)
			if !ok {
				name = ""
			}
			custom, ok := f["custom"].(bool)
			if !ok {
				custom = false
			}
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
