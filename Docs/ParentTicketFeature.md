# Parent Ticket Feature - Specification

## Overview
This feature allows users to optionally specify a parent ticket when creating a new Jira ticket. The system automatically detects whether the parent is an Epic (requiring Epic Link field) or a regular parent ticket (requiring Parent Link field), and handles the linking appropriately.

## Questions and Answers

### Q1: How should parent ticket selection be provided?
**A:** Both via command-line flag (`--parent KEY`) and interactive prompting. If a flag is provided, use it; otherwise, prompt the user interactively.

### Q2: How should parent ticket selection work?
**A:** Both - accept a ticket key directly (e.g., `PROJ-123`) and validate it exists, OR show an interactive searchable/paginated list. If a key is provided, validate it; otherwise show an interactive list.

### Q3: How should Epic detection work?
**A:** Auto-detect if the parent is an Epic by querying the parent ticket's issue type. No user intervention required.

### Q4: How should Epic Link field ID be handled?
**A:** Auto-detect the Epic Link custom field ID (similar to how story points and severity are detected). Query the Jira API for fields and search for "Epic Link" or similar.

### Q5: What tickets should appear in the interactive list?
**A:** Show recent tickets that are also epics or parent tickets. Filter to only show tickets that are Epics or have subtasks (parent tickets).

### Q6: How should "recent tickets" be tracked?
**A:** Track both recently created tickets AND tickets that were recently used as a parent. Store in `state.yaml` as `RecentParentTickets []string`.

### Q7: What should happen when a parent is specified?
**A:** Validate the parent exists and is accessible, auto-detect if it's an Epic vs Parent Link, and create the ticket silently (no confirmation message).

### Q8: What if Epic Link auto-detection fails?
**A:** Prompt the user to enter the Epic Link field ID and save it to `config.yaml` as `EpicLinkFieldID string`.

### Q9: What flag format should be used?
**A:** Be consistent with existing flags. Current flags use `StringVarP` which supports both `--parent PROJ-123` and `--parent=PROJ-123` formats.

### Q10: What information should be shown in the interactive list?
**A:** Show ticket keys, summaries, and issue types (e.g., `PROJ-123 [Epic]: Implement feature X`).

### Q11: When should a parent be added to recent list?
**A:** Always add the parent ticket to the recent parent tickets list regardless of how it was specified (flag or interactive selection).

## Requirements

### Functional Requirements

1. **Command-Line Flag Support**
   - Add `--parent` flag (short form `-P` for consistency with `--project -p`)
   - Accept ticket key format: `PROJ-123`
   - Support both `--parent PROJ-123` and `--parent=PROJ-123` formats
   - Validate ticket exists and is accessible before proceeding

2. **Interactive Parent Selection**
   - If no `--parent` flag provided, show interactive prompt
   - Display list of recent parent tickets (epics or tickets with subtasks) that match the current project
   - Show format: `PROJ-123 [Epic]: Summary text` or `PROJ-124 [Story]: Summary text`
   - Support pagination (use `cfg.ReviewPageSize` or default 10)
   - Support `--no-paging` flag if provided
   - Allow search/filtering by typing ticket key or summary
   - Show "Recent" section first, then "Other..." option to search all tickets

3. **Epic Detection**
   - When parent ticket is specified, fetch parent ticket details
   - Check `issue.Fields.IssueType.Name` field
   - If issue type is "Epic" (case-insensitive), use Epic Link field
   - Otherwise, use Parent Link field (existing `CreateTicketWithParent` method)

4. **Epic Link Field Detection**
   - Auto-detect Epic Link custom field ID by querying `/rest/api/2/field`
   - Search for fields with:
     - Name containing "Epic Link" (case-insensitive)
     - OR name containing "Epic" and type is "epiclink" or similar
   - Store detected field ID in `config.yaml` as `EpicLinkFieldID`
   - If auto-detection fails, prompt user during ticket creation to enter field ID
   - Save entered field ID to config for future use

5. **Ticket Creation with Parent**
   - If parent is Epic:
     - Use Epic Link custom field: `fields[EpicLinkFieldID] = parentKey`
   - If parent is not Epic:
     - Use existing `CreateTicketWithParent` method (uses `parent` field)
   - Handle both cases transparently to user

6. **Recent Parent Tickets Tracking**
   - Add `RecentParentTickets []string` to `State` struct in `pkg/config/state.go`
   - Track up to 6 unique parent tickets
   - Add `AddRecentParentTicket(ticketKey string)` method
   - When ticket is created with parent, add parent to recent list
   - When ticket is created (without parent), add new ticket to recent list
   - Move existing ticket to end if selected again (most recent at end)

7. **Recent Tickets Filtering**
   - When showing recent tickets, filter to only show:
     - Tickets that are Epics (issue type = "Epic")
     - Tickets that have subtasks (query for tickets with `parent = <ticket>`)
   - If no recent tickets match filter, show empty list and allow "Other..." search

