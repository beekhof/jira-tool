# Guided Review Workflow - Implementation Blueprint

## Overview

This blueprint breaks down the guided review workflow feature into small, iterative steps that build on each other. Each step is designed to be implemented safely with strong testing, ensuring incremental progress.

## Phase 1: Foundation - Configuration and State Management

### Step 1.1: Add Configuration Fields
**Goal**: Add new config fields for description quality and severity field

**Tasks**:
- Add `DescriptionMinLength` to `Config` struct in `pkg/config/config.go`
- Add `DescriptionQualityAI` boolean field
- Add `SeverityFieldID` string field
- Add `DefaultBoardID` int field
- Update `LoadConfig` to handle new fields with defaults
- Add tests for config loading with new fields

**Acceptance Criteria**:
- Config file can be loaded with new fields
- Default values are applied when fields are missing
- Existing configs continue to work

---

### Step 1.2: Add Component Tracking to State
**Goal**: Track recent component selections in state.yaml

**Tasks**:
- Add `RecentComponents []string` to `State` struct in `pkg/config/state.go`
- Add `AddRecentComponent(componentName string)` method
- Update `LoadState` and `SaveState` to handle components
- Add tests for component tracking

**Acceptance Criteria**:
- Components can be added to recent list
- Recent list maintains max 6 unique items
- State file persists components correctly

---

### Step 1.3: Implement Component API Methods
**Goal**: Add Jira API methods to fetch and update components

**Tasks**:
- Add `GetComponents(projectKey string) ([]Component, error)` to JiraClient interface
- Implement in `pkg/jira/client.go`:
  - Fetch components from `/rest/api/2/project/{projectKey}/components`
  - Cache results (check `noCache` flag)
  - Parse and return component list
- Add `UpdateTicketComponents(ticketID string, componentIDs []string) error`
- Add tests for component fetching and updating

**Acceptance Criteria**:
- Can fetch components for a project
- Components are cached appropriately
- Can update ticket components via API

---

### Step 1.4: Implement Severity Field Detection
**Goal**: Auto-detect severity custom field ID (similar to story points)

**Tasks**:
- Add `DetectSeverityField(projectKey string) (string, error)` to JiraClient interface
- Implement in `pkg/jira/client.go`:
  - Query `/rest/api/2/field` for custom fields
  - Search for fields with "severity" in name (case-insensitive)
  - Return first matching field ID
- Add `GetSeverityFieldValues(fieldID string) ([]string, error)` to fetch allowed values
- Add tests for field detection

**Acceptance Criteria**:
- Can detect severity field ID automatically
- Falls back gracefully if not found
- Can fetch allowed severity values

---

## Phase 2: Core Workflow Infrastructure

### Step 2.1: Create Workflow State Structure
**Goal**: Define data structures for tracking workflow progress

**Tasks**:
- Create `pkg/review/workflow.go` package
- Define `WorkflowStep` enum/constants for each step
- Define `TicketStatus` struct to track completion of each step
- Define `WorkflowProgress` struct to track overall progress
- Add helper functions to check step completion

**Acceptance Criteria**:
- Can track which steps are complete for a ticket
- Can determine next step to process
- Progress can be serialized/displayed

---

### Step 2.2: Implement Progress Indicator Display
**Goal**: Show checklist of workflow steps with completion status

**Tasks**:
- Add `DisplayProgress(ticket jira.Issue, status TicketStatus)` function
- Format output showing:
  - Ticket key and summary
  - Checklist with [✓] or [ ] for each step
  - Current step being processed
- Add tests for progress display formatting

**Acceptance Criteria**:
- Progress indicator displays correctly
- Shows accurate completion status
- Clear visual indication of current step

---

### Step 2.3: Implement Sequential Ticket Processing Loop
**Goal**: Process tickets one at a time in sequence

**Tasks**:
- Add `ProcessTicketWorkflow(client, geminiClient, cfg, ticket)` function
- Implement main loop that:
  - Shows progress indicator
  - Executes each step in order
  - Handles skip behavior (skip all remaining if user skips)
  - Moves to next ticket when complete
- Integrate with existing `review` command selection
- Add tests for processing loop

**Acceptance Criteria**:
- Tickets are processed sequentially
- Skip behavior works correctly
- Progress is tracked and displayed

---

## Phase 3: Individual Workflow Steps

### Step 3.1: Implement Description Quality Check
**Goal**: Check if description meets quality criteria

**Tasks**:
- Add `CheckDescriptionQuality(ticket jira.Issue, cfg *config.Config) (bool, string, error)` function
- Implement checks:
  - Minimum length check (from config)
  - Optional Gemini analysis (if enabled)
