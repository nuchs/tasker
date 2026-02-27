package cli

import (
	"fmt"
	"io"

	"github.com/nuchs/tasker/internal/index"
)

// RunRebuild implements the `tracker rebuild` subcommand.
// It drops and regenerates the SQLite index from the content files on disk.
func RunRebuild(wd string, args []string, out io.Writer) error {
	s, err := OpenStore(wd)
	if err != nil {
		return err
	}
	defer s.Close()

	if err := s.Rebuild(); err != nil {
		return fmt.Errorf("rebuild: %w", err)
	}

	issues, err := s.ListIssues(index.Filters{})
	if err != nil {
		return fmt.Errorf("rebuild: %w", err)
	}

	fmt.Fprintf(out, "rebuilt index: %d issue(s)\n", len(issues))
	return nil
}
