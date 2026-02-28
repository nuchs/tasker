package index_test

import (
	"database/sql"
	"testing"

	"github.com/nuchs/tasker/internal/index"
	"github.com/nuchs/tasker/internal/model"
)

// --- DB fixture helpers (no file parsing) ---

const fixedTime = "2025-02-19T10:00:00Z"

func insertIssue(t *testing.T, db *sql.DB, id int, typ, title, status, priority string) {
	t.Helper()
	_, err := db.Exec(
		`INSERT INTO issues (id, type, title, status, priority, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id, typ, title, status, priority, fixedTime, fixedTime,
	)
	if err != nil {
		t.Fatalf("insertIssue %d: %v", id, err)
	}
}

func insertDep(t *testing.T, db *sql.DB, issueID, dependsOn int) {
	t.Helper()
	if _, err := db.Exec(
		`INSERT INTO dependencies (issue_id, depends_on) VALUES (?, ?)`,
		issueID, dependsOn,
	); err != nil {
		t.Fatalf("insertDep %d->%d: %v", issueID, dependsOn, err)
	}
}

func insertClaim(t *testing.T, db *sql.DB, issueID int) {
	t.Helper()
	if _, err := db.Exec(
		`INSERT INTO claims (issue_id, agent_id, session_id, claimed_at)
		 VALUES (?, 'agent', 'sess', ?)`,
		issueID, fixedTime,
	); err != nil {
		t.Fatalf("insertClaim %d: %v", issueID, err)
	}
}

// --- ListIssues ---

func TestListIssues_Empty(t *testing.T) {
	idx := openIndex(t)
	issues, err := idx.ListIssues(index.Filters{})
	if err != nil {
		t.Fatalf("ListIssues: %v", err)
	}
	if len(issues) != 0 {
		t.Errorf("expected 0 issues, got %d", len(issues))
	}
}

func TestListIssues_NoFilters(t *testing.T) {
	idx := openIndex(t)
	db := idx.DB()
	insertIssue(t, db, 1, "task", "A", "open", "high")
	insertIssue(t, db, 2, "issue", "B", "done", "low")
	insertIssue(t, db, 3, "task", "C", "draft", "medium")

	issues, err := idx.ListIssues(index.Filters{})
	if err != nil {
		t.Fatalf("ListIssues: %v", err)
	}
	if len(issues) != 3 {
		t.Errorf("expected 3 issues, got %d", len(issues))
	}
}

func TestListIssues_FilterByStatus(t *testing.T) {
	idx := openIndex(t)
	db := idx.DB()
	insertIssue(t, db, 1, "task", "Open one", "open", "high")
	insertIssue(t, db, 2, "task", "Done one", "done", "low")
	insertIssue(t, db, 3, "task", "Open two", "open", "medium")

	issues, err := idx.ListIssues(index.Filters{Status: model.StatusOpen})
	if err != nil {
		t.Fatalf("ListIssues: %v", err)
	}
	if len(issues) != 2 {
		t.Errorf("expected 2 open issues, got %d", len(issues))
	}
	for _, iss := range issues {
		if iss.Status != model.StatusOpen {
			t.Errorf("unexpected status %q", iss.Status)
		}
	}
}

func TestListIssues_FilterByPriority(t *testing.T) {
	idx := openIndex(t)
	db := idx.DB()
	insertIssue(t, db, 1, "task", "High", "open", "high")
	insertIssue(t, db, 2, "task", "Medium", "open", "medium")
	insertIssue(t, db, 3, "task", "Low", "open", "low")

	issues, err := idx.ListIssues(index.Filters{Priority: model.PriorityHigh})
	if err != nil {
		t.Fatalf("ListIssues: %v", err)
	}
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(issues))
	}
	if issues[0].Priority != model.PriorityHigh {
		t.Errorf("priority: got %q", issues[0].Priority)
	}
}

func TestListIssues_FilterByType(t *testing.T) {
	idx := openIndex(t)
	db := idx.DB()
	insertIssue(t, db, 1, "task", "A task", "open", "high")
	insertIssue(t, db, 2, "issue", "A bug", "open", "high")

	issues, err := idx.ListIssues(index.Filters{Type: model.TypeIssue})
	if err != nil {
		t.Fatalf("ListIssues: %v", err)
	}
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(issues))
	}
	if issues[0].Type != model.TypeIssue {
		t.Errorf("type: got %q", issues[0].Type)
	}
}

func TestListIssues_FiltersCompose(t *testing.T) {
	idx := openIndex(t)
	db := idx.DB()
	insertIssue(t, db, 1, "task", "Open high task", "open", "high")
	insertIssue(t, db, 2, "issue", "Open high bug", "open", "high")
	insertIssue(t, db, 3, "task", "Open low task", "open", "low")
	insertIssue(t, db, 4, "task", "Done high task", "done", "high")

	issues, err := idx.ListIssues(index.Filters{
		Status:   model.StatusOpen,
		Priority: model.PriorityHigh,
		Type:     model.TypeTask,
	})
	if err != nil {
		t.Fatalf("ListIssues: %v", err)
	}
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(issues))
	}
	if issues[0].ID != 1 {
		t.Errorf("ID: got %d, want 1", issues[0].ID)
	}
}

func TestListIssues_OrderedByID(t *testing.T) {
	idx := openIndex(t)
	db := idx.DB()
	insertIssue(t, db, 3, "task", "Third", "open", "low")
	insertIssue(t, db, 1, "task", "First", "open", "high")
	insertIssue(t, db, 2, "task", "Second", "open", "medium")

	issues, err := idx.ListIssues(index.Filters{})
	if err != nil {
		t.Fatalf("ListIssues: %v", err)
	}
	for i, want := range []int{1, 2, 3} {
		if issues[i].ID != want {
			t.Errorf("issues[%d].ID: got %d, want %d", i, issues[i].ID, want)
		}
	}
}

// --- ReadyIssues ---

func TestReadyIssues_BasicOpen(t *testing.T) {
	idx := openIndex(t)
	db := idx.DB()
	insertIssue(t, db, 1, "task", "Ready", "open", "high")

	issues, err := idx.ReadyIssues()
	if err != nil {
		t.Fatalf("ReadyIssues: %v", err)
	}
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(issues))
	}
	if issues[0].ID != 1 {
		t.Errorf("ID: got %d, want 1", issues[0].ID)
	}
}

func TestReadyIssues_ExcludesClaimed(t *testing.T) {
	idx := openIndex(t)
	db := idx.DB()
	insertIssue(t, db, 1, "task", "Claimed", "open", "high")
	insertClaim(t, db, 1)

	issues, err := idx.ReadyIssues()
	if err != nil {
		t.Fatalf("ReadyIssues: %v", err)
	}
	if len(issues) != 0 {
		t.Errorf("expected 0 issues, got %d", len(issues))
	}
}

func TestReadyIssues_ExcludesDraft(t *testing.T) {
	idx := openIndex(t)
	db := idx.DB()
	insertIssue(t, db, 1, "task", "Draft", "draft", "high")

	issues, err := idx.ReadyIssues()
	if err != nil {
		t.Fatalf("ReadyIssues: %v", err)
	}
	if len(issues) != 0 {
		t.Errorf("expected 0 draft issues, got %d", len(issues))
	}
}

func TestReadyIssues_ExcludesNonOpenStatuses(t *testing.T) {
	idx := openIndex(t)
	db := idx.DB()
	for i, status := range []string{"draft", "in_progress", "review", "done", "cancelled", "blocked"} {
		insertIssue(t, db, i+1, "task", status+" issue", status, "high")
	}

	issues, err := idx.ReadyIssues()
	if err != nil {
		t.Fatalf("ReadyIssues: %v", err)
	}
	if len(issues) != 0 {
		t.Errorf("expected 0 issues, got %d", len(issues))
	}
}

func TestReadyIssues_ExcludesUnresolvedDeps(t *testing.T) {
	idx := openIndex(t)
	db := idx.DB()
	insertIssue(t, db, 1, "task", "Blocker", "open", "high")
	insertIssue(t, db, 2, "task", "Blocked", "open", "high")
	insertDep(t, db, 2, 1) // 2 depends on 1 (which is still open)

	issues, err := idx.ReadyIssues()
	if err != nil {
		t.Fatalf("ReadyIssues: %v", err)
	}
	// Only issue 1 (the blocker) is ready; issue 2 has an unresolved dep.
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(issues))
	}
	if issues[0].ID != 1 {
		t.Errorf("ID: got %d, want 1", issues[0].ID)
	}
}

func TestReadyIssues_IncludesDoneDepResolved(t *testing.T) {
	idx := openIndex(t)
	db := idx.DB()
	insertIssue(t, db, 1, "task", "Done blocker", "done", "high")
	insertIssue(t, db, 2, "task", "Unblocked", "open", "medium")
	insertDep(t, db, 2, 1) // 2 depends on 1 (which is done)

	issues, err := idx.ReadyIssues()
	if err != nil {
		t.Fatalf("ReadyIssues: %v", err)
	}
	if len(issues) != 1 {
		t.Fatalf("expected 1 ready issue, got %d", len(issues))
	}
	if issues[0].ID != 2 {
		t.Errorf("ID: got %d, want 2", issues[0].ID)
	}
}

func TestReadyIssues_IncludesCancelledDepResolved(t *testing.T) {
	idx := openIndex(t)
	db := idx.DB()
	insertIssue(t, db, 1, "task", "Cancelled blocker", "cancelled", "high")
	insertIssue(t, db, 2, "task", "Unblocked", "open", "low")
	insertDep(t, db, 2, 1)

	issues, err := idx.ReadyIssues()
	if err != nil {
		t.Fatalf("ReadyIssues: %v", err)
	}
	if len(issues) != 1 || issues[0].ID != 2 {
		t.Errorf("expected issue 2, got %v", issues)
	}
}

func TestReadyIssues_OrderedByPriority(t *testing.T) {
	idx := openIndex(t)
	db := idx.DB()
	insertIssue(t, db, 1, "task", "Low", "open", "low")
	insertIssue(t, db, 2, "task", "High", "open", "high")
	insertIssue(t, db, 3, "task", "Medium", "open", "medium")

	issues, err := idx.ReadyIssues()
	if err != nil {
		t.Fatalf("ReadyIssues: %v", err)
	}
	if len(issues) != 3 {
		t.Fatalf("expected 3 issues, got %d", len(issues))
	}
	wantOrder := []model.Priority{model.PriorityHigh, model.PriorityMedium, model.PriorityLow}
	for i, want := range wantOrder {
		if issues[i].Priority != want {
			t.Errorf("issues[%d].Priority: got %q, want %q", i, issues[i].Priority, want)
		}
	}
}

// --- GetIssueMeta ---

func TestGetIssueMeta(t *testing.T) {
	idx := openIndex(t)
	db := idx.DB()
	insertIssue(t, db, 42, "issue", "Some bug", "in_progress", "medium")

	meta, err := idx.GetIssueMeta(42)
	if err != nil {
		t.Fatalf("GetIssueMeta: %v", err)
	}
	if meta.ID != 42 {
		t.Errorf("ID: got %d, want 42", meta.ID)
	}
	if meta.Type != model.TypeIssue {
		t.Errorf("Type: got %q", meta.Type)
	}
	if meta.Title != "Some bug" {
		t.Errorf("Title: got %q", meta.Title)
	}
	if meta.Status != model.StatusInProgress {
		t.Errorf("Status: got %q", meta.Status)
	}
	if meta.Priority != model.PriorityMedium {
		t.Errorf("Priority: got %q", meta.Priority)
	}
}

func TestGetIssueMeta_NotFound(t *testing.T) {
	idx := openIndex(t)
	_, err := idx.GetIssueMeta(999)
	if err == nil {
		t.Fatal("expected error for missing issue, got nil")
	}
}

// --- ReleaseIssue ---

func TestReleaseIssue(t *testing.T) {
	idx := openIndex(t)
	db := idx.DB()
	insertIssue(t, db, 1, "task", "Issue", "open", "high")
	insertClaim(t, db, 1)

	if err := idx.ReleaseIssue(1); err != nil {
		t.Fatalf("ReleaseIssue: %v", err)
	}

	var n int
	db.QueryRow(`SELECT COUNT(*) FROM claims WHERE issue_id = 1`).Scan(&n)
	if n != 0 {
		t.Errorf("claim still present after release")
	}
}

func TestReleaseIssue_NotClaimed(t *testing.T) {
	idx := openIndex(t)
	db := idx.DB()
	insertIssue(t, db, 1, "task", "Issue", "open", "high")

	// Should be a no-op, not an error.
	if err := idx.ReleaseIssue(1); err != nil {
		t.Fatalf("ReleaseIssue on unclaimed issue: %v", err)
	}
}

// --- NextID ---

func TestNextID_EmptyDB(t *testing.T) {
	idx := openIndex(t)
	id, err := idx.NextID()
	if err != nil {
		t.Fatalf("NextID: %v", err)
	}
	if id != 1 {
		t.Errorf("NextID: got %d, want 1", id)
	}
}

func TestNextID_WithIssues(t *testing.T) {
	idx := openIndex(t)
	db := idx.DB()
	insertIssue(t, db, 1, "task", "A", "open", "high")
	insertIssue(t, db, 2, "task", "B", "open", "low")
	insertIssue(t, db, 5, "task", "C", "open", "medium") // gap in IDs

	id, err := idx.NextID()
	if err != nil {
		t.Fatalf("NextID: %v", err)
	}
	if id != 6 {
		t.Errorf("NextID: got %d, want 6", id)
	}
}
