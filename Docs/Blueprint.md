Here is a detailed, step-by-step blueprint for building the `jira-tool` project, followed by a series of iterative, test-driven prompts designed for a code-generation LLM.

-----

### **Project Blueprint & Iteration Plan**

The core philosophy is to build the application from a stable foundation outwards, ensuring each new piece of functionality is tested and integrated before moving to the next.

#### **Phase 1: The Foundation (Setup, Config & Init)**

  * **Goal:** Establish the CLI structure, configuration management, and the `init` command. This is the bedrock.
  * **Steps:**
    1.  **Root Command:** Set up the `cobra` root command.
    2.  **Config:** Define the `config.yaml` struct and helper functions (load/save).
    3.  **Keyring:** Implement a cross-platform keyring module (e.g., using `99designs/keyring`) to securely store API keys.
    4.  **`init` Command:** Create the `init` command to prompt the user for Jira/Gemini details and save them to the config file and keyring.
    5.  **Testing:** Test the `init` command by mocking user input (`stdin`) and the keyring to ensure the config file is written correctly.

#### **Phase 2: The First Read/Write (Jira Client & `estimate`)**

  * **Goal:** Implement the simplest end-to-end feature: `jira estimate`. This forces the creation of the Jira client and a simple, interactive update.
  * **Steps:**
    1.  **Jira Interface:** Define a `JiraClient` interface in `pkg/jira/client.go`. This is *critical* for testing.
    2.  **Jira Implementation:** Create a concrete `jiraClient` that reads from the config/keyring to authenticate.
    3.  **`estimate` Command:** Implement the `cmd/estimate.go` command.
    4.  **Client Method:** Implement the `UpdateTicketPoints` method on the `jiraClient`.
    5.  **Prompt:** Implement the Fibonacci prompt logic.
    6.  **Testing:**
          * `cmd/estimate_test.go`: Test the command by mocking the `JiraClient` interface and user input.
          * `pkg/jira/client_test.go`: Test the `UpdateTicketPoints` method by mocking the underlying `http.Client`.

#### **Phase 3: The First Read-Only Command (`status`)**

  * **Goal:** Implement the `status` command. This builds out the "read" capabilities of the Jira client.
  * **Steps:**
    1.  **`status` Command:** Implement `cmd/status.go` with `sprint` and `release` subcommands.
    2.  **Client Methods:** Implement `GetActiveSprints`, `GetPlannedSprints`, and `GetReleases` on the `jiraClient`.
    3.  **Logic:** Implement the "current/next" sprint/release selection logic as defined in the spec.
    4.  **Formatting:** Implement the status report output formatting.
    5.  **Testing:**
          * `cmd/status_test.go`: Test the command with a mocked `JiraClient`.
          * `pkg/jira/client_test.go`: Test the new client methods with mocked HTTP responses.

#### **Phase 4: The Core Create Flow (`create` - No AI)**

  * **Goal:** Implement the `create` command *without* AI. This solidifies the "create" capability.
  * **Steps:**
    1.  **`create` Command:** Implement `cmd/create.go` with its flags (`--project`, `--type`).
    2.  **Client Method:** Implement the `CreateTicket` method on the `jiraClient`.
    3.  **Testing:**
          * `cmd/create_test.go`: Test the command, ensuring flags override defaults and the mock client is called correctly.
          * `pkg/jira/client_test.go`: Test the `CreateTicket` method, mocking the HTTP request and response.

#### **Phase 5: AI Integration (Gemini Client & `create` Update)**

  * **Goal:** Introduce the Gemini client and the interactive Q\&A flow, integrating it into the `create` command.
  * **Steps:**
    1.  **Gemini Client:** Create `pkg/gemini/client.go` with an interface and implementation, reading the API key from the keyring.
    2.  **System Editor:** Create a reusable `pkg/editor/editor.go` module to handle the "open in editor" flow.
    3.  **Q\&A Module:** Create `pkg/qa/flow.go` for the reusable 4-question Gemini Q\&A loop.
    4.  **Integrate:** Update `cmd/create.go` to add the `[y/N]` prompt for AI and the `[Y/n/e(dit)]` flow, wiring them to the new Q\&A and Editor modules.
    5.  **Testing:**
          * `pkg/gemini/client_test.go`: Test the Gemini client by mocking its HTTP requests.
          * `pkg/editor/editor_test.go`: Test the editor flow (this is tricky; may need to mock `exec.Command`).
          * `pkg/qa/flow_test.go`: Test the Q\&A loop logic with a mock Gemini client.
          * `cmd/create_test.go`: *Expand* these tests to cover the new AI and edit flows, mocking all new dependencies.

