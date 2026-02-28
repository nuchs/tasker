package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/nuchs/tasker/internal/index"
	"github.com/nuchs/tasker/internal/model"
	"github.com/nuchs/tasker/internal/store"
)

// RunList implements the `tracker list` subcommand.
func RunList(wd string, args []string, out io.Writer) error {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	statusFlag := fs.String("status", "", "filter by status")
	priorityFlag := fs.String("priority", "", "filter by priority: high, medium, low")
	typeFlag := fs.String("type", "", "filter by type: task, issue")
	jsonOut := fs.Bool("json", false, "output as JSON")

	if err := fs.Parse(args); err != nil {
		return err
	}

	// Validate flag values when provided. When no status filter is given,
	// exclude terminal statuses (done, cancelled) in the query itself.
	f := index.Filters{ExcludeTerminal: *statusFlag == ""}
	if *statusFlag != "" {
		s := model.Status(*statusFlag)
		switch s {
		case model.StatusDraft, model.StatusOpen, model.StatusInProgress,
			model.StatusReview, model.StatusDone, model.StatusCancelled, model.StatusBlocked:
		default:
			return fmt.Errorf("list: invalid status %q", *statusFlag)
		}
		f.Status = s
	}
	if *priorityFlag != "" {
		p := model.Priority(*priorityFlag)
		switch p {
		case model.PriorityHigh, model.PriorityMedium, model.PriorityLow:
		default:
			return fmt.Errorf("list: invalid priority %q", *priorityFlag)
		}
		f.Priority = p
	}
	if *typeFlag != "" {
		it := model.IssueType(*typeFlag)
		switch it {
		case model.TypeTask, model.TypeIssue:
		default:
			return fmt.Errorf("list: invalid type %q", *typeFlag)
		}
		f.Type = it
	}

	s, err := OpenStore(wd)
	if err != nil {
		return err
	}
	defer s.Close()

	issues, err := s.ListIssues(f)
	if err != nil {
		return fmt.Errorf("list: %w", err)
	}

	if *jsonOut {
		return printListJSON(out, s, issues)
	}
	printListText(out, s, issues)
	return nil
}

// issueListEntry is the JSON representation of a single row in list output.
type issueListEntry struct {
	ID        int    `json:"id"`
	DisplayID string `json:"display_id"`
	Type      string `json:"type"`
	Title     string `json:"title"`
	Status    string `json:"status"`
	Priority  string `json:"priority"`
}

func printListJSON(out io.Writer, s *store.Store, issues []index.IssueMeta) error {
	entries := make([]issueListEntry, len(issues))
	for i, m := range issues {
		entries[i] = issueListEntry{
			ID:        m.ID,
			DisplayID: s.FormatID(m.ID),
			Type:      string(m.Type),
			Title:     m.Title,
			Status:    string(m.Status),
			Priority:  string(m.Priority),
		}
	}
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(entries)
}

func printListText(out io.Writer, s *store.Store, issues []index.IssueMeta) {
	if len(issues) == 0 {
		fmt.Fprintln(out, "no issues found")
		return
	}
	for _, m := range issues {
		fmt.Fprintf(out, "%-12s  %-10s  %-8s  %-8s  %s\n",
			s.FormatID(m.ID),
			string(m.Status),
			string(m.Priority),
			string(m.Type),
			m.Title,
		)
	}
}
