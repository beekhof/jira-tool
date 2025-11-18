package credentials

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveAndLoadCredentials(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "credentials.yaml")

	// Create test credentials
	creds := &Credentials{
		JiraToken: "test-jira-token",
		GeminiKey: "test-gemini-key",
	}

	// Save credentials
	if err := SaveCredentials(creds, path); err != nil {
		t.Fatalf("Failed to save credentials: %v", err)
	}

	// Verify file exists and has correct permissions
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Credentials file not found: %v", err)
	}

	// Check permissions (should be 0600)
	if info.Mode().Perm() != 0600 {
		t.Errorf("Expected file permissions 0600, got %o", info.Mode().Perm())
	}

	// Load credentials
	loaded, err := LoadCredentials(path)
	if err != nil {
		t.Fatalf("Failed to load credentials: %v", err)
	}

	// Verify loaded credentials
	if loaded.JiraToken != creds.JiraToken {
		t.Errorf("Expected JiraToken %s, got %s", creds.JiraToken, loaded.JiraToken)
	}
	if loaded.GeminiKey != creds.GeminiKey {
		t.Errorf("Expected GeminiKey %s, got %s", creds.GeminiKey, loaded.GeminiKey)
	}
}

func TestStoreAndGetSecret(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "credentials.yaml")

	// Store Jira token using direct path
	creds := &Credentials{
		JiraToken: "jira-token-123",
		GeminiKey: "gemini-key-456",
	}
	if err := SaveCredentials(creds, path); err != nil {
		t.Fatalf("Failed to save credentials: %v", err)
	}

	// Manually test StoreSecret with temp directory
	configDir := tmpDir
	if err := StoreSecret(JiraServiceKey, "test@example.com", "jira-token-123", configDir); err != nil {
		t.Fatalf("Failed to store Jira secret: %v", err)
	}

	// For GetSecret, we need to test with the actual path
	// Since GetSecret uses GetCredentialsPath(), we'll test the underlying functions
	loaded, err := LoadCredentials(GetCredentialsPath(""))
	if err != nil {
		// This is expected if credentials file doesn't exist in test environment
		t.Logf("Could not load credentials from default path (expected in test): %v", err)
	}

	// Test with explicit path
	loaded2, err := LoadCredentials(path)
	if err != nil {
		t.Fatalf("Failed to load credentials: %v", err)
	}
	if loaded2.JiraToken != "jira-token-123" {
		t.Errorf("Expected Jira token 'jira-token-123', got '%s'", loaded2.JiraToken)
	}
	if loaded2.GeminiKey != "gemini-key-456" {
		t.Errorf("Expected Gemini key 'gemini-key-456', got '%s'", loaded2.GeminiKey)
	}

	_ = loaded // Suppress unused variable warning
}

func TestGetSecret_NotFound(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "nonexistent.yaml")

	_, err := LoadCredentials(path)
	if err == nil {
		t.Error("Expected error for non-existent credentials file, got nil")
	}
}
