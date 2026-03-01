# Manual Test Suite — tracker

## Preamble

Each test must be run in its own isolated directory under `./testdata`. Before running any test,
generate a fresh directory and work inside it:

```sh
mkdir -p ./testdata
TEST_DIR=$(mktemp -d ./testdata/tracker-test-XXXXXX)
cd "$TEST_DIR"
```

The `tracker` binary must be built before running any test:

```sh
# From the repository root
go build -o /usr/local/bin/tracker ./cmd/tracker
# or place it on PATH some other way; adjust calls below if using a local path
```

Each test is independent. Do not reuse a directory across tests unless the test explicitly
says to continue from a prior step. Clean up with `rm -rf "$TEST_DIR"` when done, or remove all test directories with `rm -rf ./testdata`.

---

## T-001 — Init: happy path

**Purpose:** Verify that `tracker init` creates the expected directory structure and
reports success.

**Steps:**

```sh
mkdir -p ./testdata && TEST_DIR=$(mktemp -d ./testdata/tracker-test-XXXXXX) && cd "$TEST_DIR"
tracker init --prefix PROJ
ls -a .tracker/
ls .tracker/issues/
cat .tracker/config.yaml
cat .tracker/.gitignore
```

**Expected output:**

- `tracker init` prints: `Initialised tracker with prefix PROJ in <path>/.tracker`
- `ls -a .tracker/` lists: `.  ..  .gitignore  config.yaml  db.sqlite  issues`
- `ls .tracker/issues/` produces no output (empty directory)
- `cat .tracker/config.yaml` contains `prefix: PROJ`
- `cat .tracker/.gitignore` contains `db.sqlite`

---

## T-002 — Init: missing --prefix flag

**Purpose:** Verify that `tracker init` without `--prefix` exits with an error and
does not create any files.

**Steps:**

```sh
mkdir -p ./testdata && TEST_DIR=$(mktemp -d ./testdata/tracker-test-XXXXXX) && cd "$TEST_DIR"
tracker init
echo "exit code: $?"
ls .tracker 2>&1
```

**Expected output:**

- `tracker` prints an error to stderr containing `--prefix is required`
- Exit code is non-zero
- `ls .tracker` prints `No such file or directory` (nothing was created)

---

## T-003 — Init: double initialisation

**Purpose:** Verify that running `tracker init` in an already-initialised directory
fails without modifying the existing setup.

**Steps:**

```sh
mkdir -p ./testdata && TEST_DIR=$(mktemp -d ./testdata/tracker-test-XXXXXX) && cd "$TEST_DIR"
tracker init --prefix PROJ
tracker init --prefix OTHER
echo "exit code: $?"
cat .tracker/config.yaml
```

**Expected output:**

- First `tracker init` succeeds.
- Second `tracker init` prints an error to stderr containing `already initialised`
- Exit code of second call is non-zero
- `config.yaml` still contains `prefix: PROJ` (unchanged)

---

## T-004 — Create: minimal issue

**Purpose:** Verify that `tracker create` with only `--title` creates an issue with
the correct defaults and prints the assigned ID.

**Steps:**

```sh
mkdir -p ./testdata && TEST_DIR=$(mktemp -d ./testdata/tracker-test-XXXXXX) && cd "$TEST_DIR"
tracker init --prefix PROJ
tracker create --title "My first task"
ls .tracker/issues/
```

**Expected output:**

- `tracker create` prints: `PROJ-0001`
- `ls .tracker/issues/` lists: `PROJ-0001.yaml`

---

## T-005 — Create: full options

**Purpose:** Verify that all optional flags (`--description`, `--priority`, `--type`,
`--acceptance-criteria`) are accepted and the ID is correctly incremented.

**Steps:**

```sh
mkdir -p ./testdata && TEST_DIR=$(mktemp -d ./testdata/tracker-test-XXXXXX) && cd "$TEST_DIR"
tracker init --prefix BUG
tracker create --title "First task" --priority high --type task \
  --description "Do the first thing" \
  --acceptance-criteria "It works"
tracker create --title "First bug" --priority low --type issue \
  --description "Something broke"
```

**Expected output:**

- First create prints: `BUG-0001`
- Second create prints: `BUG-0002`
- No errors

---

## T-006 — Create: missing --title

**Purpose:** Verify that `tracker create` without `--title` exits with an error.

**Steps:**

