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

