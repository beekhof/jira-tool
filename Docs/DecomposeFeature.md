# Decompose Feature - Specification

## Overview
The `decompose` command takes an existing ticket and creates child tickets with story point estimates no larger than a specified value. The command works similarly to the `accept` command, but instead of creating a new Epic from a research ticket, it decomposes an existing ticket into smaller child tickets. Any existing child tickets are considered in the plan, and the user has an opportunity to confirm and edit the breakdown before tickets are created in Jira.

## Questions and Answers

### Q1: How should the story point limit be specified?
**A:** Command-line flag (e.g., `--max-points 5`) with prompt fallback, and a default from the config.

### Q2: How should the child ticket type be determined?
**A:** Use a hardcoded mapping (e.g., Epic → Story, Story → Task, Task → Sub-task). For unknown types, prompt the user to choose.

### Q3: How should existing child tickets be handled?
**A:** Include them in the breakdown display and indicate that they already exist. Skip creating duplicates. Fetch existing child tickets before planning.

### Q4: What should be shown in the confirmation/preview?
**A:** Show ticket summaries, story points, types, and which are new vs. existing. Allow editing. Save rejections to a file. Include a summary (total story points, new vs. existing).

### Q5: How should editing work?
**A:** Represent the tickets in a structured text format and allow the user to open in an editor. After editing, re-render the summary and prompt the user to confirm.

### Q6: What should happen after confirmation?
**A:** There should be a final confirmation step. Automatically link children to the parent and assign story points. Show a list of tickets and summaries after creation.

### Q7: What context should be provided to the AI?
**A:** Do not consider the parent's story point total when planning. Update it with the new total of the child ticket points. Use a configurable prompt template.

## Detailed Requirements

### 1. Command Structure
- **Command:** `jira decompose [TICKET_ID] [--max-points N]`
- **Usage:** `jira decompose ENG-123 --max-points 5`
- **Description:** Decompose an existing ticket into child tickets with story points no larger than the specified limit.

### 2. Story Point Limit
- **Flag:** `--max-points` (integer)
- **Config Field:** `default_max_decompose_points` (integer, optional)
- **Behavior:**
  - If flag is provided, use it
  - If flag is not provided, check config for default
  - If neither exists, prompt user: `Maximum story points per child ticket (default: 5): `
  - Validate that the limit is a positive integer

### 3. Child Ticket Type Mapping
- **Hardcoded Mapping:**
  - Epic → Story
  - Story → Task
  - Task → Sub-task
- **Unknown Types:**
  - If parent type doesn't have a mapping, prompt user:
    ```
    Parent ticket type "Feature" has no default child type mapping.
    What type should child tickets be? [Task/Story/Sub-task/Other]:
    ```
  - If "Other", prompt for custom type name
  - Cache the mapping in config for future use (optional enhancement)

### 4. Existing Child Tickets
- **Fetch Before Planning:**
  - Use `jira.GetChildTickets()` to fetch all existing child tickets
  - Include both subtasks (via `parent = TICKET_KEY`) and epic children (via Epic Link field)
  - Store existing children with their summaries, story points, and types
