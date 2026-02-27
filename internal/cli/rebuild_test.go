package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nuchs/tasker/internal/cli"
	"github.com/nuchs/tasker/internal/index"
)

func runRebuild(t *testing.T, wd string) string {
	t.Helper()
	var buf bytes.Buffer
	if err := cli.RunRebuild(wd, []string{}, &buf); err != nil {
		t.Fatalf("RunRebuild: %v", err)
	}
	return strings.TrimSpace(buf.String())
}

func TestRunRebuild_EmptyIssuesDirectory(t *testing.T) {
	wd := initTracker(t, "T")

	got := runRebuild(t, wd)
	if got != "rebuilt index: 0 issue(s)" {
		t.Errorf("unexpected output: %q", got)
	}
}

func TestRunRebuild_PrintsSummaryCount(t *testing.T) {
	wd := initTracker(t, "T")
	runCreate(t, wd, "--title", "Issue one")
	runCreate(t, wd, "--title", "Issue two")
	runCreate(t, wd, "--title", "Issue three")

	got := runRebuild(t, wd)
	if got != "rebuilt index: 3 issue(s)" {
		t.Errorf("unexpected output: %q", got)
	}
}

func TestRunRebuild_ProducesIdenticalDBState(t *testing.T) {
	wd := initTracker(t, "T")
	runCreate(t, wd, "--title", "Alpha", "--priority", "high")
	runCreate(t, wd, "--title", "Beta", "--priority", "low", "--type", "issue")
	runCreate(t, wd, "--title", "Gamma")
	runUpdate(t, wd, "2", "--status", "done")
	runClaim(t, wd, "1", "--agent", "agent-x", "--session", "sess-1")

	// Capture state before rebuild.
	sBefore, err := cli.OpenStore(wd)
	if err != nil {
		t.Fatalf("OpenStore (before): %v", err)
	}
	issuesBefore, err := sBefore.ListIssues(index.Filters{})
	sBefore.Close()
	if err != nil {
		t.Fatalf("ListIssues (before): %v", err)
	}

	runRebuild(t, wd)

	// Capture state after rebuild.
	sAfter, err := cli.OpenStore(wd)
	if err != nil {
		t.Fatalf("OpenStore (after): %v", err)
	}
	issuesAfter, err := sAfter.ListIssues(index.Filters{})
	sAfter.Close()
	if err != nil {
		t.Fatalf("ListIssues (after): %v", err)
	}

	if len(issuesBefore) != len(issuesAfter) {
		t.Fatalf("issue count mismatch: before %d, after %d", len(issuesBefore), len(issuesAfter))
	}
	for i := range issuesBefore {
		b, a := issuesBefore[i], issuesAfter[i]
		if b.ID != a.ID || b.Title != a.Title || b.Status != a.Status ||
			b.Priority != a.Priority || b.Type != a.Type {
			t.Errorf("issue %d differs after rebuild: before %+v, after %+v", b.ID, b, a)
		}
	}
}

func TestRunRebuild_RestoresClaimState(t *testing.T) {
	wd := initTracker(t, "T")
	runCreate(t, wd, "--title", "Claimed issue")
	runClaim(t, wd, "1", "--agent", "my-agent", "--session", "my-session")

	runRebuild(t, wd)

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
		t.Fatal("expected claim to be restored after rebuild, got nil")
	}
	if issue.Claim.AgentID != "my-agent" {
		t.Errorf("AgentID: got %q, want %q", issue.Claim.AgentID, "my-agent")
	}
}

func TestRunRebuild_IsIdempotent(t *testing.T) {
	wd := initTracker(t, "T")
	runCreate(t, wd, "--title", "Issue")

	runRebuild(t, wd)
	got := runRebuild(t, wd)
	if got != "rebuilt index: 1 issue(s)" {
		t.Errorf("unexpected output after second rebuild: %q", got)
	}

	s, err := cli.OpenStore(wd)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	defer s.Close()

	issues, err := s.ListIssues(index.Filters{})
	if err != nil {
		t.Fatalf("ListIssues: %v", err)
	}
	if len(issues) != 1 {
		t.Errorf("expected 1 issue after double rebuild, got %d", len(issues))
	}
}

func TestRunRebuild_HaltsOnCorruptedFile(t *testing.T) {
	wd := initTracker(t, "T")
	runCreate(t, wd, "--title", "Good issue")

	// Write a corrupted YAML file directly into the issues directory.
	issuesDir := filepath.Join(wd, ".tracker", "issues")
	corruptPath := filepath.Join(issuesDir, "T-0099.yaml")
	if err := os.WriteFile(corruptPath, []byte("event: !!binary |\n  not valid\n: bad yaml: {[}\n"), 0644); err != nil {
		t.Fatalf("write corrupt file: %v", err)
	}

	var buf bytes.Buffer
	err := cli.RunRebuild(wd, []string{}, &buf)
	if err == nil {
		t.Fatal("expected error when rebuilding with a corrupted file")
	}
}
