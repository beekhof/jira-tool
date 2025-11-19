package editor

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// OpenInEditor opens the given content in the system editor and returns the edited content
func OpenInEditor(initialContent string) (string, error) {
	// Create a temporary file
	tmpFile, err := os.CreateTemp("", "jira-tool-*.md")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath) // Clean up temp file

	// Write initial content to temp file
	if _, err := tmpFile.WriteString(initialContent); err != nil {
		tmpFile.Close()
		return "", fmt.Errorf("failed to write to temp file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return "", fmt.Errorf("failed to close temp file: %w", err)
	}

	// Get the editor from environment variable, default to vim
	editor := os.Getenv("EDITOR")
	if editor == "" {
		// Try common editors
		for _, e := range []string{"vim", "nano", "vi", "code", "nano"} {
			if _, err := exec.LookPath(e); err == nil {
				editor = e
				break
			}
		}
		if editor == "" {
			return "", fmt.Errorf("no editor found. Please set EDITOR environment variable")
		}
	}

	// Build the command
	cmd := exec.Command(editor, tmpPath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Run the editor (this blocks until the editor exits)
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("editor exited with error: %w", err)
	}

	// Read the edited content
	editedContent, err := os.ReadFile(tmpPath)
	if err != nil {
		return "", fmt.Errorf("failed to read edited file: %w", err)
	}

	return strings.TrimSpace(string(editedContent)), nil
}

// GetEditorPath returns the path to the editor executable
func GetEditorPath() (string, error) {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		// Try common editors
		for _, e := range []string{"vim", "nano", "vi", "code"} {
			if path, err := exec.LookPath(e); err == nil {
				return path, nil
			}
		}
		return "", fmt.Errorf("no editor found. Please set EDITOR environment variable")
	}

	// If editor is a full path, return it
	if filepath.IsAbs(editor) {
		return editor, nil
	}

	// Otherwise, look it up in PATH
	path, err := exec.LookPath(editor)
	if err != nil {
		return "", fmt.Errorf("editor '%s' not found in PATH: %w", editor, err)
	}

	return path, nil
}