#### **Phase 6: The Interactive Loop (`review`)**

  * **Goal:** Implement the most complex interactive command, reusing as many components as possible.
  * **Steps:**
    1.  **`review` Command:** Implement `cmd/review.go` with its flags.
    2.  **Client Method:** Implement `SearchTickets` on the `jiraClient`.
    3.  **Loop:** Build the main review loop.
    4.  **Reuse:** Wire up the *existing* `estimate` prompt logic, the *existing* `Gemini Q&A Flow`, and the *existing* `assign` (favorites/search) logic.
    5.  **New:** Implement the `triage` (priority) prompt.
    6.  **Testing:** `cmd/review_test.go`: This will be a complex test. Use a mock `JiraClient` and a "script" of mock user inputs to test each action (`a`, `t`, `d`, `e`, `s`, `q`).

#### **Phase 7: The Complex Workflow (`accept`)**

  * **Goal:** Implement the `accept` command, which chains multiple components and includes file parsing.
  * **Steps:**
    1.  **`accept` Command:** Implement `cmd/accept.go`.
    2.  **Client Methods:** Implement `TransitionTicket`, `GetTicketAttachments`, `GetTicketComments`.
    3.  **Source Prompt:** Implement the "select source" logic.
    4.  **Markdown:** Implement the Markdown parser (`pkg/parser/parser.go`) for the epic plan.
    5.  **Reuse:** Wire up the `Gemini Q&A Flow`, `System Editor Flow`, and the sprint/release assignment prompts.
    6.  **Testing:**
          * `pkg/parser/parser_test.go`: Test the Markdown parser with valid and invalid inputs.
          * `cmd/accept_test.go`: A comprehensive test mocking all dependencies (Jira client, Gemini client, editor) and verifying the full chain of events.

#### **Phase 8: Caching & Utilities (`refresh`)**

  * **Goal:** Implement the `refresh` command and wire up the caching layer.
  * **Steps:**
    1.  **`refresh` Command:** Implement `cmd/refresh.go` (a simple file deletion).
    2.  **Caching Logic:** Modify `pkg/jira/client.go` to add the caching layer. The `NewClient` should create a `cache` struct.
    3.  **Update Methods:** Update `GetActiveSprints`, `GetReleases`, `GetPriorities`, etc., to check the cache first before making an API call.
    4.  **Testing:**
          * `cmd/refresh_test.go`: Test that the command deletes the cache file.
          * `pkg/jira/client_test.go`: *Expand* tests to verify that:
              * Calling `GetSprints` once *misses* the cache and writes to it.
              * Calling `GetSprints` a second time *hits* the cache (the mock HTTP client is *not* called).
              * Calling `GetSprints` after a `refresh` *misses* the cache again.

-----

### **LLM Generation Prompts**

Here are the step-by-step prompts for a code-generation LLM.

```
We are building a Go CLI tool called `jira-tool` using the `cobra` library and a test-driven (TDD) approach.

Create a new `go.mod` file for this project (e.g., `module jira-tool`) and then implement the first step.

Create the main application entrypoint (`main.go`) and the root command (`cmd/root.go`). The root command should be the main entrypoint and should not have its own `Run` function, as it will only host subcommands. Include a basic `cmd/root_test.go` to ensure the command is set up, but it doesn't need to do much yet. Use standard Go project layout.
```

