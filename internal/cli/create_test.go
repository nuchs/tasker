package cli_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/nuchs/tasker/internal/cli"
)

func initTracker(t *testing.T, prefix string) string {
	t.Helper()
	wd := t.TempDir()
	if err := cli.RunInit(wd, []string{"--prefix", prefix}); err != nil {
		t.Fatalf("RunInit: %v", err)
	}
	return wd
}

func runCreate(t *testing.T, wd string, args ...string) string {
	t.Helper()
	var buf bytes.Buffer
	if err := cli.RunCreate(wd, args, &buf); err != nil {
		t.Fatalf("RunCreate(%v): %v", args, err)
	}
	return strings.TrimSpace(buf.String())
}

func TestRunCreate_PrintsFormattedID(t *testing.T) {
	wd := initTracker(t, "TEST")
	got := runCreate(t, wd, "--title", "My task")
	if got != "TEST-0001" {
		t.Errorf("expected TEST-0001, got %q", got)
	}
}

func TestRunCreate_Defaults(t *testing.T) {
	wd := initTracker(t, "T")
	runCreate(t, wd, "--title", "Defaulted")

	s, err := cli.OpenStore(wd)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	defer s.Close()

	issue, _, err := s.Show(1)
	if err != nil {
		t.Fatalf("Show: %v", err)
	}
	if issue.Priority != "medium" {
		t.Errorf("priority: got %q, want medium", issue.Priority)
	}
	if issue.Type != "task" {
		t.Errorf("type: got %q, want task", issue.Type)
	}
	if issue.Status != "open" {
		t.Errorf("status: got %q, want open", issue.Status)
	}
}

func TestRunCreate_AllFlags(t *testing.T) {
	wd := initTracker(t, "T")
	runCreate(t, wd,
		"--title", "Fix it",
		"--description", "Detailed desc",
		"--priority", "high",
		"--type", "issue",
		"--acceptance-criteria", "Tests pass",
	)

	s, err := cli.OpenStore(wd)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	defer s.Close()

	issue, _, err := s.Show(1)
	if err != nil {
		t.Fatalf("Show: %v", err)
	}
	if issue.Title != "Fix it" {
		t.Errorf("title: got %q", issue.Title)
	}
	if issue.Description != "Detailed desc" {
		t.Errorf("description: got %q", issue.Description)
	}
	if issue.Priority != "high" {
		t.Errorf("priority: got %q", issue.Priority)
	}
	if issue.Type != "issue" {
		t.Errorf("type: got %q", issue.Type)
	}
	if issue.AcceptanceCriteria != "Tests pass" {
		t.Errorf("acceptance_criteria: got %q", issue.AcceptanceCriteria)
	}
}

func TestRunCreate_SequentialIDs(t *testing.T) {
	wd := initTracker(t, "PROJ")

	for i, want := range []string{"PROJ-0001", "PROJ-0002", "PROJ-0003"} {
		got := runCreate(t, wd, "--title", "Issue")
		if got != want {
			t.Errorf("issue %d: expected %s, got %s", i+1, want, got)
		}
	}
}

func TestRunCreate_IndexIsUpdated(t *testing.T) {
	wd := initTracker(t, "T")
	runCreate(t, wd, "--title", "Indexed", "--priority", "high")

	s, err := cli.OpenStore(wd)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	defer s.Close()

	// GetIssueMeta queries the SQLite index, verifying the index was updated.
	meta, err := s.GetIssueMeta(1)
	if err != nil {
		t.Fatalf("GetIssueMeta (index not updated): %v", err)
	}
	if meta.Title != "Indexed" {
		t.Errorf("index title: got %q", meta.Title)
	}
	if meta.Priority != "high" {
		t.Errorf("index priority: got %q", meta.Priority)
	}
}

func TestRunCreate_RequiresTitle(t *testing.T) {
	wd := initTracker(t, "T")
	var buf bytes.Buffer
	if err := cli.RunCreate(wd, []string{}, &buf); err == nil {
		t.Fatal("expected error when --title is missing")
	}
}

func TestRunCreate_InvalidPriority(t *testing.T) {
	wd := initTracker(t, "T")
	var buf bytes.Buffer
	if err := cli.RunCreate(wd, []string{"--title", "T", "--priority", "critical"}, &buf); err == nil {
		t.Fatal("expected error for invalid priority")
	}
}

func TestRunCreate_InvalidType(t *testing.T) {
	wd := initTracker(t, "T")
	var buf bytes.Buffer
	if err := cli.RunCreate(wd, []string{"--title", "T", "--type", "epic"}, &buf); err == nil {
		t.Fatal("expected error for invalid type")
	}
}
