# Jira Tool

A command-line tool to streamline Jira workflows by integrating with Jira and Gemini APIs.

## Features

- **Ticket Management**: Create, estimate, and manage Jira tickets
- **AI-Powered Descriptions**: Use Gemini AI to generate ticket descriptions through interactive Q&A
- **Sprint & Release Status**: View current and upcoming sprint/release status
- **Ticket Review & Triage**: Review tickets, assign priorities, and add details
- **Epic Creation**: Convert research tickets into Epics with decomposed tasks
- **Configurable Prompts**: Customize AI prompts for questions and descriptions

## Installation

### Build from Source

```bash
git clone <repository-url>
cd jira-tool
make build
```

The binary will be created at `bin/jira-tool`.

### Install

```bash
make install
```

This installs the binary to your system PATH.

## Quick Start

1. **Initialize the tool**:
   ```bash
   jira init
   ```
   This will prompt you for:
   - Jira URL
   - Jira API Token (stored securely)
   - Gemini API Key (stored securely)
   - Default project key
   - Default task type

2. **Create a ticket**:
   ```bash
   jira create "Fix login bug"
   ```

3. **Review tickets**:
   ```bash
   jira review
   ```

## Configuration

Configuration is stored in `~/.jira-tool/config.yaml` (or a custom directory specified with `--config-dir`).

### Configuration File Structure

```yaml
jira_url: https://your-company.atlassian.net
default_project: PROJ
default_task_type: "Task"
gemini_model: gemini-2.5-flash
max_questions: 4
story_point_options:
  - 1
  - 2
  - 3
  - 5
  - 8
  - 13
favorite_assignees:
  - user1@example.com
  - user2@example.com
favorite_sprints:
  - "Sprint 1"
  - "Sprint 2"
favorite_releases:
  - "v1.0"
  - "v2.0"
```

### Configuration Options

#### Basic Settings

- **`jira_url`** (required): Your Jira instance URL
- **`default_project`** (required): Default project key for ticket creation
- **`default_task_type`** (required): Default issue type (e.g., "Task", "Story", "Bug")
- **`story_point_options`** (optional): List of story point values for estimation (default: Fibonacci sequence)

#### Gemini AI Settings

- **`gemini_model`** (optional): Gemini model to use (default: `gemini-2.5-flash`)
  - Available models can be listed with: `jira models`
  - Common options: `gemini-2.5-flash`, `gemini-2.5-pro`, `gemini-2.0-flash`
- **`max_questions`** (optional): Maximum number of questions in Q&A flow (default: `4`)
- **`review_page_size`** (optional): Number of tickets per page in review command (default: `10`)

#### Prompt Templates

You can customize the prompts used for generating questions and descriptions:

- **`question_prompt_template`** (optional): Template for question generation (used for Tasks, Bugs, Epics, etc.)
- **`description_prompt_template`** (optional): Template for description generation (used for Tasks, Bugs, Epics, etc.)
- **`spike_question_prompt_template`** (optional): Template for question generation for research spikes
- **`spike_prompt_template`** (optional): Template for research spike descriptions (used when ticket summary/key has "SPIKE" prefix)

**Template Placeholders:**
- `{{context}}` - The context string (ticket summary, epic summary, etc.)
- `{{history}}` - Formatted conversation history from Q&A flow

**Spike Detection:**
- The tool automatically detects spikes based on the ticket summary or key having a "SPIKE" prefix (case-insensitive)
- Spikes are modeled as Tasks with a "SPIKE" prefix in the summary (e.g., "SPIKE: Research authentication options")
- For **Spike** tickets (detected by "SPIKE" prefix):
  - Question generation uses `spike_question_prompt_template` (focuses on constraining research scope)
  - Description generation uses `spike_prompt_template` (creates research plans)
- For all other tickets (Task, Bug, Epic, Story, etc.):
  - Question generation uses `question_prompt_template` (focuses on implementation details)
  - Description generation uses `description_prompt_template` (creates implementation descriptions)

**Example Custom Templates:**

```yaml
question_prompt_template: |
  You are a technical assistant helping to clarify requirements.
  
  Context: {{context}}
  
  Previous conversation:
  {{history}}
  
  Ask ONE specific technical question to better understand the requirements.
  Be concise and focus on implementation details.

description_prompt_template: |
  Create a comprehensive Jira ticket description in Markdown format.
  
  Context: {{context}}
  
  Q&A History:
  {{history}}
  
  Include:
  - Overview
  - Technical requirements
  - Acceptance criteria
  - Dependencies (if any)
  
  Format as Markdown.

spike_question_prompt_template: |
  You are helping to scope a research spike. Ask questions that help constrain and focus the research area.
  
  Context: {{context}}
  
  Previous conversation:
  {{history}}
  
  Ask ONE question that helps define the scope or boundaries of the research.
  Focus on understanding what needs to be investigated, not on dictating a solution.
  Be concise and focus on research boundaries.

spike_prompt_template: |
  Create a research spike plan for investigating a technical question.
  
  Context: {{context}}
  
  Q&A History:
  {{history}}
  
  Include:
  - Research objectives and questions to answer
  - Areas to investigate
  - Expected deliverables and findings
  - Success criteria for the research
  - Timeline estimates if applicable
  
  Format as Markdown.
```

