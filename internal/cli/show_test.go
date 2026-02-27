package cli_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/nuchs/tasker/internal/cli"
	"github.com/nuchs/tasker/internal/model"
)

func runShow(t *testing.T, wd string, args ...string) string {
	t.Helper()
	var buf bytes.Buffer
	if err := cli.RunShow(wd, args, &buf); err != nil {
		t.Fatalf("RunShow(%v): %v", args, err)
	}
	return buf.String()
}

func TestRunShow_DisplaysAllFields(t *testing.T) {
	wd := initTracker(t, "PROJ")
	runCreate(t, wd,
		"--title", "My issue",
		"--description", "Full description",
		"--priority", "high",
		"--type", "issue",
		"--acceptance-criteria", "All tests pass",
	)

	out := runShow(t, wd, "PROJ-0001")

	for _, want := range []string{
		"My issue",
		"high",
		"issue",
		"Full description",
		"All tests pass",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestRunShow_JSONOutput(t *testing.T) {
	wd := initTracker(t, "PROJ")
	runCreate(t, wd,
		"--title", "JSON issue",
		"--description", "Desc",
		"--priority", "low",
	)

	out := runShow(t, wd, "--json", "1")

	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if result["title"] != "JSON issue" {
		t.Errorf("title: got %v", result["title"])
	}
	if result["display_id"] != "PROJ-0001" {
		t.Errorf("display_id: got %v", result["display_id"])
	}
	if result["priority"] != "low" {
		t.Errorf("priority: got %v", result["priority"])
	}
	if result["description"] != "Desc" {
		t.Errorf("description: got %v", result["description"])
	}
}

func TestRunShow_Events(t *testing.T) {
	wd := initTracker(t, "T")
	runCreate(t, wd, "--title", "Event test")

	out := runShow(t, wd, "--events", "1")

	if !strings.Contains(out, "created") {
		t.Errorf("events output missing 'created':\n%s", out)
	}
}

func TestRunShow_EventsJSON(t *testing.T) {
	wd := initTracker(t, "T")
	runCreate(t, wd, "--title", "EJ")

	out := runShow(t, wd, "--events", "--json", "1")

	var events []map[string]any
	if err := json.Unmarshal([]byte(out), &events); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if len(events) == 0 {
		t.Fatal("expected at least one event")
	}
	if events[0]["event"] != "created" {
		t.Errorf("first event type: got %v", events[0]["event"])
	}
}

func TestRunShow_AcceptsVariousIDFormats(t *testing.T) {
	wd := initTracker(t, "PROJ")
	runCreate(t, wd, "--title", "ID formats")

	for _, id := range []string{"1", "PROJ-1", "PROJ-01", "PROJ-0001"} {
		out := runShow(t, wd, id)
		if !strings.Contains(out, "ID formats") {
			t.Errorf("id format %q: title not found in output:\n%s", id, out)
		}
	}
}

func TestRunShow_ReportsStaleClain(t *testing.T) {
	wd := initTracker(t, "T")
	runCreate(t, wd, "--title", "Claimed")

	// Append a claim event directly via Store.
	s, err := cli.OpenStore(wd)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	claimEv := model.Event{
		Type:      model.EventClaimed,
		AgentID:   "test-agent",
		SessionID: "test-session",
	}
	if err := s.Append(1, claimEv); err != nil {
		t.Fatalf("Append claim: %v", err)
	}
	s.Close()

	// Verify claim info appears in the output.
	out := runShow(t, wd, "1")
	if !strings.Contains(out, "Claim:") {
		t.Errorf("expected claim info in output:\n%s", out)
	}
	if !strings.Contains(out, "test-agent") {
		t.Errorf("expected agent ID in output:\n%s", out)
	}
}

func TestRunShow_MissingID(t *testing.T) {
	wd := initTracker(t, "T")
	var buf bytes.Buffer
	if err := cli.RunShow(wd, []string{}, &buf); err == nil {
		t.Fatal("expected error when ID is missing")
	}
}

func TestRunShow_InvalidID(t *testing.T) {
	wd := initTracker(t, "T")
	var buf bytes.Buffer
	if err := cli.RunShow(wd, []string{"not-a-number"}, &buf); err == nil {
		t.Fatal("expected error for non-numeric ID suffix")
	}
}

func TestRunShow_NonExistentIssue(t *testing.T) {
	wd := initTracker(t, "T")
	var buf bytes.Buffer
	if err := cli.RunShow(wd, []string{"99"}, &buf); err == nil {
		t.Fatal("expected error for non-existent issue")
	}
}

func TestParseID(t *testing.T) {
	cases := []struct {
		in   string
		want int
		ok   bool
	}{
		{"1", 1, true},
		{"42", 42, true},
		{"PROJ-1", 1, true},
		{"PROJ-42", 42, true},
		{"PROJ-042", 42, true},
		{"PROJ-0001", 1, true},
		{"0", 0, false},
		{"-1", 0, false},
		{"abc", 0, false},
		{"PROJ-abc", 0, false},
	}
	for _, tc := range cases {
		got, err := cli.ParseID(tc.in)
		if tc.ok {
			if err != nil {
				t.Errorf("ParseID(%q) error: %v", tc.in, err)
			} else if got != tc.want {
				t.Errorf("ParseID(%q) = %d, want %d", tc.in, got, tc.want)
			}
		} else {
			if err == nil {
				t.Errorf("ParseID(%q) = %d, want error", tc.in, got)
			}
		}
	}
}
