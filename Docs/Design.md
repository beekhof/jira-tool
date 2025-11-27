Here is a comprehensive, developer-ready specification for the `jira-tool` tool, compiling all requirements from our brainstorming session and adding necessary architecture, data, error, and testing plans.

-----

## 1\. 📖 Overview

The `jira-tool` is a command-line interface (CLI) tool built in Go. Its primary purpose is to streamline the Jira workflow for engineers and product managers by integrating directly with the Jira and Gemini APIs.

The tool will reduce context-switching and automate repetitive tasks, including:

  * Rapidly creating and enriching tickets.
  * Systematically reviewing and triaging existing tickets.
  * Intelligently decomposing research into-work-ready epics.
  * Tracking sprint and release velocity.

## 2\. 🏗️ Core Architecture & Design

The application will be a single, standalone Go binary.

  * **Go CLI Framework:** `cobra` is recommended for robust command/subcommand and flag management.
  * **API Clients:**
      * **Jira:** A stable Jira client library (e.g., `go-jira`) will be used to handle all interactions with the Jira REST API (authentication, ticket creation, searching, updating).
      * **Gemini:** The official Google AI Go SDK (`google.golang.org/api/aiplatform` or similar) will be used for all generative AI requests.
  * **System Dependencies:**
      * **Editor:** The tool must respect the user's default system editor (via the `$EDITOR` environment variable) for all "edit" prompts.
      * **Local File System:** The tool will manage its configuration and cache in the user's home directory.

## 3\. 🗄️ Data & State Management

State is managed locally through two primary files stored in a dedicated directory (e.g., `~/.jira-tool/`).

### 3.1. Configuration File (`config.yaml`)

A plain-text YAML file for all user-specific settings.

  * **Location:** `~/.jira-tool/config.yaml`
  * **Purpose:** Stores API keys, Jira instance details, and user preferences.
  * **Example Structure:**
    ```yaml
    # ~/.jira-tool/config.yaml

    # Jira Connection
    jira_url: "https://your-company.atlassian.net"
    jira_user: "your.email@company.com"
    # User should be prompted to store an API token securely, not in plain text.
    # On first run, the tool should prompt for this and store it in the system keyring.

    # Gemini API Key
    # On first run, the tool should prompt for this and store it in the system keyring.

    # Default Jira Fields
    default_project: "ENG"
    default_task_type: "Task"

    # Favorite Lists (for quick selection)
    favorite_assignees:
      - "Alex (alex.jones@company.com)"
      - "Sam (sam.smith@company.com)"

    favorite_sprints:
      - "Current Sprint" # Tool should map this to the active sprint
      - "Next Sprint"    # Tool should map this to the next planned sprint

    favorite_releases:
      - "v1.2.0"
      - "v1.3.0"

    # Point estimation values
    # If empty, defaults to [1, 2, 3, 5, 8, 13]
    story_point_options: [1, 2, 3, 5, 8, 13]
    ```

### 3.2. Cache File (`cache.json`)

A JSON file for storing non-critical, time-consuming API responses.

  * **Location:** `~/.jira-tool/cache.json`
  * **Purpose:** To improve performance by caching lists of users, priorities, sprints, and fix versions.
  * **Mechanism:**
      * Before making a list-based API call (e.g., "get all users"), the tool checks the cache.
      * If the data is in the cache, it's used.
      * If not, the tool hits the Jira API and then writes the results to the cache.
  * **Staleness:** A dedicated command, `jira refresh --cache`, will be provided to wipe this file, forcing the tool to fetch fresh data.

## 4\. ⚙️ Functional Requirements (Commands)

### 4.1. Command: `jira create`

  * **Description:** Creates a new ticket and (optionally) generates its description using AI.
  * **Usage:** `jira create [flags] "My one-line summary"`
  * **Flags:**
      * `-p, --project <PROJECT_KEY>`: Overrides the `default_project`.
      * `-t, --type <TICKET_TYPE>`: Overrides the `default_task_type`.
  * **Workflow:**
    1.  User runs the command.
    2.  The tool **immediately creates** the Jira ticket with the summary.
    3.  Prints: `Ticket ENG-123 created.`
    4.  Asks: `Would you like to use Gemini to generate the description? [y/N]`
    5.  If 'y', the tool initiates the **Gemini Q\&A Flow (see 5.1)**.
    6.  After the Q\&A, Gemini generates a description.
    7.  The tool prints the description and asks: `Update ticket with this description? [Y/n/e(dit)]`
    8.  On 'y', it updates the ticket. On 'e', it opens the description in the system editor (see 5.2), using the saved file as the new description.

