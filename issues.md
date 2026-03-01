# Issues

---

- [x] **Race condition in `RunClaim`: check-then-append is not atomic**

`RunClaim` reads the current claim state via `s.Show(id)`, checks if `issue.Claim != nil`, then
calls `s.Append` to write the claim event. Two concurrent callers can both observe no claim and
both proceed to append their `claimed` events. Materialisation uses last-write-wins, so the first
claimer's event is silently overwritten by the second.

The index's `ClaimIssue` method uses `INSERT OR IGNORE` for atomicity, but `RunClaim` never
calls it — it goes through `Append` → `syncIndex` → `UpsertIssue`, which does a delete-then-insert
with no conflict detection.

Acceptance criteria:
- Two concurrent `tracker claim` calls on the same issue result in exactly one succeeding and
  one returning a clear error
- Content file contains at most one unmatched `claimed` event at any point

---

- [x] **SQLite foreign key constraints are silently unenforced**

SQLite does not enforce `REFERENCES` constraints unless `PRAGMA foreign_keys = ON` is set per
connection. `index.Open` never sets this pragma. The `claims` and `dependencies` tables both
declare foreign keys on `issues(id)`, but orphaned rows (e.g. a claim for a non-existent issue
ID) can be inserted without error.

Relevant file: `internal/index/index.go:18` (`Open`).

Acceptance criteria:
- `PRAGMA foreign_keys = ON` is executed on every new connection
- Inserting a claim for a non-existent issue ID returns an error

---

- [x] **Partial write in `AppendEvent` can permanently corrupt a content file**

`AppendEvent` (in `internal/store/writer.go`) opens the file with `O_APPEND`, acquires an
`flock`, marshals the event to YAML, then calls `fmt.Fprintf(f, "---\n%s", data)`. If the
process is killed or the disk fills up partway through the write, the file is left with a
truncated YAML document. The parser will then return a `ParseError` on every subsequent read,
halting all operations on that issue until the file is manually repaired.

There is no partial-write detection or rollback. A crash between writing `---\n` and the
YAML body leaves an incomplete document that is not a valid `parse_error` event.

Acceptance criteria:
- Either: detect and strip trailing incomplete documents on open (recovery mode)
- Or: document clearly that this is a known limitation and describe the manual repair steps
  (replace the truncated bytes with a well-formed `parse_error` event)

---

- [x] **`RunInit` prints its success message to `os.Stdout` directly**

All `Run*` commands accept an `io.Writer` for output, enabling output capture in tests. `RunInit`
is the only exception: it calls `fmt.Printf(...)` (which writes to `os.Stdout`) instead of
taking an `out io.Writer` parameter. The success message `"Initialised tracker with prefix…"` is
therefore untestable and the function signature is inconsistent with the rest of the CLI.

Relevant file: `internal/cli/init.go:53`.

Acceptance criteria:
- `RunInit` accepts an `io.Writer` parameter
- The success message is written to that writer
- The init test asserts the message appears in the output

---

- [x] **Schema DDL is duplicated in `internal/index/index.go`**

`initDDL` (used by `Open`) and `schemaDDL` (used by `Reset` after `dropDDL`) are almost
identical strings — one uses `CREATE TABLE IF NOT EXISTS`, the other `CREATE TABLE`. Any schema
change (new column, new index, new constraint) must be applied to both. They have already drifted
in their `IF NOT EXISTS` semantics and will continue to be a maintenance burden.

Acceptance criteria:
- A single canonical DDL string is used for schema creation
- `Reset` reuses it instead of maintaining a separate copy

---

- [x] **Parser accepts events with an empty or unrecognised `event:` field**

`ParseFile` / `parseReader` decodes each YAML document into `model.Event` without checking that
`ev.Type` is a known, non-empty value. A document with a missing `event:` key, a typo
(`evnet: comment`), or a future event type from a newer version of the tool is silently returned
as a valid event with `Type == ""` (or an unknown string). `Materialise` then silently ignores
it in the switch default. Corrupted or mistyped event types are therefore invisible.

Relevant file: `internal/store/parser.go:46`.

Acceptance criteria:
- `parseReader` returns an error (or a `parse_error`-wrapped event) when `ev.Type` is empty
- Unknown (but non-empty) types may be silently ignored for forward-compatibility, but empty
  type should always be rejected

---

- [x] **Default `tracker list` filtering is done in Go after fetching all rows**

When no `--status` flag is provided, `RunList` calls `s.ListIssues(index.Filters{})` which
returns every issue in the database, then discards rows with status `done` or `cancelled` in
Go-side loop. The SQL query and the in-memory filter are inconsistent: the DB query has no
knowledge of the default, making the behaviour hard to follow and unnecessarily fetching rows
that are immediately thrown away.

Relevant file: `internal/cli/list.go:66-80`.

Acceptance criteria:
- The `done`/`cancelled` exclusion is expressed as part of the query (either in `index.Filters`
  or via a new query parameter) rather than as a post-fetch filter

