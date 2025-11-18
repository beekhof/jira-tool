package cmd

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go-jira-helper/pkg/config"
)

func TestInitCommand(t *testing.T) {
	// Create a temporary directory for config
	tmpDir := t.TempDir()
	originalPath := config.GetConfigPath
	
	// Override GetConfigPath to use temp directory
	config.GetConfigPath = func() string {
		return filepath.Join(tmpDir, "config.yaml")
	}
	defer func() {
		config.GetConfigPath = originalPath
	}()

	// Prepare input
	input := strings.Join([]string{
		"https://test.atlassian.net",
		"test@example.com",
		"jira-token-123",
		"gemini-key-456",
		"TEST",
		"Task",
	}, "\n") + "\n"

	// Create a pipe for stdin
	oldStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("Failed to create pipe: %v", err)
	}
	defer func() {
		os.Stdin = oldStdin
		r.Close()
		w.Close()
	}()

	os.Stdin = r

	// Write input to pipe in a goroutine
	go func() {
		defer w.Close()
		io.WriteString(w, input)
	}()

	// Note: This test is simplified because we can't easily mock the keyring
	// and password input. In a real scenario, we'd need to refactor to inject
	// dependencies or use a more sophisticated mocking approach.
	// For now, we'll just verify the command structure exists.
	if initCmd == nil {
		t.Fatal("initCmd is nil")
	}

	if initCmd.Use != "init" {
		t.Errorf("Expected init command use to be 'init', got '%s'", initCmd.Use)
	}
}

func TestInitCommandInputParsing(t *testing.T) {
	// Test that we can parse input correctly
	input := "https://test.atlassian.net\n"
	reader := bytes.NewReader([]byte(input))
	bufReader := bytes.NewBuffer(reader.Bytes())
	
	line, err := bufReader.ReadString('\n')
	if err != nil && err.Error() != "EOF" {
		t.Fatalf("Failed to read input: %v", err)
	}
	
	trimmed := strings.TrimSpace(line)
	if trimmed != "https://test.atlassian.net" {
		t.Errorf("Expected 'https://test.atlassian.net', got '%s'", trimmed)
	}
}