```sh
mkdir -p ./testdata && TEST_DIR=$(mktemp -d ./testdata/tracker-test-XXXXXX) && cd "$TEST_DIR"
tracker init --prefix PROJ
tracker create --priority high
echo "exit code: $?"
ls .tracker/issues/
```

**Expected output:**

- Error to stderr containing `--title is required`
- Exit code is non-zero
- `ls .tracker/issues/` produces no output (no file created)

---

## T-007 — Create: invalid priority

**Purpose:** Verify that an unrecognised priority value is rejected.

**Steps:**

```sh
mkdir -p ./testdata && TEST_DIR=$(mktemp -d ./testdata/tracker-test-XXXXXX) && cd "$TEST_DIR"
tracker init --prefix PROJ
tracker create --title "Bad priority" --priority urgent
echo "exit code: $?"
```

**Expected output:**

- Error to stderr containing `invalid priority` and `urgent`
- Exit code is non-zero

---

## T-008 — Create: invalid type

**Purpose:** Verify that an unrecognised issue type is rejected.

**Steps:**

```sh
mkdir -p ./testdata && TEST_DIR=$(mktemp -d ./testdata/tracker-test-XXXXXX) && cd "$TEST_DIR"
tracker init --prefix PROJ
tracker create --title "Bad type" --type epic
echo "exit code: $?"
```

**Expected output:**

- Error to stderr containing `invalid type` and `epic`
- Exit code is non-zero

---

## T-009 — Show: basic display

**Purpose:** Verify that `tracker show` displays all issue fields in a readable format,
accepting both bare and prefixed IDs.

**Steps:**

```sh
mkdir -p ./testdata && TEST_DIR=$(mktemp -d ./testdata/tracker-test-XXXXXX) && cd "$TEST_DIR"
tracker init --prefix PROJ
tracker create --title "My task" --priority high --type issue \
  --description "A detailed description" \
  --acceptance-criteria "All checks pass"
tracker show 1
tracker show PROJ-1
tracker show PROJ-0001
```

**Expected output (all three show calls produce the same output):**

```
PROJ-0001  My task
Status:    open
Priority:  high
Type:      issue

Description:
  A detailed description

Acceptance Criteria:
  All checks pass
```

---

## T-010 — Show: JSON output

**Purpose:** Verify that `tracker show --json` produces valid JSON with all fields.

**Steps:**

```sh
mkdir -p ./testdata && TEST_DIR=$(mktemp -d ./testdata/tracker-test-XXXXXX) && cd "$TEST_DIR"
tracker init --prefix PROJ
tracker create --title "JSON task" --priority medium --type task \
  --description "Desc" --acceptance-criteria "AC"
tracker show 1 --json
```

**Expected output:** Valid JSON object containing at minimum:

```json
{
  "id": 1,
  "display_id": "PROJ-0001",
  "type": "task",
  "title": "JSON task",
  "status": "open",
  "priority": "medium",
  "description": "Desc",
  "acceptance_criteria": "AC"
}
```

---

## T-011 — Show: event history

**Purpose:** Verify that `tracker show --events` prints a chronological event log
including the initial `created` event.

**Steps:**

```sh
mkdir -p ./testdata && TEST_DIR=$(mktemp -d ./testdata/tracker-test-XXXXXX) && cd "$TEST_DIR"
tracker init --prefix PROJ
tracker create --title "Track me"
tracker update 1 --status in_progress
tracker comment 1 "Making progress"
tracker show 1 --events
```

**Expected output:**

- Output contains `created` with the title `Track me`
- Output contains `status_changed` with detail `in_progress`
- Output contains `comment`
- Events are listed in chronological order (one per line with index, timestamp, event type)

---

## T-012 — Show: non-existent issue

**Purpose:** Verify that requesting an issue that does not exist produces a clear error.

**Steps:**

```sh
mkdir -p ./testdata && TEST_DIR=$(mktemp -d ./testdata/tracker-test-XXXXXX) && cd "$TEST_DIR"
tracker init --prefix PROJ
tracker show 99
echo "exit code: $?"
```

**Expected output:**

- Error to stderr
- Exit code is non-zero

---

## T-013 — List: default (excludes terminal statuses)

**Purpose:** Verify that `tracker list` without filters shows all non-terminal issues
(excludes `done` and `cancelled`) and hides nothing from active issues.

**Steps:**

