package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config holds the application configuration
type Config struct {
	JiraURL                string   `yaml:"jira_url"`
	DefaultProject         string   `yaml:"default_project"`
	DefaultTaskType        string   `yaml:"default_task_type"`
	GeminiModel            string   `yaml:"gemini_model,omitempty"`
	MaxQuestions           int      `yaml:"max_questions,omitempty"`
	QuestionPromptTemplate string   `yaml:"question_prompt_template,omitempty"`
	DescriptionPromptTemplate string `yaml:"description_prompt_template,omitempty"`
	SpikeQuestionPromptTemplate string `yaml:"spike_question_prompt_template,omitempty"`
	SpikePromptTemplate    string   `yaml:"spike_prompt_template,omitempty"`
	EpicFeatureQuestionPromptTemplate string `yaml:"epic_feature_question_prompt_template,omitempty"`
	EpicFeaturePromptTemplate string `yaml:"epic_feature_prompt_template,omitempty"`
	ReviewPageSize         int      `yaml:"review_page_size,omitempty"`
	StoryPointOptions      []int    `yaml:"story_point_options,omitempty"`
	StoryPointsFieldID     string   `yaml:"story_points_field_id,omitempty"`
	DescriptionMinLength   int      `yaml:"description_min_length,omitempty"`   // Minimum description length (default: 128)
	DescriptionQualityAI   bool     `yaml:"description_quality_ai,omitempty"`   // Enable Gemini AI analysis for description quality (default: false)
	SeverityFieldID        string   `yaml:"severity_field_id,omitempty"`         // Custom field ID for severity (optional)
	DefaultBoardID         int      `yaml:"default_board_id,omitempty"`         // Default board ID if auto-detection fails (default: 0)
	EpicLinkFieldID        string   `yaml:"epic_link_field_id,omitempty"`       // Epic Link custom field ID (auto-detected or manually configured)
	TicketFilter           string   `yaml:"ticket_filter,omitempty"`            // JQL filter to append to all ticket queries (e.g., "assignee = currentUser()")
	AnswerInputMethod      string   `yaml:"answer_input_method,omitempty"`      // Answer input method: "readline", "editor", or "readline_with_preview" (default: "readline_with_preview")
}

// GetConfigPath returns the path for the config file
// If configDir is empty, uses the default ~/.jira-tool
func GetConfigPath(configDir string) string {
	if configDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			// Fallback to current directory if home dir cannot be determined
			return "./.jira-tool/config.yaml"
		}
		configDir = filepath.Join(homeDir, ".jira-tool")
	}
	return filepath.Join(configDir, "config.yaml")
}

// LoadConfig loads the configuration from the specified path
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &cfg, nil
}

// SaveConfig saves the configuration to the specified path
func SaveConfig(cfg *Config, path string) error {
	// Create directory if it doesn't exist
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}
