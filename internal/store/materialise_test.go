package store_test

import (
	"testing"
	"time"

	"github.com/nuchs/tasker/internal/model"
	"github.com/nuchs/tasker/internal/store"
)

var baseTime = time.Date(2025, 2, 19, 10, 0, 0, 0, time.UTC)

func createdEvent() model.Event {
	return model.Event{
		Type:               model.EventCreated,
		Timestamp:          baseTime,
		ID:                 1,
		IssueType:          model.TypeTask,
		Title:              "Original title",
		Description:        "Original description.\n",
		Status:             model.StatusOpen,
		Priority:           model.PriorityHigh,
		Depends:            []int{2, 3},
		AcceptanceCriteria: "Original criteria.\n",
	}
}

func TestMaterialise_CreatedOnly(t *testing.T) {
	ev := createdEvent()
	issue, err := store.Materialise([]model.Event{ev})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if issue.ID != 1 {
		t.Errorf("ID: got %d, want 1", issue.ID)
	}
	if issue.Type != model.TypeTask {
		t.Errorf("Type: got %q, want %q", issue.Type, model.TypeTask)
	}
	if issue.Title != "Original title" {
		t.Errorf("Title: got %q", issue.Title)
	}
	if issue.Description != "Original description.\n" {
		t.Errorf("Description: got %q", issue.Description)
	}
	if issue.Status != model.StatusOpen {
		t.Errorf("Status: got %q, want %q", issue.Status, model.StatusOpen)
	}
	if issue.Priority != model.PriorityHigh {
		t.Errorf("Priority: got %q, want %q", issue.Priority, model.PriorityHigh)
	}
	if len(issue.Depends) != 2 || issue.Depends[0] != 2 || issue.Depends[1] != 3 {
		t.Errorf("Depends: got %v, want [2 3]", issue.Depends)
	}
	if issue.AcceptanceCriteria != "Original criteria.\n" {
		t.Errorf("AcceptanceCriteria: got %q", issue.AcceptanceCriteria)
	}
	if issue.Claim != nil {
		t.Errorf("Claim: expected nil, got %+v", issue.Claim)
	}
}

func TestMaterialise_StatusChanged(t *testing.T) {
	events := []model.Event{
		createdEvent(),
		{Type: model.EventStatusChanged, Timestamp: baseTime.Add(time.Hour), Status: model.StatusInProgress},
	}
	issue, err := store.Materialise(events)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if issue.Status != model.StatusInProgress {
		t.Errorf("Status: got %q, want %q", issue.Status, model.StatusInProgress)
	}
}

func TestMaterialise_TitleChanged(t *testing.T) {
	events := []model.Event{
		createdEvent(),
		{Type: model.EventTitleChanged, Timestamp: baseTime.Add(time.Hour), Title: "New title"},
	}
	issue, err := store.Materialise(events)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if issue.Title != "New title" {
		t.Errorf("Title: got %q, want %q", issue.Title, "New title")
	}
}

func TestMaterialise_PriorityChanged(t *testing.T) {
	events := []model.Event{
		createdEvent(),
		{Type: model.EventPriorityChanged, Timestamp: baseTime.Add(time.Hour), Priority: model.PriorityLow},
	}
	issue, err := store.Materialise(events)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if issue.Priority != model.PriorityLow {
		t.Errorf("Priority: got %q, want %q", issue.Priority, model.PriorityLow)
	}
}

func TestMaterialise_DescriptionUpdated(t *testing.T) {
	events := []model.Event{
		createdEvent(),
		{Type: model.EventDescriptionUpdated, Timestamp: baseTime.Add(time.Hour), Description: "New description.\n"},
	}
	issue, err := store.Materialise(events)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if issue.Description != "New description.\n" {
		t.Errorf("Description: got %q", issue.Description)
	}
}

func TestMaterialise_AcceptanceCriteriaUpdated(t *testing.T) {
	events := []model.Event{
		createdEvent(),
		{Type: model.EventAcceptanceCriteriaUpdated, Timestamp: baseTime.Add(time.Hour), AcceptanceCriteria: "New criteria.\n"},
	}
	issue, err := store.Materialise(events)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if issue.AcceptanceCriteria != "New criteria.\n" {
		t.Errorf("AcceptanceCriteria: got %q", issue.AcceptanceCriteria)
	}
}