- Return quality status and reason if failing
- Add tests for quality checks

**Acceptance Criteria**:
- Can check description length
- Optional Gemini analysis works when enabled
- Returns clear quality status

---

### Step 3.2: Integrate Q&A Flow with Existing Description
**Goal**: Use Q&A flow to generate description, including existing description in context

**Tasks**:
- Modify `pkg/qa/flow.go` `RunQAFlow` to accept existing description
- Update prompt to include existing description in context
- Ensure existing description is passed to Gemini for improvement
- Update `cmd/create.go` to pass empty string for new tickets
- Add tests for Q&A with existing description

**Acceptance Criteria**:
- Q&A flow includes existing description in context
- Generated description improves on existing
- Works for both new and existing tickets

---

### Step 3.3: Implement Component Selection Step
**Goal**: Check and assign component if missing

**Tasks**:
- Add `HandleComponentStep(client, cfg, ticket)` function
- Check if ticket has components assigned
- If missing:
  - Load recent components from state
  - Fetch all components for project
  - Display selection (recent first, then all)
  - Update ticket and track selection
- Add tests for component selection

**Acceptance Criteria**:
- Only prompts if component missing
- Shows recent components first
- Tracks selection in state

---

### Step 3.4: Implement Priority Selection Step
**Goal**: Check and assign priority if missing

**Tasks**:
- Add `HandlePriorityStep(client, cfg, ticket)` function
- Check if priority is set
- If missing:
  - Fetch priorities from Jira (use existing `GetPriorities`)
  - Display selection list
  - Update ticket priority
- Add tests for priority selection

**Acceptance Criteria**:
- Only prompts if priority missing
- Shows valid priority values
- Updates ticket correctly

---

### Step 3.5: Implement Severity Selection Step
**Goal**: Check and assign severity if configured and missing

**Tasks**:
- Add `HandleSeverityStep(client, cfg, ticket)` function
- Check if severity field is configured
- If configured:
  - Check if severity is set
  - If missing:
    - Fetch allowed severity values
    - Display selection list
    - Update ticket severity
- Handle case where field not configured (allow config or skip)
- Add tests for severity selection

**Acceptance Criteria**:
- Only prompts if severity field configured and missing
- Shows valid severity values
- Handles missing field configuration gracefully

---

### Step 3.6: Integrate Story Points Estimation Step
**Goal**: Check and estimate story points if missing

**Tasks**:
- Add `HandleStoryPointsStep(client, geminiClient, cfg, ticket)` function
- Check if story points are set
- If missing:
  - Use Gemini to suggest story points (reuse `EstimateStoryPoints`)
  - Display AI suggestion with reasoning
  - Show selection interface (reuse from `estimate` command)
  - Update ticket story points
- Add tests for story points step

**Acceptance Criteria**:
- Shows AI suggestion
- Allows manual override
- Updates ticket correctly

---

### Step 3.7: Implement Backlog State Transition Step
**Goal**: Transition ticket to Backlog if in "New" state

**Tasks**:
- Add `HandleBacklogTransitionStep(client, ticket)` function
- Check if ticket is in "New" state
- If yes:
  - Get available transitions
  - Find "Backlog" transition
  - Execute transition
- Add tests for state transition

**Acceptance Criteria**:
- Only transitions if in "New" state
- Finds correct transition
- Executes transition successfully

---

## Phase 4: Assignment Flow with Auto-Actions

### Step 4.1: Implement Board Auto-Detection
**Goal**: Auto-detect boards for a project

**Tasks**:
- Add `GetBoardsForProject(projectKey string) ([]Board, error)` to JiraClient
- Implement in `pkg/jira/client.go`:
  - Query `/rest/agile/1.0/board?projectKeyOrId={projectKey}`
  - Parse and return board list with IDs and names
- Add tests for board detection

**Acceptance Criteria**:
- Can fetch boards for a project
- Returns board IDs and names
- Handles projects with no boards

---

### Step 4.2: Implement Board Selection Logic
**Goal**: Select board (auto if one, prompt if multiple)

**Tasks**:
- Add `SelectBoard(client, cfg, projectKey) (int, error)` function
- If one board found, return it automatically
- If multiple boards found:
  - Display list with selection
  - Allow user to choose
- If no boards found, use default from config or show error
- Add tests for board selection

**Acceptance Criteria**:
- Auto-selects if one board
- Prompts if multiple boards
- Handles no boards gracefully

---

### Step 4.3: Implement Assignment Step with Auto-Actions
**Goal**: Assign ticket and automatically perform additional actions

