package cli

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/nuchs/tasker/internal/model"
)

// RunClaim implements the `tracker claim <id> --agent <agent-id> --session <session-id>` subcommand.
// It fails with a clear error if the issue is already claimed.
func RunClaim(wd string, args []string, out io.Writer) error {
	fs := flag.NewFlagSet("claim", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	agentFlag := fs.String("agent", "", "agent ID claiming the issue")
	sessionFlag := fs.String("session", "", "session ID for this claim")

	if len(args) == 0 {
		return fmt.Errorf("claim: missing issue ID")
	}
	idStr := args[0]
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}

	if *agentFlag == "" {
		return fmt.Errorf("claim: --agent is required")
	}
	if *sessionFlag == "" {
		return fmt.Errorf("claim: --session is required")
	}

	id, err := ParseID(idStr)
	if err != nil {
		return fmt.Errorf("claim: %w", err)
	}

	s, err := OpenStore(wd)
	if err != nil {
		return err
	}
	defer s.Close()

	issue, _, err := s.Show(id)
	if err != nil {
		return fmt.Errorf("claim: %w", err)
	}
	if issue.Claim != nil {
		return fmt.Errorf("claim: issue %s is already claimed by agent %q", s.FormatID(id), issue.Claim.AgentID)
	}

	if err := s.Append(id, model.Event{
		Type:      model.EventClaimed,
		AgentID:   *agentFlag,
		SessionID: *sessionFlag,
	}); err != nil {
		return fmt.Errorf("claim: %w", err)
	}

	fmt.Fprintln(out, s.FormatID(id))
	return nil
}
