# Decompose Feature - Implementation Prompts

This document contains a series of prompts for implementing the decompose feature. Each prompt is standalone and builds on previous prompts. Execute them in order.

## Prompt 1: Add Configuration Fields

```text
Add configuration fields for the decompose command to the Config struct in pkg/config/config.go:

1. Add `DefaultMaxDecomposePoints int` field with YAML tag `yaml:"default_max_decompose_points,omitempty"` and comment "Default maximum story points per child ticket when decomposing (default: 5)"

2. Add `DecomposePromptTemplate string` field with YAML tag `yaml:"decompose_prompt_template,omitempty"` and comment "Prompt template for decomposition planning with Gemini AI"

3. Add a constant `defaultDecomposePromptTemplate` with a sensible default prompt that:
   - Instructs the AI to decompose a ticket into smaller child tickets
   - Includes placeholders: {{parent_summary}}, {{parent_description}}, {{existing_children}}, {{child_type}}, {{max_points}}
   - Asks for output in structured format with new tickets and existing tickets sections
   - Instructs to avoid duplicates and respect story point limits

4. Update any config loading/saving logic if needed to handle these new fields

5. Add tests in pkg/config/config_test.go to verify the new fields load and save correctly

Ensure the code follows existing patterns in the codebase and passes all linting checks.
```

## Prompt 2: Create Command Skeleton

```text
Create a new file cmd/decompose.go with a basic decompose command structure:

1. Create a cobra command `decomposeCmd` with:
   - Use: "decompose [TICKET_ID]"
   - Short: "Decompose a ticket into smaller child tickets"
   - Long: Description explaining the command
   - Args: cobra.ExactArgs(1) to require exactly one ticket ID
   - RunE: runDecompose function

2. Add a flag `--max-points` (int) to specify the maximum story points per child ticket

3. Implement `runDecompose` function that:
   - Gets config directory and loads config
   - Normalizes the ticket ID (use existing normalizeTicketID helper)
   - Creates a Jira client
   - Validates the ticket exists and is accessible (use client.GetIssue)
   - For now, just print "Ticket found: <ticket_key>" and return nil

4. Register the command in cmd/root.go by adding it to the root command

5. Follow existing code patterns and ensure it compiles and passes linting

The command should be callable as: `jira decompose ENG-123 --max-points 5`
```

## Prompt 3: Implement Story Point Limit Logic

```text
In cmd/decompose.go, implement a helper function `getMaxStoryPoints` that:

1. Takes parameters: cmd (cobra.Command), cfg (*config.Config), reader (*bufio.Reader)
2. Returns: (int, error)

3. Logic:
   - First check if --max-points flag is set and > 0, use that value
   - Else check if cfg.DefaultMaxDecomposePoints > 0, use that value
   - Else prompt user: "Maximum story points per child ticket (default: 5): "
   - If user enters empty string, default to 5
   - Validate input is a positive integer
   - Return the value

4. Update `runDecompose` to call this function and store the result

5. Add error handling for invalid input (re-prompt on error)

6. Write a test for this function in cmd/decompose_test.go

Follow existing patterns for user input handling in the codebase.
```

## Prompt 4: Create Type Mapping Logic

```text
Create a new file pkg/jira/type_mapping.go with type mapping functionality:

1. Create a function `GetChildTicketType(parentType string, reader *bufio.Reader, cfg *config.Config) (string, error)` that:
   - Implements hardcoded mapping:
     - "Epic" → "Story"
     - "Story" → "Task"
     - "Task" → "Sub-task"
   - For unknown types, prompt user interactively:
     - Show: "Parent ticket type \"<type>\" has no default child type mapping."
     - Prompt: "What type should child tickets be? [Task/Story/Sub-task/Other]: "
     - If "Other", prompt for custom type name
     - Validate the type exists in Jira (optional, can skip for now)
     - Return the selected type

2. Create a helper function `getDefaultChildType(parentType string) (string, bool)` that returns the mapped type and whether it was found

3. Add tests in pkg/jira/type_mapping_test.go for:
   - Known type mappings
   - Unknown type handling (mock reader)

4. Update cmd/decompose.go to:
   - Fetch the parent ticket
   - Get its issue type
   - Call GetChildTicketType to determine child type
   - Store the result

Follow existing patterns for interactive prompts and error handling.
```

## Prompt 5: Enhance Child Ticket Fetching

