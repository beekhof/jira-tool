package jira

import (
	"bufio"
	"fmt"
	"strings"

	"github.com/beekhof/jira-tool/pkg/config"
)

// getDefaultChildType returns the default child type for a parent type
func getDefaultChildType(parentType string) (string, bool) {
	mapping := map[string]string{
		"Epic":     "Story",
		"Story":    "Task",
		"Task":     "Sub-task",
		"Sub-task": "Sub-task", // Sub-tasks can have sub-tasks
	}

	childType, found := mapping[parentType]
	return childType, found
}

// GetChildTicketType determines the child ticket type for a given parent type
func GetChildTicketType(parentType string, reader *bufio.Reader, _ *config.Config) (string, error) {
	childType, found := getDefaultChildType(parentType)
	if found {
		return childType, nil
	}

	// Unknown type - prompt user
	fmt.Printf("Parent ticket type \"%s\" has no default child type mapping.\n", parentType)
	fmt.Print("What type should child tickets be? [Task/Story/Sub-task/Other]: ")

	choice, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	choice = strings.TrimSpace(strings.ToLower(choice))

	switch choice {
	case "task":
		return "Task", nil
	case "story":
		return "Story", nil
	case "sub-task", "subtask":
		return "Sub-task", nil
	case "other":
		fmt.Print("Enter custom ticket type: ")
		customType, err := reader.ReadString('\n')
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(customType), nil
	default:
		// Default to Task if invalid input
		fmt.Printf("Invalid choice, defaulting to Task\n")
		return "Task", nil
	}
}