**Default Templates:**

If not specified, the tool uses built-in defaults:
- **Question prompt**: Asks for ONE clarifying question about implementation details (for Tasks, Bugs, Epics, etc.)
- **Spike question prompt**: Asks for ONE question to constrain and focus the research area (for Spikes)
- **Description prompt**: Creates a professional Jira description with clear explanation, context, and acceptance criteria (for implementation tasks)
- **Spike prompt**: Creates a research plan with objectives, investigation areas, deliverables, and success criteria (for research spikes)

To view the default templates in YAML format, run:
```bash
jira templates
```

### Custom Configuration Directory

You can specify a custom configuration directory:

```bash
jira --config-dir /path/to/config create "My ticket"
```

## Commands

### `init`
Initialize the tool configuration and store API credentials.

```bash
jira init
```

### `create [SUMMARY]`
Create a new Jira ticket with optional AI-generated description.

```bash
jira create "Fix login bug"
jira create --project ENG --type Bug "Critical security issue"
```

**Flags:**
- `--project, -p`: Override default project
- `--type, -t`: Override default task type

### `estimate [TICKET_ID]`
Estimate story points for a ticket.

```bash
jira estimate ENG-123
```

### `status`
Display status for sprints, releases, or spike tickets.

```bash
jira status sprint
jira status sprint --next
jira status release
jira status release --next
jira status spikes
```

**Subcommands:**
- `sprint`: Display sprint status with progress and burndown
- `release`: Display release status with progress
- `spikes`: Display all spike tickets grouped by status

**Flags:**
- `--next, -n`: Show next sprint/release instead of current (only for sprint/release)

### `review`
Review and triage tickets interactively with paginated view.

```bash
jira review
jira review --unassigned
jira review --untriaged
```

**Features:**
- Displays tickets in pages (default 10 per page, configurable via `review_page_size` in config)
- Shows ticket key, summary, priority, assignee, and status in a table format
- Marks tickets with ✓ after actions are performed
- Supports page navigation: `n`/`next` for next page, `p`/`prev` for previous page
- After acting on a ticket, returns to the list showing updated information

**Usage:**
1. Enter a ticket number (1-N) to select a ticket
2. Choose an action: `a` (assign), `t` (triage), `d` (detail), `e` (estimate), `b` (back)
3. After completing an action, you'll return to the list view

**Flags:**
- `--needs-detail`: Show only tickets that need detail
- `--unassigned`: Show only unassigned tickets
- `--untriaged`: Show only untriaged tickets

### `accept [TICKET_ID]`
Convert a research ticket into an Epic and tasks.

```bash
jira accept ENG-456
```

### `refresh`
Clear the local cache to force fresh data from Jira.

```bash
jira refresh
```

### `models`
List available Gemini models that support `generateContent`.

```bash
jira models
```

### `templates`
Display the default prompt templates in YAML format that can be copied into your config file.

```bash
jira templates
```

This command outputs all four default templates (`question_prompt_template`, `description_prompt_template`, `spike_question_prompt_template`, and `spike_prompt_template`) in a format ready to be copied into your `config.yaml` file for customization.

## Authentication

The tool uses Bearer token authentication for Jira. Your API token is stored securely in `~/.jira-tool/credentials.yaml`.

**Getting a Jira API Token:**
1. Go to your Jira account settings
2. Navigate to Security → API tokens
3. Create a new API token
4. Use this token during `jira init`

**Getting a Gemini API Key:**
1. Go to [Google AI Studio](https://makersuite.google.com/app/apikey)
2. Create a new API key
3. Use this key during `jira init`

## Error Handling

The tool includes automatic retry logic for transient Gemini API errors:
- **503 (Service Unavailable)**: Automatically retries up to 3 times with exponential backoff (5s, 10s, 20s)
- **429 (Rate Limit)**: Automatically retries with backoff
- **500/502/504 (Server Errors)**: Automatically retries with backoff

Retry attempts are displayed to stderr so you can see when retries are happening.

## Development

### Build

```bash
make build
```

### Test

```bash
make test
```

### Lint

```bash
make lint
```

### Format

```bash
make fmt
```

## License

[Add your license here]