```sh
mkdir -p ./testdata && TEST_DIR=$(mktemp -d ./testdata/tracker-test-XXXXXX) && cd "$TEST_DIR"
tracker init --prefix PROJ
tracker create --title "Open task"
tracker create --title "Done task"
tracker create --title "Cancelled task"
tracker update 2 --status done
tracker update 3 --status cancelled
tracker list
```

**Expected output:**

- List contains `Open task`
- List does NOT contain `Done task`
- List does NOT contain `Cancelled task`
- Each row shows: ID, status, priority, type, title

---

## T-014 — List: filter by status

**Purpose:** Verify `--status` filtering shows only issues with that exact status,
including terminal ones.

**Steps:**

```sh
mkdir -p ./testdata && TEST_DIR=$(mktemp -d ./testdata/tracker-test-XXXXXX) && cd "$TEST_DIR"
tracker init --prefix PROJ
tracker create --title "Task A"
tracker create --title "Task B"
tracker update 1 --status done
tracker list --status done
tracker list --status open
```

**Expected output:**

- `tracker list --status done` lists only `Task A`
- `tracker list --status open` lists only `Task B`

---

## T-015 — List: filter by priority

**Purpose:** Verify `--priority` filtering returns only issues of that priority.

**Steps:**

```sh
mkdir -p ./testdata && TEST_DIR=$(mktemp -d ./testdata/tracker-test-XXXXXX) && cd "$TEST_DIR"
tracker init --prefix PROJ
tracker create --title "High priority task" --priority high
tracker create --title "Low priority task" --priority low
tracker list --priority high
tracker list --priority low
```

**Expected output:**

- `tracker list --priority high` lists only `High priority task`
- `tracker list --priority low` lists only `Low priority task`

---

## T-016 — List: filter by type

**Purpose:** Verify `--type` filtering returns only issues of that type.

**Steps:**

```sh
mkdir -p ./testdata && TEST_DIR=$(mktemp -d ./testdata/tracker-test-XXXXXX) && cd "$TEST_DIR"
tracker init --prefix PROJ
tracker create --title "A task" --type task
tracker create --title "A bug" --type issue
tracker list --type task
tracker list --type issue
```

**Expected output:**

- `tracker list --type task` lists only `A task`
- `tracker list --type issue` lists only `A bug`

---

## T-017 — List: JSON output

**Purpose:** Verify that `tracker list --json` produces a valid JSON array.

**Steps:**

```sh
mkdir -p ./testdata && TEST_DIR=$(mktemp -d ./testdata/tracker-test-XXXXXX) && cd "$TEST_DIR"
tracker init --prefix PROJ
tracker create --title "First" --priority high
tracker create --title "Second" --priority low
tracker list --json
```

**Expected output:** Valid JSON array where each element contains:
`id`, `display_id`, `type`, `title`, `status`, `priority`

---

## T-018 — List: empty result

**Purpose:** Verify that `tracker list` on an empty (or fully-terminal) tracker
prints a friendly message rather than crashing.

**Steps:**

```sh
mkdir -p ./testdata && TEST_DIR=$(mktemp -d ./testdata/tracker-test-XXXXXX) && cd "$TEST_DIR"
tracker init --prefix PROJ
tracker list
```

**Expected output:**

```
no issues found
```

---

## T-019 — List: invalid status filter

**Purpose:** Verify that an unrecognised status value is rejected.

**Steps:**

```sh
mkdir -p ./testdata && TEST_DIR=$(mktemp -d ./testdata/tracker-test-XXXXXX) && cd "$TEST_DIR"
tracker init --prefix PROJ
tracker list --status pending
echo "exit code: $?"
```

**Expected output:**

- Error to stderr containing `invalid status` and `pending`
- Exit code is non-zero

---

## T-020 — Ready: no issues

**Purpose:** Verify that `tracker ready` on an empty tracker prints a friendly message.

**Steps:**

```sh
mkdir -p ./testdata && TEST_DIR=$(mktemp -d ./testdata/tracker-test-XXXXXX) && cd "$TEST_DIR"
tracker init --prefix PROJ
tracker ready
```

**Expected output:**

```
no ready issues
```

---

## T-021 — Ready: basic ready issue

**Purpose:** Verify that a freshly created open issue with no dependencies and no claim
appears in `tracker ready`.

**Steps:**

```sh
mkdir -p ./testdata && TEST_DIR=$(mktemp -d ./testdata/tracker-test-XXXXXX) && cd "$TEST_DIR"
tracker init --prefix PROJ
tracker create --title "Ready to go" --priority high
tracker ready
```

