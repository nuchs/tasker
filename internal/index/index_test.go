package index_test

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/nuchs/tasker/internal/index"
	"github.com/nuchs/tasker/internal/model"
	"github.com/nuchs/tasker/internal/store"
)

// --- helpers ---

func openIndex(t *testing.T) *index.Index {
	t.Helper()
	idx, err := index.Open(filepath.Join(t.TempDir(), "db.sqlite"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { idx.Close() })
	return idx
}

func mustAppend(t *testing.T, path string, events ...model.Event) {
	t.Helper()
	for _, ev := range events {
		if err := store.AppendEvent(path, ev); err != nil {
			t.Fatalf("AppendEvent: %v", err)
		}
	}
}

func countRows(t *testing.T, db *sql.DB, table string) int {
	t.Helper()
	var n int
	if err := db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&n); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return n
}

var (
	t0 = time.Date(2025, 2, 19, 10, 0, 0, 0, time.UTC)
	t1 = t0.Add(time.Hour)
	t2 = t0.Add(2 * time.Hour)
)

// --- schema tests ---

func TestOpen_CreatesAllTables(t *testing.T) {
	idx := openIndex(t)
	db := idx.DB()

	for _, table := range []string{"issues", "dependencies", "claims"} {
		var name string
		err := db.QueryRow(
			"SELECT name FROM sqlite_master WHERE type='table' AND name=?", table,
		).Scan(&name)
		if err != nil {
			t.Errorf("table %q not found: %v", table, err)
		}
	}
}

func TestOpen_IssuesColumns(t *testing.T) {
	idx := openIndex(t)
	db := idx.DB()

	rows, err := db.Query("PRAGMA table_info(issues)")
	if err != nil {
		t.Fatalf("PRAGMA table_info: %v", err)
	}
	defer rows.Close()

	want := map[string]bool{
		"id": true, "type": true, "title": true,
		"status": true, "priority": true,
		"created_at": true, "updated_at": true,
	}
	got := map[string]bool{}
	for rows.Next() {
		var cid, notNull, pk int
		var name, colType string
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &colType, &notNull, &dflt, &pk); err != nil {
			t.Fatalf("scan: %v", err)
		}
		got[name] = true
	}
	for col := range want {
		if !got[col] {
			t.Errorf("issues table missing column %q", col)
		}
	}
}

func TestOpen_Idempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "db.sqlite")

	idx1, err := index.Open(path)
	if err != nil {
		t.Fatalf("first Open: %v", err)
	}
	idx1.Close()

	idx2, err := index.Open(path)
	if err != nil {
		t.Fatalf("second Open: %v", err)
	}
	idx2.Close()
}

// --- rebuild tests ---

func TestRebuild_EmptyDir(t *testing.T) {
	idx := openIndex(t)

	if err := idx.Rebuild(t.TempDir()); err != nil {
		t.Fatalf("Rebuild: %v", err)
	}

	db := idx.DB()
	if n := countRows(t, db, "issues"); n != 0 {
		t.Errorf("issues: got %d rows, want 0", n)
	}
}

func TestRebuild_PopulatesIssues(t *testing.T) {
	idx := openIndex(t)
	issuesDir := t.TempDir()

	mustAppend(t, filepath.Join(issuesDir, "PROJ-001.yaml"),
		model.Event{
			Type: model.EventCreated, Timestamp: t0,
			ID: 1, IssueType: model.TypeTask, Title: "First issue",
			Status: model.StatusOpen, Priority: model.PriorityHigh,
		},
		model.Event{Type: model.EventStatusChanged, Timestamp: t1, Status: model.StatusInProgress},
	)

	if err := idx.Rebuild(issuesDir); err != nil {
		t.Fatalf("Rebuild: %v", err)
	}

	db := idx.DB()
	var id int
	var issueType, title, status, priority, createdAt, updatedAt string
	err := db.QueryRow(
		`SELECT id, type, title, status, priority, created_at, updated_at
		 FROM issues WHERE id = 1`,
	).Scan(&id, &issueType, &title, &status, &priority, &createdAt, &updatedAt)
	if err != nil {
		t.Fatalf("SELECT issue 1: %v", err)
	}

	if issueType != "task" {
		t.Errorf("type: got %q, want %q", issueType, "task")
	}
	if title != "First issue" {
		t.Errorf("title: got %q", title)
	}
	if status != "in_progress" {
		t.Errorf("status: got %q, want %q", status, "in_progress")
	}
	if priority != "high" {
		t.Errorf("priority: got %q, want %q", priority, "high")
	}
	if createdAt != t0.Format(time.RFC3339) {
		t.Errorf("created_at: got %q, want %q", createdAt, t0.Format(time.RFC3339))
	}
	if updatedAt != t1.Format(time.RFC3339) {
		t.Errorf("updated_at: got %q, want %q", updatedAt, t1.Format(time.RFC3339))
	}
}

