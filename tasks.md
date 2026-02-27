# Tasks

---

- [x] **Project scaffolding**

Initialise the Go module and create the directory structure described in AGENTS.md. Set up `go.mod` with dependencies (`gopkg.in/yaml.v3`, `modernc.org/sqlite`). Create placeholder files so the package structure compiles. Add a minimal `main.go` in `cmd/tracker/` that prints usage and exits.

Acceptance criteria:
- `go build ./...` succeeds
- `go test ./...` runs (even if no tests yet)
- Directory structure matches AGENTS.md

---

- [x] **Define core types in `internal/model/`**

Create the Go types for the data model: `Issue`, `Event`, `Status`, `Priority`, `IssueType`, `Claim`. Status, Priority, and IssueType should be string types with constants. Event should be a struct that can represent all 10 event types described in SPEC.md, with fields that are populated depending on the event type.

Acceptance criteria:
- All types compile
- String constants defined for all status, priority, and type values
- Tests verify that the type constants match the values listed in SPEC.md

---

- [x] **YAML event file parser (`internal/store/`)**

Implement reading and parsing of YAML event files. Given a file path, parse the multi-document YAML stream into a slice of `Event` structs. Handle the `---` document separators correctly. On encountering a malformed YAML document, return an error identifying which event (by index/position) is corrupt. Do not skip or silently ignore bad events.

Acceptance criteria:
- Parses all 10 event types correctly
- Returns structured error on malformed YAML with position info
- Handles empty files (no events)
- Handles files with only a `created` event
- Handles multi-event files with all event types
- Tests use fixture files in the test directory

---

- [x] **YAML event file writer (`internal/store/`)**

Implement appending events to a YAML file. Given a file path and an `Event`, serialise it as a YAML document and append it to the file with a `---` separator. Use file locking (`flock`) to prevent concurrent appends to the same file.

Acceptance criteria:
- Appending to an existing file preserves all prior content
- New event is correctly separated by `---`
- File locking prevents corruption under concurrent writes
- Test: two goroutines appending to the same file produces a valid file
- Written events round-trip through the parser

---

- [ ] **Event materialisation**

Implement replaying a sequence of events to produce the current state of an issue. Given a `[]Event`, produce an `Issue` struct reflecting the final state. Apply last-write-wins for description, title, priority, acceptance criteria. Track current status, claim state, and dependency list.

Acceptance criteria:
- Created event produces a complete Issue
- Subsequent status_changed, title_changed, priority_changed, description_updated, acceptance_criteria_updated, dependencies_changed events all update the correct fields
- Claimed/released events update claim state
- Comment events don't change materialised state
- parse_error events are skipped
- Test with a sequence that exercises all event types

---

- [ ] **SQLite index — schema and rebuild (`internal/index/`)**

Implement creating the SQLite database with the schema from SPEC.md. Implement `Rebuild()`: scan all `.yaml` files in the issues directory, parse each, materialise current state, and populate the database. Drop and recreate tables on rebuild.

Acceptance criteria:
- Schema matches SPEC.md exactly (issues, dependencies, claims tables)
- Rebuild from a directory of fixture files produces correct DB state
- Rebuild is idempotent
- Dependencies are correctly populated
- Claims are correctly populated
- Handles empty issues directory

---

- [ ] **SQLite index — query operations**

Implement the query functions the CLI will need:
- `ListIssues(filters)` — filter by status, priority, type
- `ReadyIssues()` — the key query from SPEC.md (open, unclaimed, deps resolved, ordered by priority)
- `GetIssueMeta(id)` — fetch metadata for a single issue
- `ClaimIssue(id, agentID, sessionID)` — atomic claim (fails if already claimed)
- `ReleaseIssue(id)` — remove claim
- `NextID()` — return the next available issue ID

Acceptance criteria:
- Ready query correctly excludes claimed, blocked, draft, and dependency-unmet issues
- Claim is atomic — two concurrent claims on the same issue, one succeeds, one fails
- Filters compose correctly (e.g. status=open AND priority=high)
- Tests set up DB state directly, don't depend on file parsing

---

- [ ] **Store layer — tie files and index together (`internal/store/`)**

