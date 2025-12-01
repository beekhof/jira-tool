# Enhanced Answer Input Feature - Specification

## Overview
Improve the Q&A flow answer input experience by adding readline-enhanced terminal input with optional editor switching and preview/edit loop functionality.

## Requirements

### Core Functionality
1. **Enhanced Terminal Input (Readline)**
   - Use readline library for better line editing
   - Support arrow keys for navigation
   - Support backspace/delete for editing
   - Support basic text editing within the terminal

2. **Editor Switching Command**
   - Allow `:edit` or `:e` command during input to switch to external editor
   - When command is entered, open current input in editor
   - Return edited content and continue with Q&A flow

3. **Preview and Edit Loop**
   - After entering answer, show simple preview: "Your answer: [answer]"
   - Prompt: "Edit? [y/N]"
   - If yes, open in editor
   - After editing, show preview again
   - Allow another edit or accept (loop until satisfied)

4. **Configuration**
   - Add `answer_input_method` config option with values:
     - `"readline"` - Use readline only, no preview/edit
     - `"editor"` - Always use editor for answers
     - `"readline_with_preview"` - Use readline + preview/edit loop (default)
   - Default: `"readline_with_preview"`

5. **No History**
   - Each question is independent
   - No recall of previous answers

6. **Default Behavior**
   - Readline + preview (edit optional) - fastest for common case
   - Only show preview/edit if user wants to edit

## Q&A Summary

**Q1:** What types of answers are you typically entering?
**A:** Short single-line answers (1-2 sentences)

**Q2:** How often do you need to edit your answers?
**A:** Sometimes (occasional edits)

**Q3:** What editing capabilities are most important?
**A:** Basic navigation (arrow keys, backspace)

**Q4:** Preferred workflow speed?
**A:** Fast (minimal steps, stay in terminal)

**Q5:** Should this be configurable or a single approach?
**A:** Configurable (let users choose)

**Q6:** Readline behavior?
**A:** Use readline by default, but allow a command (like `:edit` or `:e`) to switch to editor mid-input

**Q7:** Preview display format?
**A:** Simple: show the answer and prompt "Edit? [y/N]"

**Q8:** Edit loop behavior?
**A:** After editing, show preview and allow another edit or accept

**Q9:** Configuration options?
**A:** Multiple options: `answer_input_method: "readline" | "editor" | "readline_with_preview"` (default: readline_with_preview)

**Q10:** History/recall?
**A:** No history (each question is independent)

**Q11:** Default behavior?
**A:** Readline + preview (edit optional) - fastest for common case

## Architecture

### Modified Components
- `pkg/qa/flow.go`: Update `RunQnAFlow` to use enhanced input
  - Add readline support for input
  - Detect `:edit` or `:e` command
  - Implement preview/edit loop
  - Respect `answer_input_method` config

- `pkg/config/config.go`: Add `AnswerInputMethod` field

- `pkg/editor/editor.go`: Already exists, reuse for editing answers

### New Dependencies
- Go readline library (e.g., `github.com/chzyer/readline` or `github.com/peterh/liner`)

### Key Implementation Details
- Readline input: Replace `reader.ReadString('\n')` with readline library
- Command detection: Check if input starts with `:edit` or `:e`
- Editor switching: If command detected, open current input in editor
- Preview loop: After input, show preview and allow editing
- Config handling: Check `cfg.AnswerInputMethod` to determine behavior

## User Experience Flow

### Default Flow (readline_with_preview):
1. Gemini asks question
2. User types answer using readline (arrow keys, backspace work)
3. User can type `:edit` or `:e` to switch to editor mid-input
4. User presses Enter to submit
5. Show preview: "Your answer: [answer]"
6. Prompt: "Edit? [y/N]"
7. If yes: open in editor, show preview again, loop
8. If no: continue to next question

### Editor-only Flow (editor):
1. Gemini asks question
2. Automatically open editor
3. User edits and saves
4. Continue to next question

### Readline-only Flow (readline):
1. Gemini asks question
2. User types answer using readline
3. User presses Enter to submit
4. Continue to next question (no preview/edit)

## Error Handling
- If readline library fails to initialize, fallback to standard input
- If editor command fails, show error and allow retry or continue with current input
- If config value is invalid, use default (`readline_with_preview`)

## Testing Plan
1. **Test readline input**
   - Verify arrow keys work
   - Verify backspace works
   - Verify basic editing works

2. **Test editor command**
   - Type `:edit` during input
   - Verify editor opens with current input
   - Verify edited content is used

3. **Test preview/edit loop**
   - Enter answer
   - Verify preview is shown
   - Edit answer
   - Verify preview shown again
   - Accept answer
   - Verify loop exits

4. **Test configuration**
   - Test each `answer_input_method` value
   - Test invalid config value (should use default)
   - Test missing config (should use default)

5. **Test rejection handling**
   - Verify `reject` and empty string still work
   - Verify `skip` and `done` still work

## Edge Cases
- User types `:edit` at start of input (empty editor)
- User types `:edit` in middle of answer (editor has partial answer)
- User cancels editor (should return to readline or use original input)
- Very long answers (should still work with readline and editor)