```
Now, let's define the configuration for our tool.

In `pkg/config/config.go`, define a `Config` struct that holds the `JiraURL`, `JiraUser`, and `DefaultProject` and `DefaultTaskType`.
Also, create `LoadConfig(path string)` and `SaveConfig(cfg *Config, path string)` functions. The config file should be in YAML format.
Define a `GetConfigPath()` function that returns the default path (e.g., `~/.jira-tool/config.yaml`).
In `pkg/config/config_test.go`, write a test for `LoadConfig` and `SaveConfig` that writes and reads from a *temporary* test file to verify the YAML (un)marshaling works.
```

```
We need to securely store API keys. We will use the `99designs/keyring` library.

In a new file, `pkg/keyring/keyring.go`, create a `StoreSecret(service, user, secret string)` function and a `GetSecret(service, user string)` function that wrap the `keyring` library. Define `JiraServiceKey` and `GeminiServiceKey` as constants.
In `pkg/keyring/keyring_test.go`, write a test. Since we can't test the *actual* system keyring reliably, create a `MockKeyring` struct that implements the `keyring.Keyring` interface and holds secrets in a simple `map[string]string`. Write your tests against this mock to verify your `StoreSecret` and `GetSecret` functions behave as expected.
```

```
Now, let's create the `init` command to set up the tool.

In `cmd/init.go`, create the `initCmd`. This command should:
1.  Prompt the user for: Jira URL, Jira User, Jira API Token, and Gemini API Key. Use `fmt.Scanln` for simplicity, but read the API keys securely (e.g., using `golang.org/x/term.ReadPassword`).
2.  Call `pkg/config.SaveConfig` to save the non-sensitive URL and User to `config.yaml`.
3.  Call `pkg/keyring.StoreSecret` to save the Jira Token and Gemini Key to the system keyring.
4.  Print a "Configuration saved" message.

In `cmd/init_test.go`, write a test. You will need to mock `stdin` (e.g., using a `bytes.Buffer`) to provide simulated user input, and mock the `pkg/keyring` functions (or use the mock from the previous step) to prevent writing to the real keyring. Verify that `SaveConfig` is called with the correct data.
```

```
We need to start building our Jira client.

In `pkg/jira/client.go`, define a `JiraClient` interface.
This interface should, for now, just have one method: `UpdateTicketPoints(ticketID string, points int) error`.

Create a `jiraClient` struct that holds the Jira URL, an `*http.Client`, and the auth token.
Create a `NewClient()` function that:
1.  Loads the config from `pkg/config.LoadConfig`.
2.  Gets the Jira token from `pkg/keyring.GetSecret`.
3.  Creates and returns a `*jiraClient` (as the `JiraClient` interface) configured with a new `http.Client` and the auth token (e.g., using Basic Auth).

We will not test this `NewClient` function directly yet, as it depends on real files and keyring.
```

```
Now, let's implement the `estimate` command and the client method it needs.

In `cmd/estimate.go`:
1.  Create the `estimateCmd`. It should take one argument: the `ticketID`.
2.  Inside its `RunE` function, call `jira.NewClient()`.
3.  Implement the Fibonacci prompt: `[1] 1, [2] 2, [3] 3, [4] 5, [5] 8, [6] 13, [7] Other...`.
4.  If "Other", prompt for a number.
5.  Call `client.UpdateTicketPoints(ticketID, points)`.

In `pkg/jira/client.go`:
1.  Implement the `UpdateTicketPoints` method on `jiraClient`. This method should:
    * Construct the Jira API JSON payload for updating the story points field (you'll need to find the correct Jira JSON format, e.g., `{"fields": {"customfield_10016": 5}}`. Assume the story point field ID is `customfield_10016` for now).
    * Create a `PUT` or `POST` request to the Jira issue endpoint (`/rest/api/2/issue/{ticketID}`).
    * Execute the request, handle errors, and return.

In `cmd/estimate_test.go`:
1.  Write a test for the `estimateCmd`.
2.  Create a `MockJiraClient` struct that implements the `JiraClient` interface.
3.  Test the command by mocking user input for the prompt and verifying that `MockJiraClient.UpdateTicketPoints` is called with the correct `ticketID` and `points`.

In `pkg/jira/client_test.go`:
1.  Write a test for the *real* `UpdateTicketPoints` method.
2.  Use `httptest.NewServer` to create a mock Jira server.
3.  Configure a `jiraClient` to point to this mock server.
4.  Call `client.UpdateTicketPoints` and have the mock server verify that it received the correct `PUT` request with the correct JSON body.
```

