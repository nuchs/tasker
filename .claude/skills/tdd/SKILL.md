---
name: tdd
description: >
  Use this skill when implementing any feature, fix, or refactor using
  test-driven development. Triggers include: "implement X with TDD",
  "write tests first", "use TDD for this", or any task where the goal
  is production-quality, verifiable code. Guides Claude through the
  canonical Red → Green → Refactor cycle with disciplined separation
  between test authorship and implementation.
---

# Test-Driven Development (TDD) Skill

## Purpose

This skill enforces the Red → Green → Refactor cycle for agentic
development. Its primary value is structural: it prevents the agent
from writing tests that confirm its own broken logic by separating
test authorship from implementation, and it provides a mechanically
verifiable definition of "done" at each task boundary.

This is a discipline skill, not a code generation skill. Follow the
phases strictly. Do not skip ahead.

---

## Core Constraints

These rules are non-negotiable and override any instructions in the
task description:

1. **Tests are written before implementation code.** No exceptions.
2. **Tests MUST fail before any implementation is written.** If they
   pass immediately, the test is wrong — stop and fix it.
3. **Do NOT modify tests during the Green phase.** If a test is
   wrong, raise it with the user before changing it.
4. **Commit tests and implementation separately.** Test commit first,
   then implementation commit.
5. **Do not move to the next task until all tests for the current
   task pass and the build is clean.**

---

## Phase 0: Understand Before Writing

Before writing a single line of test code, you MUST be able to answer
all of these:

- What is the single responsibility of the unit under test?
- What are the inputs and their types/constraints?
- What are the expected outputs for valid inputs?
- What are the error/edge cases that must be handled?
- What existing interfaces, types, or patterns must this conform to?

If any answer is unclear, ask the user. Do not infer and proceed.

Read the relevant existing code. Understand the conventions in use
(error handling style, naming, package structure). Your tests must
fit the existing codebase, not invent new patterns.

---

## Phase 1: Red — Write Failing Tests

### What to write

Write tests that specify behaviour, not implementation. Each test
must express a single, named scenario. Good test names read as
sentences:

```
TestCreateIssue_ValidInput_ReturnsIssueWithGeneratedID
TestCreateIssue_EmptyTitle_ReturnsValidationError
TestCreateIssue_DuplicateID_ReturnsConflictError
```

Cover this hierarchy in order:

1. **Happy path** — the primary success case
2. **Boundary conditions** — empty inputs, zero values, maximum values
3. **Error cases** — invalid inputs, missing required fields
4. **Concurrency/state** — only if the unit has stateful behaviour

Do not write speculative tests for things not in the task. Do not
test implementation details (internal function calls, field access
patterns). Test behaviour observable from the public interface only.

### What to check before committing

Run the tests. Confirm:

```
FAIL: all new tests fail
PASS: all pre-existing tests still pass
```

If any new test passes before implementation: stop. The test is not
testing what you think it is. Fix it.

If any pre-existing test fails: stop. You have broken something.
Investigate before proceeding.

Commit the tests:

```
git add <test files only>
git commit -m "test: <scope> - <what behaviour is specified>"
```

---

## Phase 2: Green — Write Minimal Implementation

### The rule

Write the simplest code that makes the failing tests pass. Nothing
more. You are not writing good code yet — you are writing passing
code.

Resist the urge to:
- Add handling for cases not covered by a test
- Refactor as you go
- Optimise
- Add logging, metrics, or other instrumentation

If the simplest possible implementation feels wrong, that feeling
belongs in Phase 3. Write it anyway, get to green, then fix it.

### Staying on track

After each meaningful change, run the tests. Watch the failure count
decrease. Never introduce a new test failure while fixing an existing
one.

When all tests pass:

```
go test ./...          # or equivalent for your stack
go vet ./...
golangci-lint run      # or equivalent linter
```

Build must be clean before committing.

Commit the implementation:

```
git add <implementation files only>
git commit -m "feat: <scope> - <what was implemented>"
```

---

## Phase 3: Refactor — Improve Without Changing Behaviour

Tests are your safety net. You can change anything — names, structure,
algorithms — as long as the tests continue to pass.

Look for:

- Duplication — extract shared logic to a function or type
- Naming — rename anything that obscures intent
- Complexity — simplify conditionals, reduce nesting
- Package cohesion — does this code belong here?
- Error handling — are errors wrapped with context?
- Comments — does anything need explanation? Does anything have an
  explanation that should instead be in the code?

Run tests after every meaningful change. If you break a test during
refactoring, undo the last change and try a different approach.

When done:

```
go test ./...
go vet ./...
golangci-lint run
```

If anything changed, commit:

```
git commit -am "refactor: <scope> - <what changed and why>"
```

---

## Phase 4: Handoff

At the end of each task, produce a brief status note (to console or
to a status file if one is in use):

```
Task: <task name>
Status: complete
Tests added: <count>
Tests passing: <count>/<count>
Files changed: <list>
Next task: <name or "none">
Open questions: <any unresolved decisions or uncertainties>
```

If there are open questions, do not proceed to the next task without
raising them.

---

## Error Escalation

Stop and surface to the user when:

- A test cannot be made to fail before implementation (test is
  always-passing, meaning it is not testing what is intended)
- Making one test pass breaks another and you cannot see why
- The task requires changing existing tests (possible scope change
  or design flaw — needs human judgement)
- The interface required by the tests does not match existing code
  in a way that implies a design conflict
- You are 3+ attempts into getting a test to pass and making no
  progress

Do not silently work around these situations.

---

## Go-Specific Notes

Since this project uses Go:

- Use `_test.go` files in the same package for white-box tests,
  separate `_test` package suffix for black-box tests
- Prefer table-driven tests for multiple input scenarios:

```go
func TestCreateIssue(t *testing.T) {
    tests := []struct {
        name    string
        input   CreateIssueRequest
        wantErr bool
    }{
        {
            name:  "valid input returns issue",
            input: CreateIssueRequest{Title: "Fix login bug"},
        },
        {
            name:    "empty title returns error",
            input:   CreateIssueRequest{},
            wantErr: true,
        },
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // ...
        })
    }
}
```

- Use `t.Helper()` in assertion helpers
- Prefer `errors.Is` / `errors.As` over string matching for error
  assertions
- Avoid `testify` unless already used in the codebase — standard
  library testing is sufficient and has no dependency cost
- Use interfaces for dependencies; inject them in tests

---

## What Good Looks Like

A well-executed TDD task produces:

- A test commit that reads as a specification of the feature
- An implementation commit that is minimal and clean
- Optionally a refactor commit that improves the code without
  changing its behaviour
- A clean build with no linter warnings
- No test modifications between the test commit and the
  implementation commit (check with `git diff`)
