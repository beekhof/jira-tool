# Question Rejection Feature - Implementation Blueprint

## Overview
This blueprint outlines the step-by-step implementation of the question rejection feature for the Q&A flow.

## Implementation Steps

### Step 1: Update RunQnAFlow Function Signature
- No signature changes needed
- Document the new rejection behavior in function comments

### Step 2: Add Rejection Detection Logic
- After reading user input, check if answer is:
  - Empty string (after trimming)
  - "reject" (case-insensitive comparison)
- If rejection detected, handle separately from normal answers and skip/done

### Step 3: Implement Rejection Handling
- When rejection detected:
  - Print feedback message: "Question rejected, generating a new one..."
  - Add "Q: [question] - REJECTED" to history
  - Decrement loop counter (or use separate attempt counter) to allow retry
  - Continue loop (don't break)

### Step 4: Separate Rejection from Skip/Done
- Keep existing logic for "skip" and "done" to break the loop
- Ensure empty string is handled as rejection (not skip) when it's a rejection
- Note: Currently empty string breaks the loop - need to change this behavior

### Step 5: Update Loop Counter Logic
- Current: `for i := 0; i < maxQuestions; i++`
- New: Track successful questions separately, or decrement `i` on rejection
- Option A: Use separate counter for successful Q/A pairs
- Option B: Decrement `i` on rejection (simpler)
- Recommended: Option B (decrement `i`)

### Step 6: Test Rejection Flow
- Test with "reject" keyword
- Test with empty string
- Test multiple rejections
- Test rejection doesn't count toward maxQuestions
- Test skip/done still work

### Step 7: Test Context Inclusion
- Verify rejected questions appear in history
- Verify next question generation uses rejection context
- Test that Gemini generates different questions after rejection

### Step 8: Update Documentation
- Update function comments
- Update any user-facing documentation
- Add examples of rejection usage

## Detailed Implementation Plan

### Modified File: `pkg/qa/flow.go`

**Current Loop Structure:**
```go
for i := 0; i < maxQuestions; i++ {
    question, err := client.GenerateQuestion(...)
    // ...
    answer, err := reader.ReadString('\n')
    answer = trimSpace(answer)
    question = trimSpace(question)
    
    history = append(history, fmt.Sprintf("Q: %s", question))
    history = append(history, fmt.Sprintf("A: %s", answer))
    
    if answer == "" || answer == "skip" || answer == "done" {
        break
    }
}
```

**New Loop Structure:**
```go
for i := 0; i < maxQuestions; i++ {
    question, err := client.GenerateQuestion(...)
    // ...
    answer, err := reader.ReadString('\n')
    answer = trimSpace(answer)
    question = trimSpace(question)
    
    // Handle rejection (empty string or "reject")
    if answer == "" || strings.EqualFold(answer, "reject") {
        fmt.Println("Question rejected, generating a new one...")
        history = append(history, fmt.Sprintf("Q: %s - REJECTED", question))
        i-- // Decrement to retry without counting toward maxQuestions
        continue
    }
    
    // Handle skip/done (end Q&A loop)
    if answer == "skip" || answer == "done" {
        break
    }
    
    // Normal answer
    history = append(history, fmt.Sprintf("Q: %s", question))
    history = append(history, fmt.Sprintf("A: %s", answer))
}
```

**Key Changes:**
1. Check for rejection BEFORE checking skip/done
2. Add rejection feedback message
3. Add rejected question to history with "- REJECTED" suffix
4. Decrement loop counter to allow retry
5. Continue loop (don't break)
6. Move skip/done check after rejection check
7. Only add Q/A pair to history for normal answers

## Testing Checklist

- [ ] Rejection with "reject" keyword works
- [ ] Rejection with empty string works
- [ ] Multiple rejections work
- [ ] Rejection doesn't count toward maxQuestions
- [ ] "skip" and "done" still end the loop
- [ ] Normal answers still work
- [ ] Rejected questions appear in history correctly
- [ ] Next question generation uses rejection context
- [ ] Edge case: all questions rejected
- [ ] Edge case: reject then answer normally

## Files to Modify

1. `pkg/qa/flow.go` - Main implementation
2. `pkg/qa/flow_test.go` - Add tests (if test file exists or create new)

## Dependencies

- No new dependencies required
- Uses existing `strings` package for case-insensitive comparison


