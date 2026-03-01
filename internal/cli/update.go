package cli

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/nuchs/tasker/internal/model"
)

// RunUpdate implements the `tracker update` subcommand.
// Each provided flag generates the corresponding event and is appended in order.
func RunUpdate(wd string, args []string, out io.Writer) error {
	fs := flag.NewFlagSet("update", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	statusFlag := fs.String("status", "", "new status")
	priorityFlag := fs.String("priority", "", "new priority: high, medium, low")
	titleFlag := fs.String("title", "", "new title")

	// Expect the ID as the first positional argument, before any flags.
	// This allows the natural form `tracker update <id> --status done`.
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return fmt.Errorf("update: missing issue ID")
	}
	idStr := args[0]
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if *statusFlag == "" && *priorityFlag == "" && *titleFlag == "" {
		return fmt.Errorf("update: at least one of --status, --priority, or --title is required")
	}

	id, err := ParseID(idStr)
	if err != nil {
		return fmt.Errorf("update: %w", err)
	}

	// Validate values before opening the store so we fail fast on bad input.
	var newStatus model.Status
	if *statusFlag != "" {
		newStatus = model.Status(*statusFlag)
		switch newStatus {
		case model.StatusDraft, model.StatusOpen, model.StatusInProgress,
			model.StatusReview, model.StatusDone, model.StatusCancelled, model.StatusBlocked:
		default:
			return fmt.Errorf("update: invalid status %q", *statusFlag)
		}
	}

	var newPriority model.Priority
	if *priorityFlag != "" {
		newPriority = model.Priority(*priorityFlag)
		switch newPriority {
		case model.PriorityHigh, model.PriorityMedium, model.PriorityLow:
		default:
			return fmt.Errorf("update: invalid priority %q", *priorityFlag)
		}
	}

	s, err := OpenStore(wd)
	if err != nil {
		return err
	}
	defer s.Close()

	// Append one event per changed field, in a defined order.
	if *statusFlag != "" {
		if err := s.Append(id, model.Event{
			Type:   model.EventStatusChanged,
			Status: newStatus,
		}); err != nil {
			return fmt.Errorf("update: %w", err)
		}
	}
	if *priorityFlag != "" {
		if err := s.Append(id, model.Event{
			Type:     model.EventPriorityChanged,
			Priority: newPriority,
		}); err != nil {
			return fmt.Errorf("update: %w", err)
		}
	}
	if *titleFlag != "" {
		if err := s.Append(id, model.Event{
			Type:  model.EventTitleChanged,
			Title: *titleFlag,
		}); err != nil {
			return fmt.Errorf("update: %w", err)
		}
	}

	fmt.Fprintln(out, s.FormatID(id))
	return nil
}
