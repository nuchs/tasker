// Package index manages the SQLite index of materialised issue state.
package index

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/nuchs/tasker/internal/store"
)

// Index wraps the SQLite database used as a derived index of issue state.
type Index struct {
	db *sql.DB
}

// Open opens (or creates) the SQLite database at path and ensures the schema
// exists. Call Close when done.
func Open(path string) (*Index, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("index: open %s: %w", path, err)
	}
	idx := &Index{db: db}
	if err := idx.initSchema(); err != nil {
		db.Close()
		return nil, err
	}
	return idx, nil
}

// Close closes the underlying database.
func (idx *Index) Close() error {
	return idx.db.Close()
}

// DB returns the underlying *sql.DB for direct querying (used in tests).
func (idx *Index) DB() *sql.DB {
	return idx.db
}

// initDDL creates tables only if they do not already exist (used by Open).
const initDDL = `
CREATE TABLE IF NOT EXISTS issues (
    id          INTEGER PRIMARY KEY,
    type        TEXT NOT NULL CHECK(type IN ('task', 'issue')),
    title       TEXT NOT NULL,
    status      TEXT NOT NULL CHECK(status IN ('draft', 'open', 'in_progress',
                'review', 'done', 'cancelled', 'blocked')),
    priority    TEXT NOT NULL CHECK(priority IN ('high', 'medium', 'low')),
    created_at  TEXT NOT NULL,
    updated_at  TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS dependencies (
    issue_id    INTEGER NOT NULL REFERENCES issues(id),
    depends_on  INTEGER NOT NULL REFERENCES issues(id),
    PRIMARY KEY (issue_id, depends_on)
);
CREATE TABLE IF NOT EXISTS claims (
    issue_id    INTEGER PRIMARY KEY REFERENCES issues(id),
    agent_id    TEXT NOT NULL,
    session_id  TEXT NOT NULL,
    claimed_at  TEXT NOT NULL
);`

// dropDDL removes all index tables in dependency-safe order.
const dropDDL = `
DROP TABLE IF EXISTS claims;
DROP TABLE IF EXISTS dependencies;
DROP TABLE IF EXISTS issues;`

// schemaDDL creates tables without IF NOT EXISTS (used after dropDDL).
const schemaDDL = `
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
);`

func (idx *Index) initSchema() error {
	if _, err := idx.db.Exec(initDDL); err != nil {
		return fmt.Errorf("index: init schema: %w", err)
	}
	return nil
}

// Rebuild drops and recreates the index by scanning all .yaml files in
// issuesDir. Each file is parsed and materialised; errors halt processing.
// The entire operation runs in a transaction, so a failure leaves the index
// unchanged.
func (idx *Index) Rebuild(issuesDir string) error {
	tx, err := idx.db.Begin()
	if err != nil {
		return fmt.Errorf("index: rebuild: begin: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(dropDDL); err != nil {
		return fmt.Errorf("index: rebuild: drop: %w", err)
	}
	if _, err := tx.Exec(schemaDDL); err != nil {
		return fmt.Errorf("index: rebuild: create schema: %w", err)
	}

	entries, err := os.ReadDir(issuesDir)
	if err != nil {
		return fmt.Errorf("index: rebuild: read dir %s: %w", issuesDir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}
		path := filepath.Join(issuesDir, entry.Name())
		if err := insertFile(tx, path); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// insertFile parses path, materialises the issue, and inserts it into the
// index within tx.
func insertFile(tx *sql.Tx, path string) error {
	events, err := store.ParseFile(path)
	if err != nil {
		return fmt.Errorf("index: rebuild: parse %s: %w", path, err)
	}
	if len(events) == 0 {
		return nil
	}

	issue, err := store.Materialise(events)
	if err != nil {
		return fmt.Errorf("index: rebuild: materialise %s: %w", path, err)
	}

	createdAt := events[0].Timestamp.UTC().Format(time.RFC3339)
	updatedAt := events[len(events)-1].Timestamp.UTC().Format(time.RFC3339)

	if _, err := tx.Exec(
		`INSERT INTO issues (id, type, title, status, priority, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		issue.ID, string(issue.Type), issue.Title,
		string(issue.Status), string(issue.Priority),
		createdAt, updatedAt,
	); err != nil {
		return fmt.Errorf("index: rebuild: insert issue %d: %w", issue.ID, err)
	}

	for _, depID := range issue.Depends {
		if _, err := tx.Exec(
			`INSERT INTO dependencies (issue_id, depends_on) VALUES (?, ?)`,
			issue.ID, depID,
		); err != nil {
			return fmt.Errorf("index: rebuild: insert dependency %d->%d: %w", issue.ID, depID, err)
		}
	}

	if issue.Claim != nil {
		if _, err := tx.Exec(
			`INSERT INTO claims (issue_id, agent_id, session_id, claimed_at) VALUES (?, ?, ?, ?)`,
			issue.ID, issue.Claim.AgentID, issue.Claim.SessionID,
			issue.Claim.ClaimedAt.UTC().Format(time.RFC3339),
		); err != nil {
			return fmt.Errorf("index: rebuild: insert claim for issue %d: %w", issue.ID, err)
		}
	}

	return nil
}
