# Guided Review Workflow - Code Generation Prompts

This document contains a series of prompts for implementing the guided review workflow feature. Each prompt is designed to be used independently with a code-generation LLM, building incrementally on previous work.

---

## Prompt 1: Add Configuration Fields for Description Quality and Severity

**Context**: We're adding new configuration options to support the guided review workflow. The config file is in `pkg/config/config.go` and uses YAML.

**Task**: Add the following new fields to the `Config` struct in `pkg/config/config.go`:

1. `DescriptionMinLength int` - Minimum description length in characters (default: 128)
2. `DescriptionQualityAI bool` - Enable Gemini AI analysis for description quality (default: false)
3. `SeverityFieldID string` - Custom field ID for severity (optional, empty by default)
4. `DefaultBoardID int` - Default board ID if auto-detection fails (default: 0)

**Requirements**:
- Add YAML tags with `omitempty` for optional fields
- Ensure `LoadConfig` handles missing fields gracefully (uses zero values which are fine)
- Add comments explaining each field
- Follow existing code style and patterns

**Testing**: Write a test in `pkg/config/config_test.go` that:
- Loads a config with all new fields set
- Loads a config with new fields missing (should use defaults)
- Verifies default values are correct

**Files to modify**:
- `pkg/config/config.go`
- `pkg/config/config_test.go`

---

## Prompt 2: Add Component Tracking to State

**Context**: We need to track recently selected components in `state.yaml`, similar to how we track recent assignees, sprints, and releases. The state management is in `pkg/config/state.go`.

**Task**: 
1. Add `RecentComponents []string` field to the `State` struct
2. Add `AddRecentComponent(componentName string)` method that:
   - Adds component to recent list (max 6 unique)
   - Moves existing component to end if already in list
   - Maintains only last 6 items
3. Reuse the existing `addToRecentList` helper function

**Requirements**:
- Follow the exact same pattern as `AddRecentAssignee`, `AddRecentSprint`, `AddRecentRelease`
- Use the existing `addToRecentList` helper
- Add YAML tag: `yaml:"recent_components,omitempty"`

**Testing**: Write a test in `pkg/config/state_test.go` that:
- Adds components and verifies max 6 limit
- Verifies existing components move to end
- Tests state save/load with components

