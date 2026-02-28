package store

import (
	"fmt"
	"io"
	"os"
	"syscall"

	"gopkg.in/yaml.v3"

	"github.com/nuchs/tasker/internal/model"
)

// AppendEvent serialises ev as a YAML document and appends it to the file at
// path, preceded by a "---" separator. The file is created if it does not
// exist. An exclusive flock is held for the duration of the write to prevent
// concurrent appends from interleaving.
//
// Known limitation: if the process is killed or the disk fills up between
// writing "---\n" and the YAML body, the file will contain a truncated
// document. The parser will return a ParseError on subsequent reads, halting
// all operations on that issue. Manual repair: replace the truncated bytes
// with a well-formed parse_error event, e.g.:
//
//	---
//	event: parse_error
//	timestamp: <now>
//	diagnostic: "truncated write — manually recovered"
func AppendEvent(path string, ev model.Event) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("store: open %s: %w", path, err)
	}
	defer f.Close()

	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("store: flock %s: %w", path, err)
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN) //nolint:errcheck

	data, err := yaml.Marshal(ev)
	if err != nil {
		return fmt.Errorf("store: marshal event: %w", err)
	}

	if _, err := fmt.Fprintf(f, "---\n%s", data); err != nil {
		return fmt.Errorf("store: write %s: %w", path, err)
	}
	return nil
}

// ReadThenAppend opens path with an exclusive flock, parses all existing
// events, calls check with those events, and (only if check returns nil)
// appends ev. This provides an atomic check-then-write for conditional
// appends such as claiming an issue: the check and write cannot be interleaved
// by a concurrent caller because both happen under the same exclusive flock.
func ReadThenAppend(path string, ev model.Event, check func([]model.Event) error) error {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("store: open %s: %w", path, err)
	}
	defer f.Close()

	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("store: flock %s: %w", path, err)
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN) //nolint:errcheck

	events, err := parseReader(path, f)
	if err != nil {
		return fmt.Errorf("store: read %s: %w", path, err)
	}

	if err := check(events); err != nil {
		return err
	}

	data, err := yaml.Marshal(ev)
	if err != nil {
		return fmt.Errorf("store: marshal event: %w", err)
	}

	if _, err := f.Seek(0, io.SeekEnd); err != nil {
		return fmt.Errorf("store: seek %s: %w", path, err)
	}

	if _, err := fmt.Fprintf(f, "---\n%s", data); err != nil {
		return fmt.Errorf("store: write %s: %w", path, err)
	}
	return nil
}
