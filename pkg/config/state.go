package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// State holds runtime state data (recent selections)
type State struct {
	RecentAssignees []string `yaml:"recent_assignees,omitempty"` // Last 6 unique users selected
	RecentSprints   []string `yaml:"recent_sprints,omitempty"`   // Last 6 unique sprints selected
	RecentReleases  []string `yaml:"recent_releases,omitempty"`  // Last 6 unique releases selected
	RecentComponents []string `yaml:"recent_components,omitempty"` // Last 6 unique components selected
}

// GetStatePath returns the path for the state file
// If configDir is empty, uses the default ~/.jira-tool
func GetStatePath(configDir string) string {
	if configDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "./.jira-tool/state.yaml"
		}
		configDir = filepath.Join(homeDir, ".jira-tool")
	}
	return filepath.Join(configDir, "state.yaml")
}

// LoadState loads the state from the specified path
// Returns an empty State if the file doesn't exist (not an error)
func LoadState(path string) (*State, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// State file doesn't exist yet, that's okay - return empty state
			return &State{}, nil
		}
		return nil, fmt.Errorf("failed to read state file: %w", err)
	}

	var state State
	if err := yaml.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to parse state file: %w", err)
	}

	return &state, nil
}

// SaveState saves the state to the specified path
func SaveState(state *State, path string) error {
	// Create directory if it doesn't exist
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create state directory: %w", err)
	}

	data, err := yaml.Marshal(state)
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write state file: %w", err)
	}

	return nil
}

// AddRecentAssignee adds a user to the recent assignees list (max 6 unique)
func (s *State) AddRecentAssignee(userIdentifier string) {
	s.RecentAssignees = addToRecentList(s.RecentAssignees, userIdentifier, 6)
}

// AddRecentSprint adds a sprint to the recent sprints list (max 6 unique)
func (s *State) AddRecentSprint(sprintName string) {
	s.RecentSprints = addToRecentList(s.RecentSprints, sprintName, 6)
}

// AddRecentRelease adds a release to the recent releases list (max 6 unique)
func (s *State) AddRecentRelease(releaseName string) {
	s.RecentReleases = addToRecentList(s.RecentReleases, releaseName, 6)
}

// AddRecentComponent adds a component to the recent components list (max 6 unique)
func (s *State) AddRecentComponent(componentName string) {
	s.RecentComponents = addToRecentList(s.RecentComponents, componentName, 6)
}

// addToRecentList adds an item to a recent list, keeping only the last N unique items
// If the item already exists, it's moved to the end (most recent)
func addToRecentList(list []string, item string, maxSize int) []string {
	// Remove the item if it already exists
	result := []string{}
	for _, existing := range list {
		if existing != item {
			result = append(result, existing)
		}
	}

	// Add the item to the end
	result = append(result, item)

	// Keep only the last maxSize items
	if len(result) > maxSize {
		result = result[len(result)-maxSize:]
	}

	return result
}