**Files to modify**:
- `pkg/config/state.go`
- `pkg/config/state_test.go` (create if doesn't exist)

---

## Prompt 3: Implement Component API Methods

**Context**: We need to fetch components from Jira and update tickets with components. The Jira client is in `pkg/jira/client.go`. Follow the pattern of existing methods like `GetPriorities`.

**Task**: 
1. Define a `Component` struct with fields: `ID string`, `Name string`, `Description string`
2. Add `GetComponents(projectKey string) ([]Component, error)` method to `JiraClient` interface
3. Implement in `jiraClient`:
   - Endpoint: `/rest/api/2/project/{projectKey}/components`
   - Check cache first (respect `noCache` flag)
   - Cache results in `cache.json`
   - Parse JSON response into `Component` structs
   - Return list of components
4. Add `UpdateTicketComponents(ticketID string, componentIDs []string) error` method:
   - Endpoint: `/rest/api/2/issue/{ticketID}`
   - Update `components` field with array of `{"id": componentID}` objects
   - Handle errors appropriately

**Requirements**:
- Follow existing caching pattern (check `GetPriorities` for reference)
- Use Bearer token authentication (existing `setAuth` method)
- Handle HTTP errors with clear messages
- Cache key should be `components:{projectKey}`

**Testing**: Write tests in `pkg/jira/client_test.go` that:
- Mock HTTP responses for component fetching
- Test caching behavior
- Test component update API call
- Test error handling

**Files to modify**:
- `pkg/jira/client.go`
- `pkg/jira/client_test.go`

---

## Prompt 4: Implement Severity Field Detection

**Context**: We need to auto-detect the severity custom field ID, similar to how we detect the story points field in `cmd/init.go`. The detection should query Jira's field API.

**Task**:
1. Add `DetectSeverityField(projectKey string) (string, error)` method to `JiraClient` interface
2. Implement in `jiraClient`:
   - Query `/rest/api/2/field` endpoint
   - Search for custom fields where name contains "severity" (case-insensitive)
   - Return the first matching field ID
   - Return empty string if not found (not an error)
3. Add `GetSeverityFieldValues(fieldID string) ([]string, error)` method:
   - Query field configuration to get allowed values
   - Parse and return list of severity value names
   - Handle case where field doesn't have predefined values

**Requirements**:
- Follow the pattern from `detectStoryPointsField` in `cmd/init.go`
- Return empty string (not error) if field not found
- Handle API errors gracefully
- For field values, try to extract from field schema if available

**Testing**: Write tests that:
- Mock field API responses
- Test field detection with matching field
- Test field detection with no matching field
- Test value extraction

**Files to modify**:
- `pkg/jira/client.go`
- `pkg/jira/client_test.go`

---

## Prompt 5: Create Workflow State Structure

**Context**: We need data structures to track workflow progress for each ticket. Create a new package `pkg/review` for workflow-related code.

**Task**:
1. Create `pkg/review/workflow.go` file
2. Define `WorkflowStep` type as constants:
   - `StepDescription`
   - `StepComponent`
   - `StepPriority`
   - `StepSeverity`
   - `StepStoryPoints`
   - `StepBacklog`
   - `StepAssignment`
3. Define `TicketStatus` struct:
   ```go
   type TicketStatus struct {
       DescriptionComplete bool
       ComponentComplete   bool
       PriorityComplete    bool
       SeverityComplete    bool
       StoryPointsComplete bool
       BacklogComplete     bool
       AssignmentComplete  bool
   }
   ```
4. Add helper methods:
   - `IsComplete() bool` - returns true if all steps complete
   - `GetNextStep() WorkflowStep` - returns next incomplete step
   - `MarkComplete(step WorkflowStep)` - marks a step as complete

**Requirements**:
- Use clear, descriptive names
- Follow Go naming conventions
- Add comments explaining the purpose

**Testing**: Write tests in `pkg/review/workflow_test.go` that:
- Test step completion tracking
- Test `GetNextStep` returns correct step
- Test `IsComplete` works correctly

**Files to create**:
- `pkg/review/workflow.go`
- `pkg/review/workflow_test.go`

---

## Prompt 6: Implement Progress Indicator Display

**Context**: We need to display a progress checklist showing which workflow steps are complete. This will be shown at the start of processing each ticket.

**Task**:
1. Add `DisplayProgress(ticket jira.Issue, status TicketStatus)` function in `pkg/review/workflow.go`
2. Format output as:
   ```
   Reviewing: PROJ-123 - Ticket Summary
   
   Progress:
     [✓] Description
     [ ] Component
     [ ] Priority
     [ ] Severity
     [ ] Story Points
     [ ] Backlog State
     [ ] Assignment
   ```
3. Use `[✓]` for complete steps, `[ ]` for incomplete
4. Show ticket key and summary on first line

**Requirements**:
- Use `fmt.Printf` for output
- Format should be clear and readable
- Follow existing output style in the codebase

**Testing**: Write tests that:
- Test progress display with all steps incomplete
- Test progress display with some steps complete
- Test progress display with all steps complete

**Files to modify**:
- `pkg/review/workflow.go`
- `pkg/review/workflow_test.go`

---

## Prompt 7: Implement Description Quality Check

**Context**: We need to check if a ticket's description meets quality criteria. This involves checking length and optionally using Gemini AI.

**Task**:
1. Add `CheckDescriptionQuality(ticket jira.Issue, cfg *config.Config, geminiClient gemini.GeminiClient) (bool, string, error)` function in `pkg/review/workflow.go`
2. Implement checks:
   - Get description length (from `ticket.Fields.Description`)
   - Check if length >= `cfg.DescriptionMinLength`
   - If `cfg.DescriptionQualityAI` is true:
     - Call Gemini to analyze if description answers "what", "why", "how"
     - Use a prompt like: "Does this Jira ticket description answer what needs to be done, why it's needed, and how it will be accomplished? Description: {description}"
   - Return `(isValid, reason, error)`
3. Reason should explain why description fails (e.g., "too short (64 chars, need 128)" or "AI analysis: missing 'why' explanation")

**Requirements**:
- Handle empty description (always fails)
- Only call Gemini if AI check is enabled
- Return clear reason messages
- Handle Gemini API errors gracefully

**Testing**: Write tests that:
- Test length check (pass and fail cases)
- Test with AI enabled and disabled
- Test empty description
- Mock Gemini client for AI tests

**Files to modify**:
- `pkg/review/workflow.go`
- `pkg/review/workflow_test.go`

---

## Prompt 8: Integrate Q&A Flow with Existing Description

**Context**: The Q&A flow in `pkg/qa/flow.go` needs to accept an existing description to include in context when generating a new one.

**Task**:
1. Modify `RunQAFlow` function signature in `pkg/qa/flow.go`:
   - Add parameter: `existingDescription string`
   - Include existing description in the initial context passed to Gemini
2. Update the context building to include:
   - "Existing description: {existingDescription}" if not empty
   - "Improve or expand this description based on the following questions:"
3. Update all call sites:
   - `cmd/create.go` - pass empty string for new tickets
   - `cmd/accept.go` - pass existing description if available
   - Future workflow code - pass ticket's current description

**Requirements**:
- Don't break existing functionality
- Only include existing description in context if not empty
- Update prompt to indicate we're improving existing description

**Testing**: Write tests that:
- Test Q&A with empty existing description (new ticket)
- Test Q&A with existing description (improvement case)
- Verify existing description appears in Gemini context

**Files to modify**:
- `pkg/qa/flow.go`
- `cmd/create.go`
- `cmd/accept.go`
- `pkg/qa/flow_test.go` (update existing tests)

---

## Prompt 9: Implement Component Selection Step

**Context**: We need a workflow step that checks if a ticket has components and prompts to assign one if missing. This should follow the pattern of assignee selection.

**Task**:
1. Add `HandleComponentStep(client jira.JiraClient, reader *bufio.Reader, cfg *config.Config, ticket jira.Issue, configDir string) (bool, error)` function in `pkg/review/workflow.go`
2. Implementation:
   - Check if ticket has components assigned (check `ticket.Fields.Components`)
   - If components exist, return `(true, nil)` (step complete)
   - If missing:
     - Load state and get recent components
     - Fetch all components for project using `client.GetComponents`
     - Display: "Recent components:" with recent list, then "[N] Other..." option
     - If "Other" selected, show all components
     - Allow selection or skip
     - Update ticket using `client.UpdateTicketComponents`
     - Track selection in state using `state.AddRecentComponent`
     - Save state
3. Return `(completed, error)` - true if component assigned or skipped

**Requirements**:
- Follow pattern from `handleAssign` in `cmd/review.go`
- Only prompt if no component assigned
- Show recent components first
- Track selection in state
- Handle skip gracefully

**Testing**: Write tests that:
- Test with component already assigned (should skip)
- Test component selection from recent list
- Test component selection from full list
- Test skip behavior
- Mock Jira client

**Files to modify**:
- `pkg/review/workflow.go`
- `pkg/review/workflow_test.go`

---

## Prompt 10: Implement Priority Selection Step

**Context**: We need a workflow step that checks if a ticket has priority and prompts to assign one if missing. This reuses existing priority fetching.

**Task**:
1. Add `HandlePriorityStep(client jira.JiraClient, reader *bufio.Reader, ticket jira.Issue) (bool, error)` function in `pkg/review/workflow.go`
2. Implementation:
   - Check if priority is set (check `ticket.Fields.Priority.Name`)
   - If priority exists, return `(true, nil)` (step complete)
   - If missing:
     - Fetch priorities using `client.GetPriorities`
     - Display list: "[1] Priority1\n[2] Priority2\n..."
     - Allow selection
     - Update ticket using `client.UpdateTicketPriority` (may need to create this method)
     - Return `(true, nil)` on success
3. Handle skip by returning `(false, nil)` (user skipped, which skips remaining steps)

**Requirements**:
- Only prompt if priority missing
- Show valid priority values from Jira
- Handle skip gracefully
- Create `UpdateTicketPriority` method if it doesn't exist

**Testing**: Write tests that:
- Test with priority already set (should skip)
- Test priority selection
- Test skip behavior
- Mock Jira client

**Files to modify**:
- `pkg/review/workflow.go`
- `pkg/jira/client.go` (if `UpdateTicketPriority` needed)
- `pkg/review/workflow_test.go`

---

## Prompt 11: Implement Severity Selection Step

**Context**: We need a workflow step that checks if a ticket has severity (if configured) and prompts to assign one if missing.

**Task**:
1. Add `HandleSeverityStep(client jira.JiraClient, reader *bufio.Reader, cfg *config.Config, ticket jira.Issue) (bool, error)` function in `pkg/review/workflow.go`
2. Implementation:
   - Check if `cfg.SeverityFieldID` is set
   - If not set, return `(true, nil)` (skip step - severity not configured)
   - If set:
     - Check if severity is already set (need to check custom field)
     - If set, return `(true, nil)` (step complete)
     - If missing:
       - Fetch severity values using `client.GetSeverityFieldValues`
       - Display list for selection
       - Update ticket using custom field update method
       - Return `(true, nil)` on success
3. Handle skip by returning `(false, nil)`

**Requirements**:
- Only check if severity field is configured
- Skip gracefully if not configured
- Only prompt if severity missing
- Need method to read custom field value from ticket
- Need method to update custom field

**Testing**: Write tests that:
- Test with severity field not configured (should skip)
- Test with severity already set (should skip)
- Test severity selection
- Test skip behavior
- Mock Jira client

**Files to modify**:
- `pkg/review/workflow.go`
- `pkg/jira/client.go` (may need custom field read/update methods)
- `pkg/review/workflow_test.go`

---

## Prompt 12: Integrate Story Points Estimation Step

**Context**: We need a workflow step that checks if a ticket has story points and uses Gemini to suggest them if missing. This reuses existing estimation logic.

**Task**:
1. Add `HandleStoryPointsStep(client jira.JiraClient, geminiClient gemini.GeminiClient, reader *bufio.Reader, cfg *config.Config, ticket jira.Issue) (bool, error)` function in `pkg/review/workflow.go`
2. Implementation:
   - Check if story points are set (check `ticket.Fields.StoryPoints > 0`)
   - If set, return `(true, nil)` (step complete)
   - If missing:
     - Call `geminiClient.EstimateStoryPoints(ticket.Fields.Summary, ticket.Fields.Description)` to get AI suggestion
     - Display: "🤖 AI Estimate: X story points\n   Reasoning: ..."
     - Show selection interface (reuse from `cmd/estimate.go`):
       - Letter keys: `[a] 1 [b] 2 [c] 3 [d] 5 [e] 8 [f] 13`
       - Allow direct numeric input
     - Update ticket using `client.UpdateTicketPoints`
     - Return `(true, nil)` on success
3. Handle skip by returning `(false, nil)`

**Requirements**:
- Reuse existing `EstimateStoryPoints` method
- Reuse selection interface from `estimate` command
- Show AI suggestion before manual selection
- Handle skip gracefully

**Testing**: Write tests that:
- Test with story points already set (should skip)
- Test AI suggestion display
- Test story points selection
- Test skip behavior
- Mock Gemini and Jira clients

**Files to modify**:
- `pkg/review/workflow.go`
- `pkg/review/workflow_test.go`

---

## Prompt 13: Implement Backlog State Transition Step

**Context**: We need a workflow step that transitions tickets from "New" state to "Backlog" state.

**Task**:
1. Add `HandleBacklogTransitionStep(client jira.JiraClient, ticket jira.Issue) (bool, error)` function in `pkg/review/workflow.go`
2. Implementation:
   - Check if ticket is in "New" state (check `ticket.Fields.Status.Name == "New"`)
   - If not "New", return `(true, nil)` (step complete - no transition needed)
   - If "New":
     - Get available transitions using `client.GetTransitions(ticket.Key)`
     - Find transition with `To.Name == "Backlog"`
     - Execute transition using `client.TransitionIssue` (may need to create this method)
     - Return `(true, nil)` on success
3. Handle errors gracefully (e.g., transition not available)

**Requirements**:
- Only transition if in "New" state
- Find correct "Backlog" transition
- Handle case where transition not available
- Create `TransitionIssue` method if it doesn't exist

**Testing**: Write tests that:
- Test with ticket not in "New" state (should skip)
- Test transition from "New" to "Backlog"
- Test error handling (transition not available)
- Mock Jira client

**Files to modify**:
- `pkg/review/workflow.go`
- `pkg/jira/client.go` (if `TransitionIssue` needed)
- `pkg/review/workflow_test.go`

---

## Prompt 14: Implement Board Auto-Detection

**Context**: We need to auto-detect boards for a project to support sprint selection during assignment.

**Task**:
1. Add `GetBoardsForProject(projectKey string) ([]Board, error)` method to `JiraClient` interface
2. Define `Board` struct with fields: `ID int`, `Name string`, `Type string`
3. Implement in `jiraClient`:
   - Endpoint: `/rest/agile/1.0/board?projectKeyOrId={projectKey}`
   - Parse JSON response
   - Return list of boards
4. Handle errors appropriately

**Requirements**:
- Use Agile API endpoint
- Parse board response correctly
- Return clear error messages
- Handle projects with no boards (return empty list, not error)

**Testing**: Write tests that:
- Test board fetching for project
- Test with multiple boards
- Test with no boards
- Mock HTTP responses

**Files to modify**:
- `pkg/jira/client.go`
- `pkg/jira/client_test.go`

---

## Prompt 15: Implement Board Selection Logic

**Context**: We need logic to select a board - auto-select if one board, prompt if multiple.

**Task**:
1. Add `SelectBoard(client jira.JiraClient, reader *bufio.Reader, cfg *config.Config, projectKey string) (int, error)` function in `pkg/review/workflow.go`
2. Implementation:
   - Fetch boards using `client.GetBoardsForProject(projectKey)`
   - If one board found, return its ID automatically
   - If multiple boards found:
     - Display list: "[1] Board Name 1\n[2] Board Name 2\n..."
     - Allow user selection
     - Return selected board ID
   - If no boards found:
     - Use `cfg.DefaultBoardID` if set
     - Otherwise return error asking user to configure board
3. Return board ID (int) and error

**Requirements**:
- Auto-select if only one board
- Prompt clearly if multiple boards
- Handle no boards gracefully
- Use default board ID from config if available

**Testing**: Write tests that:
- Test auto-selection with one board
- Test selection with multiple boards
- Test with no boards (uses default or error)
- Mock Jira client

**Files to modify**:
- `pkg/review/workflow.go`
- `pkg/review/workflow_test.go`

---

## Prompt 16: Implement Assignment Step with Auto-Actions

**Context**: We need a workflow step that assigns a ticket and automatically performs additional actions: transition to "In Progress", add to sprint, and add to release (unless spike).

**Task**:
1. Add `HandleAssignmentStep(client jira.JiraClient, geminiClient gemini.GeminiClient, reader *bufio.Reader, cfg *config.Config, ticket jira.Issue, configDir string) (bool, error)` function in `pkg/review/workflow.go`
2. Implementation:
   - Prompt user: "Assign this ticket? [y/N]"
   - If no, return `(true, nil)` (step complete, assignment skipped)
   - If yes:
     - Reuse assignment logic from `handleAssign` in `cmd/review.go`
     - After assignment succeeds:
       - Transition to "In Progress" state (use `HandleBacklogTransitionStep` pattern but for "In Progress")
       - Select board (use `SelectBoard`)
       - Fetch sprints for board (active and planned)
       - Show recent sprints first, then all sprints
       - Add ticket to selected sprint using `client.AddIssuesToSprint`
       - Track sprint in state
       - If ticket is NOT a spike (check using `gemini.IsSpike`):
         - Fetch releases for project
         - Show recent releases first, then unreleased versions
         - Add ticket to selected release using `client.AddIssuesToRelease`
         - Track release in state
       - Save state
     - Return `(true, nil)` on success
3. Handle all errors gracefully

**Requirements**:
- Reuse existing assignment logic
- Perform all auto-actions after assignment
- Skip release for spikes
- Track sprint/release in state
- Handle errors at each step

**Testing**: Write tests that:
- Test assignment with all auto-actions
- Test assignment skipping release for spikes
- Test error handling at each step
- Mock all clients

**Files to modify**:
- `pkg/review/workflow.go`
- `pkg/review/workflow_test.go`

---

## Prompt 17: Implement Sequential Ticket Processing Loop

**Context**: We need the main workflow loop that processes tickets sequentially, showing progress and executing each step.

**Task**:
1. Add `ProcessTicketWorkflow(client jira.JiraClient, geminiClient gemini.GeminiClient, reader *bufio.Reader, cfg *config.Config, ticket jira.Issue, configDir string) error` function in `pkg/review/workflow.go`
2. Implementation:
   - Initialize `TicketStatus` (all false)
   - Display progress indicator using `DisplayProgress`
   - For each step in order:
     - Check if step already complete (from status)
     - If complete, skip to next step
     - If not complete:
       - Execute step handler function
       - If handler returns `(false, nil)`, user skipped - break loop (skip remaining steps)
       - If handler returns error, call error handler (see next prompt)
       - If handler returns `(true, nil)`, mark step complete in status
       - Refresh ticket data from Jira
       - Update progress display
   - Return nil on completion or skip

**Requirements**:
- Process steps in correct order
- Handle skip behavior (skip all remaining)
- Refresh ticket data after each step
- Update progress display
- Handle errors appropriately

**Testing**: Write tests that:
- Test complete workflow for one ticket
- Test skip behavior
- Test error handling
- Mock all dependencies

**Files to modify**:
- `pkg/review/workflow.go`
- `pkg/review/workflow_test.go`

---

## Prompt 18: Implement Error Handling and Recovery

**Context**: We need error handling that allows users to retry, skip, or abort when a step fails.

**Task**:
1. Add `HandleWorkflowError(err error, step WorkflowStep, reader *bufio.Reader) (Action, error)` function in `pkg/review/workflow.go`
2. Define `Action` type: `ActionRetry`, `ActionSkip`, `ActionAbort`
3. Implementation:
   - Display clear error message: "Error in {step}: {error}"
   - Prompt: "What would you like to do? [r]etry | [s]kip remaining | [a]bort"
   - Read user input
   - Return appropriate `Action`
4. Integrate into `ProcessTicketWorkflow`:
   - When step handler returns error, call `HandleWorkflowError`
   - If `ActionRetry`, retry the step
   - If `ActionSkip`, break loop (skip remaining steps)
   - If `ActionAbort`, return error to exit workflow

**Requirements**:
- Clear error messages
- Three clear options for user
- Integrate seamlessly into workflow loop
- Handle invalid input gracefully

**Testing**: Write tests that:
- Test retry action
- Test skip action
- Test abort action
- Test invalid input handling
- Mock reader input

**Files to modify**:
- `pkg/review/workflow.go`
- `pkg/review/workflow_test.go`

---

## Prompt 19: Integrate Workflow into Review Command

**Context**: We need to replace the action menu in the `review` command with the guided workflow.

**Task**:
1. Modify `cmd/review.go`:
   - After ticket selection (in `handleReviewAction` or similar), instead of showing action menu, call `ProcessTicketWorkflow`
   - Remove or comment out old action menu code
   - Import `pkg/review` package
   - Pass all required parameters: client, geminiClient, reader, cfg, ticket, configDir
2. Update command help text to reflect guided workflow
3. Ensure workflow is called for both single and multiple ticket cases

**Requirements**:
- Don't break existing ticket selection logic
- Workflow should be called after ticket is selected
- Handle both single ticket and paginated list cases
- Update help text appropriately

**Testing**: 
- Manual testing: Run `jira review` and verify workflow executes
- Test with single ticket
- Test with multiple tickets

**Files to modify**:
- `cmd/review.go`

---

## Prompt 20: Add Configuration to Init Command

**Context**: We need to allow users to configure the new fields during `init`.

**Task**:
1. Modify `cmd/init.go`:
   - After existing prompts, add prompts for:
     - Description minimum length (default: 128)
     - Enable AI description quality check (default: false)
     - Severity field ID (with option to auto-detect)
     - Default board ID (optional)
   - For severity field:
     - Offer to auto-detect (call `DetectSeverityField`)
     - Or allow manual entry
     - Or skip (leave empty)
   - Preserve existing values if re-running init (don't overwrite if user just presses enter)

**Requirements**:
- Follow existing init pattern
- Preserve existing values
- Auto-detection for severity field
- Clear prompts and defaults

**Testing**: 
- Manual testing: Run `jira utils init` and verify new prompts
- Test preserving existing values
- Test auto-detection

**Files to modify**:
- `cmd/init.go`

---

## Prompt 21: Wire Everything Together and Test

**Context**: Final integration and testing of the complete workflow.

**Task**:
1. Review all code for consistency
2. Ensure all imports are correct
3. Run `make build` and fix any compilation errors
4. Run `make test` and fix any test failures
5. Test end-to-end:
   - Create a test ticket
   - Run `jira review` and select the ticket
   - Verify workflow executes all steps
   - Verify all updates are saved to Jira
   - Verify state is tracked correctly

**Requirements**:
- Everything compiles
- All tests pass
- End-to-end workflow works
- No regressions in existing functionality

**Files to review**:
- All modified files
- Integration points

---

## Summary

These prompts should be executed in order, with each building on the previous. After each prompt, verify the code compiles and tests pass before moving to the next. The final prompt ensures everything is wired together correctly.

