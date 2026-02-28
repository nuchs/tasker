package cli_test

import (
	"bytes"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/nuchs/tasker/internal/cli"
	"github.com/nuchs/tasker/internal/model"
)

func runClaim(t *testing.T, wd string, args ...string) string {
	t.Helper()
	var buf bytes.Buffer
	if err := cli.RunClaim(wd, args, &buf); err != nil {
		t.Fatalf("RunClaim(%v): %v", args, err)
	}
	return strings.TrimSpace(buf.String())
}

func TestRunClaim_SetsClaimInIndex(t *testing.T) {
	wd := initTracker(t, "T")
	runCreate(t, wd, "--title", "Claim me")

	runClaim(t, wd, "1", "--agent", "agent-1", "--session", "sess-abc")

	s, err := cli.OpenStore(wd)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	defer s.Close()

	issue, _, err := s.Show(1)
	if err != nil {
		t.Fatalf("Show: %v", err)
	}
	if issue.Claim == nil {
		t.Fatal("expected claim to be set, got nil")
	}
	if issue.Claim.AgentID != "agent-1" {
		t.Errorf("AgentID: got %q, want %q", issue.Claim.AgentID, "agent-1")
	}
	if issue.Claim.SessionID != "sess-abc" {
		t.Errorf("SessionID: got %q, want %q", issue.Claim.SessionID, "sess-abc")
	}
}

func TestRunClaim_AppearsInEventLog(t *testing.T) {
	wd := initTracker(t, "T")
	runCreate(t, wd, "--title", "Claim me")

	runClaim(t, wd, "1", "--agent", "agent-1", "--session", "sess-abc")

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
		if ev.Type == model.EventClaimed && ev.AgentID == "agent-1" && ev.SessionID == "sess-abc" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("claimed event not found in event log; events: %v", events)
	}
}

func TestRunClaim_FailsIfAlreadyClaimed(t *testing.T) {
	wd := initTracker(t, "T")
	runCreate(t, wd, "--title", "Claim me")
	runClaim(t, wd, "1", "--agent", "agent-1", "--session", "sess-1")

	var buf bytes.Buffer
	err := cli.RunClaim(wd, []string{"1", "--agent", "agent-2", "--session", "sess-2"}, &buf)
	if err == nil {
		t.Fatal("expected error when claiming an already-claimed issue")
	}
	if !strings.Contains(err.Error(), "already claimed") {
		t.Errorf("expected 'already claimed' in error, got: %v", err)
	}
}

func TestRunClaim_PrintsFormattedID(t *testing.T) {
	wd := initTracker(t, "PROJ")
	runCreate(t, wd, "--title", "Issue")

	got := runClaim(t, wd, "1", "--agent", "agent-x", "--session", "sess-x")
	if got != "PROJ-0001" {
		t.Errorf("expected PROJ-0001, got %q", got)
	}
}

func TestRunClaim_AcceptsPrefixedID(t *testing.T) {
	wd := initTracker(t, "PROJ")
	runCreate(t, wd, "--title", "Issue")

	got := runClaim(t, wd, "PROJ-0001", "--agent", "agent-x", "--session", "sess-x")
	if got != "PROJ-0001" {
		t.Errorf("expected PROJ-0001, got %q", got)
	}
}

func TestRunClaim_MissingID(t *testing.T) {
	wd := initTracker(t, "T")
	var buf bytes.Buffer
	if err := cli.RunClaim(wd, []string{}, &buf); err == nil {
		t.Fatal("expected error when ID is missing")
	}
}

func TestRunClaim_MissingAgent(t *testing.T) {
	wd := initTracker(t, "T")
	runCreate(t, wd, "--title", "Issue")
	var buf bytes.Buffer
	if err := cli.RunClaim(wd, []string{"1", "--session", "sess-1"}, &buf); err == nil {
		t.Fatal("expected error when --agent is missing")
	}
}

func TestRunClaim_MissingSession(t *testing.T) {
	wd := initTracker(t, "T")
	runCreate(t, wd, "--title", "Issue")
	var buf bytes.Buffer
	if err := cli.RunClaim(wd, []string{"1", "--agent", "agent-1"}, &buf); err == nil {
		t.Fatal("expected error when --session is missing")
	}
}

func TestRunClaim_InvalidID(t *testing.T) {
	wd := initTracker(t, "T")
	var buf bytes.Buffer
	if err := cli.RunClaim(wd, []string{"not-an-id", "--agent", "a", "--session", "s"}, &buf); err == nil {
		t.Fatal("expected error for invalid ID")
	}
}

func TestRunClaim_ConcurrentClaim(t *testing.T) {
	wd := initTracker(t, "T")
	runCreate(t, wd, "--title", "Contested issue")

	const n = 20
	var wg sync.WaitGroup
	var successes atomic.Int32

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			var buf bytes.Buffer
			err := cli.RunClaim(wd, []string{
				"1",
				"--agent", fmt.Sprintf("agent-%d", i),
				"--session", fmt.Sprintf("sess-%d", i),
			}, &buf)
			if err == nil {
				successes.Add(1)
			}
		}(i)
	}
	wg.Wait()

	if successes.Load() != 1 {
		t.Errorf("expected exactly 1 successful claim, got %d", successes.Load())
	}

	// Content file must have exactly one claimed event.
	s, err := cli.OpenStore(wd)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	events, err := s.Events(1)
	s.Close()
	if err != nil {
		t.Fatalf("Events: %v", err)
	}
	var claimCount int
	for _, ev := range events {
		if ev.Type == model.EventClaimed {
			claimCount++
		}
	}
	if claimCount != 1 {
		t.Errorf("expected 1 claimed event in file, got %d", claimCount)
	}
}