**Expected output:**

- Output contains `PROJ-0001`, `high`, `task`, `Ready to go`
- Columns: ID, priority, type, title

---

## T-022 — Ready: dependency blocks issue

**Purpose:** Verify that an issue with an unresolved dependency does not appear in
`tracker ready`, and becomes ready only after the dependency is done.

**Steps:**

```sh
mkdir -p ./testdata && TEST_DIR=$(mktemp -d ./testdata/tracker-test-XXXXXX) && cd "$TEST_DIR"
tracker init --prefix PROJ
tracker create --title "Blocker"
tracker create --title "Dependent"
# Set issue 2 to depend on issue 1 by editing the YAML directly
cat >> .tracker/issues/PROJ-0002.yaml << 'EOF'
---
event: dependencies_changed
timestamp: 2026-01-01T00:00:00Z
depends:
  - 1
EOF
tracker rebuild
tracker ready
```

**Expected output (after depends set):**

- `Blocker` appears in ready output
- `Dependent` does NOT appear

```sh
# Now mark the blocker done and check again
tracker update 1 --status done
tracker ready
```

**Expected output (after blocker done):**

- `Dependent` now appears in ready output
- `Blocker` (done) does NOT appear

---

## T-023 — Ready: dependency blocked by in_progress (not yet done)

**Purpose:** Verify that `in_progress` is not sufficient to unblock a dependent; only
`done` or `cancelled` unblocks it.

**Steps:**

```sh
mkdir -p ./testdata && TEST_DIR=$(mktemp -d ./testdata/tracker-test-XXXXXX) && cd "$TEST_DIR"
tracker init --prefix PROJ
tracker create --title "Blocker"
tracker create --title "Dependent"
cat >> .tracker/issues/PROJ-0002.yaml << 'EOF'
---
event: dependencies_changed
timestamp: 2026-01-01T00:00:00Z
depends:
  - 1
EOF
tracker rebuild
tracker update 1 --status in_progress
tracker ready
```

**Expected output:**

- `Blocker` does NOT appear (not open)
- `Dependent` does NOT appear (dep still in_progress)

---

## T-024 — Ready: dependency unblocked by cancellation

**Purpose:** Verify that cancelling a dependency unblocks the dependent issue.

**Steps:**

```sh
mkdir -p ./testdata && TEST_DIR=$(mktemp -d ./testdata/tracker-test-XXXXXX) && cd "$TEST_DIR"
tracker init --prefix PROJ
tracker create --title "Blocker"
tracker create --title "Dependent"
cat >> .tracker/issues/PROJ-0002.yaml << 'EOF'
---
event: dependencies_changed
timestamp: 2026-01-01T00:00:00Z
depends:
  - 1
EOF
tracker rebuild
tracker update 1 --status cancelled
tracker ready
```

**Expected output:**

- `Dependent` appears in ready output

---

## T-025 — Ready: claimed issue excluded

**Purpose:** Verify that a claimed issue does not appear in `tracker ready`.

**Steps:**

```sh
mkdir -p ./testdata && TEST_DIR=$(mktemp -d ./testdata/tracker-test-XXXXXX) && cd "$TEST_DIR"
tracker init --prefix PROJ
tracker create --title "Work item"
tracker ready
tracker claim 1 --agent agent-1 --session sess-1
tracker ready
```

**Expected output:**

- First `tracker ready`: `Work item` appears
- Second `tracker ready` (after claim): `Work item` does NOT appear (or output is `no ready issues`)

---

## T-026 — Ready: priority ordering

**Purpose:** Verify that `tracker ready` returns issues ordered by priority
(high before medium before low).

**Steps:**

```sh
mkdir -p ./testdata && TEST_DIR=$(mktemp -d ./testdata/tracker-test-XXXXXX) && cd "$TEST_DIR"
tracker init --prefix PROJ
tracker create --title "Low task" --priority low
tracker create --title "High task" --priority high
tracker create --title "Medium task" --priority medium
tracker ready
```

**Expected output:**

- `High task` appears first
- `Medium task` appears second
- `Low task` appears last

---

## T-027 — Ready: JSON output

**Purpose:** Verify that `tracker ready --json` produces a valid JSON array.

**Steps:**

```sh
mkdir -p ./testdata && TEST_DIR=$(mktemp -d ./testdata/tracker-test-XXXXXX) && cd "$TEST_DIR"
tracker init --prefix PROJ
tracker create --title "Ready item" --priority high
tracker ready --json
```

