# Decompose Feature - Implementation Blueprint

## Overview
This blueprint provides a detailed, step-by-step plan for implementing the `decompose` command. The implementation is broken down into small, iterative chunks that build on each other, ensuring strong testing at each stage.

## Architecture Overview

### Components
1. **Command (`cmd/decompose.go`)**: Main command entry point and orchestration
2. **Parser (`pkg/parser/decompose.go`)**: Parse decomposition plan format
3. **Gemini Integration (`pkg/gemini/decompose.go`)**: Generate decomposition plan using AI
4. **Jira Client Extensions**: Enhance existing client to fetch child tickets with full details
5. **Config Extensions**: Add decompose-related configuration fields

### Data Structures
```go
type DecomposeTicket struct {
    Summary     string
    StoryPoints int
    Type        string
    IsExisting  bool
    Key         string // Only for existing tickets
}

type DecompositionPlan struct {
    NewTickets     []DecomposeTicket
    ExistingTickets []DecomposeTicket
    ParentKey      string
    ChildType      string
}
```

## Implementation Steps

### Phase 1: Foundation and Configuration

#### Step 1.1: Add Configuration Fields
**Goal:** Extend config to support decompose command

**Tasks:**
1. Add `default_max_decompose_points` to `Config` struct in `pkg/config/config.go`
2. Add `decompose_prompt_template` to `Config` struct
3. Add default prompt template constant
4. Update `init` command to prompt for default max points (optional)
5. Write tests for config loading/saving

**Files:**
- `pkg/config/config.go`
- `cmd/init.go`
- `pkg/config/config_test.go`

**Acceptance Criteria:**
- Config can load/save new fields
- Default max points defaults to 5 if not set
- Prompt template has sensible default

#### Step 1.2: Create Command Skeleton
**Goal:** Create basic command structure

**Tasks:**
1. Create `cmd/decompose.go` with command definition
2. Add command to root command
3. Implement basic argument parsing (ticket ID)
4. Add `--max-points` flag
5. Implement basic validation (ticket exists, accessible)

**Files:**
- `cmd/decompose.go`
- `cmd/root.go`

**Acceptance Criteria:**
- Command can be invoked: `jira decompose ENG-123`
- Flag parsing works: `jira decompose ENG-123 --max-points 5`
- Validates ticket exists

### Phase 2: Child Ticket Fetching and Type Mapping

#### Step 2.1: Enhance Child Ticket Fetching
**Goal:** Fetch existing child tickets with full details

**Tasks:**
1. Create `GetChildTicketsDetailed()` in `pkg/jira/epic.go`
2. Return full ticket info (key, summary, story points, type)
3. Handle both subtasks and epic children
4. Write tests for fetching logic

**Files:**
- `pkg/jira/epic.go`
- `pkg/jira/epic_test.go`

**Acceptance Criteria:**
- Can fetch all child tickets with full details
- Handles both parent links and epic links
- Returns story points and types correctly

#### Step 2.2: Implement Type Mapping
**Goal:** Map parent ticket types to child ticket types

**Tasks:**
1. Create `GetChildTicketType()` function in `pkg/jira/client.go` or new file
2. Implement hardcoded mapping (Epic→Story, Story→Task, Task→Sub-task)
3. Add interactive prompt for unknown types
4. Cache user choices in config (optional enhancement)
5. Write tests for mapping logic

**Files:**
- `pkg/jira/type_mapping.go` (new)
- `pkg/jira/type_mapping_test.go` (new)

**Acceptance Criteria:**
- Returns correct child type for known parent types
- Prompts user for unknown types
- Handles user input validation

### Phase 3: Story Point Limit Handling

#### Step 3.1: Implement Story Point Limit Logic
**Goal:** Handle story point limit from flag, config, or prompt

**Tasks:**
1. Create `getMaxStoryPoints()` helper function
2. Check flag first, then config, then prompt
3. Validate limit is positive integer
4. Write tests for limit resolution

**Files:**
- `cmd/decompose.go`

**Acceptance Criteria:**
- Flag takes precedence
- Falls back to config
- Prompts if neither available
- Validates input correctly

### Phase 4: AI Plan Generation

#### Step 4.1: Create Decomposition Plan Parser
**Goal:** Parse structured decomposition plan format

**Tasks:**
1. Create `pkg/parser/decompose.go`
2. Implement `ParseDecompositionPlan()` function
3. Parse both new and existing tickets
4. Extract story points from format: `(N points)`
5. Handle existing ticket markers: `[EXISTING]`
6. Write comprehensive tests

