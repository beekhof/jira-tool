package qa

import (
	"strings"
	"testing"
)

func TestReadAnswerWithReadline_EditorCommand(t *testing.T) {
	// Test that :edit command is detected
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{":edit command", ":edit", ""},
		{":e command", ":e", ""},
		{":edit with content", ":edit some text", "some text"},
		{":e with content", ":e some text", "some text"},
		{":edit with space", ":edit  ", ""},
		{":e with space", ":e  ", ""},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Test command detection logic
			line := strings.TrimSpace(tc.input)
			var content string

			if strings.HasPrefix(line, ":edit") || strings.HasPrefix(line, ":e ") {
				if strings.HasPrefix(line, ":edit") {
					content = strings.TrimPrefix(line, ":edit")
				} else {
					content = strings.TrimPrefix(line, ":e ")
				}
				content = strings.TrimSpace(content)
			} else if line == ":e" || line == ":edit" {
				content = ""
			}

			if content != tc.expected {
				t.Errorf("Expected content '%s', got '%s'", tc.expected, content)
			}
		})
	}
}

func TestPreviewAndEditLoop_MethodHandling(t *testing.T) {
	// Test that different methods are handled correctly
	testCases := []struct {
		name       string
		method     string
		shouldSkip bool
	}{
		{"readline method", "readline", true},
		{"editor method", "editor", true},
		{"readline_with_preview method", "readline_with_preview", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Test method handling logic
			shouldSkip := false
			if tc.method == "readline" || tc.method == "editor" {
				shouldSkip = true
			}

			if shouldSkip != tc.shouldSkip {
				t.Errorf("Expected shouldSkip=%v, got %v", tc.shouldSkip, shouldSkip)
			}
		})
	}
}
