package index_test

import (
	"database/sql"
	"testing"
	"time"

	"github.com/nuchs/tasker/internal/index"
	"github.com/nuchs/tasker/internal/model"
)

// --- helpers ---

func openIndex(t *testing.T) *index.Index {
	t.Helper()
	idx, err := index.Open(t.TempDir() + "/db.sqlite")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { idx.Close() })
	return idx
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
	path := t.TempDir() + "/db.sqlite"

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

// --- Reset tests ---

func TestReset_ClearsAllTables(t *testing.T) {
	idx := openIndex(t)

	if _, err := idx.DB().Exec(
		`INSERT INTO issues (id, type, title, status, priority, created_at, updated_at)
		 VALUES (1, 'task', 'T', 'open', 'medium', '2025-01-01T00:00:00Z', '2025-01-01T00:00:00Z')`,
	); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if n := countRows(t, idx.DB(), "issues"); n != 1 {
		t.Fatalf("pre-reset: expected 1 issue, got %d", n)
	}

	if err := idx.Reset(); err != nil {
		t.Fatalf("Reset: %v", err)
	}

	if n := countRows(t, idx.DB(), "issues"); n != 0 {
		t.Errorf("post-reset: expected 0 issues, got %d", n)
	}
}

func TestOpen_ForeignKeysEnforced(t *testing.T) {
	idx := openIndex(t)
	// Attempting to insert a claim for a non-existent issue must fail.
	_, err := idx.DB().Exec(
		`INSERT INTO claims (issue_id, agent_id, session_id, claimed_at)
		 VALUES (999, 'agent', 'sess', '2025-01-01T00:00:00Z')`,
	)
	if err == nil {
		t.Fatal("expected foreign key error inserting claim for non-existent issue, got nil")
	}
}

// --- UpsertIssue tests ---

func TestUpsertIssue_Insert(t *testing.T) {
	idx := openIndex(t)

	issue := model.Issue{
		ID: 1, Type: model.TypeTask, Title: "New",
		Status: model.StatusOpen, Priority: model.PriorityMedium,
	}
	if err := idx.UpsertIssue(issue, t0, t1); err != nil {
		t.Fatalf("UpsertIssue: %v", err)
	}

	meta, err := idx.GetIssueMeta(1)
	if err != nil {
		t.Fatalf("GetIssueMeta: %v", err)
	}
	if meta.Title != "New" {
		t.Errorf("title: got %q", meta.Title)
	}
}

func TestUpsertIssue_Replace(t *testing.T) {
	idx := openIndex(t)

	issue := model.Issue{
		ID: 1, Type: model.TypeTask, Title: "Old",
		Status: model.StatusOpen, Priority: model.PriorityLow,
	}
	if err := idx.UpsertIssue(issue, t0, t0); err != nil {
		t.Fatalf("first UpsertIssue: %v", err)
	}

	issue.Title = "Updated"
	issue.Status = model.StatusDone
	if err := idx.UpsertIssue(issue, t0, t1); err != nil {
		t.Fatalf("second UpsertIssue: %v", err)
	}

	meta, err := idx.GetIssueMeta(1)
	if err != nil {
		t.Fatalf("GetIssueMeta: %v", err)
	}
	if meta.Title != "Updated" {
		t.Errorf("title: got %q, want Updated", meta.Title)
	}
	if meta.Status != model.StatusDone {
		t.Errorf("status: got %v, want done", meta.Status)
	}
}

func TestUpsertIssue_UpdatesDependencies(t *testing.T) {
	idx := openIndex(t)

	base := model.Issue{
		ID: 1, Type: model.TypeTask, Title: "Base",
		Status: model.StatusOpen, Priority: model.PriorityLow,
	}
	if err := idx.UpsertIssue(base, t0, t0); err != nil {
		t.Fatalf("UpsertIssue base: %v", err)
	}

	dep := model.Issue{
		ID: 2, Type: model.TypeTask, Title: "Dependent",
		Status: model.StatusOpen, Priority: model.PriorityLow,
		Depends: []int{1},
	}
	if err := idx.UpsertIssue(dep, t0, t0); err != nil {
		t.Fatalf("UpsertIssue dep: %v", err)
	}
	if n := countRows(t, idx.DB(), "dependencies"); n != 1 {
		t.Errorf("deps: got %d, want 1", n)
	}

	dep.Depends = nil
	if err := idx.UpsertIssue(dep, t0, t1); err != nil {
		t.Fatalf("UpsertIssue clear dep: %v", err)
	}
	if n := countRows(t, idx.DB(), "dependencies"); n != 0 {
		t.Errorf("deps after clear: got %d, want 0", n)
	}
}

func TestUpsertIssue_UpdatesClaim(t *testing.T) {
	idx := openIndex(t)

	issue := model.Issue{
		ID: 1, Type: model.TypeTask, Title: "T",
		Status: model.StatusOpen, Priority: model.PriorityLow,
		Claim: &model.Claim{AgentID: "a", SessionID: "s", ClaimedAt: t0},
	}
	if err := idx.UpsertIssue(issue, t0, t0); err != nil {
		t.Fatalf("UpsertIssue with claim: %v", err)
	}
	if n := countRows(t, idx.DB(), "claims"); n != 1 {
		t.Errorf("claims: got %d, want 1", n)
	}

	issue.Claim = nil
	if err := idx.UpsertIssue(issue, t0, t1); err != nil {
		t.Fatalf("UpsertIssue release claim: %v", err)
	}
	if n := countRows(t, idx.DB(), "claims"); n != 0 {
		t.Errorf("claims after release: got %d, want 0", n)
	}
}