```
Let's implement the `status` command (read-only).

In `cmd/status.go`, create the `statusCmd` and add `sprintCmd` and `releaseCmd` as subcommands to it.
The `sprintCmd` and `releaseCmd` should support a `--next` flag.

In `pkg/jira/client.go`:
1.  Add `GetActiveSprints()`, `GetPlannedSprints()`, and `GetReleases()` to the `JiraClient` interface.
2.  Implement these methods on `jiraClient`. They should make `GET` requests to the relevant Jira Agile API endpoints (e.g., `/rest/agile/1.0/board/{boardID}/sprint?state=active`). Assume a default `boardID` of `1` for now.
3.  Define structs for the JSON responses for Sprints and Releases.

In `cmd/status.go`:
1.  Implement the `RunE` for `sprintCmd`. It should:
    * Call `GetActiveSprints` (if no `--next`) or `GetPlannedSprints` (if `--next`).
    * Implement the logic to find the "current" (nearest `endDate`) or "next" (earliest `startDate`) sprint.
    * Fetch the issues for that sprint (you'll need a new client method `GetIssuesForSprint`).
    * Calculate the stats (To Do, In Progress, Done) and print the formatted report.
2.  Implement the `RunE` for `releaseCmd` similarly (you'll need `GetIssuesForRelease`).

In `pkg/jira/client.go`:
1.  Add and implement `GetIssuesForSprint(sprintID int)` and `GetIssuesForRelease(releaseID string)`.

In `cmd/status_test.go` and `pkg/jira/client_test.go`:
1.  Write tests for the new command logic (mocking the client) and the new client methods (mocking `httptest.NewServer` with sample JSON sprint/issue responses).
```

```
Implement the `create` command, *without* the AI features.

In `cmd/create.go`:
1.  Create the `createCmd`. It should take one argument: the `summary`.
2.  Add flags for `--project` and `--type`.
3.  In `RunE`, it should call `jira.NewClient()`.
4.  It should load the `default_project` and `default_task_type` from config.
5.  If the flags are present, they should override the defaults.
6.  Call `client.CreateTicket(project, taskType, summary)`.
7.  Print the new ticket ID (e.g., `Ticket ENG-123 created.`).

In `pkg/jira/client.go`:
1.  Add `CreateTicket(project, taskType, summary string) (string, error)` to the `JiraClient` interface.
2.  Implement the `CreateTicket` method. It should construct the correct JSON payload for creating a new issue and `POST` it to the `/rest/api/2/issue` endpoint. It must parse the response to find and return the new ticket key (e.g., "ENG-123").

In `cmd/create_test.go`:
1.  Test the `createCmd`. Mock the client.
2.  Test that calling *without* flags uses the default config values.
3.  Test that calling *with* flags correctly overrides the defaults.
4.  Verify the mock `CreateTicket` method is called with the correct arguments.

In `pkg/jira/client_test.go`:
1.  Test the `CreateTicket` method using `httptest.NewServer`.
2.  Verify the `POST` request contains the correct JSON body (project, type, summary).
3.  Have the mock server return a sample "ticket created" JSON response, and verify the function correctly parses and returns the new ticket key.
```

```
Now, let's build the Gemini client.

In `pkg/gemini/client.go`:
1.  Define a `GeminiClient` interface with methods:
    * `GenerateQuestion(history []string, context string) (string, error)`
    * `GenerateDescription(history []string, context string) (string, error)`
2.  Create a `geminiClient` struct.
3.  Create a `NewClient()` function that gets the Gemini API key from `pkg/keyring.GetSecret` and returns an initialized client.
4.  Implement the methods. They should:
    * Build the correct prompt for the Gemini API (e.g., "Based on this history... ask one clarifying question" or "Based on this conversation... write a Jira description").
    * Call the Gemini `generateContent` API.
    * Parse the response and return the text.

In `pkg/gemini/client_test.go`:
1.  Write tests for the methods using `httptest.NewServer` to mock the Gemini API.
2.  Verify that the correct prompt is sent.
3.  Return a mock Gemini JSON response and verify it is parsed correctly.
```

