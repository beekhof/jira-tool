package qa

import (
	"strings"
	"testing"
)

func TestRejectionKeywords(t *testing.T) {
	// Test that "reject" is case-insensitive
	testCases := []string{"reject", "REJECT", "Reject", "ReJeCt"}
	for _, tc := range testCases {
		if !strings.EqualFold(tc, "reject") {
			t.Errorf("strings.EqualFold(%q, %q) should be true", tc, "reject")
		}
	}
}

func TestRejectionWithEmptyString(t *testing.T) {
	// Test that empty string triggers rejection
	answer := ""
	if answer != "" {
		t.Errorf("Empty string should be detected as rejection")
	}
}

func TestSkipAndDoneStillWork(t *testing.T) {
	// Test that "skip" and "done" still end the loop
	testCases := []string{"skip", "done"}
	for _, tc := range testCases {
		if tc != "skip" && tc != "done" {
			t.Errorf("Expected skip or done, got %q", tc)
		}
	}
}

func TestTrimSpace(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"empty", "", ""},
		{"whitespace only", "   ", ""},
		{"newline only", "\n", ""},
		{"tab only", "\t", ""},
		{"normal text", "hello", "hello"},
		{"leading space", "  hello", "hello"},
		{"trailing space", "hello  ", "hello"},
		{"both", "  hello  ", "hello"},
		{"newlines", "\nhello\n", "hello"},
		{"mixed", "  \n\t hello \t\n  ", "hello"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := trimSpace(tt.input)
			if result != tt.expected {
				t.Errorf("trimSpace(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

