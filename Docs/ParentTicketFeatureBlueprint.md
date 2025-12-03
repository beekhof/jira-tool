# Parent Ticket Feature - Implementation Blueprint

## Overview
This blueprint provides a step-by-step plan for implementing the parent ticket feature. Each step builds incrementally on the previous ones, ensuring the codebase remains in a working state throughout development.

## Implementation Steps

### Step 1: Add Epic Link Field ID to Config
**Goal**: Add configuration field for storing the Epic Link custom field ID.

**Tasks**:
1. Add `EpicLinkFieldID string` field to `Config` struct in `pkg/config/config.go`
2. Add YAML tag: `yaml:"epic_link_field_id,omitempty"`
3. Add comment explaining the field
4. Add test in `pkg/config/config_test.go` to verify field loads/saves correctly

**Files**:
- `pkg/config/config.go`
- `pkg/config/config_test.go`

**Verification**:
- Run `go test ./pkg/config`
- Verify config loads with and without the field

---

### Step 2: Add Recent Parent Tickets to State
**Goal**: Add tracking for recently used parent tickets.

**Tasks**:
1. Add `RecentParentTickets []string` field to `State` struct in `pkg/config/state.go`
2. Add YAML tag: `yaml:"recent_parent_tickets,omitempty"`
3. Add `AddRecentParentTicket(ticketKey string)` method
4. Reuse existing `addToRecentList` helper function
5. Add tests in `pkg/config/state_test.go`:
   - Test adding parent ticket
   - Test moving existing parent to end
   - Test max 6 items limit

**Files**:
- `pkg/config/state.go`
- `pkg/config/state_test.go`

**Verification**:
- Run `go test ./pkg/config`
- Verify state saves/loads correctly

---

### Step 3: Add GetIssue Method to Jira Client
**Goal**: Add method to fetch a single ticket by key (needed for parent validation and Epic detection).

**Tasks**:
1. Add `GetIssue(issueKey string) (*Issue, error)` method to `JiraClient` interface
2. Implement method in `jiraClient` struct
3. Use existing `SearchTickets` with `key = <issueKey>` JQL query
4. Return first result or error if not found
5. Add test in `pkg/jira/client_test.go` (if exists) or create basic test

**Files**:
- `pkg/jira/client.go`
- `pkg/jira/client_test.go` (if exists)

**Verification**:
- Run `go build ./pkg/jira`
- Manually test with real Jira instance (optional)

---

### Step 4: Add Epic Link Field Detection
**Goal**: Auto-detect the Epic Link custom field ID.

**Tasks**:
1. Create new file `pkg/jira/epic.go`
2. Add `DetectEpicLinkField(projectKey string) (string, error)` method to `JiraClient` interface
3. Implement method similar to `DetectSeverityField`:
   - Query `/rest/api/2/field` endpoint
   - Search for fields with name containing "Epic Link" (case-insensitive)
   - Also check for field type "epiclink" or similar
   - Return first matching field ID or empty string if not found
4. Add method to `jiraClient` struct

**Files**:
- `pkg/jira/client.go` (interface update)
- `pkg/jira/epic.go` (new file)

**Verification**:
- Run `go build ./pkg/jira`
- Test with Jira instance that has Epic Link field

---

### Step 5: Add CreateTicketWithEpicLink Method
**Goal**: Add method to create tickets with Epic Link field.

**Tasks**:
1. Add `CreateTicketWithEpicLink(project, taskType, summary, epicKey, epicLinkFieldID string) (string, error)` method to `JiraClient` interface
2. Implement method in `jiraClient` struct:
   - Use same endpoint as `CreateTicket`: `/rest/api/2/issue`
   - Add Epic Link field to payload: `fields[epicLinkFieldID] = epicKey`
   - Handle errors similar to `CreateTicket`
3. Return ticket key on success

**Files**:
- `pkg/jira/client.go`

**Verification**:
- Run `go build ./pkg/jira`
- Test with real Jira instance (optional)

---

### Step 6: Add Helper to Check if Ticket is Epic
**Goal**: Add utility function to determine if a ticket is an Epic.

**Tasks**:
1. Add helper function `IsEpic(issue *Issue) bool` in `pkg/jira/epic.go`
2. Check `issue.Fields.IssueType.Name` field
3. Compare case-insensitively with "Epic"
4. Return true if match, false otherwise

**Files**:
- `pkg/jira/epic.go`

**Verification**:
- Run `go test ./pkg/jira` (if tests exist)
- Or run `go build ./pkg/jira`

---

### Step 7: Add Helper to Filter Recent Parent Tickets
**Goal**: Filter recent tickets to only show Epics and tickets with subtasks.

**Tasks**:
1. Add helper function `FilterValidParentTickets(client jira.JiraClient, ticketKeys []string) ([]string, error)` in `pkg/jira/epic.go` or new helper file
2. For each ticket key:
   - Fetch ticket using `GetIssue`
   - Check if it's an Epic using `IsEpic`
   - OR check if it has subtasks (query: `parent = <ticketKey>`)
