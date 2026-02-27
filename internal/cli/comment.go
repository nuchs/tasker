package cli

import (
	"fmt"
	"io"

	"github.com/nuchs/tasker/internal/model"
)

// RunComment implements the `tracker comment <id> "message"` subcommand.
func RunComment(wd string, args []string, out io.Writer) error {
	if len(args) < 2 {
		return fmt.Errorf("comment: usage: tracker comment <id> \"message\"")
	}

	id, err := ParseID(args[0])
	if err != nil {
		return fmt.Errorf("comment: %w", err)
	}

	message := args[1]
	if message == "" {
		return fmt.Errorf("comment: message must not be empty")
	}

	s, err := OpenStore(wd)
	if err != nil {
		return err
	}
	defer s.Close()

	if err := s.Append(id, model.Event{
		Type: model.EventComment,
		Body: message,
	}); err != nil {
		return fmt.Errorf("comment: %w", err)
	}

	fmt.Fprintln(out, s.FormatID(id))
	return nil
}