**Expected output:** Valid JSON array with one element containing:
`id`, `display_id`, `type`, `title`, `status`, `priority`

---

## T-028 — Update: change status

**Purpose:** Verify that `tracker update <id> --status` changes the issue status,
which is reflected in subsequent `tracker show`.

**Steps:**

```sh
mkdir -p ./testdata && TEST_DIR=$(mktemp -d ./testdata/tracker-test-XXXXXX) && cd "$TEST_DIR"
tracker init --prefix PROJ
tracker create --title "Workflow test"
tracker update 1 --status in_progress
tracker show 1
tracker update 1 --status review
tracker show 1
tracker update 1 --status done
tracker show 1
```

**Expected output:**

- After `--status in_progress`: `tracker show 1` shows `Status:    in_progress`
- After `--status review`: `tracker show 1` shows `Status:    review`
- After `--status done`: `tracker show 1` shows `Status:    done`
- Each `tracker update` prints the formatted ID: `PROJ-0001`

---

## T-029 — Update: change priority

**Purpose:** Verify that `tracker update <id> --priority` changes the priority.

**Steps:**

```sh
mkdir -p ./testdata && TEST_DIR=$(mktemp -d ./testdata/tracker-test-XXXXXX) && cd "$TEST_DIR"
tracker init --prefix PROJ
tracker create --title "Priority test" --priority low
tracker show 1
tracker update 1 --priority high
tracker show 1
```

**Expected output:**

- First show: `Priority:  low`
- Second show: `Priority:  high`

---

## T-030 — Update: change title

**Purpose:** Verify that `tracker update <id> --title` renames the issue.

**Steps:**

```sh
mkdir -p ./testdata && TEST_DIR=$(mktemp -d ./testdata/tracker-test-XXXXXX) && cd "$TEST_DIR"
tracker init --prefix PROJ
tracker create --title "Old title"
tracker update 1 --title "New title"
tracker show 1
```

**Expected output:**

- `tracker show 1` displays `New title`

---

## T-031 — Update: multiple fields at once

**Purpose:** Verify that multiple update flags can be combined in a single call.

**Steps:**

```sh
mkdir -p ./testdata && TEST_DIR=$(mktemp -d ./testdata/tracker-test-XXXXXX) && cd "$TEST_DIR"
tracker init --prefix PROJ
tracker create --title "Original" --priority low
tracker update 1 --status in_progress --priority high --title "Renamed"
tracker show 1
```

**Expected output:**

- `tracker show 1` shows `Status:    in_progress`, `Priority:  high`, title `Renamed`

---

## T-032 — Update: no flags provided

**Purpose:** Verify that calling `tracker update` without any field flags is rejected.

**Steps:**

```sh
mkdir -p ./testdata && TEST_DIR=$(mktemp -d ./testdata/tracker-test-XXXXXX) && cd "$TEST_DIR"
tracker init --prefix PROJ
tracker create --title "Test issue"
tracker update 1
echo "exit code: $?"
```

**Expected output:**

- Error to stderr containing `at least one of --status, --priority, or --title`
- Exit code is non-zero

---

## T-033 — Update: invalid status value

**Purpose:** Verify that an unrecognised status value is rejected without modifying the issue.

**Steps:**

```sh
mkdir -p ./testdata && TEST_DIR=$(mktemp -d ./testdata/tracker-test-XXXXXX) && cd "$TEST_DIR"
tracker init --prefix PROJ
tracker create --title "Test issue"
tracker update 1 --status flying
echo "exit code: $?"
tracker show 1
```

**Expected output:**

- Error to stderr containing `invalid status` and `flying`
- Exit code is non-zero
- `tracker show 1` still shows `Status:    open`

---

## T-034 — Update: missing issue ID

**Purpose:** Verify that `tracker update` without an ID argument exits with an error.

**Steps:**

```sh
mkdir -p ./testdata && TEST_DIR=$(mktemp -d ./testdata/tracker-test-XXXXXX) && cd "$TEST_DIR"
tracker init --prefix PROJ
tracker update --status done
echo "exit code: $?"
```

**Expected output:**

- Error to stderr containing `missing issue ID`
- Exit code is non-zero

---

## T-035 — Comment: add comment

**Purpose:** Verify that `tracker comment` appends a comment event visible in
`show --events`.

**Steps:**

