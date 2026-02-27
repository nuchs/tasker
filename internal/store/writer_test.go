package store_test

import (
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/nuchs/tasker/internal/model"
	"github.com/nuchs/tasker/internal/store"
)

func TestAppendEvent_NewFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.yaml")

	ev := model.Event{
		Type:      model.EventCreated,
		Timestamp: time.Date(2025, 2, 19, 10, 0, 0, 0, time.UTC),
		ID:        1,
		IssueType: model.TypeTask,
		Title:     "New issue",
		Status:    model.StatusOpen,
		Priority:  model.PriorityHigh,
	}

	if err := store.AppendEvent(path, ev); err != nil {
		t.Fatalf("AppendEvent: %v", err)
	}

	events, err := store.ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Type != model.EventCreated {
		t.Errorf("Type: got %q, want %q", events[0].Type, model.EventCreated)
	}
}

func TestAppendEvent_PreservesExistingContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.yaml")

	first := model.Event{
		Type:      model.EventCreated,
		Timestamp: time.Date(2025, 2, 19, 10, 0, 0, 0, time.UTC),
		ID:        1,
		IssueType: model.TypeTask,
		Title:     "First",
		Status:    model.StatusOpen,
		Priority:  model.PriorityMedium,
	}
	second := model.Event{
		Type:      model.EventStatusChanged,
		Timestamp: time.Date(2025, 2, 19, 11, 0, 0, 0, time.UTC),
		Status:    model.StatusInProgress,
	}

	if err := store.AppendEvent(path, first); err != nil {
		t.Fatalf("AppendEvent first: %v", err)
	}
	if err := store.AppendEvent(path, second); err != nil {
		t.Fatalf("AppendEvent second: %v", err)
	}

	events, err := store.ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].Type != model.EventCreated {
		t.Errorf("event[0].Type: got %q, want %q", events[0].Type, model.EventCreated)
	}
	if events[1].Type != model.EventStatusChanged {
		t.Errorf("event[1].Type: got %q, want %q", events[1].Type, model.EventStatusChanged)
	}
	if events[1].Status != model.StatusInProgress {
		t.Errorf("event[1].Status: got %q, want %q", events[1].Status, model.StatusInProgress)
	}
}

func TestAppendEvent_RoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.yaml")

	want := model.Event{
		Type:               model.EventCreated,
		Timestamp:          time.Date(2025, 2, 19, 10, 0, 0, 0, time.UTC),
		ID:                 42,
		IssueType:          model.TypeIssue,
		Title:              "Round-trip test",
		Status:             model.StatusDraft,
		Priority:           model.PriorityLow,
		Depends:            []int{1, 2, 3},
		Description:        "A description.\n",
		AcceptanceCriteria: "- Criterion one\n",
	}

	if err := store.AppendEvent(path, want); err != nil {
		t.Fatalf("AppendEvent: %v", err)
	}

	events, err := store.ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	got := events[0]
	if got.Type != want.Type {
		t.Errorf("Type: got %q, want %q", got.Type, want.Type)
	}
	if !got.Timestamp.Equal(want.Timestamp) {
		t.Errorf("Timestamp: got %v, want %v", got.Timestamp, want.Timestamp)
	}
	if got.ID != want.ID {
		t.Errorf("ID: got %d, want %d", got.ID, want.ID)
	}
	if got.IssueType != want.IssueType {
		t.Errorf("IssueType: got %q, want %q", got.IssueType, want.IssueType)
	}
	if got.Title != want.Title {
		t.Errorf("Title: got %q, want %q", got.Title, want.Title)
	}
	if got.Status != want.Status {
		t.Errorf("Status: got %q, want %q", got.Status, want.Status)
	}
	if got.Priority != want.Priority {
		t.Errorf("Priority: got %q, want %q", got.Priority, want.Priority)
	}
	if len(got.Depends) != len(want.Depends) {
		t.Errorf("Depends: got %v, want %v", got.Depends, want.Depends)
	} else {
		for i := range want.Depends {
			if got.Depends[i] != want.Depends[i] {
				t.Errorf("Depends[%d]: got %d, want %d", i, got.Depends[i], want.Depends[i])
			}
		}
	}
	if got.Description != want.Description {
		t.Errorf("Description: got %q, want %q", got.Description, want.Description)
	}
	if got.AcceptanceCriteria != want.AcceptanceCriteria {
		t.Errorf("AcceptanceCriteria: got %q, want %q", got.AcceptanceCriteria, want.AcceptanceCriteria)
	}
}

func TestAppendEvent_Concurrent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.yaml")

	const n = 50
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ev := model.Event{
				Type:      model.EventComment,
				Timestamp: time.Date(2025, 2, 19, 10, 0, 0, 0, time.UTC),
				Author:    "agent",
				Body:      "concurrent comment\n",
			}
			if err := store.AppendEvent(path, ev); err != nil {
				t.Errorf("goroutine %d: AppendEvent: %v", i, err)
			}
		}(i)
	}
	wg.Wait()

	events, err := store.ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile after concurrent writes: %v", err)
	}
	if len(events) != n {
		t.Errorf("expected %d events, got %d", n, len(events))
	}
	for i, ev := range events {
		if ev.Type != model.EventComment {
			t.Errorf("event[%d].Type: got %q, want %q", i, ev.Type, model.EventComment)
		}
	}
}
