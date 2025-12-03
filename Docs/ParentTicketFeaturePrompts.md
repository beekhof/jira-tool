# Parent Ticket Feature - Code Generation Prompts

This document contains a series of prompts for implementing the parent ticket feature. Each prompt is designed to be used independently with a code-generation LLM, building incrementally on previous work.

---

## Prompt 1: Add Epic Link Field ID to Config

**Context**: We're adding support for Epic Link field ID configuration. The config file is in `pkg/config/config.go` and uses YAML.

**Task**: Add the `EpicLinkFieldID` field to the `Config` struct in `pkg/config/config.go`:

1. Add field: `EpicLinkFieldID string` with YAML tag `yaml:"epic_link_field_id,omitempty"`
2. Add comment: `// Epic Link custom field ID (auto-detected or manually configured)`
3. Ensure `LoadConfig` handles missing field gracefully (uses empty string default)

**Requirements**:
- Follow existing code style and patterns
- Add YAML tag with `omitempty` for optional field
- Add descriptive comment

**Testing**: Write a test in `pkg/config/config_test.go` that:
- Loads a config with `EpicLinkFieldID` set
- Loads a config with `EpicLinkFieldID` missing (should use empty string)
- Verifies field is saved and loaded correctly

**Files to modify**:
- `pkg/config/config.go`
- `pkg/config/config_test.go`

---

## Prompt 2: Add Recent Parent Tickets to State

**Context**: We need to track recently used parent tickets in `state.yaml`, similar to how we track recent assignees, sprints, releases, and components. The state management is in `pkg/config/state.go`.

**Task**: 
1. Add `RecentParentTickets []string` field to the `State` struct with YAML tag `yaml:"recent_parent_tickets,omitempty"`
2. Add `AddRecentParentTicket(ticketKey string)` method that:
   - Adds ticket key to recent list (max 6 unique)
   - Moves existing ticket to end if already in list
   - Maintains only last 6 items
3. Reuse the existing `addToRecentList` helper function

**Requirements**:
- Follow existing pattern for `AddRecentAssignee`, `AddRecentSprint`, etc.
- Use `addToRecentList` helper with max size 6
- Add descriptive comments

**Testing**: Write tests in `pkg/config/state_test.go`:
- Test `AddRecentParentTicket` adds ticket to list
- Test `AddRecentParentTicket` moves existing ticket to end
- Test `AddRecentParentTicket` maintains max 6 items
- Test state save/load preserves recent parent tickets

**Files to modify**:
- `pkg/config/state.go`
- `pkg/config/state_test.go`

---

## Prompt 3: Add GetIssue Method to Jira Client

**Context**: We need a method to fetch a single ticket by key for parent validation and Epic detection. The Jira client is in `pkg/jira/client.go`.

**Task**: 
1. Add `GetIssue(issueKey string) (*Issue, error)` method to the `JiraClient` interface
2. Implement the method in `jiraClient` struct:
   - Use existing `SearchTickets` method with JQL query: `key = <issueKey>`
   - Return first result if found
   - Return error if not found or if search returns empty
   - Handle API errors appropriately

