# AGENTS.md — Tracker Project

## Project Overview

Tracker is a local, file-based issue tracker for AI coding agents, written in Go. See `SPEC.md` for the full design and rationale.

## Language & Build

- Go (latest stable version)
- Module path: to be set on `go mod init`
- All code must pass `go vet`, `go test -race ./...`, and have no lint warnings
- Use the standard library where possible. External dependencies: `gopkg.in/yaml.v3` for YAML parsing, `modernc.org/sqlite` (pure Go SQLite, no CGO)

## Code Standards

- Follow standard Go conventions: `gofmt`, short variable names in narrow scopes, exported names for public API
- Error handling: return errors, don't panic. Wrap errors with `fmt.Errorf("context: %w", err)` for traceability
- No global state. Pass dependencies explicitly
- Tests go in `_test.go` files alongside the code they test. Use `testing.T` and subtests. Each test creates its own temp directory

## Testing

- Every public function must have tests
- Tests must be deterministic — no reliance on wall clock time (inject time where needed)
- Use `t.TempDir()` for filesystem tests
- Run the full suite with: `go test -race ./...`
- Test both happy paths and error cases (malformed YAML, missing files, concurrent access)

## Project Structure

```
cmd/
  tracker/
    main.go           # CLI entry point
internal/
  store/              # Content file read/write, event parsing
  index/              # SQLite index operations, rebuild
  cli/                # Command implementations
  model/              # Types: Issue, Event, Status, Priority, etc.
SPEC.md               # Design specification
AGENTS.md             # This file
tasks.md              # Work to be done
issues.md             # Bugs and problems to fix
go.mod
go.sum
```

## Task & Issue Tracking

This project tracks work in `tasks.md` and `issues.md` until the tracker is self-hosting.

### Format

Each item is separated by a horizontal rule (`---`). Items have a checkbox, a short title, and a body. Example:

```markdown
---

- [ ] **Title of task**

Description of what needs doing. Include enough detail
that an agent can pick this up without further context.

Acceptance criteria:
- Thing one works
- Thing two works
```

A checked box (`- [x]`) means done. Items are worked on from top to bottom unless dependencies are noted.

### Workflow

1. Before starting work, read `tasks.md` to find the next unchecked task.
2. Work on one task at a time.
3. When the task is complete and tests pass, check the box in tasks.md, commit your work, and stop. Do not continue to the next task.
4. If you discover a bug or problem during work, add it to `issues.md`.
5. Do not modify the SPEC.md without discussing with the human first.

## Commit Conventions

- One logical change per commit
- Commit message format: short summary line, blank line, detail if needed
- Reference task/issue if applicable

## Key Design Decisions (Read SPEC.md for Detail)

- Content files (YAML event logs) are the source of truth
- SQLite index is disposable — can always be rebuilt from files
- No enforced state machine on issue statuses
- Append-only event files — never modify or delete events
- Corruption halts processing until resolved
- Claims prevent two agents working the same issue
- Stale claim detection happens on access, not via background process
