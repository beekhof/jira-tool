package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAddRecentComponent(t *testing.T) {
	state := &State{}

	// Add components
	state.AddRecentComponent("Component A")
	state.AddRecentComponent("Component B")
	state.AddRecentComponent("Component C")

	if len(state.RecentComponents) != 3 {
		t.Errorf("Expected 3 components, got %d", len(state.RecentComponents))
	}

	// Test max limit (6)
	state.AddRecentComponent("Component D")
	state.AddRecentComponent("Component E")
	state.AddRecentComponent("Component F")
	state.AddRecentComponent("Component G") // Should remove Component A

	if len(state.RecentComponents) != 6 {
		t.Errorf("Expected 6 components (max limit), got %d", len(state.RecentComponents))
	}

	// First component should be Component B (A was removed)
	if state.RecentComponents[0] != "Component B" {
		t.Errorf("Expected first component to be 'Component B', got '%s'", state.RecentComponents[0])
	}

	// Last component should be Component G
	if state.RecentComponents[5] != "Component G" {
		t.Errorf("Expected last component to be 'Component G', got '%s'", state.RecentComponents[5])
	}
}

func TestAddRecentComponentMovesToEnd(t *testing.T) {
	state := &State{}

	// Add components
	state.AddRecentComponent("Component A")
	state.AddRecentComponent("Component B")
	state.AddRecentComponent("Component C")

	// Add Component A again - should move to end
	state.AddRecentComponent("Component A")

	if len(state.RecentComponents) != 3 {
		t.Errorf("Expected 3 components, got %d", len(state.RecentComponents))
	}

	// Component A should be at the end
	if state.RecentComponents[2] != "Component A" {
		t.Errorf("Expected 'Component A' to be at the end, got '%s'", state.RecentComponents[2])
	}

	// Component B should be first
	if state.RecentComponents[0] != "Component B" {
		t.Errorf("Expected 'Component B' to be first, got '%s'", state.RecentComponents[0])
	}
}

func TestStateSaveLoadWithComponents(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.yaml")

	state := &State{
		RecentComponents: []string{"Component A", "Component B", "Component C"},
	}

	// Save state
	if err := SaveState(state, statePath); err != nil {
		t.Fatalf("Failed to save state: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(statePath); os.IsNotExist(err) {
		t.Fatal("State file was not created")
	}

	// Load state
	loaded, err := LoadState(statePath)
	if err != nil {
		t.Fatalf("Failed to load state: %v", err)
	}

	// Verify components
	if len(loaded.RecentComponents) != 3 {
		t.Errorf("Expected 3 components, got %d", len(loaded.RecentComponents))
	}

	if loaded.RecentComponents[0] != "Component A" {
		t.Errorf("Expected first component 'Component A', got '%s'", loaded.RecentComponents[0])
	}
}

func TestAddRecentParentTicket(t *testing.T) {
	state := &State{}

	// Add parent tickets
	state.AddRecentParentTicket("PROJ-123")
	state.AddRecentParentTicket("PROJ-456")
	state.AddRecentParentTicket("PROJ-789")

	if len(state.RecentParentTickets) != 3 {
		t.Errorf("Expected 3 parent tickets, got %d", len(state.RecentParentTickets))
	}

	// Test max limit (6)
	state.AddRecentParentTicket("PROJ-101")
	state.AddRecentParentTicket("PROJ-102")
	state.AddRecentParentTicket("PROJ-103")
	state.AddRecentParentTicket("PROJ-104") // Should remove PROJ-123

	if len(state.RecentParentTickets) != 6 {
		t.Errorf("Expected 6 parent tickets (max limit), got %d", len(state.RecentParentTickets))
	}

	// First ticket should be PROJ-456 (PROJ-123 was removed)
	if state.RecentParentTickets[0] != "PROJ-456" {
		t.Errorf("Expected first ticket to be 'PROJ-456', got '%s'", state.RecentParentTickets[0])
	}

	// Last ticket should be PROJ-104
	if state.RecentParentTickets[5] != "PROJ-104" {
		t.Errorf("Expected last ticket to be 'PROJ-104', got '%s'", state.RecentParentTickets[5])
	}
}

func TestAddRecentParentTicketMovesToEnd(t *testing.T) {
	state := &State{}

	// Add parent tickets
	state.AddRecentParentTicket("PROJ-123")
	state.AddRecentParentTicket("PROJ-456")
	state.AddRecentParentTicket("PROJ-789")

	// Add PROJ-123 again - should move to end
	state.AddRecentParentTicket("PROJ-123")

	if len(state.RecentParentTickets) != 3 {
		t.Errorf("Expected 3 parent tickets, got %d", len(state.RecentParentTickets))
	}

	// PROJ-123 should be at the end
	if state.RecentParentTickets[2] != "PROJ-123" {
		t.Errorf("Expected 'PROJ-123' to be at the end, got '%s'", state.RecentParentTickets[2])
	}

	// PROJ-456 should be first
	if state.RecentParentTickets[0] != "PROJ-456" {
		t.Errorf("Expected 'PROJ-456' to be first, got '%s'", state.RecentParentTickets[0])
	}
}

func TestStateSaveLoadParentTickets(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.yaml")

	state := &State{
		RecentParentTickets: []string{"PROJ-123", "PROJ-456", "PROJ-789"},
	}

	// Save state
	if err := SaveState(state, statePath); err != nil {
		t.Fatalf("Failed to save state: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(statePath); os.IsNotExist(err) {
		t.Fatal("State file was not created")
	}

	// Load state
	loaded, err := LoadState(statePath)
	if err != nil {
		t.Fatalf("Failed to load state: %v", err)
	}

	// Verify parent tickets
	if len(loaded.RecentParentTickets) != 3 {
		t.Errorf("Expected 3 parent tickets, got %d", len(loaded.RecentParentTickets))
	}

	if loaded.RecentParentTickets[0] != "PROJ-123" {
		t.Errorf("Expected first ticket 'PROJ-123', got '%s'", loaded.RecentParentTickets[0])
	}
}