```text
In pkg/jira/epic.go, enhance the GetChildTickets function or create a new function GetChildTicketsDetailed that:

1. Returns full ticket information instead of just summaries:
   - Create a new struct `ChildTicketInfo` with fields: Key, Summary, StoryPoints, Type, IsSubtask
   - Return `[]ChildTicketInfo` instead of `[]string`

2. Fetch story points for each child ticket:
   - Use the story points field ID from config
   - Extract from ticket fields

3. Fetch issue type for each child ticket:
   - Extract from ticket.Fields.IssueType.Name

4. Determine if ticket is a subtask (parent link) or epic child (epic link):
   - Check if ticket has parent field set
   - If parent field exists, it's a subtask
   - Otherwise, it's an epic child

5. Update the function signature to return `([]ChildTicketInfo, error)`

6. Keep the existing GetChildTickets function for backward compatibility (or update all callers)

7. Add tests in pkg/jira/epic_test.go for the new function

8. Update cmd/decompose.go to use the new function and store existing children

Ensure the function handles both subtasks and epic children correctly.
```

## Prompt 6: Create Decomposition Plan Parser

```text
Create a new file pkg/parser/decompose.go with parsing functionality:

1. Create a struct `DecomposeTicket` with fields:
   - Summary string
   - StoryPoints int
   - Type string
   - IsExisting bool
   - Key string (only for existing tickets, empty for new)

2. Create a struct `DecompositionPlan` with fields:
   - NewTickets []DecomposeTicket
   - ExistingTickets []DecomposeTicket

3. Create function `ParseDecompositionPlan(plan string) (*DecompositionPlan, error)` that:
   - Parses the structured text format:
     ```markdown
     # DECOMPOSITION PLAN
     
     ## NEW TICKETS
     - [ ] Task summary (3 points)
     - [ ] Another task (5 points)
     
     ## EXISTING TICKETS (for reference)
     - [x] Existing task (5 points) [EXISTING]
     ```
   - Extracts story points from format: "(N points)" or "(N point)"
   - Identifies existing tickets by "[EXISTING]" marker or checked checkbox with [EXISTING]
   - Handles various formats gracefully
   - Returns parsed plan or error

4. Create helper functions:
   - `parseStoryPoints(text string) (int, error)` - extracts number from "(N points)"
   - `isExistingTicket(line string) bool` - checks for [EXISTING] marker

5. Add comprehensive tests in pkg/parser/decompose_test.go:
   - Test parsing new tickets
   - Test parsing existing tickets
   - Test story point extraction
   - Test malformed input handling
   - Test edge cases (no points, invalid format, etc.)

6. Handle errors gracefully with clear error messages

Follow existing parser patterns in pkg/parser/parser.go.
```

## Prompt 7: Create Gemini Integration for Decomposition

```text
Create a new file pkg/gemini/decompose.go with Gemini AI integration:

1. Create function `GenerateDecompositionPlan(client GeminiClient, cfg *config.Config, parentSummary, parentDescription string, existingChildren []jira.ChildTicketInfo, childType string, maxPoints int) (string, error)` that:
   - Builds context string with:
     - Parent ticket summary
     - Parent ticket description
     - List of existing children (formatted nicely)
     - Child ticket type
     - Maximum story points per ticket
   - Uses prompt template from cfg.DecomposePromptTemplate (or default)
   - Replaces placeholders: {{parent_summary}}, {{parent_description}}, {{existing_children}}, {{child_type}}, {{max_points}}
   - Calls Gemini API using existing client patterns
   - Returns the generated plan text

2. Create helper function `formatExistingChildren(children []jira.ChildTicketInfo) string` that:
   - Formats existing children as a readable list
   - Shows: "- <summary> (<points> points) - <type> [EXISTING]"

3. Create helper function `buildDecomposeContext(parentSummary, parentDescription, existingChildrenText, childType string, maxPoints int) string` that:
   - Combines all context into a single string
   - Formats it nicely for the AI

4. Add tests in pkg/gemini/decompose_test.go with mock client:
   - Test context building
   - Test prompt template replacement
   - Test with various inputs

5. Follow existing patterns from pkg/gemini/client.go and other Gemini integrations

Ensure error handling is robust and follows existing patterns.
```

## Prompt 8: Integrate Plan Generation

