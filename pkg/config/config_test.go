package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	// Create a temporary file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Write test config
	testConfig := &Config{
		JiraURL:         "https://test.atlassian.net",
		JiraUser:        "test@example.com",
		DefaultProject:  "TEST",
		DefaultTaskType: "Task",
	}

	if err := SaveConfig(testConfig, configPath); err != nil {
		t.Fatalf("Failed to save test config: %v", err)
	}

	// Load and verify
	loaded, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if loaded.JiraURL != testConfig.JiraURL {
		t.Errorf("Expected JiraURL %s, got %s", testConfig.JiraURL, loaded.JiraURL)
	}
	if loaded.JiraUser != testConfig.JiraUser {
		t.Errorf("Expected JiraUser %s, got %s", testConfig.JiraUser, loaded.JiraUser)
	}
	if loaded.DefaultProject != testConfig.DefaultProject {
		t.Errorf("Expected DefaultProject %s, got %s", testConfig.DefaultProject, loaded.DefaultProject)
	}
	if loaded.DefaultTaskType != testConfig.DefaultTaskType {
		t.Errorf("Expected DefaultTaskType %s, got %s", testConfig.DefaultTaskType, loaded.DefaultTaskType)
	}
}

func TestSaveConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	testConfig := &Config{
		JiraURL:         "https://test.atlassian.net",
		JiraUser:        "test@example.com",
		DefaultProject:  "TEST",
		DefaultTaskType: "Task",
		StoryPointOptions: []int{1, 2, 3, 5, 8, 13},
	}

	if err := SaveConfig(testConfig, configPath); err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Fatal("Config file was not created")
	}

	// Load and verify
	loaded, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load saved config: %v", err)
	}

	if loaded.JiraURL != testConfig.JiraURL {
		t.Errorf("Expected JiraURL %s, got %s", testConfig.JiraURL, loaded.JiraURL)
	}
}

func TestGetConfigPath(t *testing.T) {
	path := GetConfigPath()
	if path == "" {
		t.Fatal("GetConfigPath returned empty string")
	}
	// Should end with config.yaml
	if filepath.Base(path) != "config.yaml" {
		t.Errorf("Expected config path to end with config.yaml, got %s", path)
	}
}

