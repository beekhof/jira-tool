package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config holds the application configuration
type Config struct {
	JiraURL           string   `yaml:"jira_url"`
	JiraUser          string   `yaml:"jira_user"`
	DefaultProject    string   `yaml:"default_project"`
	DefaultTaskType   string   `yaml:"default_task_type"`
	FavoriteAssignees []string `yaml:"favorite_assignees,omitempty"`
	FavoriteSprints   []string `yaml:"favorite_sprints,omitempty"`
	FavoriteReleases  []string `yaml:"favorite_releases,omitempty"`
	StoryPointOptions []int    `yaml:"story_point_options,omitempty"`
}

// GetConfigPath returns the default path for the config file
func GetConfigPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		// Fallback to current directory if home dir cannot be determined
		return "./.jira-helper/config.yaml"
	}
	return filepath.Join(homeDir, ".jira-helper", "config.yaml")
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
