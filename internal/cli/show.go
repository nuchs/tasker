package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/nuchs/tasker/internal/model"
	"github.com/nuchs/tasker/internal/store"
)

// RunShow implements the `tracker show` subcommand.
func RunShow(wd string, args []string, out io.Writer) error {
	fs := flag.NewFlagSet("show", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	showEvents := fs.Bool("events", false, "show full event history")
	jsonOut := fs.Bool("json", false, "output as JSON")

	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() == 0 {
		return fmt.Errorf("show: missing issue ID")
	}

	id, err := ParseID(fs.Arg(0))
	if err != nil {
		return fmt.Errorf("show: %w", err)
	}

	s, err := OpenStore(wd)
	if err != nil {
		return err
	}
	defer s.Close()

	if *showEvents {
		return runShowEvents(s, id, *jsonOut, out)
	}
	return runShowIssue(s, id, *jsonOut, out)
}

// --- issue display ---

type issueJSON struct {
	ID                 int        `json:"id"`
	DisplayID          string     `json:"display_id"`
	Type               string     `json:"type"`
	Title              string     `json:"title"`
	Status             string     `json:"status"`
	Priority           string     `json:"priority"`
	Description        string     `json:"description,omitempty"`
	AcceptanceCriteria string     `json:"acceptance_criteria,omitempty"`
	Depends            []string   `json:"depends,omitempty"`
	Claim              *claimJSON `json:"claim,omitempty"`
}

type claimJSON struct {
	AgentID   string `json:"agent_id"`
	SessionID string `json:"session_id"`
	ClaimedAt string `json:"claimed_at"`
	Stale     bool   `json:"stale"`
}

func runShowIssue(s *store.Store, id int, asJSON bool, out io.Writer) error {
	issue, stale, err := s.Show(id)
	if err != nil {
		return fmt.Errorf("show: %w", err)
	}

	if asJSON {
		return printIssueJSON(out, s, issue, stale)
	}
	printIssueText(out, s, issue, stale)
	return nil
}

func printIssueJSON(out io.Writer, s *store.Store, issue model.Issue, stale bool) error {
	ji := issueJSON{
		ID:                 issue.ID,
		DisplayID:          s.FormatID(issue.ID),
		Type:               string(issue.Type),
		Title:              issue.Title,
		Status:             string(issue.Status),
		Priority:           string(issue.Priority),
		Description:        issue.Description,
		AcceptanceCriteria: issue.AcceptanceCriteria,
	}
	for _, dep := range issue.Depends {
		ji.Depends = append(ji.Depends, s.FormatID(dep))
	}
	if issue.Claim != nil {
		ji.Claim = &claimJSON{
			AgentID:   issue.Claim.AgentID,
			SessionID: issue.Claim.SessionID,
			ClaimedAt: issue.Claim.ClaimedAt.UTC().Format(time.RFC3339),
			Stale:     stale,
		}
	}
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(ji)
}

func printIssueText(out io.Writer, s *store.Store, issue model.Issue, stale bool) {
	fmt.Fprintf(out, "%s  %s\n", s.FormatID(issue.ID), issue.Title)
	fmt.Fprintf(out, "Status:    %s\n", issue.Status)
	fmt.Fprintf(out, "Priority:  %s\n", issue.Priority)
	fmt.Fprintf(out, "Type:      %s\n", issue.Type)

	if len(issue.Depends) > 0 {
		deps := make([]string, len(issue.Depends))
		for i, d := range issue.Depends {
			deps[i] = s.FormatID(d)
		}
		fmt.Fprintf(out, "Depends:   %s\n", strings.Join(deps, ", "))
	}

	if issue.Description != "" {
		fmt.Fprintf(out, "\nDescription:\n  %s\n", issue.Description)
	}
	if issue.AcceptanceCriteria != "" {
		fmt.Fprintf(out, "\nAcceptance Criteria:\n  %s\n", issue.AcceptanceCriteria)
	}

	if issue.Claim != nil {
		claimedAt := issue.Claim.ClaimedAt.UTC().Format("2006-01-02 15:04 UTC")
		staleTag := ""
		if stale {
			staleTag = " [STALE]"
		}
		fmt.Fprintf(out, "\nClaim: %s (session: %s, since: %s)%s\n",
			issue.Claim.AgentID, issue.Claim.SessionID, claimedAt, staleTag)
	}
}

// --- events display ---

func runShowEvents(s *store.Store, id int, asJSON bool, out io.Writer) error {
	events, err := s.Events(id)
	if err != nil {
		return fmt.Errorf("show --events: %w", err)
	}

	if asJSON {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(events)
	}
	printEventsText(out, events)
	return nil
}

func printEventsText(out io.Writer, events []model.Event) {
	for i, ev := range events {
		ts := ev.Timestamp.UTC().Format("2006-01-02T15:04:05Z")
		detail := eventDetail(ev)
		if detail != "" {
			fmt.Fprintf(out, "  %3d  %s  %-30s  %s\n", i+1, ts, ev.Type, detail)
		} else {
			fmt.Fprintf(out, "  %3d  %s  %s\n", i+1, ts, ev.Type)
		}
	}
}

func eventDetail(ev model.Event) string {
	switch ev.Type {
	case model.EventCreated:
		return ev.Title
	case model.EventStatusChanged:
		return string(ev.Status)
	case model.EventTitleChanged:
		return ev.Title
	case model.EventPriorityChanged:
		return string(ev.Priority)
	case model.EventClaimed:
		return fmt.Sprintf("agent=%s session=%s", ev.AgentID, ev.SessionID)
	case model.EventReleased:
		return fmt.Sprintf("by=%s prev=%s", ev.ReleasedBy, ev.PreviousClaimant)
	case model.EventComment:
		return fmt.Sprintf("author=%s", ev.Author)
	default:
		return ""
	}
}