```sh
mkdir -p ./testdata && TEST_DIR=$(mktemp -d ./testdata/tracker-test-XXXXXX) && cd "$TEST_DIR"
tracker init --prefix PROJ
tracker create --title "Issue to comment on"
tracker comment 1 "This is my first comment"
tracker comment 1 "This is a second comment"
tracker show 1 --events
```

**Expected output:**

- Both `tracker comment` calls print: `PROJ-0001`
- `show --events` output contains `comment` event type (twice)
- Event log shows entries in order: created, comment, comment

---

## T-036 — Comment: empty message rejected

**Purpose:** Verify that a blank comment message is rejected.

**Steps:**

```sh
mkdir -p ./testdata && TEST_DIR=$(mktemp -d ./testdata/tracker-test-XXXXXX) && cd "$TEST_DIR"
tracker init --prefix PROJ
tracker create --title "Test issue"
tracker comment 1 ""
echo "exit code: $?"
```

**Expected output:**

- Error to stderr containing `message must not be empty`
- Exit code is non-zero

---

## T-037 — Comment: missing arguments

**Purpose:** Verify that `tracker comment` with only an ID (no message) is rejected.

**Steps:**

```sh
mkdir -p ./testdata && TEST_DIR=$(mktemp -d ./testdata/tracker-test-XXXXXX) && cd "$TEST_DIR"
tracker init --prefix PROJ
tracker create --title "Test issue"
tracker comment 1
echo "exit code: $?"
```

**Expected output:**

- Error to stderr containing `usage: tracker comment`
- Exit code is non-zero

---

## T-038 — Claim: basic claim

**Purpose:** Verify that `tracker claim` sets a claim on an issue, visible in
`tracker show`.

**Steps:**

```sh
mkdir -p ./testdata && TEST_DIR=$(mktemp -d ./testdata/tracker-test-XXXXXX) && cd "$TEST_DIR"
tracker init --prefix PROJ
tracker create --title "Claimable task"
tracker claim 1 --agent agent-alpha --session sess-001
tracker show 1
```

**Expected output:**

- `tracker claim` prints: `PROJ-0001`
- `tracker show 1` shows a `Claim:` line containing `agent-alpha` and `sess-001`

---

## T-039 — Claim: double-claim rejected

**Purpose:** Verify that attempting to claim an already-claimed issue fails.

**Steps:**

```sh
mkdir -p ./testdata && TEST_DIR=$(mktemp -d ./testdata/tracker-test-XXXXXX) && cd "$TEST_DIR"
tracker init --prefix PROJ
tracker create --title "Contested task"
tracker claim 1 --agent agent-1 --session sess-1
tracker claim 1 --agent agent-2 --session sess-2
echo "exit code: $?"
tracker show 1
```

**Expected output:**

- First claim succeeds.
- Second claim prints an error to stderr
- Exit code of second claim is non-zero
- `tracker show 1` still shows `agent-1` as the claimant

---

## T-040 — Claim: missing --agent flag

**Purpose:** Verify that `tracker claim` without `--agent` is rejected.

**Steps:**

```sh
mkdir -p ./testdata && TEST_DIR=$(mktemp -d ./testdata/tracker-test-XXXXXX) && cd "$TEST_DIR"
tracker init --prefix PROJ
tracker create --title "Test issue"
tracker claim 1 --session sess-1
echo "exit code: $?"
```

**Expected output:**

- Error to stderr containing `--agent is required`
- Exit code is non-zero

---

## T-041 — Claim: missing --session flag

**Purpose:** Verify that `tracker claim` without `--session` is rejected.

**Steps:**

```sh
mkdir -p ./testdata && TEST_DIR=$(mktemp -d ./testdata/tracker-test-XXXXXX) && cd "$TEST_DIR"
tracker init --prefix PROJ
tracker create --title "Test issue"
tracker claim 1 --agent agent-1
echo "exit code: $?"
```

**Expected output:**

- Error to stderr containing `--session is required`
- Exit code is non-zero

---

## T-042 — Claim: missing issue ID

**Purpose:** Verify that `tracker claim` without an ID argument is rejected.

**Steps:**

```sh
mkdir -p ./testdata && TEST_DIR=$(mktemp -d ./testdata/tracker-test-XXXXXX) && cd "$TEST_DIR"
tracker init --prefix PROJ
tracker claim --agent agent-1 --session sess-1
echo "exit code: $?"
```

**Expected output:**

- Error to stderr containing `missing issue ID`
- Exit code is non-zero

