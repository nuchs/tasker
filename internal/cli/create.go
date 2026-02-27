package cli

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/nuchs/tasker/internal/model"
)

// RunCreate implements the `tracker create` subcommand.
// Output (the assigned issue ID) is written to out.
func RunCreate(wd string, args []string, out io.Writer) error {
	fs := flag.NewFlagSet("create", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	title := fs.String("title", "", "issue title (required)")
	desc := fs.String("description", "", "issue description")
	priority := fs.String("priority", "medium", "priority: high, medium, low")
	issueType := fs.String("type", "task", "type: task, issue")
	ac := fs.String("acceptance-criteria", "", "acceptance criteria")

	if err := fs.Parse(args); err != nil {
		return err
	}
	if *title == "" {
		return fmt.Errorf("create: --title is required")
	}

	p := model.Priority(*priority)
	switch p {
	case model.PriorityHigh, model.PriorityMedium, model.PriorityLow:
	default:
		return fmt.Errorf("create: invalid priority %q (must be high, medium, or low)", *priority)
	}

	it := model.IssueType(*issueType)
	switch it {
	case model.TypeTask, model.TypeIssue:
	default:
		return fmt.Errorf("create: invalid type %q (must be task or issue)", *issueType)
	}

	s, err := OpenStore(wd)
	if err != nil {
		return err
	}
	defer s.Close()

	ev := model.Event{
		IssueType:          it,
		Title:              *title,
		Description:        *desc,
		AcceptanceCriteria: *ac,
		Status:             model.StatusOpen,
		Priority:           p,
	}

	id, err := s.Create(ev)
	if err != nil {
		return fmt.Errorf("create: %w", err)
	}

	fmt.Fprintln(out, s.FormatID(id))
	return nil
}
