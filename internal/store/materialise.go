package store

import (
	"fmt"

	"github.com/nuchs/tasker/internal/model"
)

// Materialise replays events in order to produce the current state of an
// issue. The slice must be non-empty and begin with a created event.
// Last-write-wins applies to all mutable fields.
func Materialise(events []model.Event) (model.Issue, error) {
	if len(events) == 0 {
		return model.Issue{}, fmt.Errorf("store: materialise: no events")
	}

	first := events[0]
	if first.Type != model.EventCreated {
		return model.Issue{}, fmt.Errorf("store: materialise: first event must be %q, got %q",
			model.EventCreated, first.Type)
	}

	issue := model.Issue{
		ID:                 first.ID,
		Type:               first.IssueType,
		Title:              first.Title,
		Description:        first.Description,
		Status:             first.Status,
		Priority:           first.Priority,
		Depends:            first.Depends,
		AcceptanceCriteria: first.AcceptanceCriteria,
	}

	for _, ev := range events[1:] {
		switch ev.Type {
		case model.EventStatusChanged:
			issue.Status = ev.Status
		case model.EventTitleChanged:
			issue.Title = ev.Title
		case model.EventPriorityChanged:
			issue.Priority = ev.Priority
		case model.EventDescriptionUpdated:
			issue.Description = ev.Description
		case model.EventAcceptanceCriteriaUpdated:
			issue.AcceptanceCriteria = ev.AcceptanceCriteria
		case model.EventDependenciesChanged:
			issue.Depends = ev.Depends
		case model.EventClaimed:
			issue.Claim = &model.Claim{
				AgentID:   ev.AgentID,
				SessionID: ev.SessionID,
				ClaimedAt: ev.Timestamp,
			}
		case model.EventReleased:
			issue.Claim = nil
		case model.EventComment, model.EventParseError:
			// no effect on materialised state
		}
	}

	return issue, nil
}
