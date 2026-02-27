package model

import "testing"

func TestStatusConstants(t *testing.T) {
	cases := []struct {
		got  Status
		want string
	}{
		{StatusDraft, "draft"},
		{StatusOpen, "open"},
		{StatusInProgress, "in_progress"},
		{StatusReview, "review"},
		{StatusDone, "done"},
		{StatusCancelled, "cancelled"},
		{StatusBlocked, "blocked"},
	}
	for _, c := range cases {
		if string(c.got) != c.want {
			t.Errorf("Status constant = %q, want %q", c.got, c.want)
		}
	}
}

func TestPriorityConstants(t *testing.T) {
	cases := []struct {
		got  Priority
		want string
	}{
		{PriorityHigh, "high"},
		{PriorityMedium, "medium"},
		{PriorityLow, "low"},
	}
	for _, c := range cases {
		if string(c.got) != c.want {
			t.Errorf("Priority constant = %q, want %q", c.got, c.want)
		}
	}
}

func TestIssueTypeConstants(t *testing.T) {
	cases := []struct {
		got  IssueType
		want string
	}{
		{TypeTask, "task"},
		{TypeIssue, "issue"},
	}
	for _, c := range cases {
		if string(c.got) != c.want {
			t.Errorf("IssueType constant = %q, want %q", c.got, c.want)
		}
	}
}

func TestEventTypeConstants(t *testing.T) {
	cases := []struct {
		got  EventType
		want string
	}{
		{EventCreated, "created"},
		{EventStatusChanged, "status_changed"},
		{EventTitleChanged, "title_changed"},
		{EventPriorityChanged, "priority_changed"},
		{EventDescriptionUpdated, "description_updated"},
		{EventAcceptanceCriteriaUpdated, "acceptance_criteria_updated"},
		{EventDependenciesChanged, "dependencies_changed"},
		{EventComment, "comment"},
		{EventClaimed, "claimed"},
		{EventReleased, "released"},
		{EventParseError, "parse_error"},
	}
	for _, c := range cases {
		if string(c.got) != c.want {
			t.Errorf("EventType constant = %q, want %q", c.got, c.want)
		}
	}
}

func TestIssueZeroValue(t *testing.T) {
	var i Issue
	if i.Claim != nil {
		t.Error("Issue zero value should have nil Claim")
	}
	if len(i.Depends) != 0 {
		t.Error("Issue zero value should have empty Depends")
	}
}

func TestClaimFields(t *testing.T) {
	c := Claim{AgentID: "agent-1", SessionID: "sess-1"}
	if c.AgentID != "agent-1" {
		t.Errorf("AgentID = %q, want %q", c.AgentID, "agent-1")
	}
	if c.SessionID != "sess-1" {
		t.Errorf("SessionID = %q, want %q", c.SessionID, "sess-1")
	}
}

func TestEventFields(t *testing.T) {
	e := Event{
		Type:   EventCreated,
		Status: StatusOpen,
	}
	if e.Type != EventCreated {
		t.Errorf("Event.Type = %q, want %q", e.Type, EventCreated)
	}
	if e.Status != StatusOpen {
		t.Errorf("Event.Status = %q, want %q", e.Status, StatusOpen)
	}
}
