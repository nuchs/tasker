package cli_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/nuchs/tasker/internal/cli"
)

func runUpdate(t *testing.T, wd string, args ...string) string {
	t.Helper()
	var buf bytes.Buffer
	if err := cli.RunUpdate(wd, args, &buf); err != nil {
		t.Fatalf("RunUpdate(%v): %v", args, err)
	}
	return strings.TrimSpace(buf.String())
}

func TestRunUpdate_UpdatesStatus(t *testing.T) {
	wd := initTracker(t, "T")
	runCreate(t, wd, "--title", "My issue")

	runUpdate(t, wd, "1", "--status", "in_progress")

	s, err := cli.OpenStore(wd)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	defer s.Close()

	issue, _, err := s.Show(1)
	if err != nil {
		t.Fatalf("Show: %v", err)
	}
	if issue.Status != "in_progress" {
		t.Errorf("status: got %q, want in_progress", issue.Status)
	}
}

func TestRunUpdate_UpdatesPriority(t *testing.T) {
	wd := initTracker(t, "T")
	runCreate(t, wd, "--title", "My issue")

	runUpdate(t, wd, "1", "--priority", "high")

	s, err := cli.OpenStore(wd)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	defer s.Close()

	issue, _, err := s.Show(1)
	if err != nil {
		t.Fatalf("Show: %v", err)
	}
	if issue.Priority != "high" {
		t.Errorf("priority: got %q, want high", issue.Priority)
	}
}

func TestRunUpdate_UpdatesTitle(t *testing.T) {
	wd := initTracker(t, "T")
	runCreate(t, wd, "--title", "Old title")

	runUpdate(t, wd, "1", "--title", "New title")

	s, err := cli.OpenStore(wd)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	defer s.Close()

	issue, _, err := s.Show(1)
	if err != nil {
		t.Fatalf("Show: %v", err)
	}
	if issue.Title != "New title" {
		t.Errorf("title: got %q, want 'New title'", issue.Title)
	}
}

func TestRunUpdate_MultipleFlags(t *testing.T) {
	wd := initTracker(t, "T")
	runCreate(t, wd, "--title", "Original")

	runUpdate(t, wd, "1", "--status", "review", "--priority", "low", "--title", "Revised")

	s, err := cli.OpenStore(wd)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	defer s.Close()

	issue, _, err := s.Show(1)
	if err != nil {
		t.Fatalf("Show: %v", err)
	}
	if issue.Status != "review" {
		t.Errorf("status: got %q, want review", issue.Status)
	}
	if issue.Priority != "low" {
		t.Errorf("priority: got %q, want low", issue.Priority)
	}
	if issue.Title != "Revised" {
		t.Errorf("title: got %q, want Revised", issue.Title)
	}

	// Three events should have been appended (created + 3 updates = 4 total).
	events, err := s.Events(1)
	if err != nil {
		t.Fatalf("Events: %v", err)
	}
	if len(events) != 4 {
		t.Errorf("expected 4 events, got %d", len(events))
	}
}

func TestRunUpdate_IndexIsUpdated(t *testing.T) {
	wd := initTracker(t, "T")
	runCreate(t, wd, "--title", "Indexed")

	runUpdate(t, wd, "1", "--status", "done")

	s, err := cli.OpenStore(wd)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	defer s.Close()

	meta, err := s.GetIssueMeta(1)
	if err != nil {
		t.Fatalf("GetIssueMeta: %v", err)
	}
	if meta.Status != "done" {
		t.Errorf("index status: got %q, want done", meta.Status)
	}
}

func TestRunUpdate_PrintsFormattedID(t *testing.T) {
	wd := initTracker(t, "PROJ")
	runCreate(t, wd, "--title", "Issue")

	got := runUpdate(t, wd, "1", "--status", "done")
	if got != "PROJ-0001" {
		t.Errorf("expected PROJ-0001, got %q", got)
	}
}

func TestRunUpdate_AcceptsPrefixedID(t *testing.T) {
	wd := initTracker(t, "PROJ")
	runCreate(t, wd, "--title", "Issue")

	var buf bytes.Buffer
	if err := cli.RunUpdate(wd, []string{"PROJ-0001", "--status", "done"}, &buf); err != nil {
		t.Fatalf("RunUpdate with prefixed ID: %v", err)
	}
}

func TestRunUpdate_MissingID(t *testing.T) {
	wd := initTracker(t, "T")
	var buf bytes.Buffer
	if err := cli.RunUpdate(wd, []string{"--status", "done"}, &buf); err == nil {
		t.Fatal("expected error when ID is missing")
	}
}

func TestRunUpdate_NoFlags(t *testing.T) {
	wd := initTracker(t, "T")
	runCreate(t, wd, "--title", "Issue")
	var buf bytes.Buffer
	if err := cli.RunUpdate(wd, []string{"1"}, &buf); err == nil {
		t.Fatal("expected error when no update flags are provided")
	}
}

func TestRunUpdate_InvalidStatus(t *testing.T) {
	wd := initTracker(t, "T")
	runCreate(t, wd, "--title", "Issue")
	var buf bytes.Buffer
	if err := cli.RunUpdate(wd, []string{"1", "--status", "pending"}, &buf); err == nil {
		t.Fatal("expected error for invalid status")
	}
}

func TestRunUpdate_InvalidPriority(t *testing.T) {
	wd := initTracker(t, "T")
	runCreate(t, wd, "--title", "Issue")
	var buf bytes.Buffer
	if err := cli.RunUpdate(wd, []string{"1", "--priority", "critical"}, &buf); err == nil {
		t.Fatal("expected error for invalid priority")
	}
}
