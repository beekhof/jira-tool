# Enhanced Answer Input Feature - Code Generation Prompts

## Prompt 1: Add Readline Dependency

**Context**: We need to add a readline library for enhanced terminal input with arrow key navigation and better editing capabilities.

**Task**: Add `github.com/chzyer/readline` dependency to the project:
1. Update `go.mod` to include the dependency
2. Run `go mod tidy` to download and update dependencies
3. Verify the dependency is added correctly

**Requirements**:
- Use `github.com/chzyer/readline` library
- Ensure compatibility with Go 1.21+

**Files to modify**:
- `go.mod`

---

## Prompt 2: Add Answer Input Method Configuration

**Context**: We need to add a configuration option to control how answers are input in the Q&A flow.

**Task**: Add `AnswerInputMethod` field to the config:
1. Update `pkg/config/config.go`:
   - Add `AnswerInputMethod string` field with yaml tag `answer_input_method`
   - Add comment explaining the three possible values: "readline", "editor", "readline_with_preview"
   - Default to "readline_with_preview" if empty or invalid

**Requirements**:
- Field should be optional (omitempty)
- Default value should be "readline_with_preview"
- Valid values: "readline", "editor", "readline_with_preview"

**Files to modify**:
- `pkg/config/config.go`

---

## Prompt 3: Create Input Helper Module

**Context**: We need to create a new module for handling enhanced answer input with readline support and editor switching.

**Task**: Create `pkg/qa/input.go` with the following functions:
1. `ReadAnswerWithReadline(prompt string, method string) (string, error)`:
   - Create readline instance with the prompt
   - Read input line by line
   - Detect `:edit` or `:e` command (at start of line after trimming)
   - If command detected, extract content after command (or empty if just command)
   - Open editor with extracted content
   - Return edited content or original input
   - Fallback to standard input if readline initialization fails

2. `readAnswerStandard(prompt string) (string, error)`:
   - Fallback function using standard bufio.Reader
   - Used when readline fails

**Requirements**:
- Handle readline initialization errors gracefully (fallback to standard input)
- Detect `:edit` and `:e` commands (case-sensitive, at start of line)
- Extract content after command for editor
- Handle editor errors gracefully

**Files to create**:
- `pkg/qa/input.go`

**Dependencies**:
- `github.com/chzyer/readline`
- `github.com/beekhof/jira-tool/pkg/editor`

---

## Prompt 4: Create Preview and Edit Loop Helper

**Context**: We need a function to show answer preview and allow editing in a loop.

**Task**: Add to `pkg/qa/input.go`:
1. `PreviewAndEditLoop(answer string, method string) (string, error)`:
   - If method is "readline", return answer immediately (no preview)
   - If method is "editor", return answer immediately (should have been handled earlier)
   - If method is "readline_with_preview":
     - Show preview: "Your answer: [answer]"
     - Prompt: "Edit? [y/N]"
     - If yes, open editor with current answer
     - Loop to show preview again after editing
     - Continue until user accepts (any input other than "y"/"yes")

**Requirements**:
- Respect the input method configuration
- Loop until user accepts
- Handle editor errors gracefully
- Use bufio.Reader for prompts (not readline, to avoid conflicts)

**Files to modify**:
- `pkg/qa/input.go`

---

## Prompt 5: Update RunQnAFlow to Use Enhanced Input

**Context**: We need to update the Q&A flow to use the new enhanced input functions.

**Task**: Modify `pkg/qa/flow.go`:
1. Add `answerInputMethod string` parameter to `RunQnAFlow` function
2. Replace `reader.ReadString('\n')` with `ReadAnswerWithReadline`
3. After reading answer, call `PreviewAndEditLoop` if method supports it
4. Handle all three input methods:
   - "readline": Use readline, no preview
   - "editor": Open editor immediately for each question
   - "readline_with_preview": Use readline, then preview/edit loop

**Requirements**:
- Maintain backward compatibility (existing behavior for rejection/skip/done)
- Handle editor mode (open editor immediately)
- Integrate preview/edit loop for readline_with_preview mode
- Preserve all existing functionality

**Files to modify**:
- `pkg/qa/flow.go`

---

## Prompt 6: Update All RunQnAFlow Call Sites

**Context**: All call sites of `RunQnAFlow` need to pass the answer input method from config.

**Task**: Update all call sites to pass `cfg.AnswerInputMethod`:
1. `pkg/review/workflow.go`: Pass `cfg.AnswerInputMethod` as new parameter
2. `cmd/review.go`: Load config and pass `cfg.AnswerInputMethod`
3. `cmd/create.go`: Load config and pass `cfg.AnswerInputMethod`
4. `cmd/accept.go`: Load config and pass `cfg.AnswerInputMethod`

**Requirements**:
- All call sites must pass the config value
- Default to "readline_with_preview" if config is empty
- Ensure config is loaded before calling `RunQnAFlow`

**Files to modify**:
- `pkg/review/workflow.go`
- `cmd/review.go`
- `cmd/create.go`
- `cmd/accept.go`

---

## Prompt 7: Handle Editor Mode

**Context**: When input method is "editor", we should open the editor immediately for each question instead of using readline.

**Task**: Update `ReadAnswerWithReadline` or create separate function:
1. If method is "editor", skip readline and open editor immediately
2. Return edited content
3. This should happen before the preview/edit loop

**Requirements**:
- Editor mode should bypass readline entirely
- Open editor with empty content for each question
- Return edited content directly

**Files to modify**:
- `pkg/qa/input.go`
- `pkg/qa/flow.go`

---

## Prompt 8: Add Tests for Input Functions

**Context**: We need tests to verify the input functions work correctly.

**Task**: Create `pkg/qa/input_test.go`:
1. Test `ReadAnswerWithReadline` with mock readline
2. Test `:edit` command detection
3. Test `PreviewAndEditLoop` with different methods
4. Test fallback to standard input
5. Test editor error handling

**Requirements**:
- Mock readline library where possible
- Test all three input methods
- Test edge cases (empty input, editor errors, etc.)

**Files to create**:
- `pkg/qa/input_test.go`

---

## Prompt 9: Update Documentation

**Context**: Update README and other documentation to explain the new input method configuration.

**Task**: 
1. Update `README.md` to document `answer_input_method` config option
2. Explain the three modes and when to use each
3. Add examples

**Requirements**:
- Clear explanation of each mode
- Examples of usage
- Configuration instructions

**Files to modify**:
- `README.md`

---

## Prompt 10: Final Integration and Testing

**Context**: Final integration and end-to-end testing of the complete feature.

**Task**:
1. Review all code for consistency
2. Ensure all imports are correct
3. Run `make build` and fix any compilation errors
4. Run `make test` and fix any test failures
5. Test end-to-end:
   - Test readline input (arrow keys, backspace)
   - Test `:edit` command during input
   - Test preview/edit loop
   - Test all three config options
   - Test rejection/skip/done still work
   - Test fallback behavior

**Requirements**:
- Everything compiles
- All tests pass
- End-to-end functionality works
- No regressions in existing functionality

**Files to review**:
- All modified files
- Integration points


