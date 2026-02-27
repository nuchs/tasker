package index

import (
	"fmt"
	"time"

	"github.com/nuchs/tasker/internal/model"
)

// IssueMeta holds the indexed fields for an issue. Description and
// AcceptanceCriteria are deliberately excluded; they live only in content files.
type IssueMeta struct {
	ID        int
	Type      model.IssueType
	Title     string
	Status    model.Status
	Priority  model.Priority
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Filters constrains a ListIssues query. Zero-value fields are not applied.
type Filters struct {
	Status   model.Status
	Priority model.Priority
	Type     model.IssueType
}

// ListIssues returns all issues matching f, ordered by id.
func (idx *Index) ListIssues(f Filters) ([]IssueMeta, error) {
	q := `SELECT id, type, title, status, priority, created_at, updated_at
	      FROM issues WHERE 1=1`
	var args []any
	if f.Status != "" {
		q += " AND status = ?"
		args = append(args, string(f.Status))
	}
	if f.Priority != "" {
		q += " AND priority = ?"
		args = append(args, string(f.Priority))
	}
	if f.Type != "" {
		q += " AND type = ?"
		args = append(args, string(f.Type))
	}
	q += " ORDER BY id"
	return idx.scanIssues(q, args...)
}

// ReadyIssues returns issues that are open, unclaimed, and have all
// dependencies in status done or cancelled, ordered high→medium→low priority.
func (idx *Index) ReadyIssues() ([]IssueMeta, error) {
	const q = `
		SELECT i.id, i.type, i.title, i.status, i.priority, i.created_at, i.updated_at
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
		    WHEN 'high'   THEN 1
		    WHEN 'medium' THEN 2
		    WHEN 'low'    THEN 3
		  END`
	return idx.scanIssues(q)
}

// GetIssueMeta returns the indexed metadata for a single issue, or an error
// if no issue with that id exists.
func (idx *Index) GetIssueMeta(id int) (IssueMeta, error) {
	issues, err := idx.scanIssues(
		`SELECT id, type, title, status, priority, created_at, updated_at
		 FROM issues WHERE id = ?`, id,
	)
	if err != nil {
		return IssueMeta{}, err
	}
	if len(issues) == 0 {
		return IssueMeta{}, fmt.Errorf("index: issue %d not found", id)
	}
	return issues[0], nil
}

// ClaimIssue atomically claims issue id for agentID/sessionID. It returns an
// error if the issue is already claimed. Uses INSERT OR IGNORE so that
// concurrent callers are serialised by SQLite's write lock; the one that
// inserts 0 rows knows it lost the race.
func (idx *Index) ClaimIssue(id int, agentID, sessionID string) error {
	result, err := idx.db.Exec(
		`INSERT OR IGNORE INTO claims (issue_id, agent_id, session_id, claimed_at)
		 VALUES (?, ?, ?, ?)`,
		id, agentID, sessionID, time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("index: claim issue %d: %w", id, err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("index: claim issue %d: rows affected: %w", id, err)
	}
	if n == 0 {
		return fmt.Errorf("index: issue %d is already claimed", id)
	}
	return nil
}

// ReleaseIssue removes the claim on issue id. It is a no-op if the issue is
// not currently claimed.
func (idx *Index) ReleaseIssue(id int) error {
	if _, err := idx.db.Exec(`DELETE FROM claims WHERE issue_id = ?`, id); err != nil {
		return fmt.Errorf("index: release issue %d: %w", id, err)
	}
	return nil
}

// NextID returns the next available issue ID (max existing id + 1, or 1 if
// there are no issues yet).
func (idx *Index) NextID() (int, error) {
	var maxID int
	if err := idx.db.QueryRow(`SELECT COALESCE(MAX(id), 0) FROM issues`).Scan(&maxID); err != nil {
		return 0, fmt.Errorf("index: next id: %w", err)
	}
	return maxID + 1, nil
}

// scanIssues executes q with args and returns the results as []IssueMeta.
func (idx *Index) scanIssues(q string, args ...any) ([]IssueMeta, error) {
	rows, err := idx.db.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("index: query: %w", err)
	}
	defer rows.Close()

	var issues []IssueMeta
	for rows.Next() {
		var m IssueMeta
		var issueType, status, priority, createdAt, updatedAt string
		if err := rows.Scan(&m.ID, &issueType, &m.Title, &status, &priority,
			&createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("index: scan: %w", err)
		}
		m.Type = model.IssueType(issueType)
		m.Status = model.Status(status)
		m.Priority = model.Priority(priority)
		m.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
		if err != nil {
			return nil, fmt.Errorf("index: parse created_at %q: %w", createdAt, err)
		}
		m.UpdatedAt, err = time.Parse(time.RFC3339, updatedAt)
		if err != nil {
			return nil, fmt.Errorf("index: parse updated_at %q: %w", updatedAt, err)
		}
		issues = append(issues, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("index: rows: %w", err)
	}
	return issues, nil
}