- **Display in Breakdown:**
  - Show existing children with indicator: `[EXISTING]`
  - Include their story points and types in the display
  - Do not include them in the AI planning prompt (they're already done)
- **Skip Duplicates:**
  - When generating plan, compare new ticket summaries with existing ones
  - If a new ticket matches an existing one (case-insensitive, fuzzy match), skip it
  - Show a warning: `Skipping "Task X" - already exists as ENG-456`

### 5. AI Prompt and Context
- **Context Provided:**
  - Parent ticket summary
  - Parent ticket description
  - List of existing child tickets (summaries only, marked as existing)
  - Story point limit per ticket
  - Child ticket type to use
- **Prompt Template:**
  - Configurable in `config.yaml` as `decompose_prompt_template`
  - Default template should instruct AI to:
    - Create child tickets with summaries and story points
    - Each child ticket must have story points ≤ limit
    - Consider existing child tickets and avoid duplicates
    - Do NOT consider parent's existing story points
    - Output in structured format (see below)
- **Output Format:**
  - Structured text format that can be parsed and edited:
    ```markdown
    # DECOMPOSITION PLAN
    
    ## NEW TICKETS
    - [ ] Task 1 summary (3 points)
    - [ ] Task 2 summary (5 points)
    - [ ] Task 3 summary (2 points)
    
    ## EXISTING TICKETS (for reference)
    - [x] Existing Task 1 (5 points) [EXISTING]
    - [x] Existing Task 2 (3 points) [EXISTING]
    ```

### 6. Confirmation and Preview
- **Display Format:**
  ```
  Decomposition Plan for ENG-123:
  
  NEW TICKETS:
  [1] Task 1 summary (3 points) - Task
  [2] Task 2 summary (5 points) - Task
  [3] Task 3 summary (2 points) - Task
  
  EXISTING TICKETS:
  [x] Existing Task 1 (5 points) - Task [EXISTING]
  [x] Existing Task 2 (3 points) - Task [EXISTING]
  
  Summary:
  - New tickets: 3 (10 total story points)
  - Existing tickets: 2 (8 total story points)
  - Total: 5 tickets (18 total story points)
  ```
- **User Options:**
  - `[Y]` - Confirm and create tickets
  - `[n]` - Reject and save to file
  - `[e]` - Edit plan in editor
  - `[s]` - Show detailed breakdown

### 7. Editing the Plan
- **Structured Text Format:**
  - Open plan in editor as Markdown file
  - Format:
    ```markdown
    # DECOMPOSITION PLAN
    
    ## NEW TICKETS
    - [ ] Task 1 summary (3 points)
    - [ ] Task 2 summary (5 points)
    - [ ] Task 3 summary (2 points)
    
    ## EXISTING TICKETS (read-only)
    - [x] Existing Task 1 (5 points) [EXISTING]
    - [x] Existing Task 2 (3 points) [EXISTING]
    ```
- **Editing Operations:**
  - User can add/remove tickets
  - User can modify summaries
  - User can adjust story points (must be ≤ limit)
  - Existing tickets are marked `[EXISTING]` and should not be editable
- **After Editing:**
  - Parse the edited file
  - Re-render the summary
  - Prompt user to confirm again
  - Validate story points are ≤ limit

### 8. Rejection Handling
- **Save to File:**
  - If user rejects (chooses 'n'), save the plan to a file
  - File path: `~/.jira-tool/decompose-rejections/TICKET_KEY-YYYYMMDD-HHMMSS.md`
  - Include full plan, parent ticket info, and timestamp
  - Create directory if it doesn't exist

### 9. Ticket Creation
- **Final Confirmation:**
  - After editing (if applicable), show final summary
  - Prompt: `Create these X tickets? [Y/n]`
- **Creation Process:**
  - For each new ticket in the plan:
    - Create ticket with parent link (or Epic Link if parent is Epic)
    - Set story points
    - Set ticket type (from mapping)
    - Link to parent appropriately
  - Update parent ticket's story points to sum of all children (new + existing)
- **Post-Creation:**
  - Display created tickets:
    ```
    Created tickets:
    - ENG-456: Task 1 summary (3 points)
    - ENG-457: Task 2 summary (5 points)
    - ENG-458: Task 3 summary (2 points)
    
    Updated parent ENG-123 story points to 18 (was 10)
    ```

### 10. Error Handling
- **Invalid Ticket:**
  - If ticket doesn't exist: show error and exit
  - If ticket is not accessible: show error and exit
- **Invalid Story Point Limit:**
  - If limit is ≤ 0: show error and prompt again
  - If limit is not a number: show error and prompt again
- **Type Mapping Issues:**
  - If parent type is unknown: prompt user to choose
  - If chosen type doesn't exist in Jira: show error and allow retry
- **Creation Failures:**
  - If any ticket creation fails: show error, list successfully created tickets, allow user to retry failed ones
  - If parent story point update fails: show warning but don't fail entire operation

## Configuration Changes

### config.yaml
```yaml
# Default maximum story points per child ticket when decomposing
default_max_decompose_points: 5

# Prompt template for decomposition planning
decompose_prompt_template: |
  You are helping to decompose a Jira ticket into smaller child tickets.
  
  Parent Ticket: {{parent_summary}}
  Description: {{parent_description}}
  
  Existing Child Tickets:
  {{existing_children}}
  
  Requirements:
  - Create child tickets with type: {{child_type}}
  - Each child ticket must have story points ≤ {{max_points}}
  - Avoid duplicating existing child tickets
  - Do not consider the parent's current story points
  
  Output a decomposition plan in the following format:
  
  # DECOMPOSITION PLAN
  
  ## NEW TICKETS
  - [ ] Ticket summary (story points)
  - [ ] Another ticket summary (story points)
  
  ## EXISTING TICKETS (for reference)
  - [x] Existing ticket (points) [EXISTING]
```

## Technical Implementation Notes

### 1. Parser for Decomposition Plan
- Create `pkg/parser/decompose.go` with:
  - `ParseDecompositionPlan(plan string) ([]DecomposeTicket, error)`
  - `DecomposeTicket` struct with: Summary, StoryPoints, Type, IsExisting
  - Handle both new and existing tickets in the format

### 2. Child Ticket Fetching
- Use existing `jira.GetChildTickets()` but enhance to return full ticket info (not just summaries)
- Need to fetch story points for existing children
- Need to fetch issue types for existing children

### 3. Story Point Updates
- After creating all children, calculate sum of all children (new + existing)
- Update parent ticket's story points using `client.UpdateTicketStoryPoints()`
- Handle case where story points field is not configured

### 4. Duplicate Detection
- Compare new ticket summaries with existing ones
- Use fuzzy matching (case-insensitive, ignore punctuation)
- Show warnings for potential duplicates

### 5. Editor Integration
- Use existing `pkg/editor` package
- Parse edited plan and validate format
- Re-parse and re-display summary after editing

## Testing Plan

1. **Unit Tests:**
   - Parser for decomposition plan format
   - Duplicate detection logic
   - Story point limit validation
   - Type mapping logic

2. **Integration Tests:**
   - Full decompose flow with mock Jira client
   - Test with existing child tickets
   - Test with no existing child tickets
   - Test editing flow

3. **Manual Testing:**
   - Decompose an Epic into Stories
   - Decompose a Story into Tasks
   - Decompose with existing children
   - Test rejection and file saving
   - Test editor editing flow
   - Test story point updates

