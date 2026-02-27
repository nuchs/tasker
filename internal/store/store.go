// Package store handles reading and writing of YAML event files and ties them
// together with the SQLite index.
package store

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nuchs/tasker/internal/index"
	"github.com/nuchs/tasker/internal/model"
)

const defaultStaleTTL = 2 * time.Hour

// Store owns the issues directory, SQLite index, and project prefix. It is the
// layer that CLI commands call.
type Store struct {
	issuesDir string
	idx       *index.Index
	prefix    string
	now       func() time.Time
	staleTTL  time.Duration
}

// Open opens (or creates) a Store rooted at issuesDir with the SQLite database
// at dbPath and the given project prefix (e.g. "PROJ"). An empty prefix causes
// files to be named by bare numeric ID.
func Open(issuesDir, dbPath, prefix string) (*Store, error) {
	idx, err := index.Open(dbPath)
	if err != nil {
		return nil, fmt.Errorf("store: open index: %w", err)
	}
	return &Store{
		issuesDir: issuesDir,
		idx:       idx,
		prefix:    prefix,
		now:       time.Now,
		staleTTL:  defaultStaleTTL,
	}, nil
}

// Close closes the underlying index.
func (s *Store) Close() error {
	return s.idx.Close()
}

// issueFilePath returns the path to the content file for issue id.
func (s *Store) issueFilePath(id int) string {
	var name string
	if s.prefix == "" {
		name = fmt.Sprintf("%d.yaml", id)
	} else {
		name = fmt.Sprintf("%s-%04d.yaml", s.prefix, id)
	}
	return filepath.Join(s.issuesDir, name)
}

// Create assigns the next available ID to ev, writes the created event to a
// new content file, inserts the issue into the index, and returns the assigned
// ID. The caller should set IssueType, Title, Description, Status, Priority,
// Depends, and AcceptanceCriteria on ev; other fields are set by Create.
func (s *Store) Create(ev model.Event) (int, error) {
	id, err := s.idx.NextID()
	if err != nil {
		return 0, fmt.Errorf("store: create: %w", err)
	}

	ev.Type = model.EventCreated
	ev.ID = id
	ev.Timestamp = s.now()

	path := s.issueFilePath(id)
	if err := AppendEvent(path, ev); err != nil {
		return 0, fmt.Errorf("store: create: write file: %w", err)
	}

	issue := model.Issue{
		ID:                 id,
		Type:               ev.IssueType,
		Title:              ev.Title,
		Description:        ev.Description,
		Status:             ev.Status,
		Priority:           ev.Priority,
		Depends:            ev.Depends,
		AcceptanceCriteria: ev.AcceptanceCriteria,
	}
	if err := s.idx.UpsertIssue(issue, ev.Timestamp, ev.Timestamp); err != nil {
		return 0, fmt.Errorf("store: create: update index: %w", err)
	}

	return id, nil
}

// Append sets the timestamp on ev, appends it to the content file for issue
// id, then re-materialises the issue and updates the index.
func (s *Store) Append(id int, ev model.Event) error {
	ev.Timestamp = s.now()
	path := s.issueFilePath(id)
	if err := AppendEvent(path, ev); err != nil {
		return fmt.Errorf("store: append issue %d: %w", id, err)
	}
	return s.syncIndex(id, path)
}

// Show reads and materialises issue id from its content file. If the issue has
// an active claim and the last event is older than the stale threshold, the
// second return value is true.
func (s *Store) Show(id int) (model.Issue, bool, error) {
	path := s.issueFilePath(id)
	events, err := ParseFile(path)
	if err != nil {
		return model.Issue{}, false, fmt.Errorf("store: show issue %d: %w", id, err)
	}
	if len(events) == 0 {
		return model.Issue{}, false, fmt.Errorf("store: show issue %d: empty file", id)
	}

	issue, err := Materialise(events)
	if err != nil {
		return model.Issue{}, false, fmt.Errorf("store: show issue %d: %w", id, err)
	}

	stale := false
	if issue.Claim != nil {
		lastEvent := events[len(events)-1].Timestamp
		if s.now().Sub(lastEvent) > s.staleTTL {
			stale = true
		}
	}

	return issue, stale, nil
}

// Rebuild regenerates the SQLite index by scanning all content files in the
// issues directory. It resets the index first, then re-inserts every issue
// derived from its YAML file. Errors halt processing and leave the index in
// the reset state.
func (s *Store) Rebuild() error {
	if err := s.idx.Reset(); err != nil {
		return fmt.Errorf("store: rebuild: %w", err)
	}

	entries, err := os.ReadDir(s.issuesDir)
	if err != nil {
		return fmt.Errorf("store: rebuild: read dir %s: %w", s.issuesDir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}
		path := filepath.Join(s.issuesDir, entry.Name())
		if err := s.upsertFromFile(path); err != nil {
			return err
		}
	}
	return nil
}

// upsertFromFile parses path, materialises the issue, and upserts it into the
// index.
func (s *Store) upsertFromFile(path string) error {
	events, err := ParseFile(path)
	if err != nil {
		return fmt.Errorf("store: rebuild: parse %s: %w", path, err)
	}
	if len(events) == 0 {
		return nil
	}
	issue, err := Materialise(events)
	if err != nil {
		return fmt.Errorf("store: rebuild: materialise %s: %w", path, err)
	}
	createdAt := events[0].Timestamp
	updatedAt := events[len(events)-1].Timestamp
	if err := s.idx.UpsertIssue(issue, createdAt, updatedAt); err != nil {
		return fmt.Errorf("store: rebuild: upsert %s: %w", path, err)
	}
	return nil
}

// syncIndex re-reads and re-materialises the issue at path, then updates the
// index. Called after every successful append.
func (s *Store) syncIndex(id int, path string) error {
	events, err := ParseFile(path)
	if err != nil {
		return fmt.Errorf("store: sync index issue %d: %w", id, err)
	}
	if len(events) == 0 {
		return nil
	}
	issue, err := Materialise(events)
	if err != nil {
		return fmt.Errorf("store: sync index issue %d: %w", id, err)
	}
	createdAt := events[0].Timestamp
	updatedAt := events[len(events)-1].Timestamp
	if err := s.idx.UpsertIssue(issue, createdAt, updatedAt); err != nil {
		return fmt.Errorf("store: sync index issue %d: %w", id, err)
	}
	return nil
}
