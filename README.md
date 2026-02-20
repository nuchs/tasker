# Tracker

A local, file-based issue tracker designed for developers working with AI coding agents.

Tracker stores issues as append-only YAML event logs in your project directory, with a disposable SQLite index for fast queries. No server, no cloud dependency, no configuration beyond a project prefix.

## Why

AI coding agents are ephemeral. Each session starts with no memory of previous sessions. Tracker gives agents (and you) a shared, persistent record of what needs doing, what's in progress, and what's been done. It's designed to be token-efficient for agents and human-readable for you.

## How It Works

Issues live as YAML files in `.tracker/issues/`. Each file is an append-only log of events — creation, status changes, comments, claims. The full history of an issue is its file.

A SQLite database (`.tracker/db.sqlite`) indexes the current state for fast queries. It's derived from the files and can be rebuilt at any time. It's gitignored — the YAML files are the source of truth.

Agents interact through the CLI. The `ready` command returns the next workable issue (open, unclaimed, dependencies resolved, ordered by priority). An agent claims an issue, works on it, adds comments, and updates the status. If an agent crashes, stale claims are detected automatically.

## Quick Start

```bash
# Initialise in your project root
tracker init --prefix MYPROJ

# Create some issues
tracker create --title "Set up CI pipeline" --description "Configure GitHub Actions for build and test"
tracker create --title "Fix login timeout" --type issue --priority high --description "Sessions expire after 5 minutes instead of 30"

# See what's ready to work on
tracker ready

# Look at a specific issue
tracker show 1

# Claim an issue (agents do this; you can too)
tracker claim 1 --agent human --session terminal

# Add a comment
tracker comment 1 "Decided to use singleflight instead of a mutex"

# Update status
tracker update 1 --status review

# Release the claim
tracker release 1

# Mark it done
tracker update 1 --status done
```

## Issue Lifecycle

Issues have seven possible statuses. There are no enforced transitions — any status can move to any other.

| Status      | Meaning                                    |
|-------------|--------------------------------------------|
| draft       | Half-formed idea, not yet actionable        |
| open        | Ready to be picked up                       |
| in_progress | Actively being worked on                    |
| review      | Work complete, awaiting verification        |
| done        | Verified complete                           |
| cancelled   | Won't be done                               |
| blocked     | Cannot proceed (dependencies or external)   |

## Claims

When an agent (or human) starts work on an issue, they claim it. This prevents other agents from picking up the same issue. Claims include an agent ID and session ID so you can tell who's working on what.

If an agent crashes or abandons work, the claim goes stale. Tracker detects this by checking the time since the last event on the issue. Anyone can release any claim.

## File Format

Issue files are YAML multi-document streams. Each event is a separate YAML document separated by `---`. You can read them in any text editor:

```yaml
---
event: created
timestamp: 2025-02-19T10:00:00Z
id: 1
type: task
title: Set up CI pipeline
status: open
priority: medium
description: |
  Configure GitHub Actions for build and test.
  Should run on push to main and on PRs.

---
event: status_changed
timestamp: 2025-02-19T14:30:00Z
status: in_progress

---
event: comment
timestamp: 2025-02-19T15:00:00Z
author: claude-session-abc
body: |
  Using the standard Go workflow template.
  Added caching for module downloads.
```

## Dependencies

Issues can depend on other issues. An issue with unresolved dependencies won't appear in `tracker ready` output. Set dependencies at creation or update them later:

```bash
tracker create --title "Deploy to staging" --description "..." --depends 1,2,3
tracker update 4 --depends 1,2,3
```

## Rebuilding the Index

The SQLite database is disposable. If it gets corrupted, deleted, or you've just cloned the repo:

```bash
tracker rebuild
```

This regenerates the database from the YAML files.

## For Agent Developers

See `AGENTS.md` for instructions on integrating Tracker into your agent workflow. See `SPEC.md` for the full design, rationale, and data model.

The key commands agents need:

- `tracker ready --json` — what should I work on next?
- `tracker show <id> --json` — full details of an issue
- `tracker claim <id> --agent <id> --session <id>` — take ownership
- `tracker comment <id> "message"` — leave notes for the next session
- `tracker update <id> --status review` — mark work as done pending review

## Building

Requires Go 1.23+.

```bash
go build -o tracker ./cmd/tracker
go test -race ./...
```
