package store

import (
	"errors"
	"fmt"
	"io"
	"os"

	"gopkg.in/yaml.v3"

	"github.com/nuchs/tasker/internal/model"
)

// ParseError describes a failure to parse a specific event document in a
// content file. Index is the 0-based position of the corrupted document.
type ParseError struct {
	Path  string
	Index int
	Cause error
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("%s: event %d: %v", e.Path, e.Index, e.Cause)
}

func (e *ParseError) Unwrap() error { return e.Cause }

// ParseFile reads the YAML event file at path and returns the parsed events.
// It returns a *ParseError if any document in the file is malformed.
// An empty file returns nil, nil.
func ParseFile(path string) ([]model.Event, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("store: open %s: %w", path, err)
	}
	defer f.Close()
	return parseReader(path, f)
}

// parseReader parses events from a multi-document YAML stream in r.
// path is used only in error messages.
func parseReader(path string, r io.Reader) ([]model.Event, error) {
	dec := yaml.NewDecoder(r)

	var events []model.Event
	for i := 0; ; i++ {
		var ev model.Event
		err := dec.Decode(&ev)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, &ParseError{Path: path, Index: i, Cause: err}
		}
		events = append(events, ev)
	}
	return events, nil
}
