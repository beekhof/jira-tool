# Ticket Filter Feature - Specification

## Overview
This feature allows users to configure a JQL (Jira Query Language) filter that gets automatically appended to all ticket queries. The filter can be set via a global command-line flag or in the configuration file, and applies to all commands that query tickets (not users, sprints, releases, etc.).

## Questions and Answers

### Q1: What should the filter apply to?
**A:** JQL (Jira Query Language) filter that gets appended to all ticket queries. Only applies when querying for tickets, not for users, sprints, releases, or other entities.

### Q2: How should the JQL filter be combined with existing queries?
**A:** Append with `AND`. If query is `project = PROJ` and filter is `assignee = currentUser()`, result is `(project = PROJ) AND (assignee = currentUser())`.

### Q3: Where should this filter be applied?
**A:** All commands that query tickets (review, assign, estimate, create's parent selection, etc.).

### Q4: For the command-line flag, should it be?
**A:** A global flag (e.g., `--filter`) that applies to all commands.

### Q5: For the config option, should it be?
**A:** A single `ticket_filter` field that applies globally.

### Q6: When both a config filter and a command-line flag are provided, should?
**A:** The command-line flag override the config filter.

### Q7: Should there be a way to disable the filter for a specific command?
**A:** Yes, add a `--no-filter` flag to bypass the filter.

### Q8: How should the filter handle existing JQL queries that already contain AND/OR operators?
**A:** Always wrap the filter in parentheses and append: `(existing_query) AND (filter)`.

### Q9: Should the filter be validated for syntax errors?
**A:** No, pass it through to Jira and let Jira return the error.

## Requirements

### Functional Requirements

1. **Configuration Support**
   - Add `ticket_filter` field to `Config` struct in `pkg/config/config.go`
   - YAML tag: `yaml:"ticket_filter,omitempty"`
   - Store as string containing JQL filter clause
   - Load from config file during initialization

2. **Global Command-Line Flag**
   - Add `--filter` global flag to root command
   - Accept JQL filter string as value
   - Override config filter when provided
   - Support both `--filter "assignee = currentUser()"` and `--filter=assignee=currentUser()` formats

3. **Filter Bypass**
   - Add `--no-filter` global flag
   - When set, bypass both config and command-line filters
   - Takes precedence over `--filter` flag

4. **JQL Query Modification**
   - Modify all `SearchTickets` calls to append the filter
   - Wrap existing query in parentheses: `(existing_query)`
   - Append filter with AND: `(existing_query) AND (filter)`
   - Only apply to ticket queries, not user/sprint/release queries

5. **Commands Affected**
   - `review`: Filter tickets shown in review list
   - `assign`: Filter tickets shown in assignment list
   - `estimate`: Filter tickets shown in estimation list
   - `create`: Filter tickets shown in parent selection
   - Any other command that queries tickets via `SearchTickets`

6. **Filter Precedence**
   - `--no-filter` > `--filter` (command-line) > `ticket_filter` (config)
   - If `--no-filter` is set, no filter is applied
   - If `--filter` is set, it overrides config filter
   - If only config filter is set, it is used
   - If neither is set, no filter is applied

### Technical Requirements

1. **Configuration Changes**
   - Add `TicketFilter string` field to `Config` struct
   - Add YAML tag: `yaml:"ticket_filter,omitempty"`
   - Add comment explaining the field

2. **Root Command Changes**
   - Add `filterFlag string` variable
   - Add `noFilterFlag bool` variable
   - Add `--filter` and `--no-filter` flags to root command
   - Add helper function `GetTicketFilter()` to retrieve active filter

3. **Jira Client Changes**
   - Modify `SearchTickets` method to accept optional filter parameter
   - OR create wrapper function that applies filter before calling `SearchTickets`
   - Ensure filter is only applied to ticket queries, not other entity queries

4. **Query Modification Logic**
   - Create helper function `ApplyTicketFilter(jql, filter string) string`
   - Wrap existing JQL in parentheses: `(jql)`
   - Append filter: `(jql) AND (filter)`
   - Handle empty filter (return original JQL)
   - Handle empty JQL (return filter only, wrapped if needed)

5. **Command Updates**
   - Update all commands that use `SearchTickets`:
     - `cmd/review.go`
     - `cmd/assign.go`
     - `cmd/estimate.go`
     - `cmd/create.go` (parent selection)
   - Pass filter from root command to these commands
   - Use `GetTicketFilter()` helper to get active filter

## Architecture Choices

### Filter Application Strategy
- **Decision**: Apply filter in a centralized location (Jira client or helper function)
- **Rationale**: Ensures consistent application across all commands
- **Implementation**: 
  - Option 1: Modify `SearchTickets` to accept optional filter parameter
  - Option 2: Create wrapper function that applies filter before calling `SearchTickets`
  - **Preferred**: Option 2 (wrapper function) to avoid changing JiraClient interface

### Filter Storage
- **Decision**: Store in config file, not state file
- **Rationale**: Filter is a configuration preference, not runtime state
- **Consistency**: Matches other configuration options

### Global Flag vs Per-Command Flag
- **Decision**: Global flag only
- **Rationale**: Simpler UX, filter applies consistently across all commands
- **User Experience**: One flag to set, applies everywhere

### Query Wrapping
- **Decision**: Always wrap existing query in parentheses
- **Rationale**: Ensures correct JQL syntax regardless of existing query complexity
- **Safety**: Prevents syntax errors from operator precedence issues

## Data Handling

### Config File (`config.yaml`)
```yaml
ticket_filter: "assignee = currentUser() AND status != Done"
```

### Command-Line Usage
```bash
jira --filter "assignee = currentUser()" review
jira --filter "project = PROJ AND status != Closed" assign
jira --no-filter review  # Bypass filter
```

### JQL Query Examples

**Before filter:**
```
project = PROJ
```

**After filter (`assignee = currentUser()`):**
```
(project = PROJ) AND (assignee = currentUser())
```

**Complex query before filter:**
```
project = PROJ AND (status = "In Progress" OR status = "To Do")
```

**After filter (`assignee = currentUser()`):**
```
(project = PROJ AND (status = "In Progress" OR status = "To Do")) AND (assignee = currentUser())
```

## Error Handling Strategies

1. **Invalid JQL Filter**
   - Error: Jira API will return error with details
   - Action: Display Jira error message to user
   - No pre-validation (as per requirement)

2. **Empty Filter**
   - Behavior: No filter applied, original query used as-is
   - No error, just skip filter application

3. **Filter with Special Characters**
   - Behavior: Pass through to Jira as-is
   - User responsible for proper escaping if needed

4. **Conflicting Flags**
   - Behavior: `--no-filter` takes precedence over `--filter`
   - If both are set, `--no-filter` wins, filter is ignored

## Testing Plan

### Unit Tests

1. **Filter Application Logic**
   - Test `ApplyTicketFilter` with simple queries
   - Test `ApplyTicketFilter` with complex queries (AND/OR)
   - Test `ApplyTicketFilter` with empty filter
   - Test `ApplyTicketFilter` with empty query
   - Test filter wrapping in parentheses

2. **Filter Precedence**
   - Test `GetTicketFilter` with no filter set
   - Test `GetTicketFilter` with config filter only
   - Test `GetTicketFilter` with command-line filter
   - Test `GetTicketFilter` with `--no-filter` flag
   - Test `GetTicketFilter` with both `--filter` and `--no-filter`

3. **Configuration Loading**
   - Test loading config with `ticket_filter` set
   - Test loading config without `ticket_filter` (should be empty)
   - Test saving config with `ticket_filter`

### Integration Tests

1. **End-to-End Filter Application**
   - Set filter in config
   - Run `jira review` command
   - Verify only filtered tickets appear
   - Verify JQL sent to Jira includes filter

2. **Command-Line Override**
   - Set filter in config
   - Run `jira --filter "different filter" review`
   - Verify command-line filter is used, not config filter

3. **No-Filter Bypass**
   - Set filter in config
   - Run `jira --no-filter review`
   - Verify filter is not applied

4. **Multiple Commands**
   - Set filter in config
   - Run `jira review`, `jira assign`, `jira estimate`
   - Verify filter applies to all commands

### Manual Testing Scenarios

1. **Happy Path - Config Filter**
   ```
   # Set in config.yaml: ticket_filter: "assignee = currentUser()"
   jira review
   ```
   - Verify only user's tickets shown

2. **Happy Path - Command-Line Filter**
   ```
   jira --filter "status != Done" review
   ```
   - Verify only non-Done tickets shown

3. **Happy Path - No-Filter Bypass**
   ```
   # With filter in config
   jira --no-filter review
   ```
   - Verify all tickets shown (filter bypassed)

4. **Error Handling - Invalid JQL**
   ```
   jira --filter "invalid jql syntax" review
   ```
   - Verify Jira error message displayed

5. **Complex Query**
   ```
   jira --filter "assignee = currentUser() AND project = PROJ" review
   ```
   - Verify filter correctly applied to complex queries

## Implementation Notes

### Files to Modify

1. **`pkg/config/config.go`**
   - Add `TicketFilter string` field

2. **`pkg/config/config_test.go`**
   - Add tests for `TicketFilter` field

3. **`cmd/root.go`**
   - Add `filterFlag string` variable
   - Add `noFilterFlag bool` variable
   - Add `--filter` and `--no-filter` flags
   - Add `GetTicketFilter()` helper function

4. **`pkg/jira/client.go`** or new helper file
   - Add `ApplyTicketFilter(jql, filter string) string` function
   - OR modify `SearchTickets` to accept filter parameter

5. **All command files that use `SearchTickets`**
   - `cmd/review.go`
   - `cmd/assign.go`
   - `cmd/estimate.go`
   - `cmd/create.go`
   - Update to use filter from root command

### Dependencies

- No new external dependencies required
- Uses existing Jira API client patterns
- Uses existing configuration management

### Backward Compatibility

- Existing behavior unchanged if no filter is set
- All existing commands continue to work as before
- Filter is opt-in (only applies when configured)

## Success Criteria

1. ✅ Users can set filter in config file
2. ✅ Users can set filter via `--filter` global flag
3. ✅ Users can bypass filter with `--no-filter` flag
4. ✅ Filter applies to all ticket queries
5. ✅ Filter does not apply to user/sprint/release queries
6. ✅ Filter correctly combines with existing JQL queries
7. ✅ Command-line filter overrides config filter
8. ✅ `--no-filter` takes precedence over all filters
9. ✅ All tests pass
10. ✅ Documentation updated

