package review

import (
	"bufio"
	"fmt"
	"strings"

	"github.com/beekhof/jira-tool/pkg/config"
	"github.com/beekhof/jira-tool/pkg/editor"
	"github.com/beekhof/jira-tool/pkg/gemini"
	"github.com/beekhof/jira-tool/pkg/jira"
	"github.com/beekhof/jira-tool/pkg/qa"
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

// InitializeStatusFromTicket creates a TicketStatus based on the current ticket state
func InitializeStatusFromTicket(client jira.JiraClient, ticket jira.Issue, cfg *config.Config) TicketStatus {
	status := TicketStatus{}

	// Check Description
	isValid, _, err := CheckDescriptionQuality(client, ticket, cfg)
	if err == nil && isValid {
		status.DescriptionComplete = true
	}

	// Check Component
	if len(ticket.Fields.Components) > 0 {
		status.ComponentComplete = true
	}

	// Check Priority
	if ticket.Fields.Priority.Name != "" {
		status.PriorityComplete = true
	}

	// Check Severity (only if configured)
	if cfg.SeverityFieldID != "" {
		// We can't easily check severity without fetching the ticket with that field
		// For now, assume incomplete if field is configured (will be checked in step handler)
		// This is a limitation - we'd need to fetch the ticket with severity field to check
	}

	// Check Story Points
	if ticket.Fields.StoryPoints > 0 {
		status.StoryPointsComplete = true
	}

	// Check Backlog State (not in "New" state means already transitioned)
	if ticket.Fields.Status.Name != "New" {
		status.BacklogComplete = true
	}

	// Check Assignment
	if ticket.Fields.Assignee.DisplayName != "" || ticket.Fields.Assignee.AccountID != "" || ticket.Fields.Assignee.Name != "" {
		status.AssignmentComplete = true
	}

	return status
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

// Action represents a user action in response to an error
type Action int

const (
	ActionRetry Action = iota
	ActionSkip
	ActionAbort
)

// HandleWorkflowError handles errors during workflow execution
func HandleWorkflowError(err error, step WorkflowStep, reader *bufio.Reader) (Action, error) {
	fmt.Printf("\nError in %s: %v\n", step.String(), err)
	fmt.Print("What would you like to do? [r]etry | [s]kip remaining | [a]bort > ")

	input, err := reader.ReadString('\n')
	if err != nil {
		return ActionAbort, err
	}
	input = strings.TrimSpace(strings.ToLower(input))

	switch input {
	case "r", "retry":
		return ActionRetry, nil
	case "s", "skip":
		return ActionSkip, nil
	case "a", "abort":
		return ActionAbort, nil
	default:
		// Invalid input - default to abort for safety
		fmt.Println("Invalid input, aborting workflow")
		return ActionAbort, nil
	}
}

// ProcessTicketWorkflow processes a single ticket through the guided review workflow
func ProcessTicketWorkflow(client jira.JiraClient, geminiClient gemini.GeminiClient, reader *bufio.Reader, cfg *config.Config, ticket jira.Issue, configDir string) error {
	// Initialize status based on current ticket state
	status := &TicketStatus{}
	*status = InitializeStatusFromTicket(client, ticket, cfg)

	// Display initial progress
	DisplayProgress(ticket, *status)

	// Process each step in order
	steps := []struct {
		step     WorkflowStep
		handler  func() (bool, error)
		required bool
	}{
		{
			step: StepDescription,
			handler: func() (bool, error) {
				// Check if description meets quality criteria
				isValid, reason, err := CheckDescriptionQuality(client, ticket, cfg)
				if err != nil {
					return false, err
				}
				if !isValid {
					fmt.Printf("Description issue: %s\n", reason)
					fmt.Print("Generate/update description? [y/N] ")
					response, err := reader.ReadString('\n')
					if err != nil {
						return false, err
					}
					response = strings.TrimSpace(strings.ToLower(response))
					if response == "y" || response == "yes" {
						// Get existing description
						existingDesc, _ := client.GetTicketDescription(ticket.Key)
						// Run Q&A flow (pass issueTypeName for Epic/Feature detection, include child tickets in context)
						issueTypeName := ticket.Fields.IssueType.Name
						answerInputMethod := cfg.AnswerInputMethod
						if answerInputMethod == "" {
							answerInputMethod = "readline_with_preview"
						}
						description, err := qa.RunQnAFlow(geminiClient, ticket.Fields.Summary, cfg.MaxQuestions, ticket.Fields.Summary, issueTypeName, existingDesc, client, ticket.Key, cfg.EpicLinkFieldID, answerInputMethod)
						if err != nil {
							return false, err
						}
						// Print the generated description and ask for confirmation
						fmt.Println("\nGenerated description:")
						fmt.Println("---")
						fmt.Println(description)
						fmt.Println("---")
						fmt.Print("\nUpdate ticket with this description? [Y/n/e(dit)] ")
						confirm, err := reader.ReadString('\n')
						if err != nil {
							return false, err
						}
						confirm = strings.TrimSpace(strings.ToLower(confirm))

						if confirm == "e" || confirm == "edit" {
							// Open in editor
							editedDescription, err := editor.OpenInEditor(description)
							if err != nil {
								return false, fmt.Errorf("failed to edit description: %w", err)
							}
							description = editedDescription
						}

						if confirm != "n" && confirm != "no" {
							// Update ticket
							if err := client.UpdateTicketDescription(ticket.Key, description); err != nil {
								return false, err
							}
							return true, nil
						}
						// User declined to save
						return false, nil
					}
					return false, nil // User skipped
				}
				return true, nil // Description is valid
			},
			required: true,
		},
		{
			step: StepComponent,
			handler: func() (bool, error) {
				return HandleComponentStep(client, reader, cfg, ticket, configDir)
			},
			required: true,
		},
		{
			step: StepPriority,
			handler: func() (bool, error) {
				return HandlePriorityStep(client, reader, ticket)
			},
			required: true,
		},
		{
			step: StepSeverity,
			handler: func() (bool, error) {
				return HandleSeverityStep(client, reader, cfg, ticket)
			},
			required: false, // Only if configured
		},
		{
			step: StepStoryPoints,
			handler: func() (bool, error) {
				if geminiClient == nil {
					// Skip AI estimation if Gemini not available
					fmt.Println("Gemini client not available - skipping story points estimation")
					return false, nil // Skip this step
				}
				return HandleStoryPointsStep(client, geminiClient, reader, cfg, ticket)
			},
			required: true,
		},
		{
			step: StepBacklog,
			handler: func() (bool, error) {
				return HandleBacklogTransitionStep(client, ticket)
			},
			required: true,
		},
		{
			step: StepAssignment,
			handler: func() (bool, error) {
				return HandleAssignmentStep(client, reader, cfg, ticket, configDir)
			},
			required: false, // Optional
		},
	}

	// Process each step
	for _, stepInfo := range steps {
		// Check if step is already complete in status
		if status.IsStepComplete(stepInfo.step) {
			continue
		}

		// Check if step is already complete in ticket (for component and assignment steps)
		if stepInfo.step == StepComponent {
			if len(ticket.Fields.Components) > 0 {
				// Component already set, mark as complete and skip
				status.MarkComplete(StepComponent)
				continue
			}
		}

		if stepInfo.step == StepAssignment {
			// Check if ticket is already assigned
			if ticket.Fields.Assignee.DisplayName != "" || ticket.Fields.Assignee.AccountID != "" || ticket.Fields.Assignee.Name != "" {
				// Already assigned, mark as complete and skip
				status.MarkComplete(StepAssignment)
				continue
			}
		}

		if stepInfo.step == StepStoryPoints {
			// Check if story points are already set
			if ticket.Fields.StoryPoints > 0 {
				// Story points already set, mark as complete and skip
				status.MarkComplete(StepStoryPoints)
				continue
			}
		}

		// Execute step with retry logic
		for {
			completed, err := stepInfo.handler()
			if err != nil {
				// Handle error
				action, actionErr := HandleWorkflowError(err, stepInfo.step, reader)
				if actionErr != nil {
					return actionErr
				}

				switch action {
				case ActionRetry:
					continue // Retry the step
				case ActionSkip:
					return nil // Skip remaining steps
				case ActionAbort:
					return fmt.Errorf("workflow aborted by user")
				}
			}

			if !completed {
				// User skipped this step - skip all remaining steps
				return nil
			}

			// Mark step as complete
			status.MarkComplete(stepInfo.step)

			// Refresh ticket data from Jira
			issues, err := client.SearchTickets(fmt.Sprintf("key = %s", ticket.Key))
			if err == nil && len(issues) > 0 {
				ticket = issues[0]
			}

			break // Move to next step
		}
	}

	return nil
}

// IsStepComplete checks if a specific step is complete
func (ts *TicketStatus) IsStepComplete(step WorkflowStep) bool {
	switch step {
	case StepDescription:
		return ts.DescriptionComplete
	case StepComponent:
		return ts.ComponentComplete
	case StepPriority:
		return ts.PriorityComplete
	case StepSeverity:
		return ts.SeverityComplete
	case StepStoryPoints:
		return ts.StoryPointsComplete
	case StepBacklog:
		return ts.BacklogComplete
	case StepAssignment:
		return ts.AssignmentComplete
	default:
		return false
	}
}