```text
In cmd/decompose.go, integrate the plan generation:

1. Update `runDecompose` to:
   - Fetch existing child tickets using GetChildTicketsDetailed
   - Get parent ticket details (summary, description)
   - Get child ticket type using GetChildTicketType
   - Get max story points using getMaxStoryPoints
   - Create Gemini client
   - Call GenerateDecompositionPlan
   - Parse the result using ParseDecompositionPlan
   - For now, just print the parsed plan and return

2. Add error handling for:
   - Failed child ticket fetching (log warning, continue with empty list)
   - Failed plan generation (show error, exit)
   - Failed parsing (show error with plan text, exit)

3. Store the parsed plan in a variable for later use

4. Test the full flow manually:
   - Run: `jira decompose ENG-123 --max-points 5`
   - Verify plan is generated and parsed correctly

Follow existing error handling patterns in the codebase.
```

## Prompt 9: Implement Duplicate Detection

```text
In cmd/decompose.go, implement duplicate detection:

1. Create function `detectAndFilterDuplicates(newTickets []parser.DecomposeTicket, existingTickets []jira.ChildTicketInfo) ([]parser.DecomposeTicket, []string)` that:
   - Compares each new ticket summary with existing ticket summaries
   - Uses case-insensitive comparison
   - Also does fuzzy matching (strings.Contains for partial matches)
   - Returns filtered new tickets (duplicates removed) and list of duplicate warnings

2. Create helper function `isDuplicate(newSummary string, existingSummaries []string) (bool, string)` that:
   - Checks for exact match (case-insensitive)
   - Checks for substring match
   - Returns true if duplicate found, along with matching existing summary

3. Update `runDecompose` to:
   - Call detectAndFilterDuplicates after parsing plan
   - Display warnings for duplicates: "Skipping \"<summary>\" - already exists as <key>"
   - Use filtered tickets for rest of flow

4. Add tests in cmd/decompose_test.go:
   - Test exact duplicate detection
   - Test fuzzy duplicate detection
   - Test no duplicates case

5. Ensure duplicate detection is case-insensitive and handles various formats

Follow existing patterns for user messaging and warnings.
```

## Prompt 10: Create Preview Display

```text
In cmd/decompose.go, create a function to display the decomposition plan:

1. Create function `displayDecompositionPlan(plan *parser.DecompositionPlan, parentKey string, childType string) error` that:
   - Displays a nicely formatted preview:
     ```
     Decomposition Plan for <parent_key>:
     
     NEW TICKETS:
     [1] <summary> (<points> points) - <type>
     [2] <summary> (<points> points) - <type>
     
     EXISTING TICKETS:
     [x] <summary> (<points> points) - <type> [EXISTING]
     
     Summary:
     - New tickets: <count> (<total_points> total story points)
     - Existing tickets: <count> (<total_points> total story points)
     - Total: <count> tickets (<total_points> total story points)
     ```
   - Calculates summary statistics correctly
   - Formats numbers nicely

2. Create helper function `calculatePlanSummary(plan *parser.DecompositionPlan) (newCount, newPoints, existingCount, existingPoints int)` that:
   - Counts new tickets and sums their story points
   - Counts existing tickets and sums their story points
   - Returns all four values

3. Update `runDecompose` to call displayDecompositionPlan after parsing

4. Ensure output is clear and readable

5. Add tests for summary calculation

Follow existing display patterns in the codebase (like in cmd/status.go).
```

## Prompt 11: Implement Confirmation Flow

```text
In cmd/decompose.go, implement the confirmation flow:

1. Create function `confirmDecompositionPlan(reader *bufio.Reader, plan *parser.DecompositionPlan) (bool, error)` that:
   - Prompts: "Create these <count> tickets? [Y/n/e(dit)/s(how)] "
   - Handles responses:
     - 'Y' or 'y' or empty: return true
     - 'n' or 'N': return false
     - 'e' or 'E': trigger edit flow (return special value or handle separately)
     - 's' or 'S': show detailed breakdown again, then re-prompt
   - Returns (confirmed bool, shouldEdit bool, error)

2. Create function `saveRejectedPlan(plan *parser.DecompositionPlan, parentKey string) error` that:
   - Creates directory: ~/.jira-tool/decompose-rejections/ (or configDir/decompose-rejections/)
   - Saves plan to file: <parent_key>-<timestamp>.md
   - Includes full plan, parent ticket info, timestamp
   - Returns error if file creation fails

3. Update `runDecompose` to:
   - Call confirmDecompositionPlan
   - If rejected, call saveRejectedPlan and exit
   - If edit requested, handle separately (next prompt)
   - If confirmed, proceed to creation (later prompt)

4. Add error handling for file operations

5. Format the saved file nicely with Markdown

Follow existing patterns for user confirmation and file operations.
```

