package qa

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/beekhof/jira-tool/pkg/editor"
	"github.com/chzyer/readline"
)

// ReadAnswerWithReadline reads an answer using readline with optional editor switching
// method can be "readline", "editor", or "readline_with_preview"
func ReadAnswerWithReadline(prompt, method string) (string, error) {
	// If method is "editor", open editor immediately
	if method == "editor" {
		edited, err := editor.OpenInEditor("")
		if err != nil {
			return "", fmt.Errorf("failed to open editor: %w", err)
		}
		return edited, nil
	}

	// Create readline instance
	rl, err := readline.NewEx(&readline.Config{
		Prompt:          prompt,
		HistoryFile:     "", // No history file
		AutoComplete:    nil,
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",
	})
	if err != nil {
		// Fallback to standard input
		return readAnswerStandard(prompt)
	}
	defer rl.Close()

	for {
		line, err := rl.Readline()
		if err != nil {
			// EOF or interrupt - return empty string
			if err == readline.ErrInterrupt {
				return "", nil
			}
			return "", err
		}

		line = strings.TrimSpace(line)

		// Check for editor command
		if line == ":e" || line == ":edit" {
			// Just the command - open editor with empty content
			edited, err := editor.OpenInEditor("")
			if err != nil {
				fmt.Printf("Editor error: %v. Continuing with empty input.\n", err)
				return "", nil
			}
			return edited, nil
		}

		// Check for editor command with content
		if strings.HasPrefix(line, ":edit") {
			// Extract content after ":edit"
			content := strings.TrimPrefix(line, ":edit")
			content = strings.TrimSpace(content)

			// Open editor
			edited, err := editor.OpenInEditor(content)
			if err != nil {
				fmt.Printf("Editor error: %v. Continuing with current input.\n", err)
				return content, nil
			}
			return edited, nil
		}

		if strings.HasPrefix(line, ":e") {
			// Extract content after ":e" (could be space or no space)
			content := strings.TrimPrefix(line, ":e")
			content = strings.TrimSpace(content)

			// Open editor
			edited, err := editor.OpenInEditor(content)
			if err != nil {
				fmt.Printf("Editor error: %v. Continuing with current input.\n", err)
				return content, nil
			}
			return edited, nil
		}

		// Return answer
		return line, nil
	}
}

// PreviewAndEditLoop shows preview and allows editing in a loop
// method can be "readline", "editor", or "readline_with_preview"
func PreviewAndEditLoop(answer, method string) (string, error) {
	if method == "readline" {
		// No preview for readline-only mode
		return answer, nil
	}

	if method == "editor" {
		// Editor mode - should have already been handled
		return answer, nil
	}

	// readline_with_preview mode
	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Printf("\nYour answer: %s\n", answer)
		fmt.Print("Edit? [y/N] ")

		response, err := reader.ReadString('\n')
		if err != nil {
			return answer, err
		}

		response = strings.TrimSpace(strings.ToLower(response))

		if response == "y" || response == "yes" {
			// Open editor
			edited, err := editor.OpenInEditor(answer)
			if err != nil {
				fmt.Printf("Editor error: %v. Using current answer.\n", err)
				return answer, nil
			}
			answer = edited
			// Loop to show preview again
			continue
		}

		// User accepted (or any other input)
		return answer, nil
	}
}

// readAnswerStandard is fallback for when readline fails
func readAnswerStandard(prompt string) (string, error) {
	fmt.Print(prompt)
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(line), nil
}
