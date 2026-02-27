package store

import (
	"fmt"
	"os"
	"syscall"

	"gopkg.in/yaml.v3"

	"github.com/nuchs/tasker/internal/model"
)

// AppendEvent serialises ev as a YAML document and appends it to the file at
// path, preceded by a "---" separator. The file is created if it does not
// exist. An exclusive flock is held for the duration of the write to prevent
// concurrent appends from interleaving.
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
