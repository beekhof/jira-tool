This is great. We've defined a robust and highly specific tool.

Here is a complete, step-by-step specification document based on all of our decisions. You can hand this directly to a developer to begin work.

-----

## 📄 Specification: Go Jira CLI Helper

### 1\. Overview

This tool is a command-line Go application designed to streamline Jira workflows. It helps users quickly create, review, and estimate tickets, convert research into epics, and track sprint/release progress. It leverages a config file for defaults and caches Jira data (users, sprints, etc.) for performance. It also uses the Gemini API to intelligently generate content and run interactive Q\&A sessions.

### 2\. General Requirements

  * **Configuration:**
      * A default config file (e.g., `~/.jira-tool/config.yaml`) will store Jira connection details, default project (`default_project`), default task type (`default_task_type`), and "favorite" lists for assignees, sprints, and releases.
  * **Caching:**
      * The tool **must cache** all lists queried from Jira (e.g., users, priorities, sprints, fix versions).
      * This cache should be stored locally (e.g., in `~/.jira-tool/cache.json`).
  * **Cache Busting:**
      * A command `jira refresh --cache` must be available to delete the cache and force the tool to fetch fresh data from Jira on its next run.
  * **Editor Integration:**
      * When the user is prompted to "edit" text (e.g., `[Y/n/e(dit)]`), the tool must open the content in the user's default system text editor (e.g., via the `$EDITOR` environment variable).
      * The tool will wait for the user to save and close the file, then use the saved content to proceed.

-----

### 3\. Command: `jira create`

  * **Usage:** `jira create [flags] "My one-line summary"`
  * **Flags:**
      * `-p, --project <PROJECT_KEY>`: Overrides the default project.
      * `-t, --type <TICKET_TYPE>`: Overrides the default ticket type (e.g., "Bug", "Story").
  * **Workflow:**
    1.  User runs the command (e.g., `jira create "My task summary"`).
    2.  The tool **immediately creates** the Jira ticket using the summary and the project/type (from defaults or flags).
    3.  The tool prints the new ticket ID (e.g., `Ticket ENG-123 created.`).
    4.  It then asks: `Would you like to use Gemini to generate the description? [y/N]`.
    5.  If 'y', the tool initiates the "Gemini Q\&A Flow" (see section 7).
    6.  After the Q\&A, Gemini generates a description.
    7.  The tool prints the description to the terminal and asks: `Update ticket with this description? [Y/n/e(dit)]`.
    8.  If 'y', the tool updates ENG-123's description.
    9.  If 'e', the tool opens the description in the system editor (see 2.3). After saving, the tool updates the ticket with the edited content.

-----

### 4\. Command: `jira review`

  * **Usage:**
      * `jira review`: Default. Fetches all tickets matching *any* criteria (new, untriaged, unassigned).
      * `jira review --needs-detail`: Fetches only tickets in a "new" state.
      * `jira review --unassigned`: Fetches only unassigned tickets.
      * `jira review --untriaged`: Fetches only tickets with no priority/severity.
      * `jira review ENG-123`: Reviews a single, specific ticket.
  * **Workflow:**
    1.  The tool fetches a list of tickets based on the command.
    2.  It loops through each ticket, presenting it in a **compact, single line:**
        `ENG-456: "Fix login bug" (Priority: None, Assignee: None) -> Action? [a(ssign), t(riage), d(etail), e(stimate), s(kip), q(uit)]`
    3.  The user selects an action:
          * **[a]ssign:**
            1.  Shows a "favorites" list from the config: `[1] Alex, [2] Sam, [3] Other...`
            2.  If '3' is chosen, it prompts: ` Search for user:  `.
            3.  The user searches, and the tool shows a numbered list of results to pick from.
          * **[t]riage (Priority):**
            1.  It shows the *entire* list of priorities cached for that project: `[1] High, [2] Medium, [3] Low`.
            2.  The user picks a number.
          * **[d]etail (Add Detail):**
            1.  The tool initiates the "Gemini Q\&A Flow" (see section 7).
            2.  The workflow proceeds identically to the `jira create` description generation (steps 6-9 in section 3).
          * **[e]stimate (Story Points):**
            1.  The tool shows a Fibonacci list: `[1] 1, [2] 2, [3] 3, [4] 5, [5] 8, [6] 13, [7] Other...`
            2.  If '7' is chosen, it prompts: ` Enter story points:  `.
            3.  The tool updates the ticket's story points field.
          * **[s]kip:** Moves to the next ticket.
          * **[q]uit:** Exits the review session.

