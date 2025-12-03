# Question Rejection Feature - Code Generation Prompts

## Prompt 1: Update Q&A Flow to Handle Question Rejection

**Context**: The Q&A flow currently allows users to answer questions or skip/done to end early. We need to add the ability to reject poor questions, which will skip the current question, generate a new one, and continue.

**Task**: Modify `pkg/qa/flow.go` to handle question rejection:
1. After reading user input, check if the answer is empty string or "reject" (case-insensitive)
2. If rejection detected:
   - Print: "Question rejected, generating a new one..."
   - Add "Q: [question] - REJECTED" to history (no answer recorded)
   - Decrement the loop counter (`i--`) to allow retry without counting toward maxQuestions
   - Continue the loop (don't break)
3. Keep existing behavior for "skip" and "done" (they should still end the loop)
4. Only add Q/A pairs to history for normal (non-rejected) answers

**Requirements**:
- Empty string should trigger rejection (not skip)
- "reject" keyword should be case-insensitive
- Rejected questions must be included in history for context
- Loop counter must be decremented to allow retry
- Existing skip/done behavior must be preserved

**Files to modify**:
- `pkg/qa/flow.go`

**Example behavior**:
```
Gemini asks: What is the main goal? > reject
Question rejected, generating a new one...
Gemini asks: Why is this feature needed? > 
Question rejected, generating a new one...
Gemini asks: Who will use this feature? > end users
```

---

## Prompt 2: Add Tests for Question Rejection

**Context**: We need comprehensive tests to verify the rejection feature works correctly.

**Task**: Create or update `pkg/qa/flow_test.go` with tests for:
1. Rejection with "reject" keyword
2. Rejection with empty string
3. Multiple rejections in a row
4. Rejection doesn't count toward maxQuestions
5. "skip" and "done" still end the loop
6. Normal answers still work
7. Rejected questions appear in history correctly

**Requirements**:
- Use table-driven tests where appropriate
- Mock the GeminiClient interface
- Verify history contains rejected questions with "- REJECTED" suffix
- Verify loop counter behavior (rejections don't count toward maxQuestions)
- Verify skip/done behavior is preserved

**Files to create/modify**:
- `pkg/qa/flow_test.go` (create if doesn't exist)

**Test cases to cover**:
- Single rejection
- Multiple rejections
- Rejection followed by normal answer
- All questions rejected
- Rejection with different cases ("REJECT", "Reject", "reject")

---

## Prompt 3: Verify Integration and Update Documentation

**Context**: Final verification that the feature works end-to-end and documentation is updated.

**Task**:
1. Review `pkg/qa/flow.go` for correctness
2. Run `make test` and fix any test failures
3. Run `make build` and fix any compilation errors
4. Update function comments in `pkg/qa/flow.go` to document rejection behavior
5. Test manually:
   - Run `jira create` and reject a question
   - Run `jira review` detail flow and reject a question
   - Verify rejected questions generate different follow-up questions

**Requirements**:
- All tests pass
- Code compiles without errors
- Function comments document rejection behavior
- Manual testing confirms feature works

**Files to review**:
- `pkg/qa/flow.go`
- `pkg/qa/flow_test.go`
- Any other files that use `RunQnAFlow`