---

## T-043 — Release: release a claimed issue

**Purpose:** Verify that `tracker release` clears the claim and records the previous
claimant in the event log.

**Steps:**

```sh
mkdir -p ./testdata && TEST_DIR=$(mktemp -d ./testdata/tracker-test-XXXXXX) && cd "$TEST_DIR"
tracker init --prefix PROJ
tracker create --title "Work item"
tracker claim 1 --agent agent-1 --session sess-1
tracker show 1
tracker release 1
tracker show 1
tracker show 1 --events
```

**Expected output:**

- First `tracker show 1`: shows `Claim:` line with `agent-1`
- `tracker release 1` prints: `PROJ-0001`
- Second `tracker show 1`: no `Claim:` line
- `show --events` contains a `released` event with `prev=agent-1`

---

## T-044 — Release: release unclaimed issue (no-op)

**Purpose:** Verify that releasing an unclaimed issue completes without error.

**Steps:**

```sh
mkdir -p ./testdata && TEST_DIR=$(mktemp -d ./testdata/tracker-test-XXXXXX) && cd "$TEST_DIR"
tracker init --prefix PROJ
tracker create --title "Unclaimed task"
tracker release 1
echo "exit code: $?"
tracker show 1
```

**Expected output:**

- `tracker release 1` prints `PROJ-0001` with no error
- Exit code is 0
- `tracker show 1` shows no claim

---

## T-045 — Release: re-claim after release

**Purpose:** Verify that an issue can be re-claimed by a different agent after it is
released.

**Steps:**

```sh
mkdir -p ./testdata && TEST_DIR=$(mktemp -d ./testdata/tracker-test-XXXXXX) && cd "$TEST_DIR"
tracker init --prefix PROJ
tracker create --title "Handoff task"
tracker claim 1 --agent agent-1 --session sess-1
tracker release 1
tracker claim 1 --agent agent-2 --session sess-2
tracker show 1
```

**Expected output:**

- Final `tracker show 1` shows `Claim:` line with `agent-2`

---

## T-046 — Release: missing issue ID

**Purpose:** Verify that `tracker release` without an ID argument is rejected.

**Steps:**

```sh
mkdir -p ./testdata && TEST_DIR=$(mktemp -d ./testdata/tracker-test-XXXXXX) && cd "$TEST_DIR"
tracker init --prefix PROJ
tracker release
echo "exit code: $?"
```

**Expected output:**

- Error to stderr containing `missing issue ID`
- Exit code is non-zero

---

## T-047 — Rebuild: basic rebuild

**Purpose:** Verify that `tracker rebuild` regenerates the SQLite index from YAML files
and reports the correct issue count.

**Steps:**

```sh
mkdir -p ./testdata && TEST_DIR=$(mktemp -d ./testdata/tracker-test-XXXXXX) && cd "$TEST_DIR"
tracker init --prefix PROJ
tracker create --title "Issue one"
tracker create --title "Issue two"
tracker create --title "Issue three"
rm .tracker/db.sqlite
tracker rebuild
tracker list
```

**Expected output:**

- `tracker rebuild` prints: `rebuilt index: 3 issue(s)`
- `tracker list` shows all three issues

---

## T-048 — Rebuild: consistency with incremental state

**Purpose:** Verify that state built incrementally matches the state after a full rebuild.

**Steps:**

```sh
mkdir -p ./testdata && TEST_DIR=$(mktemp -d ./testdata/tracker-test-XXXXXX) && cd "$TEST_DIR"
tracker init --prefix PROJ
tracker create --title "Alpha" --priority high
tracker create --title "Beta" --type issue
tracker update 1 --status in_progress --title "Alpha revised"
tracker update 2 --priority low
tracker claim 2 --agent agent-x --session sess-x

# Record state before rebuild
tracker list > before.txt
tracker show 1 > show1-before.txt
tracker show 2 > show2-before.txt

# Rebuild
tracker rebuild

# Record state after rebuild
tracker list > after.txt
tracker show 1 > show1-after.txt
tracker show 2 > show2-after.txt

# Compare
diff before.txt after.txt
diff show1-before.txt show1-after.txt
diff show2-before.txt show2-after.txt
```

**Expected output:**

- `diff` commands produce no output (files are identical)
- In particular: title change (`Alpha revised`) and claim on issue 2 survive the rebuild

---

## T-049 — Rebuild: claim state preserved