func TestMaterialise_DependenciesChanged(t *testing.T) {
	events := []model.Event{
		createdEvent(),
		{Type: model.EventDependenciesChanged, Timestamp: baseTime.Add(time.Hour), Depends: []int{5, 6, 7}},
	}
	issue, err := store.Materialise(events)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(issue.Depends) != 3 || issue.Depends[0] != 5 || issue.Depends[1] != 6 || issue.Depends[2] != 7 {
		t.Errorf("Depends: got %v, want [5 6 7]", issue.Depends)
	}
}

func TestMaterialise_DependenciesCleared(t *testing.T) {
	events := []model.Event{
		createdEvent(),
		{Type: model.EventDependenciesChanged, Timestamp: baseTime.Add(time.Hour), Depends: nil},
	}
	issue, err := store.Materialise(events)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(issue.Depends) != 0 {
		t.Errorf("Depends: got %v, want empty", issue.Depends)
	}
}

func TestMaterialise_ClaimLifecycle(t *testing.T) {
	claimTime := baseTime.Add(time.Hour)
	events := []model.Event{
		createdEvent(),
		{
			Type:      model.EventClaimed,
			Timestamp: claimTime,
			AgentID:   "agent-1",
			SessionID: "sess-abc",
		},
	}

	issue, err := store.Materialise(events)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if issue.Claim == nil {
		t.Fatal("Claim: expected non-nil after claimed event")
	}
	if issue.Claim.AgentID != "agent-1" {
		t.Errorf("Claim.AgentID: got %q, want %q", issue.Claim.AgentID, "agent-1")
	}
	if issue.Claim.SessionID != "sess-abc" {
		t.Errorf("Claim.SessionID: got %q, want %q", issue.Claim.SessionID, "sess-abc")
	}
	if !issue.Claim.ClaimedAt.Equal(claimTime) {
		t.Errorf("Claim.ClaimedAt: got %v, want %v", issue.Claim.ClaimedAt, claimTime)
	}

	// Now release it.
	events = append(events, model.Event{
		Type:             model.EventReleased,
		Timestamp:        baseTime.Add(2 * time.Hour),
		ReleasedBy:       "human",
		PreviousClaimant: "agent-1",
	})
	issue, err = store.Materialise(events)
	if err != nil {
		t.Fatalf("unexpected error after release: %v", err)
	}
	if issue.Claim != nil {
		t.Errorf("Claim: expected nil after released event, got %+v", issue.Claim)
	}
}

func TestMaterialise_CommentNoEffect(t *testing.T) {
	events := []model.Event{
		createdEvent(),
		{Type: model.EventComment, Timestamp: baseTime.Add(time.Hour), Author: "agent", Body: "a comment\n"},
	}
	before, _ := store.Materialise([]model.Event{createdEvent()})
	after, err := store.Materialise(events)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if before.Title != after.Title {
		t.Errorf("Title changed by comment: %q -> %q", before.Title, after.Title)
	}
	if before.Status != after.Status {
		t.Errorf("Status changed by comment: %q -> %q", before.Status, after.Status)
	}
	if before.Priority != after.Priority {
		t.Errorf("Priority changed by comment: %q -> %q", before.Priority, after.Priority)
	}
	if before.Description != after.Description {
		t.Errorf("Description changed by comment")
	}
}

func TestMaterialise_ParseErrorSkipped(t *testing.T) {
	events := []model.Event{
		createdEvent(),
		{
			Type:          model.EventParseError,
			Timestamp:     baseTime.Add(time.Hour),
			OriginalBytes: "some corrupt bytes",
			Diagnostic:    "unexpected EOF",
		},
		{Type: model.EventStatusChanged, Timestamp: baseTime.Add(2 * time.Hour), Status: model.StatusDone},
	}
	issue, err := store.Materialise(events)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// parse_error is skipped; status_changed should still apply.
	if issue.Status != model.StatusDone {
		t.Errorf("Status: got %q, want %q", issue.Status, model.StatusDone)
	}
}

