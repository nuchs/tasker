package cli_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/nuchs/tasker/internal/cli"
	"github.com/nuchs/tasker/internal/model"
)

func runList(t *testing.T, wd string, args ...string) string {
	t.Helper()
	var buf bytes.Buffer
	if err := cli.RunList(wd, args, &buf); err != nil {
		t.Fatalf("RunList(%v): %v", args, err)
	}
	return buf.String()
}

func TestRunList_ShowsActiveIssuesByDefault(t *testing.T) {
	wd := initTracker(t, "PROJ")
	runCreate(t, wd, "--title", "Open task")

	out := runList(t, wd)

	if !strings.Contains(out, "Open task") {
		t.Errorf("expected 'Open task' in output:\n%s", out)
	}
}

func TestRunList_DefaultExcludesDoneAndCancelled(t *testing.T) {
	wd := initTracker(t, "T")
	runCreate(t, wd, "--title", "Active")
	runCreate(t, wd, "--title", "Finished")
	runCreate(t, wd, "--title", "Dropped")

	// Move issue 2 to done and issue 3 to cancelled via the store.
	s, err := cli.OpenStore(wd)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	if err := s.Append(2, model.Event{Type: model.EventStatusChanged, Status: model.StatusDone}); err != nil {
		t.Fatalf("Append done: %v", err)
	}
	if err := s.Append(3, model.Event{Type: model.EventStatusChanged, Status: model.StatusCancelled}); err != nil {
		t.Fatalf("Append cancelled: %v", err)
	}
	s.Close()

	out := runList(t, wd)

	if !strings.Contains(out, "Active") {
		t.Errorf("expected 'Active' in default list:\n%s", out)
	}
	if strings.Contains(out, "Finished") {
		t.Errorf("done issue should be excluded from default list:\n%s", out)
	}
	if strings.Contains(out, "Dropped") {
		t.Errorf("cancelled issue should be excluded from default list:\n%s", out)
	}
}

func TestRunList_FilterByStatus(t *testing.T) {
	wd := initTracker(t, "T")
	runCreate(t, wd, "--title", "To do")
	runCreate(t, wd, "--title", "Complete")

	s, err := cli.OpenStore(wd)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	if err := s.Append(2, model.Event{Type: model.EventStatusChanged, Status: model.StatusDone}); err != nil {
		t.Fatalf("Append done: %v", err)
	}
	s.Close()

	// Filter for done only.
	out := runList(t, wd, "--status", "done")
	if !strings.Contains(out, "Complete") {
		t.Errorf("expected 'Complete' when filtering by done:\n%s", out)
	}
	if strings.Contains(out, "To do") {
		t.Errorf("open issue should not appear when filtering by done:\n%s", out)
	}
}

func TestRunList_FilterByPriority(t *testing.T) {
	wd := initTracker(t, "T")
	runCreate(t, wd, "--title", "High priority", "--priority", "high")
	runCreate(t, wd, "--title", "Low priority", "--priority", "low")

	out := runList(t, wd, "--priority", "high")

	if !strings.Contains(out, "High priority") {
		t.Errorf("expected 'High priority':\n%s", out)
	}
	if strings.Contains(out, "Low priority") {
		t.Errorf("low issue should be excluded:\n%s", out)
	}
}

func TestRunList_FilterByType(t *testing.T) {
	wd := initTracker(t, "T")
	runCreate(t, wd, "--title", "A task", "--type", "task")
	runCreate(t, wd, "--title", "A bug", "--type", "issue")

	out := runList(t, wd, "--type", "issue")

	if !strings.Contains(out, "A bug") {
		t.Errorf("expected 'A bug':\n%s", out)
	}
	if strings.Contains(out, "A task") {
		t.Errorf("task should be excluded when filtering by type=issue:\n%s", out)
	}
}

func TestRunList_CombinedFilters(t *testing.T) {
	wd := initTracker(t, "T")
	runCreate(t, wd, "--title", "High task", "--priority", "high", "--type", "task")
	runCreate(t, wd, "--title", "High bug", "--priority", "high", "--type", "issue")
	runCreate(t, wd, "--title", "Low task", "--priority", "low", "--type", "task")

	out := runList(t, wd, "--priority", "high", "--type", "task")

	if !strings.Contains(out, "High task") {
		t.Errorf("expected 'High task':\n%s", out)
	}
	if strings.Contains(out, "High bug") {
		t.Errorf("bug should be excluded:\n%s", out)
	}
	if strings.Contains(out, "Low task") {
		t.Errorf("low task should be excluded:\n%s", out)
	}
}

func TestRunList_JSONOutput(t *testing.T) {
	wd := initTracker(t, "PROJ")
	runCreate(t, wd, "--title", "JSON test", "--priority", "high")

	out := runList(t, wd, "--json")

	var result []map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 result, got %d", len(result))
	}
	entry := result[0]
	if entry["title"] != "JSON test" {
		t.Errorf("title: got %v", entry["title"])
	}
	if entry["display_id"] != "PROJ-0001" {
		t.Errorf("display_id: got %v", entry["display_id"])
	}
	if entry["priority"] != "high" {
		t.Errorf("priority: got %v", entry["priority"])
	}
	if entry["status"] != "open" {
		t.Errorf("status: got %v", entry["status"])
	}
	if entry["type"] != "task" {
		t.Errorf("type: got %v", entry["type"])
	}
}

func TestRunList_EmptyResult(t *testing.T) {
	wd := initTracker(t, "T")
	out := runList(t, wd)
	if !strings.Contains(out, "no issues found") {
		t.Errorf("expected 'no issues found' for empty list:\n%s", out)
	}
}

func TestRunList_InvalidStatus(t *testing.T) {
	wd := initTracker(t, "T")
	var buf bytes.Buffer
	if err := cli.RunList(wd, []string{"--status", "invalid"}, &buf); err == nil {
		t.Fatal("expected error for invalid status")
	}
}

func TestRunList_InvalidPriority(t *testing.T) {
	wd := initTracker(t, "T")
	var buf bytes.Buffer
	if err := cli.RunList(wd, []string{"--priority", "critical"}, &buf); err == nil {
		t.Fatal("expected error for invalid priority")
	}
}

func TestRunList_InvalidType(t *testing.T) {
	wd := initTracker(t, "T")
	var buf bytes.Buffer
	if err := cli.RunList(wd, []string{"--type", "epic"}, &buf); err == nil {
		t.Fatal("expected error for invalid type")
	}
}