```
Create the reusable System Editor module.

In `pkg/editor/editor.go`:
1.  Create a function `OpenInEditor(initialContent string) (string, error)`.
2.  This function should:
    * Create a temporary file (e.g., `jira-tool-*.md`).
    * Write `initialContent` to it.
    * Get the system editor from `os.Getenv("EDITOR")` (default to `vim` or `nano`).
    * Use `exec.Command` to run the editor with the temp file as an argument, and set `cmd.Stdin`, `cmd.Stdout`, `cmd.Stderr` to `os.Stdin`, `os.Stdout`, `os.Stderr`.
    * Wait for the command to exit using `cmd.Run()`.
    * Read the contents of the temp file *after* the editor has closed.
    * Delete the temp file.
    * Return the new content.

In `pkg/editor/editor_test.go`:
1.  This is hard to test. Write a simple unit test that mocks `exec.Command`. You can't test the interactive part, but you can test that the temp file is created and (in theory) `cmd.Run` is called.
```

```
Create the reusable Gemini Q&A Flow module.

In `pkg/qa/flow.go`:
1.  Create a function `RunQAFlow(client gemini.GeminiClient, initialContext string) (string, error)`.
2.  This function should:
    * Initialize an empty `history` slice.
    * Loop 4 times:
        * Call `client.GenerateQuestion(history, initialContext)`.
        * Print the question: `Gemini asks: [question]? > `.
        * Read the user's answer from `stdin`.
        * Append both the question and answer to the `history` slice.
    * After the loop, call `client.GenerateDescription(history, initialContext)`.
    * Return the final description.

In `pkg/qa/flow_test.go`:
1.  Write a test for `RunQAFlow`.
2.  Create a `MockGeminiClient` that implements the `GeminiClient` interface.
3.  Have the mock return a sequence of questions (e.g., "Q1", "Q2") and a final description.
4.  Simulate user input by mocking `stdin`.
5.  Verify that the function returns the correct final description and that the history was built correctly.
```

```
Now, integrate the AI/Edit features into the `create` command.

Modify `cmd/create.go`:
1.  After the ticket is created, add the prompt: `Would you like to use Gemini to generate the description? [y/N]`.
2.  If 'y':
    * Instantiate the `gemini.NewClient()`.
    * Get the `initialContext` (the summary).
    * Call `qa.RunQAFlow()`.
    * Print the generated description and ask: `Update ticket with this description? [Y/n/e(dit)]`.
    * If 'y', call a new client method `UpdateTicketDescription(ticketID, description)`.
    * If 'e', call `editor.OpenInEditor(description)`. Get the new content and call `UpdateTicketDescription` with it.
    * If 'n', do nothing.

In `pkg/jira/client.go`:
1.  Add and implement `UpdateTicketDescription(ticketID, description string)` (similar to `UpdateTicketPoints`, but for the `description` field).

In `cmd/create_test.go` and `pkg/jira/client_test.go`:
1.  Expand the tests to cover this new flow.
2.  Mock the Gemini client, the Q&A flow, the editor, and user input to test all branches (`[y/N]`, `[Y/n/e]`).
3.  Test the new `UpdateTicketDescription` client method.
```

