package cli

import (
	"fmt"
	"strconv"
	"strings"
)

// ParseID extracts the numeric issue ID from s. Accepted formats:
//
//	"42"       → 42
//	"PROJ-42"  → 42  (any prefix, split on the last '-')
//	"PROJ-042" → 42
func ParseID(s string) (int, error) {
	// Plain integer.
	if n, err := strconv.Atoi(s); err == nil {
		if n <= 0 {
			return 0, fmt.Errorf("invalid issue id %q: must be positive", s)
		}
		return n, nil
	}
	// PREFIX-NNN form: take everything after the last '-'.
	idx := strings.LastIndex(s, "-")
	if idx < 0 {
		return 0, fmt.Errorf("invalid issue id %q", s)
	}
	n, err := strconv.Atoi(s[idx+1:])
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("invalid issue id %q", s)
	}
	return n, nil
}
