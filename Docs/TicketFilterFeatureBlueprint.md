# Ticket Filter Feature - Implementation Blueprint

## Overview
This blueprint provides a step-by-step plan for implementing the ticket filter feature. Each step builds incrementally on the previous ones, ensuring the codebase remains in a working state throughout development.

## Implementation Steps

### Step 1: Add Ticket Filter to Config
**Goal**: Add configuration field for storing the JQL filter.

**Tasks**:
1. Add `TicketFilter string` field to `Config` struct in `pkg/config/config.go`
2. Add YAML tag: `yaml:"ticket_filter,omitempty"`
3. Add comment explaining the field
4. Add test in `pkg/config/config_test.go` to verify field loads/saves correctly

**Files**:
- `pkg/config/config.go`
- `pkg/config/config_test.go`

**Verification**:
- Run `go test ./pkg/config`
- Verify config loads with and without the field

---

### Step 2: Add Global Flags to Root Command
**Goal**: Add `--filter` and `--no-filter` global flags.

**Tasks**:
1. Add `filterFlag string` variable in `cmd/root.go`
2. Add `noFilterFlag bool` variable in `cmd/root.go`
3. Add `--filter` persistent flag to root command
4. Add `--no-filter` persistent flag to root command
5. Add `GetTicketFilter()` helper function that returns active filter based on precedence

**Files**:
- `cmd/root.go`

**Verification**:
- Run `go build`
- Run `./bin/jira-tool --help` to verify flags appear
- Test `GetTicketFilter()` with different flag combinations

---

### Step 3: Create Filter Application Helper
**Goal**: Create helper function to apply filter to JQL queries.

**Tasks**:
1. Create new file `pkg/jira/filter.go`
2. Add `ApplyTicketFilter(jql, filter string) string` function
3. Handle empty filter (return original JQL)
4. Handle empty JQL (return filter only)
5. Wrap existing JQL in parentheses: `(jql)`
6. Append filter with AND: `(jql) AND (filter)`

**Files**:
- `pkg/jira/filter.go` (new file)

**Verification**:
- Run `go build ./pkg/jira`
- Add unit tests for various query combinations

---

### Step 4: Add Tests for Filter Application
**Goal**: Test the filter application logic thoroughly.

**Tasks**:
1. Create `pkg/jira/filter_test.go`
2. Test simple queries
3. Test complex queries with AND/OR
4. Test empty filter
5. Test empty query
6. Test filter wrapping

**Files**:
- `pkg/jira/filter_test.go` (new file)

**Verification**:
- Run `go test ./pkg/jira -v -run TestApplyTicketFilter`
- All tests pass

---

### Step 5: Update Review Command
**Goal**: Apply filter to review command ticket queries.

**Tasks**:
1. Modify `cmd/review.go` to get filter from `GetTicketFilter()`
2. Apply filter to all `SearchTickets` calls
3. Ensure filter is applied to both single ticket and list queries

**Files**:
- `cmd/review.go`

**Verification**:
- Run `go build`
- Test with `--filter` flag
- Test with config filter
- Test with `--no-filter` flag

---

### Step 6: Update Assign Command
**Goal**: Apply filter to assign command ticket queries.

**Tasks**:
1. Modify `cmd/assign.go` to get filter from `GetTicketFilter()`
2. Apply filter to all `SearchTickets` calls
3. Ensure filter is applied to both single ticket and list queries

**Files**:
- `cmd/assign.go`

**Verification**:
- Run `go build`
- Test with filter applied

---

### Step 7: Update Estimate Command
**Goal**: Apply filter to estimate command ticket queries.

**Tasks**:
1. Modify `cmd/estimate.go` to get filter from `GetTicketFilter()`
2. Apply filter to all `SearchTickets` calls
3. Ensure filter is applied to both single ticket and list queries

**Files**:
- `cmd/estimate.go`

**Verification**:
- Run `go build`
- Test with filter applied

---

### Step 8: Update Create Command (Parent Selection)
**Goal**: Apply filter to create command's parent ticket selection queries.

**Tasks**:
1. Modify `cmd/create.go` to get filter from `GetTicketFilter()`
2. Apply filter to `SearchTickets` calls in `selectParentTicket` function
3. Ensure filter only applies to ticket queries, not other operations

**Files**:
- `cmd/create.go`

**Verification**:
- Run `go build`
- Test parent selection with filter applied

---

### Step 9: Update Status Command
**Goal**: Apply filter to status command ticket queries.

**Tasks**:
1. Modify `cmd/status.go` to get filter from `GetTicketFilter()`
2. Apply filter to `SearchTickets` calls
3. Ensure filter applies to spike status queries

**Files**:
- `cmd/status.go`

**Verification**:
- Run `go build`
- Test status commands with filter

---

### Step 10: Update Accept Command
**Goal**: Apply filter to accept command ticket queries (if applicable).

**Tasks**:
1. Review `cmd/accept.go` for `SearchTickets` calls
2. Apply filter if ticket queries are made
3. Ensure single ticket lookups don't need filter (they're specific)

**Files**:
- `cmd/accept.go`

**Verification**:
- Run `go build`
- Verify behavior is correct

---

### Step 11: Add Filter to Init Command
**Goal**: Allow users to set filter during initialization.

**Tasks**:
1. Modify `cmd/init.go` to prompt for ticket filter
2. Save filter to config
3. Make it optional (press Enter to skip)

**Files**:
- `cmd/init.go`

**Verification**:
- Run `jira utils init`
- Verify filter is saved to config

---

### Step 12: Update Documentation
**Goal**: Update README with filter feature documentation.

**Tasks**:
1. Add filter documentation to README.md
2. Document `--filter` and `--no-filter` flags
3. Document `ticket_filter` config option
4. Add usage examples

**Files**:
- `README.md`

**Verification**:
- Review documentation for accuracy

---

### Step 13: Integration Testing
**Goal**: Test the complete feature end-to-end.

**Tasks**:
1. Test filter in config file
2. Test `--filter` flag override
3. Test `--no-filter` bypass
4. Test filter with all affected commands
5. Test complex JQL queries with filter
6. Test error handling (invalid JQL)

**Verification**:
- All commands work correctly with filter
- Filter precedence is correct
- No regressions in existing functionality

---

## Testing Strategy

### Unit Tests
- Test `ApplyTicketFilter` with various query combinations
- Test `GetTicketFilter` with different flag/config combinations
- Test config loading/saving with filter

### Integration Tests
- Test filter application in each command
- Test filter precedence
- Test filter bypass

### Manual Testing
- Test with real Jira instance
- Test various JQL filter combinations
- Test error cases

## Rollout Plan

1. **Phase 1**: Foundation (Steps 1-4)
   - Config and flag support
   - Filter application logic
   - Tests

2. **Phase 2**: Integration (Steps 5-10)
   - Apply filter to all commands
   - Ensure consistent behavior

3. **Phase 3**: Polish (Steps 11-13)
   - Init command integration
   - Documentation
   - Final testing

## Risk Mitigation

1. **Query Syntax Errors**
   - Mitigation: Let Jira handle validation, show clear error messages

2. **Filter Breaking Existing Queries**
   - Mitigation: Always wrap existing query in parentheses
   - Test with complex queries

3. **Performance Impact**
   - Mitigation: Filter is applied at query level, minimal overhead
   - No additional API calls

4. **Backward Compatibility**
   - Mitigation: Filter is opt-in, existing behavior unchanged if not set

