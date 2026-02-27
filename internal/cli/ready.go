package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/nuchs/tasker/internal/index"
	"github.com/nuchs/tasker/internal/store"
)

// RunReady implements the `tracker ready` subcommand.
func RunReady(wd string, args []string, out io.Writer) error {
	fs := flag.NewFlagSet("ready", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	jsonOut := fs.Bool("json", false, "output as JSON")

	if err := fs.Parse(args); err != nil {
		return err
	}

	s, err := OpenStore(wd)
	if err != nil {
		return err
	}
	defer s.Close()

	issues, err := s.ReadyIssues()
	if err != nil {
		return fmt.Errorf("ready: %w", err)
	}

	if *jsonOut {
		return printReadyJSON(out, s, issues)
	}
	printReadyText(out, s, issues)
	return nil
}

func printReadyJSON(out io.Writer, s *store.Store, issues []index.IssueMeta) error {
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

func printReadyText(out io.Writer, s *store.Store, issues []index.IssueMeta) {
	if len(issues) == 0 {
		fmt.Fprintln(out, "no ready issues")
		return
	}
	for _, m := range issues {
		fmt.Fprintf(out, "%-12s  %-8s  %-8s  %s\n",
			s.FormatID(m.ID),
			string(m.Priority),
			string(m.Type),
			m.Title,
		)
	}
}