func TestRebuild_PopulatesDependencies(t *testing.T) {
	idx := openIndex(t)
	issuesDir := t.TempDir()

	mustAppend(t, filepath.Join(issuesDir, "PROJ-001.yaml"),
		model.Event{
			Type: model.EventCreated, Timestamp: t0,
			ID: 1, IssueType: model.TypeTask, Title: "Base",
			Status: model.StatusOpen, Priority: model.PriorityMedium,
		},
	)
	mustAppend(t, filepath.Join(issuesDir, "PROJ-002.yaml"),
		model.Event{
			Type: model.EventCreated, Timestamp: t0,
			ID: 2, IssueType: model.TypeTask, Title: "Dependent",
			Status: model.StatusOpen, Priority: model.PriorityMedium,
			Depends: []int{1},
		},
	)

	if err := idx.Rebuild(issuesDir); err != nil {
		t.Fatalf("Rebuild: %v", err)
	}

	db := idx.DB()
	var depOn int
	if err := db.QueryRow(
		`SELECT depends_on FROM dependencies WHERE issue_id = 2`,
	).Scan(&depOn); err != nil {
		t.Fatalf("SELECT dependency: %v", err)
	}
	if depOn != 1 {
		t.Errorf("depends_on: got %d, want 1", depOn)
	}

	if n := countRows(t, db, "dependencies"); n != 1 {
		t.Errorf("dependency rows: got %d, want 1", n)
	}
}

func TestRebuild_PopulatesClaims(t *testing.T) {
	idx := openIndex(t)
	issuesDir := t.TempDir()

	mustAppend(t, filepath.Join(issuesDir, "PROJ-001.yaml"),
		model.Event{
			Type: model.EventCreated, Timestamp: t0,
			ID: 1, IssueType: model.TypeTask, Title: "Claimed issue",
			Status: model.StatusInProgress, Priority: model.PriorityHigh,
		},
		model.Event{
			Type: model.EventClaimed, Timestamp: t1,
			AgentID: "agent-x", SessionID: "sess-y",
		},
	)

	if err := idx.Rebuild(issuesDir); err != nil {
		t.Fatalf("Rebuild: %v", err)
	}

	db := idx.DB()
	var agentID, sessionID, claimedAt string
	if err := db.QueryRow(
		`SELECT agent_id, session_id, claimed_at FROM claims WHERE issue_id = 1`,
	).Scan(&agentID, &sessionID, &claimedAt); err != nil {
		t.Fatalf("SELECT claim: %v", err)
	}
	if agentID != "agent-x" {
		t.Errorf("agent_id: got %q, want %q", agentID, "agent-x")
	}
	if sessionID != "sess-y" {
		t.Errorf("session_id: got %q, want %q", sessionID, "sess-y")
	}
	if claimedAt != t1.UTC().Format(time.RFC3339) {
		t.Errorf("claimed_at: got %q", claimedAt)
	}
}

func TestRebuild_ReleasedClaimNotStored(t *testing.T) {
	idx := openIndex(t)
	issuesDir := t.TempDir()

	mustAppend(t, filepath.Join(issuesDir, "PROJ-001.yaml"),
		model.Event{
			Type: model.EventCreated, Timestamp: t0,
			ID: 1, IssueType: model.TypeTask, Title: "Released",
			Status: model.StatusOpen, Priority: model.PriorityLow,
		},
		model.Event{Type: model.EventClaimed, Timestamp: t1, AgentID: "agent", SessionID: "s"},
		model.Event{Type: model.EventReleased, Timestamp: t2, ReleasedBy: "human", PreviousClaimant: "agent"},
	)

	if err := idx.Rebuild(issuesDir); err != nil {
		t.Fatalf("Rebuild: %v", err)
	}

	if n := countRows(t, idx.DB(), "claims"); n != 0 {
		t.Errorf("claims: got %d rows after release, want 0", n)
	}
}

