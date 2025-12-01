# Enhanced Answer Input Feature - Implementation Blueprint

## Overview
This blueprint outlines the step-by-step implementation of enhanced answer input with readline support, editor switching, and preview/edit loop.

## Implementation Steps

### Step 1: Add Readline Dependency
- Add `github.com/chzyer/readline` to `go.mod`
- Run `go mod tidy`

### Step 2: Add Configuration Field
- Update `pkg/config/config.go`:
  - Add `AnswerInputMethod string` field with yaml tag
  - Default to `"readline_with_preview"` if empty

### Step 3: Create Input Helper Module
- Create `pkg/qa/input.go`:
  - `ReadAnswerWithReadline(prompt string, method string) (string, error)` function
  - Handles readline input
  - Detects `:edit` or `:e` command
  - Switches to editor if command detected
  - Returns final answer

### Step 4: Create Preview/Edit Loop Helper
- Add to `pkg/qa/input.go`:
  - `PreviewAndEditLoop(answer string, method string) (string, error)` function
  - Shows preview: "Your answer: [answer]"
  - Prompts: "Edit? [y/N]"
  - Opens editor if yes
  - Loops until user accepts

### Step 5: Update RunQnAFlow
- Modify `pkg/qa/flow.go`:
  - Replace `reader.ReadString('\n')` with new input function
  - Add preview/edit loop after input
  - Respect `answer_input_method` config
  - Pass config to input functions

### Step 6: Update Call Sites
- Update all `RunQnAFlow` call sites to pass config:
  - `pkg/review/workflow.go`
  - `cmd/review.go`
  - `cmd/create.go`
  - `cmd/accept.go`

### Step 7: Handle Edge Cases
- Empty input with `:edit` command
- Editor cancellation
- Invalid config values
- Readline initialization failures

### Step 8: Testing
- Test readline input (arrow keys, backspace)
- Test `:edit` command
- Test preview/edit loop
- Test all config options
- Test rejection/skip/done still work

## Detailed Implementation Plan

### New File: `pkg/qa/input.go`

```go
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
func ReadAnswerWithReadline(prompt string, method string) (string, error) {
    // Create readline instance
    rl, err := readline.New(prompt)
    if err != nil {
        // Fallback to standard input
        return readAnswerStandard(prompt)
    }
    defer rl.Close()
    
    for {
        line, err := rl.Readline()
        if err != nil {
            return "", err
        }
        
        line = strings.TrimSpace(line)
        
        // Check for editor command
        if strings.HasPrefix(line, ":edit") || strings.HasPrefix(line, ":e") {
            // Get current line (before command)
            currentInput := strings.TrimPrefix(line, ":edit")
            currentInput = strings.TrimPrefix(currentInput, ":e")
            currentInput = strings.TrimSpace(currentInput)
            
            // Open editor
            edited, err := editor.OpenInEditor(currentInput)
            if err != nil {
                fmt.Printf("Editor error: %v. Continuing with current input.\n", err)
                return currentInput, nil
            }
            return edited, nil
        }
        
        // Return answer
        return line, nil
    }
}

// PreviewAndEditLoop shows preview and allows editing in a loop
func PreviewAndEditLoop(answer string, method string) (string, error) {
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
    return reader.ReadString('\n')
}
```

### Modified File: `pkg/qa/flow.go`

- Add config parameter to `RunQnAFlow`
- Replace input reading with `ReadAnswerWithReadline`
- Add preview/edit loop after input
- Handle all three input methods

## Files to Modify

1. `go.mod` - Add readline dependency
2. `pkg/config/config.go` - Add `AnswerInputMethod` field
3. `pkg/qa/input.go` - New file with input helpers
4. `pkg/qa/flow.go` - Update to use new input functions
5. `pkg/review/workflow.go` - Pass config to `RunQnAFlow`
6. `cmd/review.go` - Pass config to `RunQnAFlow`
7. `cmd/create.go` - Pass config to `RunQnAFlow`
8. `cmd/accept.go` - Pass config to `RunQnAFlow`

## Dependencies

- `github.com/chzyer/readline` - For enhanced terminal input

## Testing Checklist

- [ ] Readline input works (arrow keys, backspace)
- [ ] `:edit` command switches to editor
- [ ] Preview/edit loop works
- [ ] All three config options work
- [ ] Invalid config uses default
- [ ] Rejection/skip/done still work
- [ ] Fallback to standard input if readline fails
- [ ] Editor cancellation handled gracefully

