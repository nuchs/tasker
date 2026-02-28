// Package index manages the SQLite index of materialised issue state.
package index

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
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
	// Use a single connection so per-connection PRAGMAs apply consistently.
	db.SetMaxOpenConns(1)
	// Wait up to 10 seconds for a lock rather than failing immediately.
	if _, err := db.Exec("PRAGMA busy_timeout = 10000"); err != nil {
		db.Close()
		return nil, fmt.Errorf("index: set busy timeout: %w", err)
	}
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("index: enable foreign keys: %w", err)
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


func (idx *Index) initSchema() error {
	if _, err := idx.db.Exec(initDDL); err != nil {
		return fmt.Errorf("index: init schema: %w", err)
	}
	return nil
}

// Reset drops all tables and recreates them empty. Used by the store layer
// before repopulating the index from content files.
func (idx *Index) Reset() error {
	if _, err := idx.db.Exec(dropDDL); err != nil {
		return fmt.Errorf("index: reset: drop: %w", err)
	}
	if _, err := idx.db.Exec(initDDL); err != nil {
		return fmt.Errorf("index: reset: create schema: %w", err)
	}
	return nil
}
