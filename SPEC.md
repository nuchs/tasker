# Tracker — Local Issue Tracker for AI Agents

## Overview

Tracker is a local, file-based issue tracking system designed for use by a single developer working with one or more AI coding agents. It stores issues as append-only YAML event logs alongside a disposable SQLite index, providing token-efficient querying for agents and human-readable inspection via the filesystem.

## Goals

- Provide organised state for agents to track work on a project
- Allow agents to record problems for later instances to fix
- Give human developers visibility into agent progress
- Allow humans to task agents
- Be token-efficient
- Work without running services
- Handle contention between multiple agents

## Architecture

### Storage Model

Two-layer storage with a clear source-of-truth hierarchy:

**Content files** (source of truth): One YAML file per issue in `.tracker/issues/`. Each file is an append-only sequence of YAML documents separated by `---`. Events are never modified or deleted, only appended. The full history of an issue is its file.

**SQLite index** (derived, disposable): `.tracker/db.sqlite` materialises the current state of all issues for fast querying. It can be deleted and rebuilt from the content files at any time via `tracker rebuild`. It is excluded from version control via `.gitignore`.

**Config file**: `.tracker/config.yaml` stores project configuration (prefix, settings). Versioned alongside content files.

### Rationale

- **Append-only files**: No concurrent-write conflicts (agents only append). Git-friendly (appends merge cleanly). Full history preserved.
- **SQLite as derived index**: Enables fast structured queries (filter, sort, dependency checks) without scanning all files. Disposable means no migration pain — schema changes just require a rebuild.
- **File-per-issue**: Easy to inspect, browse, and reason about. Contention only occurs if two agents work on the same issue, which the claim system prevents.
- **YAML multi-document format**: Human-readable, parseable with standard libraries, `---` separators allow isolating malformed events.

### Directory Layout

```
.tracker/
  issues/
    PROJ-001.yaml
    PROJ-002.yaml
    ...
  config.yaml
  .gitignore          # contains: db.sqlite
  db.sqlite           # local only, rebuilt from issues/
```

## Data Model

### Issue Fields

| Field               | Type                | Description                                    |
|---------------------|---------------------|------------------------------------------------|
| id                  | integer             | Auto-incrementing, unique within project       |
| type                | task \| issue       | Task = new work, Issue = fix existing           |
| title               | string              | Short summary                                  |
| description         | string              | Full detail of the work                        |
| status              | enum (see below)    | Current state                                  |
| priority            | high \| medium \| low | Importance                                   |
| depends             | list of issue IDs   | Issues that must be done/cancelled before this |
| acceptance_criteria | string              | How to verify completion                       |
| claim               | object (nullable)   | Agent/session holding this issue               |

### Statuses

`draft`, `open`, `in_progress`, `review`, `done`, `cancelled`, `blocked`

No state machine is enforced at the storage layer. Any transition is permitted. Workflow rules (e.g. "must be claimed before moving to in_progress") belong in higher-level tooling built on top.

- **draft**: Half-formed idea, not yet actionable. Excluded from `ready` queries.
- **open**: Ready to be picked up (subject to dependency checks).
- **in_progress**: Actively being worked on.
- **review**: Work complete, awaiting verification.
- **done**: Verified complete.
- **cancelled**: Will not be done. Preserved in history, filtered from active queries.
- **blocked**: Cannot proceed. May be due to dependencies or external factors.

### Claims

A claim records which agent holds an issue:

| Field      | Description                                      |
|------------|--------------------------------------------------|
| agent_id   | Identifier for the agent                         |
| session_id | Session identifier for crash/stale detection     |
| claimed_at | Timestamp                                        |

Stale detection: when a ticket is accessed, the system checks the time since the last event on that issue. If it exceeds a configurable threshold (default 2h), the claim is reported as likely stale.

Any agent or human can release any claim. The release event records who released it and the previous claimant for audit purposes.

### IDs

- Stored internally as integers.
- Displayed with a configurable project prefix: `PROJ-042`.
- CLI accepts bare numbers (`42`) or prefixed (`PROJ-42` or `PROJ-042`).
- Filenames include the prefix: `PROJ-042.yaml`.
- Zero-padded to 4 digits. Rolls over naturally to 5+ digits at 10000.
- Auto-incremented. Next ID derived from config or by scanning the issues directory.

## Event Format

Content files use YAML multi-document format. Each event is a separate YAML document:

```yaml
---
event: created
timestamp: 2025-02-19T10:00:00Z
id: 1
type: task
title: Fix auth token refresh race condition
status: open
priority: high
depends:
  - 3
acceptance_criteria: |
  - Token refresh under concurrent load does not panic
  - Tests pass with -race flag
description: |
  Auth tokens expire after 1h. The refresh logic in
  tokenStore.Refresh() has a race condition when multiple
  goroutines call it concurrently. The mutex doesn't
  cover the HTTP round-trip.

---
event: status_changed
timestamp: 2025-02-19T14:30:00Z
status: in_progress

---
event: claimed
timestamp: 2025-02-19T14:30:00Z
agent_id: claude-session-abc
session_id: sess-12345

---
event: comment
timestamp: 2025-02-19T15:00:00Z
author: claude-session-abc
body: |
  Investigated. Straightforward fix: hold the mutex
  across the HTTP call, or switch to singleflight.

---
event: description_updated
timestamp: 2025-02-19T15:10:00Z
description: |
  Auth tokens expire after 1h. The refresh logic in
  tokenStore.Refresh() has a race condition when multiple
  goroutines call it concurrently.

  Root cause: mutex released before HTTP call completes.
  Fix: use golang.org/x/sync/singleflight.

---
event: released
timestamp: 2025-02-20T09:00:00Z
released_by: human
previous_claimant: claude-session-abc
reason: stale claim
```