## Prompt 12: Implement Editor Integration

```text
In cmd/decompose.go, implement editor integration for plan editing:

1. Create function `formatPlanForEditing(plan *parser.DecompositionPlan) string` that:
   - Formats plan as structured Markdown:
     ```markdown
     # DECOMPOSITION PLAN
     
     ## NEW TICKETS
     - [ ] Task summary (3 points)
     - [ ] Another task (5 points)
     
     ## EXISTING TICKETS (read-only)
     - [x] Existing task (5 points) [EXISTING]
     ```
   - Includes instructions at the top
   - Marks existing tickets clearly as read-only

2. Create function `editDecompositionPlan(reader *bufio.Reader, plan *parser.DecompositionPlan) (*parser.DecompositionPlan, error)` that:
   - Formats plan using formatPlanForEditing
   - Opens in editor using pkg/editor.OpenInEditor
   - Parses edited plan using ParseDecompositionPlan
   - Validates edited plan:
     - All new tickets have valid story points
     - Story points don't exceed limit
     - Format is valid
   - Returns parsed plan or error

3. Create function `validateEditedPlan(plan *parser.DecompositionPlan, maxPoints int) error` that:
   - Checks all new tickets have story points > 0
   - Checks all new tickets have story points <= maxPoints
   - Checks all tickets have non-empty summaries
   - Returns descriptive errors

4. Update `runDecompose` to:
   - If edit requested in confirmation, call editDecompositionPlan
   - Re-display plan after editing
   - Re-prompt for confirmation
   - Handle validation errors gracefully

5. Add tests for:
   - Plan formatting
   - Plan validation
   - Editor integration (mock editor)

Follow existing editor patterns from cmd/accept.go.
```

## Prompt 13: Create Child Tickets

```text
In cmd/decompose.go, implement child ticket creation:

1. Create function `createChildTickets(client jira.JiraClient, cfg *config.Config, plan *parser.DecompositionPlan, parentKey string, parentIsEpic bool, childType string, configDir string) ([]string, error)` that:
   - For each new ticket in plan.NewTickets:
     - Create ticket using appropriate method:
       - If parent is Epic: use CreateTicketWithEpicLink
       - Otherwise: use CreateTicketWithParent
     - Set story points using UpdateTicketStoryPoints
     - Track created ticket keys
   - Returns list of created ticket keys
   - Handles creation errors gracefully:
     - If one fails, log error but continue with others
     - Return list of successfully created keys and any errors

2. Create helper function `createSingleChildTicket(client jira.JiraClient, cfg *config.Config, ticket parser.DecomposeTicket, parentKey string, parentIsEpic bool, childType string, configDir string) (string, error)` that:
   - Creates one ticket with proper linking
   - Sets story points
   - Returns ticket key or error

3. Update `runDecompose` to:
   - After confirmation, call createChildTickets
   - Display progress: "Creating ticket 1 of N..."
   - Show created ticket keys as they're created
   - Handle partial failures gracefully

4. Add error handling:
   - If all tickets fail: show error and exit
   - If some tickets fail: show which succeeded and which failed
   - Allow user to retry failed tickets (optional enhancement)

5. Add tests with mock Jira client:
   - Test successful creation
   - Test partial failures
   - Test epic vs parent linking

Follow existing ticket creation patterns from cmd/accept.go and cmd/create.go.
```

## Prompt 14: Update Parent Story Points