Create a `Store` type that owns both the filesystem path and the SQLite index. Provide methods that combine file operations with index updates:
- `Create(issue)` — write the content file, update the index
- `Append(id, event)` — append to content file, update the index
- `Show(id)` — read the content file, materialise, check stale claim
- `Rebuild()` — delegate to index rebuild

This is the layer the CLI commands call.

Acceptance criteria:
- Create writes a valid content file and the issue appears in the index
- Append adds to the file and the index reflects the change
- Show returns the full materialised issue including description and acceptance criteria
- Stale claim detection works (use injected time)
- Tests use temp directories

---

- [ ] **CLI — init command**

Implement `tracker init --prefix PROJ`. Creates `.tracker/` directory structure, writes `config.yaml` with the prefix, creates empty `issues/` directory, initialises SQLite database, creates `.gitignore` containing `db.sqlite`.

Acceptance criteria:
- Running in an empty directory creates the expected structure
- Config file contains the prefix
- Running twice is an error (or idempotent — decide and document)
- SQLite database is created with correct schema

---

- [ ] **CLI — create command**

Implement `tracker create --title "..." --description "..." [--priority medium] [--type task]`. Assigns next available ID, writes the content file with a `created` event, updates the index. Prints the new issue ID.

Acceptance criteria:
- Creates the content file with correct YAML
- Assigns sequential IDs
- Defaults: priority=medium, type=task
- Index is updated
- Prints the new ID to stdout

---

- [ ] **CLI — show command**

Implement `tracker show <id> [--events] [--json]`. Reads the content file, materialises current state, displays it. With `--events`, shows the full event history. Accepts bare numeric IDs or prefixed IDs. Performs stale claim detection.

Acceptance criteria:
- Displays all materialised fields including description and acceptance criteria
- `--events` shows full history
- `--json` outputs structured JSON
- Accepts `42`, `PROJ-42`, `PROJ-042` for the same issue
- Reports stale claims

---

- [ ] **CLI — list command**

Implement `tracker list [--status ...] [--priority ...] [--type ...] [--json]`. Queries the SQLite index and displays matching issues. Default (no filters) shows all non-done, non-cancelled issues.

Acceptance criteria:
- Filters work individually and in combination
- Default excludes done and cancelled
- `--json` outputs structured JSON
- Output includes id, title, status, priority, type

---

- [ ] **CLI — ready command**

Implement `tracker ready [--json]`. Runs the ready query and displays results.

Acceptance criteria:
- Shows only open, unclaimed, dependency-resolved issues
- Ordered by priority (high first)
- `--json` outputs structured JSON

---

- [ ] **CLI — update command**

Implement `tracker update <id> --status <s> --priority <p> --title "..."`. Each flag generates the appropriate event and appends it to the content file. Updates the index.

Acceptance criteria:
- Each flag generates the correct event type
- Multiple flags in one command generate multiple events
- Invalid status/priority values are rejected
- Index is updated

---

- [ ] **CLI — comment command**

Implement `tracker comment <id> "message"`. Appends a comment event to the content file.

Acceptance criteria:
- Comment appears in the event log
- Does not change materialised state in the index
- Works with bare and prefixed IDs

---

- [ ] **CLI — claim and release commands**

Implement `tracker claim <id> --agent <agent-id> --session <session-id>` and `tracker release <id>`. Claim appends a `claimed` event and updates the index atomically. Release appends a `released` event recording the previous claimant.

Acceptance criteria:
- Claiming an already-claimed issue fails with a clear error
- Release records previous claimant in the event
- Any user can release any claim
- Index is updated correctly

---

- [ ] **CLI — rebuild command**

Implement `tracker rebuild`. Deletes and regenerates the SQLite database from content files.

Acceptance criteria:
- Produces identical DB state to what incremental operations would
- Handles empty issues directory
- Handles corrupted files (reports error, halts on that file)
- Prints summary of what was rebuilt

---

- [ ] **End-to-end integration tests**

Write tests that exercise the full workflow through the CLI: init, create several issues with dependencies, list, ready, claim, comment, update status, release, show with events. Verify the entire lifecycle works correctly.

Acceptance criteria:
- Full lifecycle test passes
- Dependency resolution is correct in ready output
- Claim/release cycle works
- Rebuild produces consistent state