### Event Types

| Event                      | Effect on materialised state                     |
|----------------------------|--------------------------------------------------|
| created                    | New issue with all initial fields                |
| status_changed             | Updates status                                   |
| title_changed              | Updates title (last-write-wins)                  |
| priority_changed           | Updates priority                                 |
| description_updated        | Replaces description (last-write-wins)           |
| acceptance_criteria_updated| Replaces acceptance criteria (last-write-wins)   |
| dependencies_changed       | Replaces full dependency list                    |
| comment                    | No effect on indexed fields (content log only)   |
| claimed                    | Sets claim on issue                              |
| released                   | Clears claim on issue                            |

### Corruption Handling

If an event cannot be parsed, the system halts processing of that file. No new events may be appended until the corrupted event is resolved. Resolution options:

1. Fix the malformed YAML manually.
2. Replace it with a `parse_error` event preserving the original bytes and diagnostic information.

A `parse_error` event is valid YAML and is skipped during materialisation. It preserves evidence of what went wrong.

```yaml
---
event: parse_error
timestamp: 2025-02-19T16:00:00Z
original_bytes: |
  event: comment
  timestamp: 2025-02-19T15:45:00Z
  body: |
    some truncated conte
diagnostic: "unexpected EOF in YAML scalar"
```

## SQLite Schema

```sql
CREATE TABLE issues (
    id          INTEGER PRIMARY KEY,
    type        TEXT NOT NULL CHECK(type IN ('task', 'issue')),
    title       TEXT NOT NULL,
    status      TEXT NOT NULL CHECK(status IN ('draft', 'open', 'in_progress',
                'review', 'done', 'cancelled', 'blocked')),
    priority    TEXT NOT NULL CHECK(priority IN ('high', 'medium', 'low')),
    created_at  TEXT NOT NULL,
    updated_at  TEXT NOT NULL
);

CREATE TABLE dependencies (
    issue_id    INTEGER NOT NULL REFERENCES issues(id),
    depends_on  INTEGER NOT NULL REFERENCES issues(id),
    PRIMARY KEY (issue_id, depends_on)
);

CREATE TABLE claims (
    issue_id    INTEGER PRIMARY KEY REFERENCES issues(id),
    agent_id    TEXT NOT NULL,
    session_id  TEXT NOT NULL,
    claimed_at  TEXT NOT NULL
);
```

Description and acceptance_criteria are deliberately excluded from SQLite. They are large, never queried against, and live in the content files. The CLI reads them from the file when a specific issue's detail is requested.

### Key Query: Ready Issues

```sql
SELECT i.id, i.title, i.priority
FROM issues i
WHERE i.status = 'open'
  AND i.id NOT IN (SELECT issue_id FROM claims)
  AND NOT EXISTS (
      SELECT 1 FROM dependencies d
      JOIN issues dep ON dep.id = d.depends_on
      WHERE d.issue_id = i.id
        AND dep.status NOT IN ('done', 'cancelled')
  )
ORDER BY
  CASE i.priority
    WHEN 'high' THEN 1
    WHEN 'medium' THEN 2
    WHEN 'low' THEN 3
  END;
```

## CLI Commands

```
tracker init --prefix PROJ
tracker create --title "..." --description "..." [--priority high] [--type issue]
tracker show <id> [--events] [--json]
tracker list [--status open] [--priority high] [--type task] [--json]
tracker ready [--json]
tracker update <id> --field value [--field value ...]
tracker comment <id> "message"
tracker claim <id> --agent <agent-id> --session <session-id>
tracker release <id>
tracker rebuild
```

All read commands support `--json` for structured output. Default output is human-readable formatted text.

The CLI accepts bare numeric IDs (`42`) or prefixed IDs (`PROJ-42`, `PROJ-042`).

`tracker show` performs stale claim detection on access.

## Git Integration

No special git integration required. The directory layout naturally supports version control:

- Content files and config are tracked.
- SQLite is gitignored.
- `tracker init` appends `db.sqlite` to `.tracker/.gitignore`.
- On clone, run `tracker rebuild` to regenerate the index.

## Testing Strategy

All tests use temporary directories. Each test:

1. Creates a temp dir.
2. Runs `tracker init`.
3. Exercises commands.
4. Asserts against file contents and/or SQLite state.
5. Tears down.

No mocking required — the entire system is filesystem + SQLite.

`tracker rebuild` is inherently testable: create content files manually, run rebuild, assert DB state.

## Logging

`--verbose` / `--debug` flag for development diagnostics. Uses Go's `log/slog`. Debug output to stderr, normal output to stdout.

## Future Extensions (Not in Scope)

- Local HTTP server for web-based project view
- MCP server for direct agent integration
- Task decomposition (breaking large tasks into subtasks)
- Labels/tags for categorisation
- "Related to" links between issues (distinct from dependencies)
- Workflow enforcement (state machine rules)
- Configurable stale claim timeout
