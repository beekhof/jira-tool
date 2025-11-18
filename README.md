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

#### Prompt Templates

You can customize the prompts used for generating questions and descriptions:

- **`question_prompt_template`** (optional): Template for question generation
- **`description_prompt_template`** (optional): Template for description generation

**Template Placeholders:**
- `{{context}}` - The context string (ticket summary, epic summary, etc.)
- `{{history}}` - Formatted conversation history from Q&A flow

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
```

**Default Templates:**

If not specified, the tool uses built-in defaults:
- Question prompt: Asks for ONE clarifying question about the context
- Description prompt: Creates a professional Jira description with clear explanation, context, and acceptance criteria

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
Display status for sprints or releases.

```bash
jira status sprint
jira status sprint --next
jira status release
jira status release --next
```

**Flags:**
- `--next, -n`: Show next sprint/release instead of current

### `review`
Review and triage tickets interactively.

```bash
jira review
jira review --unassigned
jira review --untriaged
```

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

