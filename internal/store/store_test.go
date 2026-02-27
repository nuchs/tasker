package store

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/nuchs/tasker/internal/index"
	"github.com/nuchs/tasker/internal/model"
)

// openTestStore creates a Store backed by a temp directory, with an empty
// prefix so files are named by bare numeric ID.
func openTestStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	issuesDir := filepath.Join(dir, "issues")
	if err := os.MkdirAll(issuesDir, 0755); err != nil {
		t.Fatalf("mkdir issues: %v", err)
	}
	dbPath := filepath.Join(dir, "db.sqlite")
	s, err := Open(issuesDir, dbPath, "")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestCreate_WritesFileAndUpdatesIndex(t *testing.T) {
	s := openTestStore(t)

	ev := model.Event{
		IssueType:          model.TypeTask,
		Title:              "First issue",
		Description:        "Do something useful",
		AcceptanceCriteria: "It works",
		Status:             model.StatusOpen,
		Priority:           model.PriorityMedium,
	}

	id, err := s.Create(ev)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if id != 1 {
		t.Errorf("expected id 1, got %d", id)
	}

	// Verify content file was written with a created event.
	path := filepath.Join(s.issuesDir, "1.yaml")
	events, err := ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Type != model.EventCreated {
		t.Errorf("expected created event, got %v", events[0].Type)
	}
	if events[0].ID != 1 {
		t.Errorf("expected event id 1, got %d", events[0].ID)
	}

	// Verify the index has the issue.
	meta, err := s.idx.GetIssueMeta(1)
	if err != nil {
		t.Fatalf("GetIssueMeta: %v", err)
	}
	if meta.Title != "First issue" {
		t.Errorf("expected title %q, got %q", "First issue", meta.Title)
	}
	if meta.Status != model.StatusOpen {
		t.Errorf("expected status open, got %v", meta.Status)
	}
}

func TestCreate_AssignsSequentialIDs(t *testing.T) {
	s := openTestStore(t)

	for i := 1; i <= 3; i++ {
		ev := model.Event{
			IssueType: model.TypeTask,
			Title:     "Issue",
			Status:    model.StatusOpen,
			Priority:  model.PriorityLow,
		}
		id, err := s.Create(ev)
		if err != nil {
			t.Fatalf("Create #%d: %v", i, err)
		}
		if id != i {
			t.Errorf("expected id %d, got %d", i, id)
		}
	}
}

