package cli_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/nuchs/tasker/internal/cli"
	"github.com/nuchs/tasker/internal/model"
)

func runReady(t *testing.T, wd string, args ...string) string {
	t.Helper()
	var buf bytes.Buffer
	if err := cli.RunReady(wd, args, &buf); err != nil {
		t.Fatalf("RunReady(%v): %v", args, err)
	}
	return buf.String()
}

func TestRunReady_ShowsOpenUnclaimedIssues(t *testing.T) {
	wd := initTracker(t, "T")
	runCreate(t, wd, "--title", "Ready task")

	out := runReady(t, wd)

	if !strings.Contains(out, "Ready task") {
		t.Errorf("expected 'Ready task' in ready output:\n%s", out)
	}
}

func TestRunReady_ExcludesClaimedIssues(t *testing.T) {
	wd := initTracker(t, "T")
	runCreate(t, wd, "--title", "Claimed task")

	s, err := cli.OpenStore(wd)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	if err := s.Append(1, model.Event{
		Type:      model.EventClaimed,
		AgentID:   "agent-1",
		SessionID: "sess-1",
	}); err != nil {
		t.Fatalf("Append claim: %v", err)
	}
	s.Close()

	out := runReady(t, wd)

	if strings.Contains(out, "Claimed task") {
		t.Errorf("claimed issue should not appear in ready output:\n%s", out)
	}
}

func TestRunReady_ExcludesBlockedByDependencies(t *testing.T) {
	wd := initTracker(t, "T")

	// Create a blocker and a dependent issue.
	s, err := cli.OpenStore(wd)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	_, err = s.Create(model.Event{
		Type:      model.EventCreated,
		IssueType: model.TypeTask,
		Title:     "Blocker",
		Status:    model.StatusOpen,
		Priority:  model.PriorityMedium,
	})
	if err != nil {
		t.Fatalf("Create blocker: %v", err)
	}
	_, err = s.Create(model.Event{
		Type:      model.EventCreated,
		IssueType: model.TypeTask,
		Title:     "Dependent",
		Status:    model.StatusOpen,
		Priority:  model.PriorityMedium,
		Depends:   []int{1},
	})
	if err != nil {
		t.Fatalf("Create dependent: %v", err)
	}
	s.Close()

	out := runReady(t, wd)

	if !strings.Contains(out, "Blocker") {
		t.Errorf("blocker should appear in ready output:\n%s", out)
	}
	if strings.Contains(out, "Dependent") {
		t.Errorf("dependent issue should be excluded from ready output:\n%s", out)
	}
}

func TestRunReady_OrderedByPriority(t *testing.T) {
	wd := initTracker(t, "T")
	runCreate(t, wd, "--title", "Low task", "--priority", "low")
	runCreate(t, wd, "--title", "High task", "--priority", "high")
	runCreate(t, wd, "--title", "Medium task", "--priority", "medium")

	out := runReady(t, wd)

	highPos := strings.Index(out, "High task")
	medPos := strings.Index(out, "Medium task")
	lowPos := strings.Index(out, "Low task")

	if highPos < 0 || medPos < 0 || lowPos < 0 {
		t.Fatalf("not all issues found in output:\n%s", out)
	}
	if !(highPos < medPos && medPos < lowPos) {
		t.Errorf("expected order high→medium→low, got high=%d med=%d low=%d\n%s",
			highPos, medPos, lowPos, out)
	}
}

func TestRunReady_JSONOutput(t *testing.T) {
	wd := initTracker(t, "PROJ")
	runCreate(t, wd, "--title", "Ready JSON", "--priority", "high")

	out := runReady(t, wd, "--json")

	var result []map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 result, got %d", len(result))
	}
	entry := result[0]
	if entry["title"] != "Ready JSON" {
		t.Errorf("title: got %v", entry["title"])
	}
	if entry["display_id"] != "PROJ-0001" {
		t.Errorf("display_id: got %v", entry["display_id"])
	}
	if entry["priority"] != "high" {
		t.Errorf("priority: got %v", entry["priority"])
	}
}

func TestRunReady_EmptyResult(t *testing.T) {
	wd := initTracker(t, "T")
	out := runReady(t, wd)
	if !strings.Contains(out, "no ready issues") {
		t.Errorf("expected 'no ready issues' for empty result:\n%s", out)
	}
}

func TestRunReady_DependencyResolvedWhenBlockerDone(t *testing.T) {
	wd := initTracker(t, "T")

	s, err := cli.OpenStore(wd)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	// Create blocker + dependent.
	_, err = s.Create(model.Event{
		Type:      model.EventCreated,
		IssueType: model.TypeTask,
		Title:     "Blocker",
		Status:    model.StatusOpen,
		Priority:  model.PriorityMedium,
	})
	if err != nil {
		t.Fatalf("Create blocker: %v", err)
	}
	_, err = s.Create(model.Event{
		Type:      model.EventCreated,
		IssueType: model.TypeTask,
		Title:     "Dependent",
		Status:    model.StatusOpen,
		Priority:  model.PriorityMedium,
		Depends:   []int{1},
	})
	if err != nil {
		t.Fatalf("Create dependent: %v", err)
	}
	// Resolve the blocker.
	if err := s.Append(1, model.Event{Type: model.EventStatusChanged, Status: model.StatusDone}); err != nil {
		t.Fatalf("Append done: %v", err)
	}
	s.Close()

	out := runReady(t, wd)

	if !strings.Contains(out, "Dependent") {
		t.Errorf("dependent issue should appear once blocker is done:\n%s", out)
	}
}