**Files:**
- `pkg/parser/decompose.go` (new)
- `pkg/parser/decompose_test.go` (new)

**Acceptance Criteria:**
- Parses new tickets correctly
- Parses existing tickets correctly
- Extracts story points
- Handles malformed input gracefully

#### Step 4.2: Create Gemini Integration
**Goal:** Generate decomposition plan using AI

**Tasks:**
1. Create `pkg/gemini/decompose.go`
2. Implement `GenerateDecompositionPlan()` function
3. Build context from parent ticket and existing children
4. Use configurable prompt template
5. Call Gemini API and parse response
6. Write tests with mock Gemini client

**Files:**
- `pkg/gemini/decompose.go` (new)
- `pkg/gemini/decompose_test.go` (new)

**Acceptance Criteria:**
- Generates valid plan format
- Respects story point limit
- Considers existing children
- Uses configurable prompt

#### Step 4.3: Integrate Plan Generation
**Goal:** Wire up plan generation in decompose command

**Tasks:**
1. Fetch existing child tickets
2. Determine child ticket type
3. Get story point limit
4. Generate plan using Gemini
5. Parse and validate plan
6. Display plan to user

**Files:**
- `cmd/decompose.go`

**Acceptance Criteria:**
- Full flow works end-to-end
- Plan is displayed correctly
- Handles errors gracefully

### Phase 5: Duplicate Detection

#### Step 5.1: Implement Duplicate Detection
**Goal:** Detect and skip duplicate tickets

**Tasks:**
1. Create `detectDuplicates()` function
2. Compare new ticket summaries with existing ones
3. Use fuzzy matching (case-insensitive)
4. Show warnings for duplicates
5. Filter duplicates from plan
6. Write tests for duplicate detection

**Files:**
- `cmd/decompose.go` or `pkg/parser/decompose.go`

**Acceptance Criteria:**
- Detects exact matches
- Detects similar matches (fuzzy)
- Shows clear warnings
- Filters duplicates correctly

### Phase 6: Preview and Confirmation

#### Step 6.1: Create Preview Display
**Goal:** Display decomposition plan in readable format

**Tasks:**
1. Create `displayDecompositionPlan()` function
2. Show new vs. existing tickets clearly
3. Display story points and types
4. Calculate and show summary statistics
5. Format output nicely

**Files:**
- `cmd/decompose.go`

**Acceptance Criteria:**
- Clear visual distinction between new/existing
- Summary statistics are accurate
- Format is readable

#### Step 6.2: Implement Confirmation Flow
**Goal:** Allow user to confirm, reject, or edit

**Tasks:**
1. Create `confirmDecompositionPlan()` function
2. Prompt for Y/n/e/s options
3. Handle each option appropriately
4. Save rejections to file if rejected
5. Write tests for confirmation logic

**Files:**
- `cmd/decompose.go`

**Acceptance Criteria:**
- All options work correctly
- Rejections saved to file
- File path is correct

### Phase 7: Editing Support

#### Step 7.1: Implement Editor Integration
**Goal:** Allow editing plan in external editor

**Tasks:**
1. Create `editDecompositionPlan()` function
2. Format plan as structured text
3. Open in editor using `pkg/editor`
4. Parse edited plan
5. Validate edited plan
6. Re-display summary after editing
7. Write tests for editing flow

**Files:**
- `cmd/decompose.go`

**Acceptance Criteria:**
- Plan opens in editor correctly
- Edited plan parses correctly
- Validation catches errors
- Summary re-displays after edit

#### Step 7.2: Plan Formatting
**Goal:** Format plan for editing and display

**Tasks:**
1. Create `formatDecompositionPlan()` function
2. Convert plan to Markdown format
3. Mark existing tickets as read-only
4. Include instructions in editor
5. Write tests for formatting

**Files:**
- `cmd/decompose.go` or `pkg/parser/decompose.go`

**Acceptance Criteria:**
- Format is clear and editable
- Existing tickets marked correctly
- Instructions are helpful

### Phase 8: Ticket Creation

#### Step 8.1: Create Child Tickets
**Goal:** Create all new tickets in Jira

**Tasks:**
1. Create `createChildTickets()` function
2. For each new ticket:
   - Create ticket with correct type
   - Link to parent (parent link or epic link)
   - Set story points
3. Handle creation errors gracefully
4. Track successfully created tickets
5. Write tests with mock client

**Files:**
- `cmd/decompose.go`

**Acceptance Criteria:**
- All tickets created correctly
- Links established properly
- Story points set correctly
- Errors handled gracefully