func TestCreate_WithPrefix(t *testing.T) {
	dir := t.TempDir()
	issuesDir := filepath.Join(dir, "issues")
	if err := os.MkdirAll(issuesDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	s, err := Open(issuesDir, filepath.Join(dir, "db.sqlite"), "PROJ")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	_, err = s.Create(model.Event{
		IssueType: model.TypeTask,
		Title:     "Prefixed",
		Status:    model.StatusOpen,
		Priority:  model.PriorityLow,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	expected := filepath.Join(issuesDir, "PROJ-0001.yaml")
	if _, err := os.Stat(expected); err != nil {
		t.Errorf("expected file %s to exist: %v", expected, err)
	}
}

func TestAppend_UpdatesFileAndIndex(t *testing.T) {
	s := openTestStore(t)

	id, err := s.Create(model.Event{
		IssueType: model.TypeTask,
		Title:     "My issue",
		Status:    model.StatusOpen,
		Priority:  model.PriorityMedium,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Append a status change.
	if err := s.Append(id, model.Event{
		Type:   model.EventStatusChanged,
		Status: model.StatusDone,
	}); err != nil {
		t.Fatalf("Append: %v", err)
	}

	// File should now have two events.
	path := filepath.Join(s.issuesDir, "1.yaml")
	events, err := ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}

	// Index should reflect the new status.
	meta, err := s.idx.GetIssueMeta(id)
	if err != nil {
		t.Fatalf("GetIssueMeta: %v", err)
	}
	if meta.Status != model.StatusDone {
		t.Errorf("expected status done, got %v", meta.Status)
	}
}

func TestShow_ReturnsMaterialisedIssue(t *testing.T) {
	s := openTestStore(t)

	id, err := s.Create(model.Event{
		IssueType:          model.TypeIssue,
		Title:              "Show me",
		Description:        "Full description here",
		AcceptanceCriteria: "Works as expected",
		Status:             model.StatusOpen,
		Priority:           model.PriorityHigh,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	issue, stale, err := s.Show(id)
	if err != nil {
		t.Fatalf("Show: %v", err)
	}
	if stale {
		t.Error("expected not stale on fresh unclaimed issue")
	}
	if issue.Title != "Show me" {
		t.Errorf("title: got %q", issue.Title)
	}
	if issue.Description != "Full description here" {
		t.Errorf("description: got %q", issue.Description)
	}
	if issue.AcceptanceCriteria != "Works as expected" {
		t.Errorf("acceptance_criteria: got %q", issue.AcceptanceCriteria)
	}
	if issue.Type != model.TypeIssue {
		t.Errorf("type: got %v", issue.Type)
	}
	if issue.Priority != model.PriorityHigh {
		t.Errorf("priority: got %v", issue.Priority)
	}
}

func TestShow_StaleClaimDetection(t *testing.T) {
	epoch := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	s := openTestStore(t)
	s.now = func() time.Time { return epoch }

	id, err := s.Create(model.Event{
		IssueType: model.TypeTask,
		Title:     "Claimed",
		Status:    model.StatusOpen,
		Priority:  model.PriorityMedium,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := s.Append(id, model.Event{
		Type:      model.EventClaimed,
		AgentID:   "agent-1",
		SessionID: "sess-1",
	}); err != nil {
		t.Fatalf("Append claim: %v", err)
	}

	t.Run("not stale within threshold", func(t *testing.T) {
		s.now = func() time.Time { return epoch.Add(1 * time.Hour) }
		_, stale, err := s.Show(id)
		if err != nil {
			t.Fatalf("Show: %v", err)
		}
		if stale {
			t.Error("expected not stale at 1h")
		}
	})

	t.Run("stale beyond threshold", func(t *testing.T) {
		s.now = func() time.Time { return epoch.Add(3 * time.Hour) }
		_, stale, err := s.Show(id)
		if err != nil {
			t.Fatalf("Show: %v", err)
		}
		if !stale {
			t.Error("expected stale at 3h")
		}
	})

	t.Run("not stale after release", func(t *testing.T) {
		s.now = func() time.Time { return epoch.Add(1 * time.Hour) }
		if err := s.Append(id, model.Event{
			Type:       model.EventReleased,
			ReleasedBy: "human",
		}); err != nil {
			t.Fatalf("Append release: %v", err)
		}
		// Far in the future, but no claim — not stale.
		s.now = func() time.Time { return epoch.Add(48 * time.Hour) }
		issue, stale, err := s.Show(id)
		if err != nil {
			t.Fatalf("Show: %v", err)
		}
		if stale {
			t.Error("expected not stale after release")
		}
		if issue.Claim != nil {
			t.Error("expected nil claim after release")
		}
	})
}

func TestRebuild_ReproducesIndexFromFiles(t *testing.T) {
	s := openTestStore(t)

	for _, title := range []string{"Alpha", "Beta"} {
		_, err := s.Create(model.Event{
			IssueType: model.TypeTask,
			Title:     title,
			Status:    model.StatusOpen,
			Priority:  model.PriorityMedium,
		})
		if err != nil {
			t.Fatalf("Create %q: %v", title, err)
		}
	}

	if err := s.Rebuild(); err != nil {
		t.Fatalf("Rebuild: %v", err)
	}

	issues, err := s.idx.ListIssues(index.Filters{})
	if err != nil {
		t.Fatalf("ListIssues: %v", err)
	}
	if len(issues) != 2 {
		t.Errorf("expected 2 issues after rebuild, got %d", len(issues))
	}
}

func TestRebuild_EmptyDir(t *testing.T) {
	s := openTestStore(t)

	if err := s.Rebuild(); err != nil {
		t.Fatalf("Rebuild on empty dir: %v", err)
	}

	issues, err := s.idx.ListIssues(index.Filters{})
	if err != nil {
		t.Fatalf("ListIssues: %v", err)
	}
	if len(issues) != 0 {
		t.Errorf("expected 0 issues, got %d", len(issues))
	}
}

func TestRebuild_Idempotent(t *testing.T) {
	s := openTestStore(t)

	_, err := s.Create(model.Event{
		IssueType: model.TypeTask,
		Title:     "Bug",
		Status:    model.StatusOpen,
		Priority:  model.PriorityHigh,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := s.Rebuild(); err != nil {
		t.Fatalf("first Rebuild: %v", err)
	}
	if err := s.Rebuild(); err != nil {
		t.Fatalf("second Rebuild: %v", err)
	}

	issues, err := s.idx.ListIssues(index.Filters{})
	if err != nil {
		t.Fatalf("ListIssues: %v", err)
	}
	if len(issues) != 1 {
		t.Errorf("expected 1 issue after two rebuilds, got %d", len(issues))
	}
}

func TestRebuild_IgnoresNonYAML(t *testing.T) {
	s := openTestStore(t)

	_, err := s.Create(model.Event{
		IssueType: model.TypeTask,
		Title:     "Real",
		Status:    model.StatusOpen,
		Priority:  model.PriorityLow,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	// Write a non-YAML file that must be ignored.
	if err := os.WriteFile(filepath.Join(s.issuesDir, "notes.txt"), []byte("not yaml"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if err := s.Rebuild(); err != nil {
		t.Fatalf("Rebuild: %v", err)
	}

	issues, err := s.idx.ListIssues(index.Filters{})
	if err != nil {
		t.Fatalf("ListIssues: %v", err)
	}
	if len(issues) != 1 {
		t.Errorf("expected 1 issue, got %d", len(issues))
	}
}

func TestRebuild_CorruptFileReturnsError(t *testing.T) {
	s := openTestStore(t)

	_, err := s.Create(model.Event{
		IssueType: model.TypeTask,
		Title:     "Good",
		Status:    model.StatusOpen,
		Priority:  model.PriorityLow,
	})
	if err != nil {
		t.Fatalf("Create good: %v", err)
	}

	// Write a corrupt second file: valid created event then bad YAML.
	badPath := filepath.Join(s.issuesDir, "2.yaml")
	if err := AppendEvent(badPath, model.Event{
		Type:      model.EventCreated,
		Timestamp: time.Now(),
		ID:        2,
		IssueType: model.TypeTask,
		Title:     "Corrupt",
		Status:    model.StatusOpen,
		Priority:  model.PriorityLow,
	}); err != nil {
		t.Fatalf("AppendEvent: %v", err)
	}
	f, err := os.OpenFile(badPath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatalf("open bad file: %v", err)
	}
	f.WriteString("---\nevent: comment\n  bad_indent: breaks_yaml\n")
	f.Close()

	if err := s.Rebuild(); err == nil {
		t.Fatal("expected error for corrupt file, got nil")
	}
}

func TestRebuild_PopulatesDependencies(t *testing.T) {
	epoch := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	s := openTestStore(t)
	s.now = func() time.Time { return epoch }

	id1, err := s.Create(model.Event{
		IssueType: model.TypeTask, Title: "Base",
		Status: model.StatusOpen, Priority: model.PriorityMedium,
	})
	if err != nil {
		t.Fatalf("Create base: %v", err)
	}

	_, err = s.Create(model.Event{
		IssueType: model.TypeTask, Title: "Dependent",
		Status: model.StatusOpen, Priority: model.PriorityMedium,
		Depends: []int{id1},
	})
	if err != nil {
		t.Fatalf("Create dependent: %v", err)
	}

	if err := s.Rebuild(); err != nil {
		t.Fatalf("Rebuild: %v", err)
	}

	// Only issue 1 should be ready (issue 2 has an unresolved dep).
	ready, err := s.idx.ReadyIssues()
	if err != nil {
		t.Fatalf("ReadyIssues: %v", err)
	}
	if len(ready) != 1 || ready[0].ID != id1 {
		t.Errorf("expected only issue %d ready, got %v", id1, ready)
	}
}
