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
		JiraURL:           "https://test.atlassian.net",
		DefaultProject:    "TEST",
		DefaultTaskType:   "Task",
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
	// Test with empty configDir (should use default)
	path := GetConfigPath("")
	if path == "" {
		t.Fatal("GetConfigPath returned empty string")
	}
	// Should end with config.yaml
	if filepath.Base(path) != "config.yaml" {
		t.Errorf("Expected config path to end with config.yaml, got %s", path)
	}

	// Test with custom configDir
	customPath := GetConfigPath("/custom/path")
	expected := filepath.Join("/custom/path", "config.yaml")
	if customPath != expected {
		t.Errorf("Expected config path %s, got %s", expected, customPath)
	}
}

func TestNewConfigFields(t *testing.T) {
	t.Run("Load config with new fields missing (should use defaults)", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "config.yaml")

		// Save config without new fields
		cfg := &Config{
			JiraURL:        "https://test.atlassian.net",
			DefaultProject: "TEST",
		}

		if err := SaveConfig(cfg, configPath); err != nil {
			t.Fatalf("Failed to save config: %v", err)
		}

		loaded, err := LoadConfig(configPath)
		if err != nil {
			t.Fatalf("Failed to load config: %v", err)
		}

		// Verify new fields have defaults
		if loaded.DescriptionMinLength != 0 {
			t.Errorf("Expected DescriptionMinLength 0 (default), got %d", loaded.DescriptionMinLength)
		}
		if loaded.DescriptionQualityAI != false {
			t.Errorf("Expected DescriptionQualityAI false (default), got %v", loaded.DescriptionQualityAI)
		}
		if loaded.SeverityFieldID != "" {
			t.Errorf("Expected SeverityFieldID '' (default), got '%s'", loaded.SeverityFieldID)
		}
		if loaded.DefaultBoardID != 0 {
			t.Errorf("Expected DefaultBoardID 0 (default), got %d", loaded.DefaultBoardID)
		}
	})

	t.Run("Load config with new fields set", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "config.yaml")

		cfg := &Config{
			JiraURL:              "https://test.atlassian.net",
			DefaultProject:       "TEST",
			DescriptionMinLength: 256,
			DescriptionQualityAI: true,
			SeverityFieldID:      "customfield_12345",
			DefaultBoardID:       5,
		}

		if err := SaveConfig(cfg, configPath); err != nil {
			t.Fatalf("Failed to save config: %v", err)
		}

		loaded, err := LoadConfig(configPath)
		if err != nil {
			t.Fatalf("Failed to load config: %v", err)
		}

		if loaded.DescriptionMinLength != 256 {
			t.Errorf("Expected DescriptionMinLength 256, got %d", loaded.DescriptionMinLength)
		}
		if loaded.DescriptionQualityAI != true {
			t.Errorf("Expected DescriptionQualityAI true, got %v", loaded.DescriptionQualityAI)
		}
		if loaded.SeverityFieldID != "customfield_12345" {
			t.Errorf("Expected SeverityFieldID 'customfield_12345', got '%s'", loaded.SeverityFieldID)
		}
		if loaded.DefaultBoardID != 5 {
			t.Errorf("Expected DefaultBoardID 5, got %d", loaded.DefaultBoardID)
		}
	})
}

func TestEpicLinkFieldID(t *testing.T) {
	t.Run("Load config with EpicLinkFieldID missing (should use empty string)", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "config.yaml")

		cfg := &Config{
			JiraURL:        "https://test.atlassian.net",
			DefaultProject: "TEST",
		}

		if err := SaveConfig(cfg, configPath); err != nil {
			t.Fatalf("Failed to save config: %v", err)
		}

		loaded, err := LoadConfig(configPath)
		if err != nil {
			t.Fatalf("Failed to load config: %v", err)
		}

		if loaded.EpicLinkFieldID != "" {
			t.Errorf("Expected EpicLinkFieldID '' (default), got '%s'", loaded.EpicLinkFieldID)
		}
	})

	t.Run("Load config with EpicLinkFieldID set", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "config.yaml")

		cfg := &Config{
			JiraURL:         "https://test.atlassian.net",
			DefaultProject:  "TEST",
			EpicLinkFieldID: "customfield_10011",
		}

		if err := SaveConfig(cfg, configPath); err != nil {
			t.Fatalf("Failed to save config: %v", err)
		}

		loaded, err := LoadConfig(configPath)
		if err != nil {
			t.Fatalf("Failed to load config: %v", err)
		}

		if loaded.EpicLinkFieldID != "customfield_10011" {
			t.Errorf("Expected EpicLinkFieldID 'customfield_10011', got '%s'", loaded.EpicLinkFieldID)
		}
	})
}