**Purpose:** Verify that claim information is reconstructed correctly after rebuild.

**Steps:**

```sh
mkdir -p ./testdata && TEST_DIR=$(mktemp -d ./testdata/tracker-test-XXXXXX) && cd "$TEST_DIR"
tracker init --prefix PROJ
tracker create --title "Claimed task"
tracker claim 1 --agent agent-z --session sess-z
rm .tracker/db.sqlite
tracker rebuild
tracker show 1
```

**Expected output:**

- `tracker show 1` shows `Claim:` line with `agent-z` and `sess-z`

---

## T-050 — ID formats: bare number, prefixed, zero-padded

**Purpose:** Verify that all supported ID formats are accepted interchangeably across
commands.

**Steps:**

```sh
mkdir -p ./testdata && TEST_DIR=$(mktemp -d ./testdata/tracker-test-XXXXXX) && cd "$TEST_DIR"
tracker init --prefix PROJ
tracker create --title "Format test"

# All three forms should refer to the same issue
tracker show 1
tracker show PROJ-1
tracker show PROJ-0001

tracker update 1 --status in_progress
tracker show PROJ-1

tracker update PROJ-1 --status review
tracker show PROJ-0001
```

**Expected output:**

- All `show` variants display the same issue
- Status progresses correctly regardless of which ID format was used in `update`

---

## T-051 — Full lifecycle workflow

**Purpose:** End-to-end test of the complete issue lifecycle: init → create →
list → ready → claim → comment → update → release → ready again → rebuild.

**Steps:**

```sh
mkdir -p ./testdata && TEST_DIR=$(mktemp -d ./testdata/tracker-test-XXXXXX) && cd "$TEST_DIR"
tracker init --prefix WF

# Create three issues
tracker create --title "Foundation" --priority high --type task \
  --description "Must complete first" --acceptance-criteria "Foundation done"
tracker create --title "Build on Foundation" --priority medium
tracker create --title "Independent Work" --priority low

# Manually set issue 2 to depend on issue 1
cat >> .tracker/issues/WF-0002.yaml << 'EOF'
---
event: dependencies_changed
timestamp: 2026-01-01T00:00:00Z
depends:
  - 1
EOF
tracker rebuild

# List: all three visible
tracker list

# Ready: issues 1 and 3 (issue 2 blocked by dep)
tracker ready

# Claim issue 1
tracker claim 1 --agent agent-alpha --session sess-001

# Ready: only issue 3 now (issue 1 claimed, issue 2 still blocked)
tracker ready

# Comment and complete issue 1
tracker comment 1 "Working on the foundation now"
tracker update WF-0001 --status done

# Release issue 1
tracker release 1

# Ready: issue 2 now unblocked; issue 3 still there
tracker ready

# Show issue 1 event history
tracker show 1 --events

# Claim/release cycle on issue 2
tracker claim 2 --agent agent-beta --session sess-002
tracker update 2 --status in_progress
tracker release 2

# Rebuild and verify consistency
tracker rebuild
tracker list
```

**Expected output (key assertions):**

- After init: three issues created with IDs `WF-0001`, `WF-0002`, `WF-0003`
- `tracker list` shows all three issues
- First `tracker ready`: shows `Foundation` and `Independent Work`; does NOT show `Build on Foundation`
- After claiming issue 1: `tracker ready` shows only `Independent Work`
- After marking issue 1 done and releasing: `tracker ready` shows `Build on Foundation` and `Independent Work`; does NOT show `WF-0001` (done)
- `tracker show 1 --events` contains events: `created`, `claimed`, `comment`, `status_changed`, `released`
- `tracker rebuild` prints: `rebuilt index: 3 issue(s)`
- Final `tracker list` shows `Build on Foundation` and `Independent Work`

---

## T-052 — No tracker directory: commands fail gracefully

**Purpose:** Verify that all commands that require an initialised tracker print a clear
error when run in a directory without `.tracker/`.

**Steps:**

```sh
mkdir -p ./testdata && TEST_DIR=$(mktemp -d ./testdata/tracker-test-XXXXXX) && cd "$TEST_DIR"
# Do NOT run tracker init

tracker create --title "Should fail"
echo "create exit code: $?"

tracker list
echo "list exit code: $?"

tracker show 1
echo "show exit code: $?"

tracker ready
echo "ready exit code: $?"
```

**Expected output:**

- Each command prints an error to stderr
- Each exit code is non-zero
- No `.tracker/` directory is created
