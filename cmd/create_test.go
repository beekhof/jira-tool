package cmd

import (
	"testing"
)

// TestCreateCommand tests the create command logic
// Note: This is a simplified test. Full integration testing would require
// mocking the config and client creation, which is complex without dependency injection.
func TestCreateCommand(t *testing.T) {
	// This test verifies the command structure
	// Full testing would require:
	// 1. Mocking config.LoadConfig
	// 2. Mocking jira.NewClient
	// 3. Verifying CreateTicket is called with correct arguments

	// For now, we verify the command exists and has the right structure
	if createCmd == nil {
		t.Error("createCmd is nil")
	}

	if createCmd.Use != "create [SUMMARY]" {
		t.Errorf("Expected use 'create [SUMMARY]', got '%s'", createCmd.Use)
	}
}

// MockJiraClientForCreate extends MockJiraClient for create command testing
type MockJiraClientForCreate struct {
	*MockJiraClient
	CreateTicketFunc func(project, taskType, summary string) (string, error)
}

func (m *MockJiraClientForCreate) CreateTicket(project, taskType, summary string) (string, error) {
	if m.CreateTicketFunc != nil {
		return m.CreateTicketFunc(project, taskType, summary)
	}
	return "ENG-123", nil
}
