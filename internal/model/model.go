// Package model defines the core types for the tracker issue system.
package model

import "time"

// Status represents the current state of an issue.
type Status string

const (
	StatusDraft      Status = "draft"
	StatusOpen       Status = "open"
	StatusInProgress Status = "in_progress"
	StatusReview     Status = "review"
	StatusDone       Status = "done"
	StatusCancelled  Status = "cancelled"
	StatusBlocked    Status = "blocked"
)

// Priority represents the importance of an issue.
type Priority string

const (
	PriorityHigh   Priority = "high"
	PriorityMedium Priority = "medium"
	PriorityLow    Priority = "low"
)

// IssueType distinguishes new work from bug fixes.
type IssueType string

const (
	TypeTask  IssueType = "task"
	TypeIssue IssueType = "issue"
)

// EventType names the kind of event recorded in a content file.
type EventType string

const (
	EventCreated                   EventType = "created"
	EventStatusChanged             EventType = "status_changed"
	EventTitleChanged              EventType = "title_changed"
	EventPriorityChanged           EventType = "priority_changed"
	EventDescriptionUpdated        EventType = "description_updated"
	EventAcceptanceCriteriaUpdated EventType = "acceptance_criteria_updated"
	EventDependenciesChanged       EventType = "dependencies_changed"
	EventComment                   EventType = "comment"
	EventClaimed                   EventType = "claimed"
	EventReleased                  EventType = "released"
	EventParseError                EventType = "parse_error"
)

// Claim records which agent currently holds an issue.
type Claim struct {
	AgentID   string    `yaml:"agent_id"`
	SessionID string    `yaml:"session_id"`
	ClaimedAt time.Time `yaml:"claimed_at"`
}

// Issue is the materialised state of an issue, derived by replaying its events.
type Issue struct {
	ID                 int
	Type               IssueType
	Title              string
	Description        string
	Status             Status
	Priority           Priority
	Depends            []int
	AcceptanceCriteria string
	Claim              *Claim
}

// Event represents a single YAML document in a content file. Fields are
// populated depending on the event type; unused fields remain zero values.
type Event struct {
	// Common fields present on every event.
	Type      EventType `yaml:"event"`
	Timestamp time.Time `yaml:"timestamp"`

	// Fields for EventCreated.
	ID                 int       `yaml:"id,omitempty"`
	IssueType          IssueType `yaml:"type,omitempty"`
	Status             Status    `yaml:"status,omitempty"`
	Priority           Priority  `yaml:"priority,omitempty"`
	Depends            []int     `yaml:"depends,omitempty"`
	AcceptanceCriteria string    `yaml:"acceptance_criteria,omitempty"`
	Description        string    `yaml:"description,omitempty"`

	// Fields for EventTitleChanged (and EventCreated).
	Title string `yaml:"title,omitempty"`

	// Fields for EventClaimed.
	AgentID   string `yaml:"agent_id,omitempty"`
	SessionID string `yaml:"session_id,omitempty"`

	// Fields for EventReleased.
	ReleasedBy        string `yaml:"released_by,omitempty"`
	PreviousClaimant  string `yaml:"previous_claimant,omitempty"`
	Reason            string `yaml:"reason,omitempty"`

	// Fields for EventComment.
	Author string `yaml:"author,omitempty"`
	Body   string `yaml:"body,omitempty"`

	// Fields for EventParseError.
	OriginalBytes string `yaml:"original_bytes,omitempty"`
	Diagnostic    string `yaml:"diagnostic,omitempty"`
}
