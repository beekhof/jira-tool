# Ticket Filter Feature - Code Generation Prompts

This document contains a series of prompts for implementing the ticket filter feature. Each prompt is designed to be used independently with a code-generation LLM, building incrementally on previous work.

---

## Prompt 1: Add Ticket Filter to Config

**Context**: We're adding support for a JQL filter that applies to all ticket queries. The config file is in `pkg/config/config.go` and uses YAML.

**Task**: Add the `TicketFilter` field to the `Config` struct in `pkg/config/config.go`:

1. Add field: `TicketFilter string` with YAML tag `yaml:"ticket_filter,omitempty"`
2. Add comment: `// JQL filter to append to all ticket queries (e.g., "assignee = currentUser()")`
3. Ensure `LoadConfig` handles missing field gracefully (uses empty string default)

**Requirements**:
- Follow existing code style and patterns
- Add YAML tag with `omitempty` for optional field
- Add descriptive comment

**Testing**: Write a test in `pkg/config/config_test.go` that:
- Loads a config with `TicketFilter` set
- Loads a config with `TicketFilter` missing (should use empty string)
- Verifies field is saved and loaded correctly

**Files to modify**:
- `pkg/config/config.go`
- `pkg/config/config_test.go`

---

## Prompt 2: Add Global Flags to Root Command

**Context**: We need to add `--filter` and `--no-filter` global flags to the root command. The root command is in `cmd/root.go`.

**Task**:
1. Add `filterFlag string` variable at the top of `cmd/root.go` (with other flag variables)
2. Add `noFilterFlag bool` variable
3. Add `--filter` persistent flag in `init()`: `rootCmd.PersistentFlags().StringVar(&filterFlag, "filter", "", "JQL filter to append to all ticket queries")`
4. Add `--no-filter` persistent flag: `rootCmd.PersistentFlags().BoolVar(&noFilterFlag, "no-filter", false, "Bypass ticket filter (overrides --filter and config)")`
5. Add `GetTicketFilter()` function that:
   - Returns empty string if `--no-filter` is set
   - Returns `filterFlag` if set
   - Returns `cfg.TicketFilter` from config if set
   - Returns empty string if none are set
   - Takes `cfg *config.Config` as parameter

**Requirements**:
- Follow existing flag patterns in `root.go`
- Use `PersistentFlags()` so flags are available to all commands
- Add descriptive help text

**Files to modify**:
- `cmd/root.go`

---

## Prompt 3: Create Filter Application Helper

**Context**: We need a helper function to apply the filter to JQL queries. Create a new file `pkg/jira/filter.go`.

**Task**:
1. Create new file `pkg/jira/filter.go`
2. Add `ApplyTicketFilter(jql, filter string) string` function:
   - If `filter` is empty, return `jql` unchanged
   - If `jql` is empty, return `filter` (no wrapping needed for single filter)
   - Otherwise, wrap `jql` in parentheses: `(jql)`
   - Append filter with AND: `(jql) AND (filter)`
   - Return the combined query

**Requirements**:
- Simple, pure function
- Handle edge cases (empty inputs)
- Always wrap existing query in parentheses for safety

**Files to modify**:
- `pkg/jira/filter.go` (new file)

---

## Prompt 4: Add Tests for Filter Application

**Context**: We need comprehensive tests for the filter application logic.

**Task**:
1. Create `pkg/jira/filter_test.go` (new file)
2. Add tests:
   - `TestApplyTicketFilter_SimpleQuery`: Test simple query with filter
   - `TestApplyTicketFilter_ComplexQuery`: Test query with AND/OR operators
   - `TestApplyTicketFilter_EmptyFilter`: Test with empty filter (should return original)
   - `TestApplyTicketFilter_EmptyQuery`: Test with empty query (should return filter)
   - `TestApplyTicketFilter_BothEmpty`: Test with both empty (should return empty)
   - `TestApplyTicketFilter_Parentheses`: Test that existing query is wrapped correctly

**Requirements**:
- Use Go testing package
- Test all edge cases
- Use table-driven tests where appropriate

**Files to modify**:
- `pkg/jira/filter_test.go` (new file)

---

## Prompt 5: Update Review Command

**Context**: We need to apply the filter to all ticket queries in the review command. The review command is in `cmd/review.go`.

**Task**:
1. Modify `runReview` function in `cmd/review.go`:
   - Get filter using `GetTicketFilter(cfg)` at the start
   - For all `client.SearchTickets(jql)` calls:
     - Apply filter using `jira.ApplyTicketFilter(jql, filter)`
     - Use the filtered JQL in the `SearchTickets` call
   - Apply to both single ticket queries and list queries

**Requirements**:
- Import `jira` package if not already imported
- Apply filter consistently to all ticket queries
- Don't apply filter to non-ticket queries (if any)

**Files to modify**:
- `cmd/review.go`

---

## Prompt 6: Update Assign Command