-----

### 5\. Command: `jira accept`

  * **Usage:** `jira accept ENG-123`
  * **Description:** This command converts a completed research ticket (ENG-123) into a new Epic and decomposed sub-tasks.
  * **Workflow:**
    1.  The tool moves the source ticket (ENG-123) to a "Done" status.
    2.  It then scans ENG-123 for all possible content sources and asks the user to pick one:
        ```
        Where is the research?
        [1] Ticket Description
        [2] Attachment: research.md
        [3] Attachment: analysis.pdf
        [4] Comment #5 (by Alex on...)
        [5] ...
        ```
    3.  After the user selects a source, the tool prompts for the new Epic's summary: ` New Epic Summary:  `.
    4.  The tool sends the user's summary *and* the full research text to Gemini.
    5.  It initiates the "Gemini Q\&A Flow" (see section 7) to clarify the Epic's scope.
    6.  After the Q\&A, Gemini returns a full plan (Epic details + list of task summaries).
    7.  The tool prints this plan to the terminal and asks: `Create this Epic and all sub-tasks? [Y/n/e(dit)]`.
    8.  If 'e', the tool opens the *entire plan* in the system editor as a **Markdown** file for editing. The tool will parse the saved file to get the final plan.
        ```markdown
        # EPIC: Implement new auth system

        The description of the epic...

        ## TASKS
        - [ ] Task 1 summary
        - [ ] Task 2 summary
        ```
    9.  If 'y' (or after editing), the tool creates the new Epic and all sub-tasks in Jira.
    10. **Assign to Sprint:** The tool immediately asks: `Add this Epic and its tasks to an active Sprint? [y/N]`.
          * If 'y', it shows a config-based list: `[1] Current Sprint, [2] Next Sprint, [3] Other...`
          * If '3' is chosen, it fetches, lists, and caches all other *active* Sprints.
    11. **Assign to Release:** The tool then asks: `Add this Epic and its tasks to a Release/Fix Version? [y/N]`.
          * If 'y', it shows a config-based list: `[1] Current Release, [2] Next Release, [3] Other...`
          * If '3' is chosen, it fetches, lists, and caches all other *unreleased* Fix Versions.

-----

### 6\. Command: `jira status`

  * **Usage:**
      * `jira status sprint`
      * `jira status sprint --next`
      * `jira status release`
      * `jira status release --next`
  * **Logic:**
      * `sprint` (current): Finds the "active" sprint with the nearest `endDate`.
      * `sprint --next`: Finds the "planned" sprint with the earliest `startDate`.
      * `release` (current): Finds the "unreleased" version with the nearest `releaseDate`.
      * `release --next`: Finds the "unreleased" version with the second-nearest `releaseDate`.
  * **Output:** The tool displays a detailed progress report:
    ```
    Sprint: Sprint A (ends in 3 days)
    Progress: [#######----] 60% (30/50 points)
    On Track: Yes (ahead of ideal burndown)
    ---
    To Do:       10 points
    In Progress: 10 points
    Done:        30 points
    ```

-----

### 7\. Command: `jira estimate`

  * **Usage:** `jira estimate ENG-123`
  * **Workflow:**
    1.  This command triggers the *exact same* story point estimation flow as in the `review` command.
    2.  The tool shows a Fibonacci list: `[1] 1, [2] 2, [3] 3, [4] 5, [5] 8, [6] 13, [7] Other...`
    3.  If '7' is chosen, it prompts: ` Enter story points:  `.
    4.  The tool updates the ticket's story points and exits.

-----

### 8\. Core Component: Gemini Q\&A Flow

This is a reusable flow triggered by `jira create` and `jira accept`.

1.  **Initiation:** The tool has a summary (from `create`) or a summary + research text (from `accept`).
2.  **Q\&A Loop:** The tool sends this context to Gemini and asks for a clarifying question.
3.  The tool prompts the user with a simple, single-line prompt: ` Gemini asks: [Question from Gemini]? >  `
4.  The user provides their answer.
5.  The tool sends the *full history* (original summary + all previous Q\&As) back to Gemini for the *next* question.
6.  This loop repeats up to a **maximum of 4 times**.
7.  **Generation:** After the loop, the tool asks Gemini to generate the final output (a task description or an epic-with-tasks plan) based on the complete conversation.