**Tasks**:
- Add `HandleAssignmentStep(client, geminiClient, cfg, ticket)` function
- Reuse existing assignment logic from `handleAssign`
- After assignment:
  - Transition to "In Progress" state
  - Select board (auto-detect or prompt)
  - Select sprint (show recent, then active/planned)
  - Add ticket to sprint
  - If not a spike:
    - Select release (show recent, then unreleased)
    - Add ticket to release
- Track sprint/release selections in state
- Add tests for assignment with auto-actions

**Acceptance Criteria**:
- Assigns ticket to user
- Transitions to "In Progress"
- Adds to sprint
- Adds to release (unless spike)
- Tracks recent selections

---

## Phase 5: Error Handling and Integration

### Step 5.1: Implement Error Handling and Recovery
**Goal**: Handle errors gracefully with retry/skip/abort options

**Tasks**:
- Add `HandleWorkflowError(err error, step WorkflowStep) (Action, error)` function
- Display clear error message
- Prompt user for action:
  - Retry: Retry the failed step
  - Skip: Skip remaining steps for this ticket
  - Abort: Exit entire workflow
- Integrate error handling into workflow loop
- Add tests for error handling

**Acceptance Criteria**:
- Errors are displayed clearly
- User can retry, skip, or abort
- Workflow continues appropriately based on choice

---

### Step 5.2: Integrate Workflow into Review Command
**Goal**: Replace action menu with guided workflow

**Tasks**:
- Modify `cmd/review.go` to use guided workflow
- After ticket selection, call `ProcessTicketWorkflow` instead of action menu
- Remove old action menu code
- Update command help text
- Add tests for integration

**Acceptance Criteria**:
- Review command uses guided workflow
- Old action menu is removed
- Workflow integrates seamlessly

---

### Step 5.3: Add Configuration to Init Command
**Goal**: Allow users to configure new fields during init

**Tasks**:
- Update `cmd/init.go` to prompt for:
  - Description minimum length
  - Enable/disable AI description quality check
  - Severity field ID (with auto-detection option)
  - Default board ID
- Preserve existing values if re-running init
- Add tests for init with new fields

**Acceptance Criteria**:
- Init prompts for new configuration options
- Auto-detection works for severity field
- Existing configs are preserved

---

## Phase 6: Testing and Polish

### Step 6.1: Comprehensive Unit Tests
**Goal**: Ensure all components are well-tested

**Tasks**:
- Write unit tests for all workflow steps
- Test error handling paths
- Test state management
- Test API interactions (with mocks)
- Achieve >80% code coverage

**Acceptance Criteria**:
- All functions have unit tests
- Error cases are covered
- Tests pass consistently

---

### Step 6.2: Integration Tests
**Goal**: Test end-to-end workflows

**Tasks**:
- Test complete workflow for single ticket
- Test multiple tickets sequential processing
- Test skip behavior
- Test error recovery flows
- Test with real Jira API (optional, with test instance)

**Acceptance Criteria**:
- End-to-end workflows work correctly
- Multiple tickets process correctly
- Error recovery works as expected

---

### Step 6.3: User Experience Polish
**Goal**: Improve user feedback and messages

**Tasks**:
- Review all user-facing messages
- Ensure consistent formatting
- Add helpful hints and tips
- Improve error messages
- Add progress indicators where helpful

**Acceptance Criteria**:
- Messages are clear and helpful
- Formatting is consistent
- User experience is smooth

---

## Implementation Order Summary

1. **Phase 1**: Foundation (Config, State, API Methods)
   - Step 1.1: Config fields
   - Step 1.2: Component tracking
   - Step 1.3: Component API
   - Step 1.4: Severity detection

2. **Phase 2**: Workflow Infrastructure
   - Step 2.1: Workflow state structure
   - Step 2.2: Progress indicator
   - Step 2.3: Processing loop

3. **Phase 3**: Individual Steps
   - Step 3.1: Description check
   - Step 3.2: Q&A integration
   - Step 3.3: Component step
   - Step 3.4: Priority step
   - Step 3.5: Severity step
   - Step 3.6: Story points step
   - Step 3.7: Backlog transition

4. **Phase 4**: Assignment Flow
   - Step 4.1: Board detection
   - Step 4.2: Board selection
   - Step 4.3: Assignment with auto-actions

5. **Phase 5**: Error Handling & Integration
   - Step 5.1: Error handling
   - Step 5.2: Review command integration
   - Step 5.3: Init command updates

6. **Phase 6**: Testing & Polish
   - Step 6.1: Unit tests
   - Step 6.2: Integration tests
   - Step 6.3: UX polish

## Success Metrics

- All workflow steps implemented and tested
- Error handling works gracefully
- Recent selections tracked correctly
- Assignment triggers all required actions
- Configuration options work as specified
- User experience is smooth and intuitive