func TestRebuild_Idempotent(t *testing.T) {
	idx := openIndex(t)
	issuesDir := t.TempDir()

	mustAppend(t, filepath.Join(issuesDir, "PROJ-001.yaml"),
		model.Event{
			Type: model.EventCreated, Timestamp: t0,
			ID: 1, IssueType: model.TypeIssue, Title: "Bug",
			Status: model.StatusOpen, Priority: model.PriorityHigh,
		},
	)

	if err := idx.Rebuild(issuesDir); err != nil {
		t.Fatalf("first Rebuild: %v", err)
	}
	if err := idx.Rebuild(issuesDir); err != nil {
		t.Fatalf("second Rebuild: %v", err)
	}

	if n := countRows(t, idx.DB(), "issues"); n != 1 {
		t.Errorf("issues after two rebuilds: got %d, want 1", n)
	}
}

func TestRebuild_MultipleIssues(t *testing.T) {
	idx := openIndex(t)
	issuesDir := t.TempDir()

	for i, name := range []string{"PROJ-001.yaml", "PROJ-002.yaml", "PROJ-003.yaml"} {
		id := i + 1
		mustAppend(t, filepath.Join(issuesDir, name),
			model.Event{
				Type: model.EventCreated, Timestamp: t0,
				ID: id, IssueType: model.TypeTask, Title: "Issue",
				Status: model.StatusOpen, Priority: model.PriorityMedium,
			},
		)
	}

	if err := idx.Rebuild(issuesDir); err != nil {
		t.Fatalf("Rebuild: %v", err)
	}

	if n := countRows(t, idx.DB(), "issues"); n != 3 {
		t.Errorf("issues: got %d, want 3", n)
	}
}

func TestRebuild_IgnoresNonYAML(t *testing.T) {
	idx := openIndex(t)
	issuesDir := t.TempDir()

	mustAppend(t, filepath.Join(issuesDir, "PROJ-001.yaml"),
		model.Event{
			Type: model.EventCreated, Timestamp: t0,
			ID: 1, IssueType: model.TypeTask, Title: "Real",
			Status: model.StatusOpen, Priority: model.PriorityLow,
		},
	)
	// Write a non-YAML file that should be ignored.
	os.WriteFile(filepath.Join(issuesDir, "notes.txt"), []byte("not yaml"), 0644)

	if err := idx.Rebuild(issuesDir); err != nil {
		t.Fatalf("Rebuild: %v", err)
	}

	if n := countRows(t, idx.DB(), "issues"); n != 1 {
		t.Errorf("issues: got %d, want 1", n)
	}
}

func TestRebuild_CorruptFileReturnsError(t *testing.T) {
	idx := openIndex(t)
	issuesDir := t.TempDir()

	mustAppend(t, filepath.Join(issuesDir, "PROJ-001.yaml"),
		model.Event{
			Type: model.EventCreated, Timestamp: t0,
			ID: 1, IssueType: model.TypeTask, Title: "Good",
			Status: model.StatusOpen, Priority: model.PriorityLow,
		},
	)

	// Write a corrupt second file: valid first event then bad YAML.
	badPath := filepath.Join(issuesDir, "PROJ-002.yaml")
	mustAppend(t, badPath, model.Event{
		Type: model.EventCreated, Timestamp: t0,
		ID: 2, IssueType: model.TypeTask, Title: "Corrupt",
		Status: model.StatusOpen, Priority: model.PriorityLow,
	})
	f, err := os.OpenFile(badPath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatalf("open bad file: %v", err)
	}
	f.WriteString("---\nevent: comment\n  bad_indent: breaks_yaml\n")
	f.Close()

	if err := idx.Rebuild(issuesDir); err == nil {
		t.Fatal("expected error for corrupt file, got nil")
	}
}