```
Implement the `review` command.

In `cmd/review.go`:
1.  Create the `reviewCmd` with flags (`--needs-detail`, `--unassigned`, etc.).
2.  In `RunE`, call `client.SearchTickets(query)` to build the JQL query based on the flags.
3.  Loop through each ticket found.
4.  Print the compact, single-line summary.
5.  Show the action prompt: `[a(ssign), t(riage), d(etail), e(stimate), s(kip), q(uit)]`.
6.  Use a `switch` statement on the user's input:
    * `'a'`: Implement the assign flow (show favorites, then search). You'll need `client.SearchUsers()` and `client.AssignTicket()`.
    * `'t'`: Implement the triage flow (list priorities). You'll need `client.GetPriorities()` and `client.UpdateTicketPriority()`.
    * `'d'`: *Reuse* the `qa.RunQAFlow()` and `editor.OpenInEditor()` flow, just like in `create`.
    * `'e'`: *Reuse* the Fibonacci prompt logic and call `client.UpdateTicketPoints()`.
    * `'s'`: `continue` the loop.
    * `'q'`: `break` the loop.

In `pkg/jira/client.go`:
1.  Add and implement `SearchTickets`, `SearchUsers`, `AssignTicket`, `GetPriorities`, and `UpdateTicketPriority`.

In `cmd/review_test.go` and `pkg/jira/client_test.go`:
1.  Write tests for all new client methods (mocking `httptest.NewServer`).
2.  Write a complex integration test for `cmd/review.go`. Mock the client and mock a *sequence* of user inputs to test each action in the `switch` statement.
```

```
Implement the `accept` command.

In `cmd/accept.go`:
1.  Create the `acceptCmd`.
2.  Call `client.TransitionTicket(ticketID, "Done")`.
3.  Call `client.GetTicketDescription()`, `client.GetTicketAttachments()`, `client.GetTicketComments()`.
4.  Build and show the "select source" prompt.
5.  Prompt for the `New Epic Summary:`.
6.  *Reuse* `qa.RunQAFlow()` using the summary + source text as context.
7.  Get the returned plan, which is *expected* to be Markdown.
8.  Show the `[Y/n/e(dit)]` prompt.
9.  If 'e', *reuse* `editor.OpenInEditor()` with the Markdown plan.
10. On 'y' or after edit, pass the Markdown text to a new `parser.ParseEpicPlan()`.
11. This parser returns an `Epic` struct and a slice of `Task` structs.
12. Call `client.CreateTicket()` to create the Epic.
13. Loop and call `client.CreateTicket()` for each task, linking it to the Epic.
14. *Reuse* the sprint/release assignment prompts (from `review`/`status`) and call `client.AddIssuesToSprint()` / `client.AddIssuesToRelease()`.

In `pkg/parser/parser.go`:
1.  Create `ParseEpicPlan(markdown string) (Epic, []Task, error)`.
2.  Use regex or a simple string-splitting logic to parse the text.

In `pkg/jira/client.go`:
1.  Add and implement `TransitionTicket`, `GetTicketDescription`, `GetTicketAttachments`, `GetTicketComments`, `AddIssuesToSprint`, `AddIssuesToRelease`.

In `cmd/accept_test.go`, `pkg/parser/parser_test.go`, `pkg/jira/client_test.go`:
1.  Test the parser with valid and invalid Markdown.
2.  Test all new client methods.
3.  Write the final, complex integration test for `cmd/accept.go`, mocking all dependencies and user inputs.
```

```
Finally, implement the caching layer and the `refresh` command.

In `cmd/refresh.go`:
1.  Create the `refreshCmd` with a `--cache` flag.
2.  In `RunE`, get the cache path (e.g., `~/.jira-tool/cache.json`) and delete the file.

Modify `pkg/jira/client.go`:
1.  Add a `cache` field to the `jiraClient` struct (e.g., `cache *Cache`).
2.  Define the `Cache` struct and its `Load(path)`, `Save(path)` methods.
3.  In `NewClient()`, load the cache file if it exists.
4.  Modify all "list" methods (`GetPriorities`, `GetActiveSprints`, `GetReleases`, `SearchUsers`):
    * Before making an API call, check if the data is in the `cache` struct.
    * If yes, return the cached data.
    * If no, make the API call, save the result to the `cache` struct, and call `cache.Save()`.

In `pkg/jira/client_test.go`:
1.  Expand your tests for the "list" methods.
2.  Create a client with a *mock* HTTP server.
3.  Call `client.GetPriorities()` once. Verify the mock server was called.
4.  Call `client.GetPriorities()` a *second time*. Verify the mock server was *NOT* called, and the data is the same.
5.  Simulate a "refresh" (e.g., delete the temp cache file) and call again. Verify the server *is* called.
```