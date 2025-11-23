# Guided Review Workflow - Specification

## Overview

This document specifies the implementation of a guided review workflow that automatically ensures tickets meet quality criteria. After selecting tickets to review, the tool guides users through a sequential process to ensure each ticket has:

- A reasonable description
- A component assigned (if applicable)
- Priority and severity set
- Story points estimated
- Moved to Backlog state (if in "New" state)
- Optionally assigned to someone (which triggers additional actions)

## Functional Requirements

### Workflow Structure

1. **Sequential Processing**: Process each ticket one at a time, completing all checks for one ticket before moving to the next
2. **Progress Indicator**: Show a checklist/progress indicator at the start of each ticket review showing all steps
3. **Skip Behavior**: If user skips any step, skip all remaining steps for that ticket and move to the next ticket

### Workflow Steps (in order)

1. **Description Check/Update**
   - Check if description meets quality criteria
   - If not, use Q&A flow to generate new description (include existing description in context)
   - Quality checks:
     - Minimum length (configurable in `config.yaml`, default: 128 characters)
     - Contains required elements (optional Gemini analysis, off by default)
     - Description length check via Jira API (from search results)

2. **Component Assignment**
   - Only prompt if no component is currently assigned
   - Show recent components first (from `state.yaml`), then full list
   - Track selection in recent components list

3. **Priority Assignment**
   - Check if priority is set
   - If not, show list of valid priority values from Jira API
   - Allow selection from list

4. **Severity Assignment**
   - Check if severity field is configured (auto-detect or manual config)
   - If configured and not set, show list of valid severity values from Jira API
   - If not configured, allow manual configuration during workflow or skip
   - Allow selection from list

5. **Story Points Estimation**
   - Check if story points are set
   - If not, use Gemini AI to suggest story points (like `estimate` command)
   - Show AI suggestion with reasoning
   - Allow manual override/selection
   - Use letter keys for selection, allow direct numeric input

6. **Backlog State Transition**
   - Check if ticket is in "New" state
   - If yes, automatically transition to "Backlog" state
   - If not, skip this step

7. **Optional Assignment**
   - Prompt user if they want to assign the ticket
   - If yes:
     - Show recent assignees first, then search results
     - Assign ticket to selected user
     - **Automatically perform additional actions:**
       - Move ticket to "In Progress" state
       - Add ticket to a sprint:
         - Auto-detect board(s) for the project
         - If multiple boards found, prompt user to select
         - Show recent sprints first, then active/planned sprints
         - Add ticket to selected sprint
       - Add ticket to a release (unless ticket is a spike):
         - Show recent releases first, then unreleased versions
         - Add ticket to selected release

### Configuration Requirements

#### New Config Fields (`config.yaml`)

```yaml
# Description quality checks
description_min_length: 128  # Minimum description length in characters
description_quality_ai: false  # Use Gemini to analyze description quality (optional, off by default)

# Severity field (auto-detected or manually configured)
severity_field_id: customfield_XXXXX  # Optional: custom field ID for severity

# Board selection
default_board_id: 1  # Optional: default board ID if auto-detection fails
```

#### State File Updates (`state.yaml`)

Add tracking for recent components:
```yaml
recent_components:
  - "Component Name 1"
  - "Component Name 2"
  # ... up to 6 unique components
```

### API Requirements

#### New Jira API Methods Needed

1. **Get Components** - Fetch list of components for a project
   - Endpoint: `/rest/api/2/project/{projectKey}/components`
   - Cache results

2. **Get Severity Field** - Auto-detect severity custom field
   - Similar to story points field detection
   - Query Jira fields API for severity-related fields

3. **Get Boards for Project** - Find boards associated with a project
   - Endpoint: `/rest/agile/1.0/board?projectKeyOrId={projectKey}`
   - Return list of boards with IDs and names

4. **Update Component** - Assign component to ticket
   - Endpoint: `/rest/api/2/issue/{issueId}`
   - Update `components` field

5. **Update Severity** - Set severity custom field
   - Endpoint: `/rest/api/2/issue/{issueId}`
   - Update custom field by ID

6. **Get Valid Severity Values** - Fetch allowed values for severity field
   - Query field configuration to get allowed values