### 4.2. Command: `jira review`

  * **Description:** Iterates through a queue of tickets needing review.
  * **Usage:**
      * `jira review`: Default. Fetches all tickets matching any criteria (new, untriaged, unassigned).
      * `jira review --needs-detail`, `--unassigned`, `--untriaged`: Filters for specific queues.
      * `jira review ENG-123`: Reviews a single ticket.
  * **Workflow:**
    1.  The tool fetches tickets and loops through them one by one.
    2.  For each ticket, it prints a compact line:
        `ENG-456: "Fix login bug" (Priority: None, Assignee: None) -> Action? [a(ssign), t(riage), d(etail), e(stimate), s(kip), q(uit)]`
    3.  User selects an action:
          * **[a]ssign:**
            1.  Presents the `favorite_assignees` list from config (e.g., `[1] Alex, [2] Sam, [3] Other...`).
            2.  If 'Other', prompts ` Search for user:  `, queries Jira, and presents a list of results.
          * **[t]riage (Priority):**
            1.  Fetches/uses cached list of priorities for the project (e.g., `[1] High, [2] Medium, [3] Low`).
            2.  User picks a number to set the priority.
          * **[d]etail (Add Detail):**
            1.  Initiates the **Gemini Q\&A Flow (5.1)**.
            2.  Follows the same `[Y/n/e(dit)]` confirmation as `jira create`.
          * **[e]stimate (Story Points):**
            1.  Presents the `story_point_options` from config, plus 'Other' (e.g., `[1] 1, [2] 2, [3] 3, [4] 5, [5] 8, [6] 13, [7] Other...`).
            2.  If 'Other', prompts ` Enter story points:  ` for a direct number input.
          * **[s]kip:** Moves to the next ticket.
          * **[q]uit:** Exits the review session.

### 4.3. Command: `jira accept`

  * **Description:** Converts a "Done" research ticket into a new Epic and decomposed sub-tasks.
  * **Usage:** `jira accept ENG-123`
  * **Workflow:**
    1.  The tool transitions `ENG-123` to "Done" status.
    2.  It scans `ENG-123` and presents a list of all possible research sources (description, all attachments, all comments) for the user to choose from.
    3.  Prompts: ` New Epic Summary:  `
    4.  Sends the summary + selected research text to Gemini.
    5.  Initiates the **Gemini Q\&A Flow (5.1)** to clarify the epic.
    6.  Gemini returns a full plan (Epic summary, description, list of task summaries).
    7.  The tool prints this plan to the terminal, asking: `Create this Epic and all sub-tasks? [Y/n/e(dit)]`
    8.  If 'e', the plan is opened in the system editor (see 5.2) as a **Markdown** file for editing. The tool must be able to parse this file back.
        ```markdown
        # EPIC: Implement new auth system

        The description of the epic...

        ## TASKS
        - [ ] Task 1 summary
        - [ ] Task 2 summary
        ```
    9.  On confirmation ('y' or save/close editor), the tool creates the Epic and all sub-tasks in Jira.
    10. **Post-Creation:**
          * Asks: `Add this Epic and its tasks to an active Sprint? [y/N]`. If 'y', shows the 'favorite sprints' list (with an 'Other' option to list all active sprints).
          * Asks: `Add this Epic and its tasks to a Release/Fix Version? [y/N]`. If 'y', shows the 'favorite releases' list (with an 'Other' option to list all unreleased versions).

### 4.4. Command: `jira status`

  * **Description:** Displays a progress report for a sprint or release.
  * **Usage:**
      * `jira status sprint` (Current)
      * `jira status sprint --next`
      * `jira status release` (Current)
      * `jira status release --next`
  * **Logic:**
      * `sprint`: Finds the "active" sprint with the nearest `endDate`.
      * `sprint --next`: Finds the "planned" sprint with the earliest `startDate`.
      * `release`: Finds the "unreleased" version with the nearest `releaseDate`.
      * `release --next`: Finds the "unreleased" version with the second-nearest `releaseDate`.
  * **Output:** A detailed text-based report:
    ```
    Sprint: Sprint A (ends in 3 days)
    Progress: [#######----] 60% (30/50 points)
    On Track: Yes (ahead of ideal burndown)
    ---
    To Do:       10 points
    In Progress: 10 points
    Done:        30 points
    ```

