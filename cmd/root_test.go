package cmd

import (
	"testing"
)

func TestRootCommand(t *testing.T) {
	if rootCmd == nil {
		t.Fatal("rootCmd is nil")
	}

	if rootCmd.Use != "jira" {
		t.Errorf("Expected root command use to be 'jira', got '%s'", rootCmd.Use)
	}
}
