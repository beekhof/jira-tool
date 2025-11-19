package credentials

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Credentials holds API keys and tokens
type Credentials struct {
	JiraToken string `yaml:"jira_token"`
	GeminiKey string `yaml:"gemini_key"`
}

// GetCredentialsPath returns the path for the credentials file
// If configDir is empty, uses the default ~/.jira-tool
func GetCredentialsPath(configDir string) string {
	if configDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			// Fallback to current directory if home dir cannot be determined
			return "./.jira-tool/credentials.yaml"
		}
		configDir = filepath.Join(homeDir, ".jira-tool")
	}
	return filepath.Join(configDir, "credentials.yaml")
}

// LoadCredentials loads credentials from the specified path
func LoadCredentials(path string) (*Credentials, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read credentials file: %w", err)
	}

	var creds Credentials
	if err := yaml.Unmarshal(data, &creds); err != nil {
		return nil, fmt.Errorf("failed to parse credentials file: %w", err)
	}

	return &creds, nil
}

// SaveCredentials saves credentials to the specified path
func SaveCredentials(creds *Credentials, path string) error {
	// Create directory if it doesn't exist
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create credentials directory: %w", err)
	}

	data, err := yaml.Marshal(creds)
	if err != nil {
		return fmt.Errorf("failed to marshal credentials: %w", err)
	}

	// Write with restricted permissions (owner read/write only)
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write credentials file: %w", err)
	}

	return nil
}

// StoreSecret stores a secret in the credentials file
// For backward compatibility with the old keyring interface
// Note: This function now requires configDir to be passed via GetCredentialsPath
func StoreSecret(service, user, secret, configDir string) error {
	path := GetCredentialsPath(configDir)

	// Try to load existing credentials, or create new
	creds, err := LoadCredentials(path)
	if err != nil {
		// File doesn't exist, create new
		creds = &Credentials{}
	}

	// Store based on service type
	if service == "jira-tool-jira" {
		creds.JiraToken = secret
	} else if service == "jira-tool-gemini" {
		creds.GeminiKey = secret
	} else {
		return fmt.Errorf("unknown service: %s", service)
	}

	return SaveCredentials(creds, path)
}

// GetSecret retrieves a secret from the credentials file
// For backward compatibility with the old keyring interface
// Note: This function now requires configDir to be passed via GetCredentialsPath
func GetSecret(service, user, configDir string) (string, error) {
	path := GetCredentialsPath(configDir)

	creds, err := LoadCredentials(path)
	if err != nil {
		return "", fmt.Errorf("failed to load credentials: %w. Please run 'jira init'", err)
	}

	if service == "jira-tool-jira" {
		if creds.JiraToken == "" {
			return "", fmt.Errorf("jira token not found. Please run 'jira init'")
		}
		return creds.JiraToken, nil
	} else if service == "jira-tool-gemini" {
		if creds.GeminiKey == "" {
			return "", fmt.Errorf("gemini key not found. Please run 'jira init'")
		}
		return creds.GeminiKey, nil
	}

	return "", fmt.Errorf("unknown service: %s", service)
}

// Constants for backward compatibility
const (
	JiraServiceKey   = "jira-tool-jira"
	GeminiServiceKey = "jira-tool-gemini"
)
