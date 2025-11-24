package review

import (
	"fmt"

	"github.com/beekhof/jira-tool/pkg/jira"
)

// WorkflowStep represents a step in the guided review workflow
type WorkflowStep int

const (
	StepDescription WorkflowStep = iota
	StepComponent
	StepPriority
	StepSeverity
	StepStoryPoints
	StepBacklog
	StepAssignment
)

// String returns the string representation of a workflow step
func (ws WorkflowStep) String() string {
	switch ws {
	case StepDescription:
		return "Description"
	case StepComponent:
		return "Component"
	case StepPriority:
		return "Priority"
	case StepSeverity:
		return "Severity"
	case StepStoryPoints:
		return "Story Points"
	case StepBacklog:
		return "Backlog State"
	case StepAssignment:
		return "Assignment"
	default:
		return "Unknown"
	}
}

// TicketStatus tracks the completion status of each workflow step for a ticket
type TicketStatus struct {
	DescriptionComplete bool
	ComponentComplete   bool
	PriorityComplete    bool
	SeverityComplete    bool
	StoryPointsComplete bool
	BacklogComplete     bool
	AssignmentComplete  bool
}

// IsComplete returns true if all required steps are complete
func (ts *TicketStatus) IsComplete() bool {
	return ts.DescriptionComplete &&
		ts.ComponentComplete &&
		ts.PriorityComplete &&
		ts.SeverityComplete &&
		ts.StoryPointsComplete &&
		ts.BacklogComplete &&
		ts.AssignmentComplete
}

// GetNextStep returns the first incomplete step, or nil if all complete
func (ts *TicketStatus) GetNextStep() WorkflowStep {
	if !ts.DescriptionComplete {
		return StepDescription
	}
	if !ts.ComponentComplete {
		return StepComponent
	}
	if !ts.PriorityComplete {
		return StepPriority
	}
	if !ts.SeverityComplete {
		return StepSeverity
	}
	if !ts.StoryPointsComplete {
		return StepStoryPoints
	}
	if !ts.BacklogComplete {
		return StepBacklog
	}
	if !ts.AssignmentComplete {
		return StepAssignment
	}
	// All complete - return last step as sentinel
	return StepAssignment
}

// MarkComplete marks a step as complete
func (ts *TicketStatus) MarkComplete(step WorkflowStep) {
	switch step {
	case StepDescription:
		ts.DescriptionComplete = true
	case StepComponent:
		ts.ComponentComplete = true
	case StepPriority:
		ts.PriorityComplete = true
	case StepSeverity:
		ts.SeverityComplete = true
	case StepStoryPoints:
		ts.StoryPointsComplete = true
	case StepBacklog:
		ts.BacklogComplete = true
	case StepAssignment:
		ts.AssignmentComplete = true
	}
}

// DisplayProgress shows a progress checklist for the ticket
func DisplayProgress(ticket jira.Issue, status TicketStatus) {
	fmt.Printf("\nReviewing: %s - %s\n\n", ticket.Key, ticket.Fields.Summary)
	fmt.Println("Progress:")

	// Display each step with completion indicator
	marker := " "
	if status.DescriptionComplete {
		marker = "✓"
	}
	fmt.Printf("  [%s] Description\n", marker)

	marker = " "
	if status.ComponentComplete {
		marker = "✓"
	}
	fmt.Printf("  [%s] Component\n", marker)

	marker = " "
	if status.PriorityComplete {
		marker = "✓"
	}
	fmt.Printf("  [%s] Priority\n", marker)

	marker = " "
	if status.SeverityComplete {
		marker = "✓"
	}
	fmt.Printf("  [%s] Severity\n", marker)

	marker = " "
	if status.StoryPointsComplete {
		marker = "✓"
	}
	fmt.Printf("  [%s] Story Points\n", marker)

	marker = " "
	if status.BacklogComplete {
		marker = "✓"
	}
	fmt.Printf("  [%s] Backlog State\n", marker)

	marker = " "
	if status.AssignmentComplete {
		marker = "✓"
	}
	fmt.Printf("  [%s] Assignment\n", marker)
	fmt.Println()
}

