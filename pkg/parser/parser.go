package parser

import (
	"fmt"
	"regexp"
	"strings"
)

// Epic represents a parsed epic
type Epic struct {
	Title       string
	Description string
}

// Task represents a parsed task
type Task struct {
	Summary string
}

// ParseEpicPlan parses a Markdown epic plan into an Epic and list of Tasks
// Expected format:
// # EPIC: Title
// Description text...
//
// ## TASKS
// - [ ] Task 1
// - [ ] Task 2
func ParseEpicPlan(markdown string) (Epic, []Task, error) {
	var epic Epic
	var tasks []Task

	lines := strings.Split(markdown, "\n")

	// Find epic title
	epicTitleRegex := regexp.MustCompile(`^#\s*EPIC:\s*(.+)$`)
	epicDescStart := -1

	for i, line := range lines {
		if matches := epicTitleRegex.FindStringSubmatch(line); matches != nil {
			epic.Title = strings.TrimSpace(matches[1])
			epicDescStart = i + 1
			break
		}
	}

	if epic.Title == "" {
		return epic, tasks, fmt.Errorf("epic title not found. Expected format: # EPIC: Title")
	}

	// Find tasks section
	tasksStart := -1
	tasksRegex := regexp.MustCompile(`^##\s*TASKS`)

	for i := epicDescStart; i < len(lines); i++ {
		if tasksRegex.MatchString(lines[i]) {
			tasksStart = i + 1
			break
		}
		// Collect description until we hit TASKS
		if tasksStart == -1 {
			line := strings.TrimSpace(lines[i])
			if line != "" {
				epic.Description += line + "\n"
			}
		}
	}

	epic.Description = strings.TrimSpace(epic.Description)

	// Parse tasks
	if tasksStart == -1 {
		return epic, tasks, fmt.Errorf("TASKS section not found. Expected format: ## TASKS")
	}

	taskRegex := regexp.MustCompile(`^-\s*\[[ xX]\]\s*(.+)$`)

	for i := tasksStart; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}

		if matches := taskRegex.FindStringSubmatch(line); matches != nil {
			tasks = append(tasks, Task{
				Summary: strings.TrimSpace(matches[1]),
			})
		} else if strings.HasPrefix(line, "-") {
			// Allow tasks without checkbox format
			taskText := strings.TrimPrefix(line, "-")
			taskText = strings.TrimSpace(taskText)
			if taskText != "" {
				tasks = append(tasks, Task{
					Summary: taskText,
				})
			}
		}
	}

	if len(tasks) == 0 {
		return epic, tasks, fmt.Errorf("no tasks found in TASKS section")
	}

	return epic, tasks, nil
}