func TestMaterialise_LastWriteWins(t *testing.T) {
	events := []model.Event{
		createdEvent(),
		{Type: model.EventStatusChanged, Timestamp: baseTime.Add(time.Hour), Status: model.StatusInProgress},
		{Type: model.EventStatusChanged, Timestamp: baseTime.Add(2 * time.Hour), Status: model.StatusReview},
		{Type: model.EventTitleChanged, Timestamp: baseTime.Add(3 * time.Hour), Title: "Second title"},
		{Type: model.EventTitleChanged, Timestamp: baseTime.Add(4 * time.Hour), Title: "Final title"},
	}
	issue, err := store.Materialise(events)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if issue.Status != model.StatusReview {
		t.Errorf("Status: got %q, want %q", issue.Status, model.StatusReview)
	}
	if issue.Title != "Final title" {
		t.Errorf("Title: got %q, want %q", issue.Title, "Final title")
	}
}

func TestMaterialise_AllEventTypes(t *testing.T) {
	claimTime := baseTime.Add(8 * time.Hour)
	events := []model.Event{
		createdEvent(),
		{Type: model.EventStatusChanged, Timestamp: baseTime.Add(time.Hour), Status: model.StatusInProgress},
		{Type: model.EventTitleChanged, Timestamp: baseTime.Add(2 * time.Hour), Title: "Updated title"},
		{Type: model.EventPriorityChanged, Timestamp: baseTime.Add(3 * time.Hour), Priority: model.PriorityMedium},
		{Type: model.EventDescriptionUpdated, Timestamp: baseTime.Add(4 * time.Hour), Description: "Updated description.\n"},
		{Type: model.EventAcceptanceCriteriaUpdated, Timestamp: baseTime.Add(5 * time.Hour), AcceptanceCriteria: "Updated criteria.\n"},
		{Type: model.EventDependenciesChanged, Timestamp: baseTime.Add(6 * time.Hour), Depends: []int{10}},
		{Type: model.EventComment, Timestamp: baseTime.Add(7 * time.Hour), Author: "agent", Body: "noted\n"},
		{Type: model.EventClaimed, Timestamp: claimTime, AgentID: "agent-x", SessionID: "sess-y"},
		{Type: model.EventParseError, Timestamp: baseTime.Add(9 * time.Hour), Diagnostic: "test error"},
		{Type: model.EventReleased, Timestamp: baseTime.Add(10 * time.Hour), ReleasedBy: "human", PreviousClaimant: "agent-x"},
	}

	issue, err := store.Materialise(events)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if issue.Status != model.StatusInProgress {
		t.Errorf("Status: got %q, want %q", issue.Status, model.StatusInProgress)
	}
	if issue.Title != "Updated title" {
		t.Errorf("Title: got %q", issue.Title)
	}
	if issue.Priority != model.PriorityMedium {
		t.Errorf("Priority: got %q", issue.Priority)
	}
	if issue.Description != "Updated description.\n" {
		t.Errorf("Description: got %q", issue.Description)
	}
	if issue.AcceptanceCriteria != "Updated criteria.\n" {
		t.Errorf("AcceptanceCriteria: got %q", issue.AcceptanceCriteria)
	}
	if len(issue.Depends) != 1 || issue.Depends[0] != 10 {
		t.Errorf("Depends: got %v, want [10]", issue.Depends)
	}
	if issue.Claim != nil {
		t.Errorf("Claim: expected nil after release, got %+v", issue.Claim)
	}
}

func TestMaterialise_EmptyEvents(t *testing.T) {
	_, err := store.Materialise(nil)
	if err == nil {
		t.Fatal("expected error for nil events, got nil")
	}

	_, err = store.Materialise([]model.Event{})
	if err == nil {
		t.Fatal("expected error for empty events, got nil")
	}
}

func TestMaterialise_FirstEventNotCreated(t *testing.T) {
	events := []model.Event{
		{Type: model.EventStatusChanged, Timestamp: baseTime, Status: model.StatusOpen},
	}
	_, err := store.Materialise(events)
	if err == nil {
		t.Fatal("expected error when first event is not created, got nil")
	}
}