3. Return filtered list of valid parent ticket keys
4. Handle errors gracefully (skip tickets that can't be fetched)

**Files**:
- `pkg/jira/epic.go` or `pkg/jira/parent.go` (new file)

**Verification**:
- Run `go build ./pkg/jira`
- Test with real tickets (optional)

---

### Step 8: Add Parent Flag to Create Command
**Goal**: Add `--parent` command-line flag.

**Tasks**:
1. Add `parentFlag string` variable in `cmd/create.go`
2. Add flag in `init()` function: `createCmd.Flags().StringVarP(&parentFlag, "parent", "P", "", "Parent ticket key (Epic or parent ticket)")`
3. Use short form `-P` for consistency with `--project -p`

**Files**:
- `cmd/create.go`

**Verification**:
- Run `go build`
- Run `./bin/jira-tool create --help` to verify flag appears

---

### Step 9: Add Parent Ticket Selection Helper
**Goal**: Create interactive parent ticket selection function.

**Tasks**:
1. Add function `selectParentTicket(client jira.JiraClient, reader *bufio.Reader, cfg *config.Config, projectKey string, configPath string) (string, error)` in `cmd/create.go`
2. Load state and get recent parent tickets
3. Filter recent tickets to only Epics and parent tickets
4. Show paginated list with format: `PROJ-123 [Epic]: Summary`
5. Support search/filtering
6. Return selected ticket key
7. Handle "Other..." option to search all tickets in project

**Files**:
- `cmd/create.go`

**Verification**:
- Run `go build`
- Test interactively (optional)

---

### Step 10: Integrate Parent Selection into Create Flow
**Goal**: Wire parent selection into ticket creation flow.

**Tasks**:
1. Modify `runCreate` function in `cmd/create.go`:
   - Check if `parentFlag` is provided
   - If yes, validate parent ticket exists using `GetIssue`
   - If no, call `selectParentTicket` for interactive selection
   - After parent is determined, fetch parent ticket to check if Epic
   - Get Epic Link field ID (from config or auto-detect)
   - Call appropriate creation method (`CreateTicketWithEpicLink` or `CreateTicketWithParent`)
   - Add parent to recent list
   - Add newly created ticket to recent list
   - Save state

**Files**:
- `cmd/create.go`

**Verification**:
- Run `go build`
- Test with `--parent` flag
- Test with interactive selection

---

### Step 11: Add Epic Link Field Detection to Init
**Goal**: Auto-detect Epic Link field during `init` command.

**Tasks**:
1. Modify `cmd/init.go`:
   - After detecting story points field, attempt to detect Epic Link field
   - Use `jira.DetectEpicLinkField` method
   - If detected, save to config
   - If not detected, prompt user to enter manually (optional)
   - Save to config

**Files**:
- `cmd/init.go`

**Verification**:
- Run `go build`
- Run `jira-tool init` and verify Epic Link detection

---

### Step 12: Handle Epic Link Field Detection Failure
**Goal**: Prompt user for Epic Link field ID if auto-detection fails during ticket creation.

**Tasks**:
1. In `runCreate`, if Epic Link field ID is needed but not in config:
   - Attempt auto-detection
   - If fails, prompt user: `"Epic Link field not detected. Please enter the custom field ID (e.g., customfield_10011):"`
   - Validate input format (should start with "customfield_")
   - Save to config for future use
   - Use provided field ID for ticket creation

**Files**:
- `cmd/create.go`

**Verification**:
- Run `go build`
- Test with Jira instance without Epic Link field

---

### Step 13: Add Error Handling
**Goal**: Add comprehensive error handling for all failure cases.

**Tasks**:
1. Handle parent ticket not found errors
2. Handle parent ticket not accessible errors
3. Handle Epic Link field ID validation errors
4. Handle API errors during ticket creation
5. All errors should be user-friendly with actionable messages
6. Allow user to retry or skip parent assignment on errors

**Files**:
- `cmd/create.go`
- `pkg/jira/epic.go`
- `pkg/jira/client.go`

**Verification**:
- Run `go build`
- Test error scenarios

---

### Step 14: Add Tests
**Goal**: Add comprehensive unit and integration tests.

**Tasks**:
1. Add tests for `AddRecentParentTicket` in `pkg/config/state_test.go`
2. Add tests for `IsEpic` in `pkg/jira/epic_test.go` (new file)
3. Add tests for `DetectEpicLinkField` (mock API responses)
4. Add tests for `CreateTicketWithEpicLink` (mock API responses)
5. Add integration test for full create flow with parent

**Files**:
- `pkg/config/state_test.go`
- `pkg/jira/epic_test.go` (new file)
- `pkg/jira/client_test.go` (if exists)

**Verification**:
- Run `go test ./...`
- All tests pass

---

### Step 15: Update Documentation
**Goal**: Update README and any relevant documentation.

**Tasks**:
1. Update `README.md` to document `--parent` flag
2. Add examples of using parent tickets
3. Document Epic Link field detection
4. Update any command help text

**Files**:
- `README.md`
- `cmd/create.go` (help text)

**Verification**:
- Review documentation for accuracy
- Test examples work

---

## Testing Strategy

### Unit Tests
- Test each component in isolation
- Mock Jira API responses where needed
- Test error cases

### Integration Tests
- Test full create flow with `--parent` flag
- Test full create flow with interactive selection
- Test Epic vs Parent Link detection
- Test recent tickets tracking

### Manual Testing
- Test with real Jira instance
- Test with Epic parent
- Test with Story parent (subtask)
- Test with invalid parent
- Test Epic Link field detection
- Test recent tickets filtering

## Rollout Plan

1. **Phase 1**: Core functionality (Steps 1-7)
   - Config and state changes
   - Jira client methods
   - Epic detection

2. **Phase 2**: Integration (Steps 8-12)
   - Command-line flag
   - Interactive selection
   - Create flow integration

3. **Phase 3**: Polish (Steps 13-15)
   - Error handling
   - Tests
   - Documentation

## Risk Mitigation

1. **Epic Link Field Detection Failure**
   - Mitigation: Manual entry fallback
   - User can configure field ID in config

2. **Performance Impact of Filtering Recent Tickets**
   - Mitigation: Cache ticket lookups
   - Limit filtering to recent tickets only (not all tickets)

3. **API Rate Limiting**
   - Mitigation: Use existing cache mechanisms
   - Batch operations where possible

4. **Backward Compatibility**
   - Mitigation: All changes are opt-in
   - Existing functionality unchanged