8. **Error Handling**
   - If parent ticket doesn't exist: show error and allow retry or skip
   - If parent ticket is not accessible: show error and allow retry or skip
   - If Epic Link field detection fails: prompt for manual entry
   - If Epic Link field ID is invalid: show error and allow retry or skip
   - All errors should be user-friendly with actionable messages

### Technical Requirements

1. **Configuration Changes**
   - Add `EpicLinkFieldID string` to `Config` struct in `pkg/config/config.go`
   - Add YAML tag: `yaml:"epic_link_field_id,omitempty"`
   - Add comment explaining the field

2. **State Management Changes**
   - Add `RecentParentTickets []string` to `State` struct in `pkg/config/state.go`
   - Add `AddRecentParentTicket(ticketKey string)` method
   - Reuse existing `addToRecentList` helper function
   - Update `SaveState` to persist recent parent tickets

3. **Jira Client Interface Changes**
   - Add `DetectEpicLinkField(projectKey string) (string, error)` method
   - Add `GetIssue(issueKey string) (*Issue, error)` method (if not exists, for fetching single ticket)
   - Add `CreateTicketWithEpicLink(project, taskType, summary, epicKey, epicLinkFieldID string) (string, error)` method
   - Update `CreateTicket` to optionally accept parent information
   - OR create unified method: `CreateTicketWithParentOrEpic(project, taskType, summary, parentKey string, isEpic bool, epicLinkFieldID string) (string, error)`

4. **Create Command Changes**
   - Add `parentFlag string` variable
   - Add `--parent` flag in `init()` function
   - Modify `runCreate` to:
     - Check for `parentFlag`
     - If provided, validate and use it
     - If not provided, show interactive selection
     - Detect if parent is Epic
     - Call appropriate creation method
     - Add parent to recent list
     - Add newly created ticket to recent list

5. **Interactive Selection Implementation**
   - Create helper function `selectParentTicket(client jira.JiraClient, reader *bufio.Reader, cfg *config.Config, projectKey string, configPath string) (string, error)`
   - Load recent parent tickets from state
   - Filter recent tickets to only Epics and parent tickets
   - Show paginated list with ticket key, summary, and issue type
   - Support search/filter functionality
   - Return selected ticket key

6. **Epic Link Field Detection**
   - Implement `DetectEpicLinkField` similar to `DetectSeverityField`
   - Query `/rest/api/2/field` endpoint
   - Search for fields matching Epic Link criteria
   - Return field ID or empty string if not found

## Architecture Choices

### Parent vs Epic Link Handling
- **Decision**: Use separate API methods for Parent Link vs Epic Link
- **Rationale**: Jira API requires different field structures:
  - Parent Link: `fields.parent.key = "PROJ-123"`
  - Epic Link: `fields.customfield_XXXXX = "PROJ-123"` (custom field)
- **Implementation**: 
  - Keep existing `CreateTicketWithParent` for subtasks
  - Add new `CreateTicketWithEpicLink` for epics
  - Unified wrapper method that calls appropriate one based on detection

### Recent Tickets Storage
- **Decision**: Store in `state.yaml`, not `config.yaml`
- **Rationale**: Recent selections are runtime state, not configuration
- **Consistency**: Matches existing pattern for recent assignees, sprints, releases, components

### Epic Detection Strategy
- **Decision**: Query parent ticket's issue type before creation
- **Rationale**: Most reliable way to determine if Epic Link is needed
- **Performance**: Single API call, acceptable overhead
- **Alternative Considered**: Require user to specify `--epic-parent` flag - rejected for better UX

### Field ID Detection
- **Decision**: Auto-detect with manual fallback
- **Rationale**: Most Jira instances use standard field IDs, but some are customized
- **User Experience**: Seamless for most users, configurable for edge cases
- **Consistency**: Matches pattern used for story points and severity fields

## Data Handling

### State File (`state.yaml`)
```yaml
recent_parent_tickets:
  - PROJ-123
  - PROJ-456
  - PROJ-789
```

### Config File (`config.yaml`)
```yaml
epic_link_field_id: customfield_10011  # Auto-detected or manually configured
```

### API Payload Examples

**Parent Link (Subtask):**
```json
{
  "fields": {
    "project": {"key": "PROJ"},
    "summary": "Subtask summary",
    "issuetype": {"name": "Subtask"},
    "parent": {"key": "PROJ-123"}
  }
}
```

**Epic Link:**
```json
{
  "fields": {
    "project": {"key": "PROJ"},
    "summary": "Story summary",
    "issuetype": {"name": "Story"},
    "customfield_10011": "PROJ-456"  // Epic Link field
  }
}
```

## Error Handling Strategies

1. **Parent Ticket Not Found**
   - Error: `"Parent ticket PROJ-123 not found or not accessible"`
   - Action: Allow user to retry with different key or skip parent

2. **Epic Link Field Detection Failure**
   - Error: `"Could not auto-detect Epic Link field. Please enter the custom field ID:"`
   - Action: Prompt for manual entry, save to config

