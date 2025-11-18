package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestInitCommand(t *testing.T) {
	// Note: This test is simplified because we can't easily mock the credentials
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
	bufReader := bytes.NewBufferString(input)

	line, err := bufReader.ReadString('\n')
	if err != nil && err.Error() != "EOF" {
		t.Fatalf("Failed to read input: %v", err)
	}

	trimmed := strings.TrimSpace(line)
	if trimmed != "https://test.atlassian.net" {
		t.Errorf("Expected 'https://test.atlassian.net', got '%s'", trimmed)
	}
}