#### Step 8.2: Update Parent Story Points
**Goal:** Update parent ticket's story points to sum of children

**Tasks:**
1. Create `updateParentStoryPoints()` function
2. Calculate sum of all children (new + existing)
3. Update parent ticket's story points
4. Handle update failures gracefully
5. Write tests

**Files:**
- `cmd/decompose.go`

**Acceptance Criteria:**
- Calculates sum correctly
- Updates parent ticket
- Handles failures gracefully

#### Step 8.3: Post-Creation Summary
**Goal:** Display summary of created tickets

**Tasks:**
1. Create `displayCreationSummary()` function
2. List all created tickets with keys
3. Show story points for each
4. Show parent update status
5. Format output nicely

**Files:**
- `cmd/decompose.go`

**Acceptance Criteria:**
- Summary is clear and complete
- All created tickets listed
- Parent update status shown

### Phase 9: Integration and Polish

#### Step 9.1: End-to-End Integration
**Goal:** Wire all components together

**Tasks:**
1. Complete main `runDecompose()` function
2. Integrate all helper functions
3. Add comprehensive error handling
4. Add user-friendly messages
5. Test full flow manually

**Files:**
- `cmd/decompose.go`

**Acceptance Criteria:**
- Full flow works end-to-end
- All error cases handled
- User experience is smooth

#### Step 9.2: Error Handling and Edge Cases
**Goal:** Handle all edge cases gracefully

**Tasks:**
1. Handle ticket not found
2. Handle ticket not accessible
3. Handle no children possible (already too small)
4. Handle AI generation failures
5. Handle partial creation failures
6. Handle invalid story point limits
7. Write tests for edge cases

**Files:**
- `cmd/decompose.go`
- Test files

**Acceptance Criteria:**
- All edge cases handled
- Error messages are helpful
- User can recover from errors

#### Step 9.3: Documentation and Testing
**Goal:** Complete documentation and comprehensive tests

**Tasks:**
1. Update README with decompose command
2. Add examples to documentation
3. Write integration tests
4. Write unit tests for all components
5. Test with various ticket types
6. Test with existing children
7. Test without existing children

**Files:**
- `README.md`
- All test files

**Acceptance Criteria:**
- Documentation is complete
- All tests pass
- Examples are clear

## Testing Strategy

### Unit Tests
- Config loading/saving
- Type mapping logic
- Parser for decomposition plan
- Duplicate detection
- Story point limit validation
- Plan formatting

### Integration Tests
- Full decompose flow with mock Jira client
- Full decompose flow with mock Gemini client
- Editor integration
- File saving for rejections

### Manual Testing Scenarios
1. Decompose Epic → Stories
2. Decompose Story → Tasks
3. Decompose Task → Sub-tasks
4. Decompose with existing children
5. Decompose without existing children
6. Test editing flow
7. Test rejection and file saving
8. Test story point updates
9. Test duplicate detection
10. Test error cases

## Dependencies

### Existing Components to Reuse
- `pkg/jira/client.go` - Jira API client
- `pkg/gemini/client.go` - Gemini API client
- `pkg/editor/editor.go` - Editor integration
- `pkg/config/` - Configuration management
- `pkg/parser/` - Parsing utilities (extend)

### New Components Needed
- `pkg/parser/decompose.go` - Decomposition plan parser
- `pkg/gemini/decompose.go` - Gemini integration for decompose
- `pkg/jira/type_mapping.go` - Type mapping logic
- `cmd/decompose.go` - Main command

## Risk Mitigation

### Potential Issues
1. **AI generates invalid format**: Add robust parsing with clear error messages
2. **Story points exceed limit**: Validate before creation, show warnings
3. **Parent type unknown**: Prompt user interactively
4. **Creation failures**: Track successes, allow retry of failures
5. **Editor parsing fails**: Validate format, show helpful errors

### Mitigation Strategies
- Comprehensive input validation
- Clear error messages
- Graceful degradation
- User-friendly prompts
- Extensive testing

## Success Criteria

The feature is complete when:
1. ✅ Command can decompose tickets of various types
2. ✅ Story point limit is respected
3. ✅ Existing children are considered and displayed
4. ✅ Duplicates are detected and skipped
5. ✅ Plan can be edited in external editor
6. ✅ Rejections are saved to file
7. ✅ Tickets are created with correct links and story points
8. ✅ Parent story points are updated correctly
9. ✅ All error cases are handled gracefully
10. ✅ Comprehensive tests pass
11. ✅ Documentation is complete