**Requirements**:
- Follow existing patterns in `client.go`
- Use existing `SearchTickets` method (don't duplicate code)
- Return appropriate errors for not found cases
- Handle authentication errors

**Files to modify**:
- `pkg/jira/client.go`

---

## Prompt 4: Add Epic Link Field Detection

**Context**: We need to auto-detect the Epic Link custom field ID, similar to how story points and severity fields are detected. Create a new file `pkg/jira/epic.go` following the pattern of `pkg/jira/severity.go`.

**Task**:
1. Create new file `pkg/jira/epic.go`
2. Add `DetectEpicLinkField(projectKey string) (string, error)` method to `JiraClient` interface in `pkg/jira/client.go`
3. Implement the method in `jiraClient` struct:
   - Query `/rest/api/2/field` endpoint
   - Search for fields with name containing "Epic Link" (case-insensitive)
   - Also check for field type containing "epic" (case-insensitive)
   - Return first matching field ID
   - Return empty string if not found (not an error)
   - Handle API errors appropriately

**Requirements**:
- Follow the pattern from `DetectSeverityField` in `pkg/jira/severity.go`
- Use case-insensitive matching
- Return empty string (not error) if field not found
- Handle authentication and API errors

**Files to modify**:
- `pkg/jira/client.go` (interface update)
- `pkg/jira/epic.go` (new file)

---

## Prompt 5: Add CreateTicketWithEpicLink Method

**Context**: We need a method to create tickets with Epic Link field. The Jira client already has `CreateTicket` and `CreateTicketWithParent` methods in `pkg/jira/client.go`.

**Task**:
1. Add `CreateTicketWithEpicLink(project, taskType, summary, epicKey, epicLinkFieldID string) (string, error)` method to `JiraClient` interface
2. Implement the method in `jiraClient` struct:
   - Use same endpoint as `CreateTicket`: `/rest/api/2/issue`
   - Construct payload with project, summary, issuetype
   - Add Epic Link field: `fields[epicLinkFieldID] = epicKey`
   - Handle errors similar to `CreateTicket`
   - Return ticket key on success

**Requirements**:
- Follow existing `CreateTicket` pattern
- Use `epicLinkFieldID` as the field name in payload
- Set Epic Link value to `epicKey` (ticket key string)
- Handle all error cases appropriately

**Files to modify**:
- `pkg/jira/client.go`

---

## Prompt 6: Add Helper to Check if Ticket is Epic

**Context**: We need a utility function to determine if a ticket is an Epic. Add this to `pkg/jira/epic.go`.

**Task**:
1. Add helper function `IsEpic(issue *Issue) bool` in `pkg/jira/epic.go`
2. Check `issue.Fields.IssueType.Name` field
3. Compare case-insensitively with "Epic"
4. Return true if match, false otherwise

**Requirements**:
- Simple, pure function
- Case-insensitive comparison
- Handle nil issue gracefully

**Files to modify**:
- `pkg/jira/epic.go`

---

## Prompt 7: Add Helper to Filter Recent Parent Tickets

**Context**: We need to filter recent tickets to only show Epics and tickets with subtasks. Add this functionality to `pkg/jira/epic.go`.

**Task**:
1. Add helper function `FilterValidParentTickets(client jira.JiraClient, ticketKeys []string) ([]string, error)` in `pkg/jira/epic.go`
2. For each ticket key:
   - Fetch ticket using `GetIssue`
   - Check if it's an Epic using `IsEpic` helper
   - OR check if it has subtasks by querying: `parent = <ticketKey>` (use `SearchTickets`)
   - If either condition is true, include in result
3. Return filtered list of valid parent ticket keys
4. Handle errors gracefully (skip tickets that can't be fetched, log warnings)

**Requirements**:
- Use existing `GetIssue` and `SearchTickets` methods
- Use `IsEpic` helper function
- Handle errors gracefully (don't fail entire operation)
- Return filtered list even if some tickets fail to fetch

**Files to modify**:
- `pkg/jira/epic.go`

---

## Prompt 8: Add Parent Flag to Create Command

**Context**: We need to add a `--parent` command-line flag to the create command. The create command is in `cmd/create.go` and uses Cobra flags.

**Task**:
1. Add `parentFlag string` variable at the top of `cmd/create.go` (with other flag variables)
2. Add flag in `init()` function: `createCmd.Flags().StringVarP(&parentFlag, "parent", "P", "", "Parent ticket key (Epic or parent ticket)")`
3. Use short form `-P` for consistency with `--project -p`

**Requirements**:
- Follow existing flag pattern (see `projectFlag` and `typeFlag`)
- Use `StringVarP` for both long and short form support
- Add descriptive help text

**Files to modify**:
- `cmd/create.go`

---

## Prompt 9: Add Parent Ticket Selection Helper

**Context**: We need an interactive parent ticket selection function for the create command. This should show recent parent tickets and allow searching.

**Task**:
1. Add function `selectParentTicket(client jira.JiraClient, reader *bufio.Reader, cfg *config.Config, projectKey string, configPath string) (string, error)` in `cmd/create.go`
2. Load state from `state.yaml` using `config.GetStatePath` and `config.LoadState`
3. Get recent parent tickets from state
4. Filter recent tickets using `jira.FilterValidParentTickets` to only show Epics and parent tickets
5. Show paginated list with format: `PROJ-123 [Epic]: Summary text` or `PROJ-124 [Story]: Summary text`
   - Use `cfg.ReviewPageSize` or default 10 for pagination
   - Show issue type in brackets
   - Show ticket key and summary
6. Support "Other..." option to search all tickets in project
7. Return selected ticket key
8. Handle user cancellation (empty input or "q")

**Requirements**:
- Follow existing pagination patterns (see `cmd/review.go` or `cmd/assign.go`)
- Use `bufio.Reader` for input
- Show "Recent" section first, then "Other..." option
- Fetch ticket details to show summary and issue type
- Handle errors gracefully

**Files to modify**:
- `cmd/create.go`

---

## Prompt 10: Integrate Parent Selection into Create Flow

**Context**: We need to wire parent selection into the ticket creation flow in `runCreate` function.

**Task**:
1. Modify `runCreate` function in `cmd/create.go`:
   - After determining project and taskType, check if `parentFlag` is provided
   - If `parentFlag` is provided:
     - Validate parent ticket exists using `client.GetIssue(parentFlag)`
     - If not found, return error with helpful message
   - If `parentFlag` is not provided:
     - Call `selectParentTicket` for interactive selection
     - If user cancels, proceed without parent (optional parent)
   - After parent is determined:
     - Fetch parent ticket using `client.GetIssue` to check if Epic
     - Check if parent is Epic using `jira.IsEpic` helper
     - If Epic:
       - Get Epic Link field ID from config (`cfg.EpicLinkFieldID`)
       - If empty, attempt auto-detection using `client.DetectEpicLinkField`
       - If still empty, prompt user to enter field ID and save to config
       - Call `client.CreateTicketWithEpicLink(project, taskType, summary, parentKey, epicLinkFieldID)`
     - If not Epic:
       - Call `client.CreateTicketWithParent(project, taskType, summary, parentKey)`
   - After ticket creation:
     - Load state
     - Add parent to recent list using `state.AddRecentParentTicket(parentKey)`
     - Add newly created ticket to recent list using `state.AddRecentParentTicket(ticketKey)`
     - Save state using `config.SaveState`

**Requirements**:
- Handle all error cases gracefully
- Save Epic Link field ID to config if manually entered
- Update state after successful creation
- Continue with existing description flow after ticket creation

**Files to modify**:
- `cmd/create.go`

---

## Prompt 11: Add Epic Link Field Detection to Init

**Context**: We should auto-detect Epic Link field during `init` command, similar to how story points field is detected. The init command is in `cmd/init.go`.

**Task**:
1. Modify `cmd/init.go`:
   - After detecting story points field (around line 130-154), add Epic Link field detection
   - If Jira token and URL are available:
     - Create temporary Jira client
     - Call `client.DetectEpicLinkField(cfg.DefaultProject)` (or use project from config)
     - If detected, save to config: `cfg.EpicLinkFieldID = detectedID`
     - If not detected, keep existing value from `existingCfg` if present
   - Display message: `"Detected Epic Link field ID: <id>"` or `"Epic Link field not detected (optional)"`

**Requirements**:
- Follow existing pattern for story points detection
- Handle errors gracefully (don't fail init if detection fails)
- Preserve existing config value if detection fails
- Only attempt detection if Jira credentials are available

**Files to modify**:
- `cmd/init.go`

---

## Prompt 12: Handle Epic Link Field Detection Failure During Creation

**Context**: If Epic Link field ID is needed but not in config during ticket creation, we should prompt the user to enter it.

**Task**:
1. In `runCreate` function in `cmd/create.go`, when Epic Link is needed but `cfg.EpicLinkFieldID` is empty:
   - Attempt auto-detection using `client.DetectEpicLinkField(project)`
   - If detection fails (returns empty string):
     - Prompt user: `"Epic Link field not detected. Please enter the custom field ID (e.g., customfield_10011) or press Enter to skip:"`
     - Read input from `reader`
     - If user enters value:
       - Validate format (should start with "customfield_")
       - Save to config: `cfg.EpicLinkFieldID = input`
       - Save config using `config.SaveConfig(cfg, configPath)`
     - If user skips (empty input), return error: `"Epic Link field ID required for Epic parent"`
   - Use detected or entered field ID for ticket creation

**Requirements**:
- Validate field ID format
- Save to config for future use
- Handle user cancellation gracefully
- Provide helpful error messages

**Files to modify**:
- `cmd/create.go`

---

## Prompt 13: Add Comprehensive Error Handling

**Context**: We need to add comprehensive error handling for all failure cases in the parent ticket feature.

**Task**:
1. In `runCreate` function, add error handling for:
   - Parent ticket not found: `"Parent ticket %s not found or not accessible. Please check the ticket key."`
   - Parent ticket not accessible: Same as above (GetIssue will return error)
   - Epic Link field ID validation: `"Invalid Epic Link field ID format. Must start with 'customfield_'"`
   - API errors during ticket creation: Show Jira API error message
   - State save failures: Log warning but don't fail ticket creation
2. For interactive selection, handle:
   - No valid parent tickets found: Show message and allow "Other..." search
   - Search returns no results: Show message and allow retry
   - User cancellation: Return empty string (no error)

**Requirements**:
- All errors should be user-friendly
- Provide actionable error messages
- Don't fail silently on critical errors
- Log warnings for non-critical errors (like state save failures)

**Files to modify**:
- `cmd/create.go`
- `pkg/jira/epic.go` (if needed)

---

## Prompt 14: Add Tests for Recent Parent Tickets

**Context**: We need tests for the recent parent tickets functionality in state management.

**Task**:
1. In `pkg/config/state_test.go`, add tests:
   - `TestAddRecentParentTicket`: Test adding a parent ticket to empty list
   - `TestAddRecentParentTicketMovesToEnd`: Test that selecting existing parent moves it to end
   - `TestAddRecentParentTicketMaxSize`: Test that list maintains max 6 items
   - `TestStateSaveLoadParentTickets`: Test that recent parent tickets are saved and loaded correctly

**Requirements**:
- Follow existing test patterns in `state_test.go`
- Test all edge cases
- Use table-driven tests where appropriate

**Files to modify**:
- `pkg/config/state_test.go`

---

## Prompt 15: Add Tests for Epic Detection and Creation

**Context**: We need tests for Epic detection and ticket creation with Epic Link.

**Task**:
1. Create `pkg/jira/epic_test.go` (new file):
   - `TestIsEpic`: Test `IsEpic` function with various issue types
   - `TestIsEpicCaseInsensitive`: Test case-insensitive matching
   - `TestIsEpicNilIssue`: Test nil issue handling
2. Add tests for `DetectEpicLinkField` (mock HTTP responses):
   - Test successful detection
   - Test field not found (returns empty string)
   - Test API error handling
3. Add tests for `CreateTicketWithEpicLink` (mock HTTP responses):
   - Test successful creation
   - Test API error handling
   - Test invalid Epic Link field ID

**Requirements**:
- Use Go testing package
- Mock HTTP responses where needed
- Test all error cases
- Follow existing test patterns in `pkg/jira/client_test.go` if it exists

**Files to modify**:
- `pkg/jira/epic_test.go` (new file)

---

## Prompt 16: Update README Documentation

**Context**: We need to update the README to document the new `--parent` flag and parent ticket feature.

**Task**:
1. In `README.md`, add documentation for:
   - `--parent` flag in the create command section
   - Examples of using parent tickets:
     - `jira-tool create --parent PROJ-123 "New story"`
     - `jira-tool create "New subtask"` (interactive parent selection)
   - Explanation of Epic Link vs Parent Link
   - Epic Link field auto-detection

**Requirements**:
- Follow existing README style
- Add clear examples
- Explain when to use Epic Link vs Parent Link
- Document interactive selection

**Files to modify**:
- `README.md`

---

## Prompt 17: Wire Everything Together and Test

**Context**: Final integration and testing of the complete parent ticket feature.

**Task**:
1. Review all code for consistency:
   - Check all imports are correct
   - Verify all methods are properly implemented
   - Ensure error handling is consistent
2. Run `make build` and fix any compilation errors
3. Run `go test ./...` and fix any test failures
4. Test end-to-end:
   - Create ticket with `--parent` flag (Epic)
   - Create ticket with `--parent` flag (Story/Subtask)
   - Create ticket with interactive parent selection
   - Verify parent is linked correctly in Jira
   - Verify recent parent tickets are tracked
   - Test Epic Link field detection
   - Test error cases (invalid parent, etc.)

**Requirements**:
- Everything compiles
- All tests pass
- End-to-end workflow works
- No regressions in existing functionality

**Files to review**:
- All modified files
- Integration points


