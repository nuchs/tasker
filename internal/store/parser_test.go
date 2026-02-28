package store_test

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/nuchs/tasker/internal/model"
	"github.com/nuchs/tasker/internal/store"
)

func TestParseFile_Empty(t *testing.T) {
	events, err := store.ParseFile(filepath.Join("testdata", "empty.yaml"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 0 {
		t.Errorf("expected 0 events, got %d", len(events))
	}
}

func TestParseFile_CreatedOnly(t *testing.T) {
	events, err := store.ParseFile(filepath.Join("testdata", "created_only.yaml"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	ev := events[0]
	if ev.Type != model.EventCreated {
		t.Errorf("Type: got %q, want %q", ev.Type, model.EventCreated)
	}
	if ev.ID != 1 {
		t.Errorf("ID: got %d, want 1", ev.ID)
	}
	if ev.IssueType != model.TypeTask {
		t.Errorf("IssueType: got %q, want %q", ev.IssueType, model.TypeTask)
	}
	if ev.Title != "Fix auth token refresh race condition" {
		t.Errorf("Title: got %q", ev.Title)
	}
	if ev.Status != model.StatusOpen {
		t.Errorf("Status: got %q, want %q", ev.Status, model.StatusOpen)
	}
	if ev.Priority != model.PriorityHigh {
		t.Errorf("Priority: got %q, want %q", ev.Priority, model.PriorityHigh)
	}
	if len(ev.Depends) != 1 || ev.Depends[0] != 3 {
		t.Errorf("Depends: got %v, want [3]", ev.Depends)
	}
	wantTime := time.Date(2025, 2, 19, 10, 0, 0, 0, time.UTC)
	if !ev.Timestamp.Equal(wantTime) {
		t.Errorf("Timestamp: got %v, want %v", ev.Timestamp, wantTime)
	}
}

func TestParseFile_AllEventTypes(t *testing.T) {
	events, err := store.ParseFile(filepath.Join("testdata", "all_events.yaml"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantTypes := []model.EventType{
		model.EventCreated,
		model.EventStatusChanged,
		model.EventTitleChanged,
		model.EventPriorityChanged,
		model.EventDescriptionUpdated,
		model.EventAcceptanceCriteriaUpdated,
		model.EventDependenciesChanged,
		model.EventComment,
		model.EventClaimed,
		model.EventReleased,
	}

	if len(events) != len(wantTypes) {
		t.Fatalf("expected %d events, got %d", len(wantTypes), len(events))
	}

	for i, want := range wantTypes {
		if events[i].Type != want {
			t.Errorf("event[%d]: got type %q, want %q", i, events[i].Type, want)
		}
	}
}

func TestParseFile_AllEventFields(t *testing.T) {
	events, err := store.ParseFile(filepath.Join("testdata", "all_events.yaml"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	t.Run("created", func(t *testing.T) {
		ev := events[0]
		if ev.ID != 1 {
			t.Errorf("ID: got %d, want 1", ev.ID)
		}
		if ev.IssueType != model.TypeTask {
			t.Errorf("IssueType: got %q, want %q", ev.IssueType, model.TypeTask)
		}
		if ev.Status != model.StatusOpen {
			t.Errorf("Status: got %q, want %q", ev.Status, model.StatusOpen)
		}
		if ev.Priority != model.PriorityHigh {
			t.Errorf("Priority: got %q, want %q", ev.Priority, model.PriorityHigh)
		}
		if len(ev.Depends) != 1 || ev.Depends[0] != 3 {
			t.Errorf("Depends: got %v, want [3]", ev.Depends)
		}
	})

	t.Run("status_changed", func(t *testing.T) {
		ev := events[1]
		if ev.Status != model.StatusInProgress {
			t.Errorf("Status: got %q, want %q", ev.Status, model.StatusInProgress)
		}
	})

	t.Run("title_changed", func(t *testing.T) {
		ev := events[2]
		if ev.Title == "" {
			t.Error("Title: expected non-empty")
		}
	})

	t.Run("priority_changed", func(t *testing.T) {
		ev := events[3]
		if ev.Priority != model.PriorityMedium {
			t.Errorf("Priority: got %q, want %q", ev.Priority, model.PriorityMedium)
		}
	})

	t.Run("description_updated", func(t *testing.T) {
		ev := events[4]
		if ev.Description == "" {
			t.Error("Description: expected non-empty")
		}
	})

	t.Run("acceptance_criteria_updated", func(t *testing.T) {
		ev := events[5]
		if ev.AcceptanceCriteria == "" {
			t.Error("AcceptanceCriteria: expected non-empty")
		}
	})

	t.Run("dependencies_changed", func(t *testing.T) {
		ev := events[6]
		if len(ev.Depends) != 2 {
			t.Errorf("Depends: got %v, want [2 4]", ev.Depends)
		}
	})

	t.Run("comment", func(t *testing.T) {
		ev := events[7]
		if ev.Author != "claude-session-abc" {
			t.Errorf("Author: got %q, want %q", ev.Author, "claude-session-abc")
		}
		if ev.Body == "" {
			t.Error("Body: expected non-empty")
		}
	})

	t.Run("claimed", func(t *testing.T) {
		ev := events[8]
		if ev.AgentID != "claude-session-abc" {
			t.Errorf("AgentID: got %q, want %q", ev.AgentID, "claude-session-abc")
		}
		if ev.SessionID != "sess-12345" {
			t.Errorf("SessionID: got %q, want %q", ev.SessionID, "sess-12345")
		}
	})

	t.Run("released", func(t *testing.T) {
		ev := events[9]
		if ev.ReleasedBy != "human" {
			t.Errorf("ReleasedBy: got %q, want %q", ev.ReleasedBy, "human")
		}
		if ev.PreviousClaimant != "claude-session-abc" {
			t.Errorf("PreviousClaimant: got %q, want %q", ev.PreviousClaimant, "claude-session-abc")
		}
		if ev.Reason != "stale claim" {
			t.Errorf("Reason: got %q, want %q", ev.Reason, "stale claim")
		}
	})
}

func TestParseFile_MalformedFirstEvent(t *testing.T) {
	_, err := store.ParseFile(filepath.Join("testdata", "malformed_first.yaml"))
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var pe *store.ParseError
	if !errors.As(err, &pe) {
		t.Fatalf("expected *store.ParseError, got %T: %v", err, err)
	}
	if pe.Index != 0 {
		t.Errorf("Index: got %d, want 0", pe.Index)
	}
	if pe.Path == "" {
		t.Error("Path: expected non-empty")
	}
	if pe.Cause == nil {
		t.Error("Cause: expected non-nil")
	}
}

func TestParseFile_MalformedSecondEvent(t *testing.T) {
	_, err := store.ParseFile(filepath.Join("testdata", "malformed_second.yaml"))
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var pe *store.ParseError
	if !errors.As(err, &pe) {
		t.Fatalf("expected *store.ParseError, got %T: %v", err, err)
	}
	if pe.Index != 1 {
		t.Errorf("Index: got %d, want 1", pe.Index)
	}
}

func TestParseFile_EmptyEventType(t *testing.T) {
	_, err := store.ParseFile(filepath.Join("testdata", "empty_event_type.yaml"))
	if err == nil {
		t.Fatal("expected error for empty event type, got nil")
	}
	var pe *store.ParseError
	if !errors.As(err, &pe) {
		t.Fatalf("expected *store.ParseError, got %T: %v", err, err)
	}
	if pe.Index != 0 {
		t.Errorf("Index: got %d, want 0", pe.Index)
	}
}

func TestParseFile_NotFound(t *testing.T) {
	_, err := store.ParseFile("testdata/nonexistent.yaml")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	// Should not be a ParseError — it's a file open error.
	var pe *store.ParseError
	if errors.As(err, &pe) {
		t.Error("expected file-open error, got ParseError")
	}
}