```text
In cmd/decompose.go, implement parent story points update:

1. Create function `updateParentStoryPoints(client jira.JiraClient, cfg *config.Config, parentKey string, plan *parser.DecompositionPlan, existingChildren []jira.ChildTicketInfo) error` that:
   - Calculates total story points:
     - Sum all new ticket story points
     - Sum all existing child ticket story points
     - Total = new + existing
   - Updates parent ticket's story points using UpdateTicketStoryPoints
   - Returns error if update fails

2. Create helper function `calculateTotalStoryPoints(plan *parser.DecompositionPlan, existingChildren []jira.ChildTicketInfo) int` that:
   - Sums story points from new tickets
   - Sums story points from existing children
   - Returns total

3. Update `runDecompose` to:
   - After creating all tickets, call updateParentStoryPoints
   - Display message: "Updated parent <key> story points to <total> (was <old>)"
   - Handle update failure gracefully (show warning but don't fail entire operation)

4. Add error handling:
   - If story points field is not configured: show warning and skip
   - If update fails: show warning with error details
   - Log the old value before updating

5. Add tests:
   - Test calculation logic
   - Test update with mock client
   - Test error handling

Follow existing story point update patterns from cmd/estimate.go.
```

## Prompt 15: Display Creation Summary

```text
In cmd/decompose.go, create a function to display creation summary:

1. Create function `displayCreationSummary(createdKeys []string, plan *parser.DecompositionPlan, parentKey string, oldStoryPoints, newStoryPoints int) error` that:
   - Displays nicely formatted summary:
     ```
     Created tickets:
     - <key>: <summary> (<points> points)
     - <key>: <summary> (<points> points)
     
     Updated parent <parent_key> story points to <new> (was <old>)
     ```
   - Matches created keys with plan tickets (by order)
   - Shows story points for each created ticket
   - Shows parent update status

2. Create helper function `matchCreatedTickets(keys []string, plan *parser.DecompositionPlan) []struct{Key, Summary string, Points int}` that:
   - Matches created keys with plan tickets
   - Returns slice of matched tickets with key, summary, points

3. Update `runDecompose` to:
   - After all creation and updates, call displayCreationSummary
   - Show final success message

4. Ensure output is clear and celebratory

5. Add tests for matching logic

Follow existing summary display patterns in the codebase.
```

## Prompt 16: Complete Integration and Error Handling

```text
In cmd/decompose.go, complete the full integration:

1. Wire up the complete flow in `runDecompose`:
   - Load config and create clients
   - Validate ticket exists
   - Get story point limit
   - Fetch existing children
   - Determine child ticket type
   - Generate plan with Gemini
   - Parse plan
   - Detect and filter duplicates
   - Display plan
   - Confirm with user
   - Handle edit if requested
   - Create tickets
   - Update parent story points
   - Display summary

2. Add comprehensive error handling:
   - Ticket not found: clear error message
   - Ticket not accessible: clear error message
   - No children possible: informative message
   - AI generation fails: show error with retry option
   - Partial creation failures: show what succeeded/failed
   - Invalid story point limit: re-prompt
   - Type mapping issues: handle gracefully

3. Add user-friendly messages throughout:
   - Progress indicators
   - Clear prompts
   - Helpful error messages
   - Success messages

4. Test the complete flow:
   - Decompose Epic → Stories
   - Decompose Story → Tasks
   - Decompose with existing children
   - Decompose without existing children
   - Test editing flow
   - Test rejection
   - Test error cases

5. Ensure all code follows existing patterns and passes linting

6. Update README.md with decompose command documentation

The command should be fully functional and production-ready.
```

## Prompt 17: Add Tests and Documentation

```text
Complete testing and documentation:

1. Write comprehensive unit tests:
   - All helper functions in cmd/decompose.go
   - Parser functions in pkg/parser/decompose.go
   - Type mapping in pkg/jira/type_mapping.go
   - Gemini integration in pkg/gemini/decompose.go
   - Child ticket fetching in pkg/jira/epic.go

2. Write integration tests:
   - Full decompose flow with mock clients
   - Test with various ticket types
   - Test with existing children
   - Test without existing children
   - Test editing flow
   - Test error cases

3. Update documentation:
   - Add decompose command to README.md
   - Add examples and use cases
   - Document configuration options
   - Document file format for rejections

4. Add code comments:
   - Document all public functions
   - Add inline comments for complex logic
   - Document data structures

5. Run full test suite:
   - `go test ./...`
   - `make lint`
   - `make build`

6. Manual testing checklist:
   - [ ] Decompose Epic → Stories
   - [ ] Decompose Story → Tasks  
   - [ ] Decompose Task → Sub-tasks
   - [ ] With existing children
   - [ ] Without existing children
   - [ ] Edit plan
   - [ ] Reject and save
   - [ ] Story point updates
   - [ ] Duplicate detection
   - [ ] Error handling

Ensure all tests pass and documentation is complete.
```

