package review

import (
	"testing"
)

func TestTicketStatus(t *testing.T) {
	status := &TicketStatus{}

	// Initially, nothing is complete
	if status.IsComplete() {
		t.Error("Expected status to be incomplete initially")
	}

	// Mark all steps complete
	status.MarkComplete(StepDescription)
	status.MarkComplete(StepComponent)
	status.MarkComplete(StepPriority)
	status.MarkComplete(StepSeverity)
	status.MarkComplete(StepStoryPoints)
	status.MarkComplete(StepBacklog)
	status.MarkComplete(StepAssignment)

	if !status.IsComplete() {
		t.Error("Expected status to be complete after marking all steps")
	}
}

func TestGetNextStep(t *testing.T) {
	status := &TicketStatus{}

	// First step should be Description
	next := status.GetNextStep()
	if next != StepDescription {
		t.Errorf("Expected next step to be Description, got %s", next)
	}

	// Mark Description complete, next should be Component
	status.MarkComplete(StepDescription)
	next = status.GetNextStep()
	if next != StepComponent {
		t.Errorf("Expected next step to be Component, got %s", next)
	}

	// Mark all but Assignment complete
	status.MarkComplete(StepComponent)
	status.MarkComplete(StepPriority)
	status.MarkComplete(StepSeverity)
	status.MarkComplete(StepStoryPoints)
	status.MarkComplete(StepBacklog)

	next = status.GetNextStep()
	if next != StepAssignment {
		t.Errorf("Expected next step to be Assignment, got %s", next)
	}

	// Mark Assignment complete
	status.MarkComplete(StepAssignment)
	next = status.GetNextStep()
	// When all complete, should return last step as sentinel
	if next != StepAssignment {
		t.Errorf("Expected sentinel step Assignment when all complete, got %s", next)
	}
}

func TestWorkflowStepString(t *testing.T) {
	tests := []struct {
		step     WorkflowStep
		expected string
	}{
		{StepDescription, "Description"},
		{StepComponent, "Component"},
		{StepPriority, "Priority"},
		{StepSeverity, "Severity"},
		{StepStoryPoints, "Story Points"},
		{StepBacklog, "Backlog State"},
		{StepAssignment, "Assignment"},
	}

	for _, test := range tests {
		if test.step.String() != test.expected {
			t.Errorf("Expected %s.String() to be '%s', got '%s'", test.step, test.expected, test.step.String())
		}
	}
}