**Context**: We need to apply the filter to all ticket queries in the assign command. The assign command is in `cmd/assign.go`.

**Task**:
1. Modify `assignMultipleTickets` and `assignSingleTicket` functions in `cmd/assign.go`:
   - Get filter using `GetTicketFilter(cfg)` at the start of each function
   - For all `client.SearchTickets(jql)` calls:
     - Apply filter using `jira.ApplyTicketFilter(jql, filter)`
     - Use the filtered JQL in the `SearchTickets` call

**Requirements**:
- Import `jira` package if not already imported
- Apply filter to all ticket queries
- Ensure filter is applied consistently

**Files to modify**:
- `cmd/assign.go`

---

## Prompt 7: Update Estimate Command

**Context**: We need to apply the filter to all ticket queries in the estimate command. The estimate command is in `cmd/estimate.go`.

**Task**:
1. Modify `estimateMultipleTickets` and `estimateSingleTicket` functions in `cmd/estimate.go`:
   - Get filter using `GetTicketFilter(cfg)` at the start of each function
   - For all `client.SearchTickets(jql)` calls:
     - Apply filter using `jira.ApplyTicketFilter(jql, filter)`
     - Use the filtered JQL in the `SearchTickets` call

**Requirements**:
- Import `jira` package if not already imported
- Apply filter to all ticket queries
- Ensure filter is applied consistently

**Files to modify**:
- `cmd/estimate.go`

---

## Prompt 8: Update Create Command (Parent Selection)

**Context**: We need to apply the filter to ticket queries in the create command's parent selection. The create command is in `cmd/create.go`.

**Task**:
1. Modify `selectParentTicket` function in `cmd/create.go`:
   - Get filter using `GetTicketFilter(cfg)` at the start
   - For all `client.SearchTickets(jql)` calls:
     - Apply filter using `jira.ApplyTicketFilter(jql, filter)`
     - Use the filtered JQL in the `SearchTickets` call
   - Apply to queries that search for parent tickets

**Requirements**:
- Import `jira` package if not already imported
- Apply filter only to ticket queries, not other operations
- Ensure filter is applied consistently

**Files to modify**:
- `cmd/create.go`

---

## Prompt 9: Update Status Command

**Context**: We need to apply the filter to ticket queries in the status command. The status command is in `cmd/status.go`.

**Task**:
1. Modify `runSpikesStatus` function in `cmd/status.go`:
   - Get filter using `GetTicketFilter(cfg)` at the start
   - For `client.SearchTickets(jql)` call:
     - Apply filter using `jira.ApplyTicketFilter(jql, filter)`
     - Use the filtered JQL in the `SearchTickets` call
   - Review other status functions and apply filter if they query tickets

**Requirements**:
- Import `jira` package if not already imported
- Apply filter to ticket queries only
- Ensure filter is applied consistently

**Files to modify**:
- `cmd/status.go`

---

## Prompt 10: Add Filter to Init Command

**Context**: We should allow users to set the ticket filter during initialization. The init command is in `cmd/init.go`.

**Task**:
1. Modify `runInit` function in `cmd/init.go`:
   - After other config prompts, add prompt for ticket filter
   - Prompt: `"Ticket filter (JQL to append to all ticket queries, optional, press Enter to skip): "`
   - Read input from reader
   - If input is not empty, set `cfg.TicketFilter = input`
   - If input is empty and `existingCfg` has a filter, preserve it

**Requirements**:
- Follow existing prompt patterns in `init.go`
- Make it optional (can skip with Enter)
- Preserve existing filter if user skips

**Files to modify**:
- `cmd/init.go`

---

## Prompt 11: Update README Documentation

**Context**: We need to update the README to document the new filter feature.

**Task**:
1. In `README.md`, add documentation for:
   - `--filter` global flag
   - `--no-filter` global flag
   - `ticket_filter` config option
   - Examples of using filters
   - Filter precedence explanation

**Requirements**:
- Follow existing README style
- Add clear examples
- Explain filter precedence
- Document in appropriate sections

**Files to modify**:
- `README.md`

---

## Prompt 12: Wire Everything Together and Test

**Context**: Final integration and testing of the complete ticket filter feature.

**Task**:
1. Review all code for consistency:
   - Check all imports are correct
   - Verify all `SearchTickets` calls have filter applied
   - Ensure filter is not applied to non-ticket queries
2. Run `make build` and fix any compilation errors
3. Run `go test ./...` and fix any test failures
4. Test end-to-end:
   - Set filter in config file
   - Test `jira review` with config filter
   - Test `jira --filter "assignee = currentUser()" review`
   - Test `jira --no-filter review` (should bypass filter)
   - Test filter with all affected commands
   - Test complex JQL queries with filter

**Requirements**:
- Everything compiles
- All tests pass
- End-to-end workflow works
- No regressions in existing functionality

**Files to review**:
- All modified files
- Integration points