7. **Transition to State** - Move ticket to specific state
   - Endpoint: `/rest/api/2/issue/{issueId}/transitions`
   - Support "Backlog" and "In Progress" transitions

### User Experience

#### Progress Indicator Format

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

Checking description...
```

#### Error Handling

When any step fails:
1. Display clear error message
2. Prompt user with options:
   - **Retry**: Retry the failed step
   - **Skip**: Skip remaining steps for this ticket
   - **Abort**: Exit the entire review workflow

#### Description Quality Check Flow

1. Check description length (from Jira API search results)
2. If optional Gemini analysis is enabled:
   - Fetch full ticket description
   - Use Gemini to analyze if description answers "what", "why", "how"
   - Display analysis results
3. If description fails checks:
   - Show current description (if any)
   - Run Q&A flow (include existing description in context)
   - Update ticket with new description

### Integration Points

#### Existing Code to Reuse

1. **Q&A Flow** (`pkg/qa/flow.go`)
   - Reuse `RunQAFlow` function
   - Modify to accept existing description as context

2. **Story Points Estimation** (`cmd/estimate.go`)
   - Reuse Gemini story point estimation logic
   - Reuse selection interface

3. **Component Selection** (similar to assignee/sprint/release)
   - Follow same pattern as recent selections
   - Use same UI patterns

4. **Sprint/Release Selection** (`cmd/accept.go`)
   - Reuse sprint and release selection logic
   - Reuse recent tracking

### Data Flow

```
User selects tickets to review
  ↓
For each ticket:
  ↓
Show progress checklist
  ↓
1. Check description → [Update if needed] → [Q&A flow if needed]
  ↓
2. Check component → [Select if missing] → [Track in recent]
  ↓
3. Check priority → [Select if missing]
  ↓
4. Check severity → [Select if missing] (if configured)
  ↓
5. Check story points → [AI suggest] → [Select]
  ↓
6. Check state → [Transition to Backlog if "New"]
  ↓
7. Optional assignment → [Select user] → [Auto: In Progress, Sprint, Release]
  ↓
Next ticket or complete
```

### Edge Cases

1. **No components available**: Skip component step if project has no components
2. **Severity field not found**: Allow manual configuration or skip
3. **No boards found**: Show error, allow manual board ID entry
4. **No active sprints**: Show planned sprints only
5. **No unreleased versions**: Skip release assignment
6. **Spike detection**: Skip release assignment for spikes (check "SPIKE" prefix)
7. **Invalid transitions**: Handle cases where transition is not available
8. **API rate limiting**: Implement retry logic with exponential backoff

### Testing Requirements

#### Unit Tests

1. Description quality checks (length, required elements)
2. Component selection and recent tracking
3. Priority/severity value validation
4. Story points estimation flow
5. State transition logic
6. Spike detection for release assignment

#### Integration Tests

1. End-to-end workflow for a single ticket
2. Multiple tickets sequential processing
3. Error handling and recovery flows
4. Recent selections persistence
5. Board auto-detection and selection

#### Manual Testing Scenarios

1. Ticket with all fields missing
2. Ticket with some fields already set
3. Ticket that's already in Backlog
4. Spike ticket (should skip release)
5. Ticket with no components available
6. Project with multiple boards
7. API failures at various steps
8. Skip behavior at different steps

### Implementation Phases

#### Phase 1: Foundation
- Add new config fields
- Add component tracking to state
- Implement component API methods
- Implement severity field detection

#### Phase 2: Core Workflow
- Implement sequential ticket processing
- Implement progress indicator
- Implement each workflow step
- Integrate with existing Q&A and estimation flows

#### Phase 3: Assignment Flow
- Implement assignment with auto-actions
- Implement board auto-detection
- Implement sprint/release selection with recent tracking
- Handle spike detection

#### Phase 4: Error Handling & Polish
- Implement error handling and recovery
- Add skip functionality
- Improve user feedback and messages
- Add comprehensive testing

### Success Criteria

1. User can select multiple tickets and process them sequentially
2. Each ticket is checked and updated according to all criteria
3. Recent selections are tracked and displayed appropriately
4. Assignment automatically triggers state transition, sprint, and release assignment
5. Error handling allows graceful recovery or skipping
6. All configuration options work as specified
7. Spike tickets are correctly identified and handled