### 4.5. Command: `jira estimate`

  * **Description:** A standalone command to quickly estimate a ticket.
  * **Usage:** `jira estimate ENG-123`
  * **Workflow:**
    1.  Presents the exact same estimation prompt as `jira review [e]stimate`.
    2.  Updates the ticket and exits.

### 4.6. Command: `jira refresh`

  * **Description:** Manages the local cache.
  * **Usage:** `jira refresh --cache`
  * **Workflow:**
    1.  Deletes the `cache.json` file.
    2.  Prints `Cache cleared.`

## 5\. 🧩 Core Components (Reusable)

### 5.1. Gemini Q\&A Flow

This flow is a reusable module triggered by `jira create` and `jira accept`.

1.  **Context:** Receives initial context (a summary, or summary + research text).
2.  **Q\&A Loop:**
      * Sends context to Gemini, asks for a clarifying question.
      * Prompts user: ` Gemini asks: [Question from Gemini]? >  `
      * User types a single-line answer.
      * This loop repeats up to **4 times**. With each loop, the *entire* previous conversation history is sent to Gemini for context.
3.  **Generation:** After the loop, the tool asks Gemini for the final output (description or epic plan) based on the full conversation.

### 5.2. System Editor Flow

This flow is triggered by any `[e(dit)]` prompt.

1.  The tool generates the text to be edited (e.g., a description, or a Markdown epic plan).
2.  This text is written to a temporary file (e.g., `/tmp/jira-tool-edit.md`).
3.  The tool opens this file using the system editor (`$EDITOR`).
4.  The Go application **blocks** and waits for the editor process to terminate.
5.  Once the user saves and closes the editor, the tool reads the contents of the temporary file.
6.  The tool parses this new content to proceed.
7.  The temporary file is deleted.

## 6\. ⚠️ Error Handling & Resilience

The tool must be robust and provide clear, helpful error messages.

  * **Config Errors:**
      * `config.yaml` not found: On *any* command, if the config is not found, the tool should stop and instruct the user to run a `jira init` command (a new, required command to handle first-time setup, API key storage, and `config.yaml` creation).
      * Missing Fields: If `jira_url` or API keys are missing, the tool should provide a specific error and point to `jira init`.
  * **API Errors:**
      * **Jira (401/403):** "Authentication failed. Your Jira token may be invalid. Please run `jira init`."
      * **Jira (404):** "Error: Ticket ENG-123 not found."
      * **Jira (500+):** "Jira API returned a server error. Please try again."
      * **Gemini (401/403):** "Authentication failed. Your Gemini API key may be invalid. Please run `jira init`."
      * **Gemini (429):** "Gemini API rate limit exceeded. Please wait and try again."
  * **Network Errors:** "Error: Could not connect to Jira/Gemini. Please check your internet connection."
  * **Parsing Errors:** If the user saves an invalid format during the "edit" flow (e.g., broken Markdown), the tool must not crash. It should detect the error, print a message ("Error: Could not parse the plan."), and re-open the editor with the problematic file.

## 7\. 🧪 Testing Plan

A comprehensive testing strategy is required.

  * **Unit Tests:**
      * Mock all external API clients (Jira, Gemini) completely.
      * Test individual functions in isolation (e.g., config parsing, flag validation, `find_current_sprint` logic).
      * Test the Markdown parser for the `jira accept` edit flow.
      * Test the `jira status` burndown calculation logic.
  * **Integration Tests:**
      * Test the flow of commands using mocked APIs.
      * Use a library (e.g., `go-expect`) to simulate user input/output in the terminal.
      * **Example:** A test for `jira create` would:
        1.  Simulate the user typing `jira create "test"`.
        2.  Verify the `CreateTicket` mock was called with "test".
        3.  Simulate the user pressing 'y' for Gemini.
        4.  Verify the `Gemini` mock was called.
        5.  Simulate the user pressing 'y' to confirm.
        6.  Verify the `UpdateTicket` mock was called with the Gemini text.
  * **End-to-End (E2E) Tests:**
      * These tests should run against a **dedicated, non-production Jira test project**.
      * The test script will require a live Jira token and Gemini key (passed as environment variables).
      * The script will compile the Go binary.
      * It will execute the binary (e.g., `./jira-tool create "E2E Test Ticket"`) and then use the Jira API *itself* to verify that the ticket was *actually* created, updated, and (finally) deleted to clean up.