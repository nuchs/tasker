package cli

import (
	"fmt"
	"io"

	"github.com/nuchs/tasker/internal/model"
)

// RunRelease implements the `tracker release <id>` subcommand.
// It appends a released event recording the previous claimant.
// Any user can release any claim; it is a no-op if the issue is not claimed.
func RunRelease(wd string, args []string, out io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("release: missing issue ID")
	}

	id, err := ParseID(args[0])
	if err != nil {
		return fmt.Errorf("release: %w", err)
	}

	s, err := OpenStore(wd)
	if err != nil {
		return err
	}
	defer s.Close()

	issue, _, err := s.Show(id)
	if err != nil {
		return fmt.Errorf("release: %w", err)
	}

	var previousClaimant string
	if issue.Claim != nil {
		previousClaimant = issue.Claim.AgentID
	}

	if err := s.Append(id, model.Event{
		Type:             model.EventReleased,
		PreviousClaimant: previousClaimant,
	}); err != nil {
		return fmt.Errorf("release: %w", err)
	}

	fmt.Fprintln(out, s.FormatID(id))
	return nil
}