3. **Invalid Epic Link Field ID**
   - Error: `"Epic Link field ID 'customfield_XXXXX' is invalid or not accessible"`
   - Action: Allow retry or skip Epic Link assignment

4. **API Errors During Creation**
   - Error: Show Jira API error message
   - Action: Allow retry or abort ticket creation

5. **State Save Failures**
   - Error: Log warning but don't fail ticket creation
   - Action: Continue with ticket creation, recent list update is optional

## Testing Plan

### Unit Tests

1. **State Management**
   - Test `AddRecentParentTicket` adds ticket to list
   - Test `AddRecentParentTicket` moves existing ticket to end
   - Test `AddRecentParentTicket` maintains max 6 items
   - Test state save/load preserves recent parent tickets

2. **Epic Link Field Detection**
   - Test `DetectEpicLinkField` finds field by name
   - Test `DetectEpicLinkField` handles missing field gracefully
   - Test `DetectEpicLinkField` handles API errors

3. **Epic Detection**
   - Test detection logic correctly identifies Epic issue type
   - Test detection logic correctly identifies non-Epic issue types
   - Test case-insensitive matching

4. **Ticket Creation**
   - Test `CreateTicketWithEpicLink` creates ticket with Epic Link
   - Test `CreateTicketWithParent` still works for subtasks
   - Test error handling for invalid parent keys

### Integration Tests

1. **End-to-End Create with Parent Flag**
   - Create ticket with `--parent PROJ-123` flag
   - Verify parent is linked correctly
   - Verify parent added to recent list
   - Verify ticket added to recent list

2. **End-to-End Create with Interactive Selection**
   - Create ticket without `--parent` flag
   - Select parent from interactive list
   - Verify parent is linked correctly
   - Verify parent added to recent list

3. **Epic vs Parent Link**
   - Create ticket with Epic parent
   - Verify Epic Link field is used
   - Create ticket with Story parent
   - Verify Parent Link field is used

4. **Recent Tickets Filtering**
   - Create several tickets (some Epics, some regular)
   - Create subtasks for some tickets
   - Verify interactive list only shows Epics and parent tickets

### Manual Testing Scenarios

1. **Happy Path - Epic Parent via Flag**
   ```
   jira-tool create --parent PROJ-123 "New story"
   ```
   - Verify ticket created with Epic Link

2. **Happy Path - Story Parent via Flag**
   ```
   jira-tool create --parent PROJ-124 "New subtask"
   ```
   - Verify ticket created with Parent Link

3. **Happy Path - Interactive Selection**
   ```
   jira-tool create "New story"
   # Select parent from list
   ```
   - Verify interactive list shows recent Epics/parents
   - Verify selection works
   - Verify ticket created correctly

4. **Error Handling - Invalid Parent**
   ```
   jira-tool create --parent INVALID-999 "New story"
   ```
   - Verify error message
   - Verify graceful failure

5. **Epic Link Field Detection**
   - Test with Jira instance that has Epic Link field
   - Verify auto-detection works
   - Test with Jira instance without Epic Link field
   - Verify manual entry prompt appears

## Implementation Notes

### Files to Modify

1. **`pkg/config/config.go`**
   - Add `EpicLinkFieldID string` field

2. **`pkg/config/config_test.go`**
   - Add tests for `EpicLinkFieldID` field

3. **`pkg/config/state.go`**
   - Add `RecentParentTickets []string` field
   - Add `AddRecentParentTicket` method

4. **`pkg/config/state_test.go`**
   - Add tests for `AddRecentParentTicket` method

5. **`pkg/jira/client.go`**
   - Add `DetectEpicLinkField` method
   - Add `GetIssue` method (if needed)
   - Add `CreateTicketWithEpicLink` method
   - Update `JiraClient` interface

6. **`pkg/jira/epic.go`** (new file)
   - Move Epic Link detection logic here (similar to `severity.go`)

7. **`cmd/create.go`**
   - Add `parentFlag` variable
   - Add `--parent` flag
   - Modify `runCreate` to handle parent selection
   - Add `selectParentTicket` helper function
   - Update recent tickets tracking

### Dependencies

- No new external dependencies required
- Uses existing Jira API client patterns
- Uses existing state management patterns

### Backward Compatibility

- Existing `CreateTicket` method remains unchanged
- Existing `CreateTicketWithParent` method remains unchanged
- New functionality is opt-in (only when `--parent` flag used or interactive selection made)
- No breaking changes to existing commands

## Success Criteria

1. ✅ Users can specify parent ticket via `--parent` flag
2. ✅ Users can select parent ticket interactively
3. ✅ System auto-detects Epic vs Parent Link
4. ✅ System auto-detects Epic Link field ID
5. ✅ Recent parent tickets are tracked and displayed
6. ✅ Recent tickets list filters to Epics and parent tickets only
7. ✅ All error cases handled gracefully
8. ✅ Tests pass (unit and integration)
9. ✅ Documentation updated