---

- [x] **`index.ClaimIssue` is unused by the CLI and uses wall-clock time**

`index.(*Index).ClaimIssue` records `claimed_at` using `time.Now().UTC()` directly, bypassing
the `Store.now` injection used everywhere else. No CLI code calls this method — claims are
written via `Store.Append` → `syncIndex` → `UpsertIssue`, which correctly uses the event
timestamp. The method is exported and its signature implies it is part of the public API, but
calling it would:

1. Write a claim to the DB with no corresponding event in the content file (source of truth
   would be inconsistent with the index).
2. Use a different clock source, so the `claimed_at` in the DB would differ from the content
   file after a rebuild.

Relevant file: `internal/index/queries.go:89`.

Acceptance criteria:
- Either remove `ClaimIssue` if it is not needed as a standalone index operation
- Or document clearly that it is a low-level primitive that must only be called after the
  corresponding event has been written to the content file, and fix the clock source to accept
  a `time.Time` parameter

---

- [x] **`TestRebuildConsistency` captures `issue1Before` but never compares it**

In `internal/cli/integration_test.go`, `issue1Before` is assigned from `s.Show(1)` before the
rebuild, but the post-rebuild assertion only checks `issue1After.Title` against the hardcoded
string `"Alpha revised"`. `issue1Before` is never compared against `issue1After`; it is
suppressed with `_ = issue1Before`. The test therefore does not verify that the pre/post values
are equal — it only verifies the post value is correct in isolation.

Acceptance criteria:
- Either compare `issue1Before` fields to `issue1After` fields directly
- Or remove the `issue1Before` capture and the `_ = issue1Before` line if the direct string
  check is sufficient

---

- [x] **`tracker show`: flags after positional ID are silently ignored (T-010, T-011)**

Go's `flag.FlagSet.Parse` stops at the first non-flag argument. In `RunShow`, `fs.Parse(args)`
is called with all arguments including the positional ID, so when the ID comes first (e.g.
`tracker show 1 --json`), parsing stops at `"1"` and `--json` / `--events` are never processed.
The command silently falls back to plain text output with no error.

Affected tests:
- T-010: `tracker show 1 --json` outputs plain text instead of JSON
- T-011: `tracker show 1 --events` outputs plain issue view instead of event log

Relevant file: `internal/cli/show.go:23` (`fs.Parse(args)`).

The fix is to split positional arguments from flags before parsing, matching the pattern used by
`RunUpdate`, `RunClaim`, and `RunRelease` (which take the ID as `args[0]` and call
`fs.Parse(args[1:])`).

Acceptance criteria:
- `tracker show 1 --json` outputs valid JSON
- `tracker show 1 --events` outputs the event history
- `tracker show --json 1` (flag before ID) continues to work as before

---

- [x] **`tracker update` and `tracker claim` report wrong error when ID is omitted but flags are provided (T-034, T-042)**

Both `RunUpdate` and `RunClaim` take the first element of `args` as the issue ID, then call
`fs.Parse(args[1:])`. When the user omits the ID and provides only flags (e.g.
`tracker update --status done` or `tracker claim --agent a --session s`), `args[0]` becomes
the first flag name (`"--status"` or `"--agent"`). `fs.Parse(args[1:])` then sees the flag
value as a positional argument, parses no flags, and the subsequent validation checks fire with
misleading errors:

- `tracker update --status done` → `"at least one of --status, --priority, or --title is required"` (T-034)
- `tracker claim --agent a --session s` → `"--agent is required"` (T-042)

Expected error in both cases: `"missing issue ID"`.

Relevant files: `internal/cli/update.go:24`, `internal/cli/claim.go:19`.

Fix: before taking `args[0]` as the ID, check whether it starts with `"-"`. If it does, return
`"missing issue ID"` immediately.

Acceptance criteria:
- `tracker update --status done` prints error containing `missing issue ID` and exits non-zero
- `tracker claim --agent a --session s` prints error containing `missing issue ID` and exits non-zero

---

- [x] **T-001: test document expects `ls .tracker/` to show `.gitignore` but Linux `ls` hides dotfiles**

In `tests.md` T-001, the expected output states:

> `ls .tracker/` lists: `config.yaml  db.sqlite  issues  .gitignore`

On Linux (and macOS), plain `ls` does not display files whose names begin with `.`, so `.gitignore` is silently omitted from the output. Running the test produces:

```
config.yaml
db.sqlite
issues
```

The `.gitignore` file is created correctly (confirmed by `cat .tracker/.gitignore` showing `db.sqlite`), so the tracker behaviour is correct. The defect is in the test document.

Acceptance criteria:
- Either change the step to `ls -a .tracker/` so that `.gitignore` appears in the listing, or
- Update the expected output to note that `.gitignore` is a hidden file and will not appear with plain `ls`, and verify its existence via `cat` instead.

