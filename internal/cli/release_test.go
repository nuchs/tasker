package cli_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/nuchs/tasker/internal/cli"
	"github.com/nuchs/tasker/internal/model"
)

func runRelease(t *testing.T, wd string, args ...string) string {
	t.Helper()
	var buf bytes.Buffer
	if err := cli.RunRelease(wd, args, &buf); err != nil {
		t.Fatalf("RunRelease(%v): %v", args, err)
	}
	return strings.TrimSpace(buf.String())
}

func TestRunRelease_ClearsClaim(t *testing.T) {
	wd := initTracker(t, "T")
	runCreate(t, wd, "--title", "Claim me")
	runClaim(t, wd, "1", "--agent", "agent-1", "--session", "sess-1")

	runRelease(t, wd, "1")

	s, err := cli.OpenStore(wd)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	defer s.Close()

	issue, _, err := s.Show(1)
	if err != nil {
		t.Fatalf("Show: %v", err)
	}
	if issue.Claim != nil {
		t.Errorf("expected claim to be nil after release, got %+v", issue.Claim)
	}
}

func TestRunRelease_RecordsPreviousClaimant(t *testing.T) {
	wd := initTracker(t, "T")
	runCreate(t, wd, "--title", "Claim me")
	runClaim(t, wd, "1", "--agent", "agent-1", "--session", "sess-1")

	runRelease(t, wd, "1")

	s, err := cli.OpenStore(wd)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	defer s.Close()

	events, err := s.Events(1)
	if err != nil {
		t.Fatalf("Events: %v", err)
	}

	found := false
	for _, ev := range events {
		if ev.Type == model.EventReleased && ev.PreviousClaimant == "agent-1" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("released event with previous claimant not found; events: %v", events)
	}
}

func TestRunRelease_NoOpWhenUnclaimed(t *testing.T) {
	wd := initTracker(t, "T")
	runCreate(t, wd, "--title", "Unclaimed issue")

	// Should not error even though the issue has no claim.
	runRelease(t, wd, "1")

	s, err := cli.OpenStore(wd)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	defer s.Close()

	issue, _, err := s.Show(1)
	if err != nil {
		t.Fatalf("Show: %v", err)
	}
	if issue.Claim != nil {
		t.Errorf("expected claim to remain nil, got %+v", issue.Claim)
	}
}

func TestRunRelease_PrintsFormattedID(t *testing.T) {
	wd := initTracker(t, "PROJ")
	runCreate(t, wd, "--title", "Issue")
	runClaim(t, wd, "1", "--agent", "a", "--session", "s")

	got := runRelease(t, wd, "1")
	if got != "PROJ-0001" {
		t.Errorf("expected PROJ-0001, got %q", got)
	}
}

func TestRunRelease_AcceptsPrefixedID(t *testing.T) {
	wd := initTracker(t, "PROJ")
	runCreate(t, wd, "--title", "Issue")
	runClaim(t, wd, "1", "--agent", "a", "--session", "s")

	got := runRelease(t, wd, "PROJ-0001")
	if got != "PROJ-0001" {
		t.Errorf("expected PROJ-0001, got %q", got)
	}
}

func TestRunRelease_MissingID(t *testing.T) {
	wd := initTracker(t, "T")
	var buf bytes.Buffer
	if err := cli.RunRelease(wd, []string{}, &buf); err == nil {
		t.Fatal("expected error when ID is missing")
	}
}

func TestRunRelease_InvalidID(t *testing.T) {
	wd := initTracker(t, "T")
	var buf bytes.Buffer
	if err := cli.RunRelease(wd, []string{"not-an-id"}, &buf); err == nil {
		t.Fatal("expected error for invalid ID")
	}
}

func TestRunRelease_AllowsAnyUserToRelease(t *testing.T) {
	// Release doesn't require the same agent that claimed — any caller can release.
	wd := initTracker(t, "T")
	runCreate(t, wd, "--title", "Issue")
	runClaim(t, wd, "1", "--agent", "original-agent", "--session", "sess-1")

	// Release without any --agent flag (different agent releasing).
	runRelease(t, wd, "1")

	s, err := cli.OpenStore(wd)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	defer s.Close()

	issue, _, err := s.Show(1)
	if err != nil {
		t.Fatalf("Show: %v", err)
	}
	if issue.Claim != nil {
		t.Errorf("expected claim nil after release, got %+v", issue.Claim)
	}
}
