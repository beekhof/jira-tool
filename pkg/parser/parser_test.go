package parser

import (
	"strings"
	"testing"
)

func TestParseEpicPlan(t *testing.T) {
	markdown := `# EPIC: Implement new auth system

This is the description of the epic.
It can span multiple lines.

## TASKS
- [ ] Task 1 summary
- [ ] Task 2 summary
- [ ] Task 3 summary
`

	epic, tasks, err := ParseEpicPlan(markdown)
	if err != nil {
		t.Fatalf("ParseEpicPlan failed: %v", err)
	}

	if epic.Title != "Implement new auth system" {
		t.Errorf("Expected title 'Implement new auth system', got '%s'", epic.Title)
	}

	if !strings.Contains(epic.Description, "This is the description") {
		t.Errorf("Description should contain 'This is the description', got '%s'", epic.Description)
	}

	if len(tasks) != 3 {
		t.Errorf("Expected 3 tasks, got %d", len(tasks))
	}

	if tasks[0].Summary != "Task 1 summary" {
		t.Errorf("Expected first task 'Task 1 summary', got '%s'", tasks[0].Summary)
	}
}

func TestParseEpicPlan_NoTitle(t *testing.T) {
	markdown := `## TASKS
- [ ] Task 1
`

	_, _, err := ParseEpicPlan(markdown)
	if err == nil {
		t.Error("Expected error for missing epic title, got nil")
	}
}

func TestParseEpicPlan_NoTasks(t *testing.T) {
	markdown := `# EPIC: Test Epic

Description here.
`

	_, _, err := ParseEpicPlan(markdown)
	if err == nil {
		t.Error("Expected error for missing TASKS section, got nil")
	}
}
